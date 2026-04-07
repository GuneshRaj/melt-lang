package parser

import (
	"fmt"

	"meltlang/compiler/internal/ast"
	"meltlang/compiler/internal/diag"
	"meltlang/compiler/internal/lexer"
	"meltlang/compiler/internal/token"
)

type Parser struct {
	tokens []token.Token
	pos    int
	diags  []diag.Diagnostic
}

func Parse(tokens []token.Token) (*ast.Module, []diag.Diagnostic) {
	p := &Parser{tokens: tokens}
	return p.parseModule(), p.diags
}

func (p *Parser) parseModule() *ast.Module {
	mod := &ast.Module{}
	p.skipNewlines()
	for !p.at(token.EOF) {
		decl := p.parseTopDecl()
		if decl != nil {
			mod.Decls = append(mod.Decls, decl)
		} else {
			p.synchronizeTopDecl()
		}
		p.skipNewlines()
	}
	return mod
}

func (p *Parser) parseTopDecl() ast.TopDecl {
	if p.match(token.STRUCT) {
		return p.parseStructDecl()
	}
	isKernel := p.match(token.KERNEL)
	if p.match(token.FN) {
		return p.parseFunctionDecl(isKernel)
	}
	p.errorAtCurrent("expected top-level declaration")
	return nil
}

func (p *Parser) parseStructDecl() ast.TopDecl {
	name := p.expect(token.IDENT, "expected struct name")
	p.expect(token.COLON, "expected ':' after struct name")
	p.expect(token.NEWLINE, "expected newline after struct declaration")
	p.skipNewlines()
	p.expect(token.INDENT, "expected indented struct body")

	fields := []ast.FieldDecl{}
	for !p.at(token.DEDENT) && !p.at(token.EOF) {
		if p.at(token.NEWLINE) {
			p.advance()
			continue
		}
		fieldName := p.expect(token.IDENT, "expected field name")
		p.expect(token.COLON, "expected ':' after field name")
		ty := p.parseTypeExpr()
		end := p.expect(token.NEWLINE, "expected newline after field")
		fields = append(fields, ast.FieldDecl{
			Name: fieldName.Lexeme,
			Ty:   ty,
			Span: spanFrom(fieldName, end),
		})
	}
	dedent := p.expect(token.DEDENT, "expected end of struct body")
	return &ast.StructDecl{
		Name:   name.Lexeme,
		Fields: fields,
		Span:   spanFrom(name, dedent),
	}
}

func (p *Parser) parseFunctionDecl(isKernel bool) ast.TopDecl {
	name := p.expect(token.IDENT, "expected function name")
	p.expect(token.LPAREN, "expected '(' after function name")
	params := []ast.Param{}
	if !p.at(token.RPAREN) {
		for {
			paramName := p.expect(token.IDENT, "expected parameter name")
			p.expect(token.COLON, "expected ':' after parameter name")
			ty := p.parseTypeExpr()
			params = append(params, ast.Param{
				Name: paramName.Lexeme,
				Ty:   ty,
				Span: spanFrom(paramName, typeSpan(ty)),
			})
			if !p.match(token.COMMA) {
				break
			}
		}
	}
	p.expect(token.RPAREN, "expected ')' after parameter list")
	var returnType ast.TypeExpr
	if p.match(token.ARROW) {
		returnType = p.parseTypeExpr()
	}
	p.expect(token.COLON, "expected ':' after function signature")
	p.expect(token.NEWLINE, "expected newline after function signature")
	p.skipNewlines()
	p.expect(token.INDENT, "expected indented function body")
	body := p.parseBlock()
	dedent := p.expect(token.DEDENT, "expected end of function body")
	return &ast.FunctionDecl{
		Name:       name.Lexeme,
		IsKernel:   isKernel,
		Params:     params,
		ReturnType: returnType,
		Body:       body,
		Span:       spanFrom(name, dedent),
	}
}

func (p *Parser) parseBlock() []ast.Stmt {
	var out []ast.Stmt
	for !p.at(token.DEDENT) && !p.at(token.EOF) {
		if p.at(token.NEWLINE) {
			p.advance()
			continue
		}
		stmt := p.parseStmt()
		if stmt != nil {
			out = append(out, stmt)
		} else {
			p.synchronizeStmt()
		}
	}
	return out
}

func (p *Parser) parseStmt() ast.Stmt {
	if p.match(token.RETURN) {
		return p.parseReturnStmt()
	}
	if p.match(token.IF) {
		return p.parseIfStmt()
	}
	if p.at(token.IDENT) && (p.peekKind(1) == token.ASSIGN || p.peekKind(1) == token.COLON) {
		return p.parseLetOrAssignStmt()
	}
	expr := p.parseExpr()
	end := p.expect(token.NEWLINE, "expected newline after expression")
	return &ast.ExprStmt{Value: expr, Span: spanFrom(expr.GetSpan(), end)}
}

func (p *Parser) parseLetOrAssignStmt() ast.Stmt {
	name := p.expect(token.IDENT, "expected binding name")
	if p.match(token.COLON) {
		ty := p.parseTypeExpr()
		p.expect(token.ASSIGN, "expected '=' in let statement")
		value := p.parseExpr()
		end := p.expect(token.NEWLINE, "expected newline after let statement")
		return &ast.LetStmt{Name: name.Lexeme, Ty: ty, Value: value, Span: spanFrom(name, end)}
	}
	if p.match(token.ASSIGN) {
		value := p.parseExpr()
		end := p.expect(token.NEWLINE, "expected newline after assignment")
		return &ast.LetStmt{Name: name.Lexeme, Value: value, Span: spanFrom(name, end)}
	}

	base := ast.Expr(&ast.NameExpr{Name: name.Lexeme, Span: spanFrom(name, name)})
	for p.at(token.DOT) || p.at(token.LBRACKET) {
		if p.match(token.DOT) {
			field := p.expect(token.IDENT, "expected field name")
			base = &ast.FieldExpr{Base: base, Name: field.Lexeme, Span: spanFrom(base.GetSpan(), field)}
			continue
		}
		p.expect(token.LBRACKET, "expected '['")
		index := p.parseExpr()
		rbrack := p.expect(token.RBRACKET, "expected ']'")
		base = &ast.IndexExpr{Base: base, Index: index, Span: spanFrom(base.GetSpan(), rbrack)}
	}
	if p.match(token.ASSIGN) {
		value := p.parseExpr()
		end := p.expect(token.NEWLINE, "expected newline after assignment")
		target := toLValue(base)
		if target == nil {
			p.errorAt(name, "invalid assignment target")
			return nil
		}
		return &ast.AssignStmt{Target: target, Value: value, Span: spanFrom(name, end)}
	}
	p.errorAtCurrent("expected '=' after assignment target")
	return nil
}

func (p *Parser) parseReturnStmt() ast.Stmt {
	start := p.previous()
	if p.at(token.NEWLINE) {
		end := p.expect(token.NEWLINE, "expected newline after return")
		return &ast.ReturnStmt{Span: spanFrom(start, end)}
	}
	value := p.parseExpr()
	end := p.expect(token.NEWLINE, "expected newline after return")
	return &ast.ReturnStmt{Value: value, Span: spanFrom(start, end)}
}

func (p *Parser) parseIfStmt() ast.Stmt {
	start := p.previous()
	cond := p.parseExpr()
	p.expect(token.COLON, "expected ':' after if condition")
	p.expect(token.NEWLINE, "expected newline after if header")
	p.skipNewlines()
	p.expect(token.INDENT, "expected indented if body")
	thenBody := p.parseBlock()
	endTok := p.expect(token.DEDENT, "expected end of if body")

	var elifs []ast.IfBranch
	for p.match(token.ELIF) {
		elifStart := p.previous()
			elifCond := p.parseExpr()
			p.expect(token.COLON, "expected ':' after elif condition")
			p.expect(token.NEWLINE, "expected newline after elif header")
			p.skipNewlines()
			p.expect(token.INDENT, "expected indented elif body")
			body := p.parseBlock()
		elifEnd := p.expect(token.DEDENT, "expected end of elif body")
		elifs = append(elifs, ast.IfBranch{
			Cond: elifCond,
			Body: body,
			Span: spanFrom(elifStart, elifEnd),
		})
		endTok = elifEnd
	}

	var elseBody []ast.Stmt
	if p.match(token.ELSE) {
		p.expect(token.COLON, "expected ':' after else")
		p.expect(token.NEWLINE, "expected newline after else")
		p.skipNewlines()
		p.expect(token.INDENT, "expected indented else body")
		elseBody = p.parseBlock()
		endTok = p.expect(token.DEDENT, "expected end of else body")
	}

	return &ast.IfStmt{
		Cond:     cond,
		ThenBody: thenBody,
		Elifs:    elifs,
		ElseBody: elseBody,
		Span:     spanFrom(start, endTok),
	}
}

func (p *Parser) parseTypeExpr() ast.TypeExpr {
	name := p.expect(token.IDENT, "expected type name")
	ty := &ast.NamedTypeExpr{Name: name.Lexeme, Span: spanFrom(name, name)}
	if p.match(token.LBRACKET) {
		for {
			ty.Args = append(ty.Args, p.parseTypeExpr())
			if !p.match(token.COMMA) {
				break
			}
		}
		end := p.expect(token.RBRACKET, "expected ']' after type arguments")
		ty.Span = spanFrom(name, end)
	}
	return ty
}

func (p *Parser) parseExpr() ast.Expr { return p.parseLambdaExpr() }

func (p *Parser) parseLambdaExpr() ast.Expr {
	if !p.looksLikeLambda() {
		return p.parseLogicOr()
	}
	start := p.current()
	params := p.parseLambdaParams()
	p.expect(token.ARROW, "expected '->' after lambda params")
	body := p.parseExpr()
	return &ast.LambdaExpr{Params: params, Body: body, Span: spanFrom(start, body.GetSpan())}
}

func (p *Parser) parseLambdaParams() []string {
	if p.at(token.IDENT) {
		name := p.expect(token.IDENT, "expected lambda parameter")
		return []string{name.Lexeme}
	}
	p.expect(token.LPAREN, "expected '(' for lambda parameters")
	var params []string
	if !p.at(token.RPAREN) {
		for {
			name := p.expect(token.IDENT, "expected lambda parameter")
			params = append(params, name.Lexeme)
			if !p.match(token.COMMA) {
				break
			}
		}
	}
	p.expect(token.RPAREN, "expected ')' after lambda parameters")
	return params
}

func (p *Parser) looksLikeLambda() bool {
	if p.at(token.IDENT) && p.peekKind(1) == token.ARROW {
		return true
	}
	if !p.at(token.LPAREN) {
		return false
	}
	depth := 0
	for i := p.pos; i < len(p.tokens); i++ {
		switch p.tokens[i].Kind {
		case token.LPAREN:
			depth++
		case token.RPAREN:
			depth--
			if depth == 0 {
				return i+1 < len(p.tokens) && p.tokens[i+1].Kind == token.ARROW
			}
		}
	}
	return false
}

func (p *Parser) parseLogicOr() ast.Expr {
	return p.parseBinaryLeftAssoc(p.parseLogicAnd, token.OR)
}

func (p *Parser) parseLogicAnd() ast.Expr {
	return p.parseBinaryLeftAssoc(p.parseEquality, token.AND)
}

func (p *Parser) parseEquality() ast.Expr {
	return p.parseBinaryLeftAssoc(p.parseCompare, token.EQ, token.NEQ)
}

func (p *Parser) parseCompare() ast.Expr {
	return p.parseBinaryLeftAssoc(p.parseAdditive, token.LT, token.LTE, token.GT, token.GTE)
}

func (p *Parser) parseAdditive() ast.Expr {
	return p.parseBinaryLeftAssoc(p.parseMultiplicative, token.PLUS, token.MINUS)
}

func (p *Parser) parseMultiplicative() ast.Expr {
	return p.parseBinaryLeftAssoc(p.parseUnary, token.STAR, token.SLASH, token.PERCENT)
}

func (p *Parser) parseBinaryLeftAssoc(next func() ast.Expr, ops ...token.Kind) ast.Expr {
	expr := next()
	for p.atAny(ops...) {
		op := p.advance()
		right := next()
		expr = &ast.BinaryExpr{
			Left:  expr,
			Op:    op.Lexeme,
			Right: right,
			Span:  spanFrom(expr.GetSpan(), right.GetSpan()),
		}
	}
	return expr
}

func (p *Parser) parseUnary() ast.Expr {
	if p.match(token.MINUS) || p.match(token.NOT) {
		op := p.previous()
		value := p.parseUnary()
		return &ast.UnaryExpr{Op: op.Lexeme, Expr: value, Span: spanFrom(op, value.GetSpan())}
	}
	return p.parsePostfix()
}

func (p *Parser) parsePostfix() ast.Expr {
	expr := p.parsePrimary()
	for {
		switch {
		case p.match(token.DOT):
			name := p.expect(token.IDENT, "expected field name")
			expr = &ast.FieldExpr{Base: expr, Name: name.Lexeme, Span: spanFrom(expr.GetSpan(), name)}
		case p.match(token.LPAREN):
			args := []ast.Arg{}
			if !p.at(token.RPAREN) {
				for {
					argStart := p.current()
					if p.at(token.IDENT) && p.peekKind(1) == token.ASSIGN {
						name := p.advance()
						p.advance()
						var value ast.Expr
						if name.Lexeme == "as" {
							ty := p.parseTypeExpr()
							value = &ast.TypeExprValue{Value: ty, Span: ty.GetSpan()}
						} else {
							value = p.parseExpr()
						}
						args = append(args, ast.Arg{Name: name.Lexeme, Value: value, Span: spanFrom(argStart, value.GetSpan())})
					} else {
						value := p.parseExpr()
						args = append(args, ast.Arg{Value: value, Span: value.GetSpan()})
					}
					if !p.match(token.COMMA) {
						break
					}
				}
			}
			end := p.expect(token.RPAREN, "expected ')' after call")
			expr = &ast.CallExpr{Callee: expr, Args: args, Span: spanFrom(expr.GetSpan(), end)}
		case p.match(token.LBRACKET):
			if p.match(token.COLON) {
				var endExpr ast.Expr
				if !p.at(token.RBRACKET) {
					endExpr = p.parseExpr()
				}
				end := p.expect(token.RBRACKET, "expected ']' after slice")
				expr = &ast.SliceExpr{Base: expr, End: endExpr, Span: spanFrom(expr.GetSpan(), end)}
				continue
			}
			index := p.parseExpr()
			if p.match(token.COLON) {
				var endExpr ast.Expr
				if !p.at(token.RBRACKET) {
					endExpr = p.parseExpr()
				}
				end := p.expect(token.RBRACKET, "expected ']' after slice")
				expr = &ast.SliceExpr{Base: expr, Start: index, End: endExpr, Span: spanFrom(expr.GetSpan(), end)}
			} else {
				end := p.expect(token.RBRACKET, "expected ']' after index")
				expr = &ast.IndexExpr{Base: expr, Index: index, Span: spanFrom(expr.GetSpan(), end)}
			}
		default:
			return expr
		}
	}
}

func (p *Parser) parsePrimary() ast.Expr {
	switch {
	case p.match(token.IDENT):
		tok := p.previous()
		return &ast.NameExpr{Name: tok.Lexeme, Span: spanFrom(tok, tok)}
	case p.match(token.INT):
		tok := p.previous()
		value, err := lexer.ParseInt(tok)
		if err != nil {
			p.errorAt(tok, "invalid integer literal")
			value = 0
		}
		return &ast.IntExpr{Value: value, Span: spanFrom(tok, tok)}
	case p.match(token.FLOAT):
		tok := p.previous()
		value, err := lexer.ParseFloat(tok)
		if err != nil {
			p.errorAt(tok, "invalid float literal")
			value = 0
		}
		return &ast.FloatExpr{Value: value, Span: spanFrom(tok, tok)}
	case p.match(token.STRING):
		tok := p.previous()
		return &ast.StringExpr{Value: tok.Lexeme, Span: spanFrom(tok, tok)}
	case p.match(token.TRUE):
		tok := p.previous()
		return &ast.BoolExpr{Value: true, Span: spanFrom(tok, tok)}
	case p.match(token.FALSE):
		tok := p.previous()
		return &ast.BoolExpr{Value: false, Span: spanFrom(tok, tok)}
	case p.match(token.LPAREN):
		expr := p.parseExpr()
		p.expect(token.RPAREN, "expected ')' after expression")
		return expr
	case p.match(token.LBRACKET):
		start := p.previous()
		var elems []ast.Expr
		if !p.at(token.RBRACKET) {
			for {
				elems = append(elems, p.parseExpr())
				if !p.match(token.COMMA) {
					break
				}
			}
		}
		end := p.expect(token.RBRACKET, "expected ']' after array literal")
		return &ast.ArrayExpr{Elements: elems, Span: spanFrom(start, end)}
	default:
		p.errorAtCurrent("expected expression")
		return &ast.NameExpr{Name: "<error>", Span: tokenSpan(p.current())}
	}
}

func (p *Parser) synchronizeTopDecl() {
	for !p.at(token.EOF) {
		if p.at(token.STRUCT) || p.at(token.FN) || p.at(token.KERNEL) {
			return
		}
		p.advance()
	}
}

func (p *Parser) synchronizeStmt() {
	for !p.at(token.EOF) && !p.at(token.NEWLINE) && !p.at(token.DEDENT) {
		p.advance()
	}
	if p.at(token.NEWLINE) {
		p.advance()
	}
}

func (p *Parser) skipNewlines() {
	for p.match(token.NEWLINE) {
	}
}

func (p *Parser) expect(kind token.Kind, msg string) token.Token {
	if p.at(kind) {
		return p.advance()
	}
	tok := p.current()
	p.errorAt(tok, msg)
	if !p.at(token.EOF) {
		p.advance()
	}
	return tok
}

func (p *Parser) match(kind token.Kind) bool {
	if !p.at(kind) {
		return false
	}
	p.advance()
	return true
}

func (p *Parser) at(kind token.Kind) bool {
	return p.current().Kind == kind
}

func (p *Parser) atAny(kinds ...token.Kind) bool {
	for _, kind := range kinds {
		if p.at(kind) {
			return true
		}
	}
	return false
}

func (p *Parser) current() token.Token {
	if p.pos >= len(p.tokens) {
		return p.tokens[len(p.tokens)-1]
	}
	return p.tokens[p.pos]
}

func (p *Parser) previous() token.Token {
	if p.pos == 0 {
		return p.tokens[0]
	}
	return p.tokens[p.pos-1]
}

func (p *Parser) advance() token.Token {
	tok := p.current()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return tok
}

func (p *Parser) peekKind(offset int) token.Kind {
	idx := p.pos + offset
	if idx >= len(p.tokens) {
		return token.EOF
	}
	return p.tokens[idx].Kind
}

func (p *Parser) errorAtCurrent(msg string) {
	p.errorAt(p.current(), msg)
}

func (p *Parser) errorAt(tok token.Token, msg string) {
	p.diags = append(p.diags, diag.Diagnostic{
		Kind:      diag.ParseErrorKind,
		Message:   msg,
		File:      tok.File,
		Line:      tok.Line,
		Column:    tok.Column,
		EndLine:   tok.EndLine,
		EndColumn: tok.EndColumn,
	})
}

func tokenSpan(tok token.Token) ast.Span {
	return ast.Span{
		File:      tok.File,
		Line:      tok.Line,
		Column:    tok.Column,
		EndLine:   tok.EndLine,
		EndColumn: tok.EndColumn,
	}
}

func spanFrom(start interface{}, end interface{}) ast.Span {
	s := asSpan(start)
	e := asSpan(end)
	return ast.Span{
		File:      s.File,
		Line:      s.Line,
		Column:    s.Column,
		EndLine:   e.EndLine,
		EndColumn: e.EndColumn,
	}
}

func asSpan(v interface{}) ast.Span {
	switch x := v.(type) {
	case ast.Span:
		return x
	case token.Token:
		return tokenSpan(x)
	case ast.Expr:
		return x.GetSpan()
	case ast.TypeExpr:
		return x.GetSpan()
	default:
		panic(fmt.Sprintf("unsupported span value %T", v))
	}
}

func typeSpan(t ast.TypeExpr) ast.Span {
	if t == nil {
		return ast.Span{}
	}
	return t.GetSpan()
}

func toLValue(expr ast.Expr) ast.LValue {
	switch e := expr.(type) {
	case *ast.NameExpr:
		return &ast.NameLValue{Name: e.Name, Span: e.Span}
	case *ast.FieldExpr:
		return &ast.FieldLValue{Base: e.Base, Name: e.Name, Span: e.Span}
	case *ast.IndexExpr:
		return &ast.IndexLValue{Base: e.Base, Index: e.Index, Span: e.Span}
	default:
		return nil
	}
}
