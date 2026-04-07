package lexer

import (
	"strconv"
	"unicode"

	"meltlang/compiler/internal/diag"
	"meltlang/compiler/internal/token"
)

type Lexer struct {
	file          string
	src           []rune
	pos           int
	line          int
	column        int
	lineStart     bool
	indentStack   []int
	pendingDedent int
}

func Lex(file string, src string) ([]token.Token, []diag.Diagnostic) {
	l := &Lexer{
		file:        file,
		src:         []rune(src),
		line:        1,
		column:      1,
		lineStart:   true,
		indentStack: []int{0},
	}
	return l.lexAll()
}

func (l *Lexer) lexAll() ([]token.Token, []diag.Diagnostic) {
	var toks []token.Token
	var diags []diag.Diagnostic

	for {
		if l.pendingDedent > 0 {
			toks = append(toks, l.emit(token.DEDENT, ""))
			l.pendingDedent--
			continue
		}

		if l.atEOF() {
			for len(l.indentStack) > 1 {
				l.indentStack = l.indentStack[:len(l.indentStack)-1]
				toks = append(toks, l.emit(token.DEDENT, ""))
			}
			toks = append(toks, l.emit(token.EOF, ""))
			return toks, diags
		}

		if l.lineStart {
			startTok, err := l.lexIndentation()
			if err.Message != "" {
				diags = append(diags, err)
			}
			if startTok != nil {
				toks = append(toks, *startTok)
				continue
			}
		}

		r := l.peek()
		switch {
		case r == ' ' || r == '\r':
			l.advance()
		case r == '\t':
			diags = append(diags, l.error("tabs are not allowed for indentation"))
			l.advance()
		case l.startsWith("//"):
			l.skipLineComment()
		case l.startsWith("/*"):
			if err := l.skipBlockComment(); err.Message != "" {
				diags = append(diags, err)
			}
		case r == '\n':
			toks = append(toks, l.emit(token.NEWLINE, "\n"))
			l.advance()
			l.line++
			l.column = 1
			l.lineStart = true
		case unicode.IsLetter(r) || r == '_':
			toks = append(toks, l.lexIdent())
		case unicode.IsDigit(r):
			tok, err := l.lexNumber()
			toks = append(toks, tok)
			if err.Message != "" {
				diags = append(diags, err)
			}
		case r == '"':
			tok, err := l.lexString()
			toks = append(toks, tok)
			if err.Message != "" {
				diags = append(diags, err)
			}
		default:
			tok, ok := l.lexPunct()
			if ok {
				toks = append(toks, tok)
			} else {
				diags = append(diags, l.error("invalid character"))
				l.advance()
			}
		}
	}
}

func (l *Lexer) lexIndentation() (*token.Token, diag.Diagnostic) {
	spaces := 0
	startCol := l.column
	for !l.atEOF() {
		switch {
		case l.peek() == ' ':
			spaces++
			l.advance()
		case l.peek() == '\t':
			return nil, l.error("tabs are not allowed for indentation")
		case l.startsWith("//"):
			l.skipLineComment()
			l.lineStart = false
			return nil, diag.Diagnostic{}
		case l.startsWith("/*"):
			if err := l.skipBlockComment(); err.Message != "" {
				return nil, err
			}
		case l.peek() == '\n':
			l.lineStart = false
			return nil, diag.Diagnostic{}
		default:
			goto done
		}
	}

done:
	if l.atEOF() || l.peek() == '\n' {
		l.lineStart = false
		return nil, diag.Diagnostic{}
	}

	l.lineStart = false
	top := l.indentStack[len(l.indentStack)-1]
	if spaces == top {
		return nil, diag.Diagnostic{}
	}
	if spaces > top {
		l.indentStack = append(l.indentStack, spaces)
		tok := token.Token{
			Kind:      token.INDENT,
			File:      l.file,
			Line:      l.line,
			Column:    startCol,
			EndLine:   l.line,
			EndColumn: l.column,
		}
		return &tok, diag.Diagnostic{}
	}

	match := -1
	for i := len(l.indentStack) - 1; i >= 0; i-- {
		if l.indentStack[i] == spaces {
			match = i
			break
		}
	}
	if match == -1 {
		return nil, l.error("inconsistent indentation")
	}
	l.pendingDedent = len(l.indentStack) - 1 - match
	l.indentStack = l.indentStack[:match+1]
	if l.pendingDedent > 0 {
		l.pendingDedent--
		tok := token.Token{
			Kind:      token.DEDENT,
			File:      l.file,
			Line:      l.line,
			Column:    startCol,
			EndLine:   l.line,
			EndColumn: l.column,
		}
		return &tok, diag.Diagnostic{}
	}
	return nil, diag.Diagnostic{}
}

func (l *Lexer) lexIdent() token.Token {
	startPos, startLine, startCol := l.pos, l.line, l.column
	for !l.atEOF() && (unicode.IsLetter(l.peek()) || unicode.IsDigit(l.peek()) || l.peek() == '_') {
		l.advance()
	}
	lexeme := string(l.src[startPos:l.pos])
	kind := token.IDENT
	switch lexeme {
	case "struct":
		kind = token.STRUCT
	case "fn":
		kind = token.FN
	case "kernel":
		kind = token.KERNEL
	case "return":
		kind = token.RETURN
	case "if":
		kind = token.IF
	case "elif":
		kind = token.ELIF
	case "else":
		kind = token.ELSE
	case "and":
		kind = token.AND
	case "or":
		kind = token.OR
	case "not":
		kind = token.NOT
	case "true":
		kind = token.TRUE
	case "false":
		kind = token.FALSE
	}
	return token.Token{
		Kind:      kind,
		Lexeme:    lexeme,
		File:      l.file,
		Line:      startLine,
		Column:    startCol,
		EndLine:   l.line,
		EndColumn: l.column,
	}
}

func (l *Lexer) lexNumber() (token.Token, diag.Diagnostic) {
	startPos, startLine, startCol := l.pos, l.line, l.column
	isFloat := false
	for !l.atEOF() && unicode.IsDigit(l.peek()) {
		l.advance()
	}
	if !l.atEOF() && l.peek() == '.' && l.peekNextIsDigit() {
		isFloat = true
		l.advance()
		for !l.atEOF() && unicode.IsDigit(l.peek()) {
			l.advance()
		}
	}
	lexeme := string(l.src[startPos:l.pos])
	kind := token.INT
	if isFloat {
		kind = token.FLOAT
	}
	return token.Token{
		Kind:      kind,
		Lexeme:    lexeme,
		File:      l.file,
		Line:      startLine,
		Column:    startCol,
		EndLine:   l.line,
		EndColumn: l.column,
	}, diag.Diagnostic{}
}

func (l *Lexer) lexString() (token.Token, diag.Diagnostic) {
	startLine, startCol := l.line, l.column
	l.advance()
	startPos := l.pos
	for !l.atEOF() && l.peek() != '"' {
		if l.peek() == '\n' {
			return token.Token{}, l.error("unterminated string")
		}
		l.advance()
	}
	if l.atEOF() {
		return token.Token{}, l.error("unterminated string")
	}
	lexeme := string(l.src[startPos:l.pos])
	l.advance()
	return token.Token{
		Kind:      token.STRING,
		Lexeme:    lexeme,
		File:      l.file,
		Line:      startLine,
		Column:    startCol,
		EndLine:   l.line,
		EndColumn: l.column,
	}, diag.Diagnostic{}
}

func (l *Lexer) lexPunct() (token.Token, bool) {
	startLine, startCol := l.line, l.column
	if l.matchString("->") {
		return l.makeToken(token.ARROW, "->", startLine, startCol), true
	}
	if l.matchString("==") {
		return l.makeToken(token.EQ, "==", startLine, startCol), true
	}
	if l.matchString("!=") {
		return l.makeToken(token.NEQ, "!=", startLine, startCol), true
	}
	if l.matchString("<=") {
		return l.makeToken(token.LTE, "<=", startLine, startCol), true
	}
	if l.matchString(">=") {
		return l.makeToken(token.GTE, ">=", startLine, startCol), true
	}

	r := l.peek()
	var kind token.Kind
	switch r {
	case ':':
		kind = token.COLON
	case ',':
		kind = token.COMMA
	case '.':
		kind = token.DOT
	case '(':
		kind = token.LPAREN
	case ')':
		kind = token.RPAREN
	case '[':
		kind = token.LBRACKET
	case ']':
		kind = token.RBRACKET
	case '+':
		kind = token.PLUS
	case '-':
		kind = token.MINUS
	case '*':
		kind = token.STAR
	case '/':
		kind = token.SLASH
	case '%':
		kind = token.PERCENT
	case '=':
		kind = token.ASSIGN
	case '<':
		kind = token.LT
	case '>':
		kind = token.GT
	default:
		return token.Token{}, false
	}
	l.advance()
	return l.makeToken(kind, string(r), startLine, startCol), true
}

func (l *Lexer) startsWith(s string) bool {
	rs := []rune(s)
	if l.pos+len(rs) > len(l.src) {
		return false
	}
	for i, r := range rs {
		if l.src[l.pos+i] != r {
			return false
		}
	}
	return true
}

func (l *Lexer) skipLineComment() {
	for !l.atEOF() && l.peek() != '\n' {
		l.advance()
	}
}

func (l *Lexer) skipBlockComment() diag.Diagnostic {
	startLine, startCol := l.line, l.column
	l.advance()
	l.advance()
	for !l.atEOF() {
		if l.startsWith("*/") {
			l.advance()
			l.advance()
			return diag.Diagnostic{}
		}
		if l.peek() == '\n' {
			l.advance()
			l.line++
			l.column = 1
			l.lineStart = true
			continue
		}
		l.advance()
	}
	return diag.Diagnostic{
		Kind:      diag.LexErrorKind,
		Message:   "unterminated block comment",
		File:      l.file,
		Line:      startLine,
		Column:    startCol,
		EndLine:   l.line,
		EndColumn: l.column,
	}
}

func (l *Lexer) matchString(s string) bool {
	if !l.startsWith(s) {
		return false
	}
	for range s {
		l.advance()
	}
	return true
}

func (l *Lexer) makeToken(kind token.Kind, lexeme string, line int, col int) token.Token {
	return token.Token{
		Kind:      kind,
		Lexeme:    lexeme,
		File:      l.file,
		Line:      line,
		Column:    col,
		EndLine:   l.line,
		EndColumn: l.column,
	}
}

func (l *Lexer) emit(kind token.Kind, lexeme string) token.Token {
	return token.Token{
		Kind:      kind,
		Lexeme:    lexeme,
		File:      l.file,
		Line:      l.line,
		Column:    l.column,
		EndLine:   l.line,
		EndColumn: l.column,
	}
}

func (l *Lexer) error(msg string) diag.Diagnostic {
	return diag.Diagnostic{
		Kind:      diag.LexErrorKind,
		Message:   msg,
		File:      l.file,
		Line:      l.line,
		Column:    l.column,
		EndLine:   l.line,
		EndColumn: l.column,
	}
}

func (l *Lexer) atEOF() bool {
	return l.pos >= len(l.src)
}

func (l *Lexer) peek() rune {
	if l.atEOF() {
		return 0
	}
	return l.src[l.pos]
}

func (l *Lexer) peekNextIsDigit() bool {
	if l.pos+1 >= len(l.src) {
		return false
	}
	return unicode.IsDigit(l.src[l.pos+1])
}

func (l *Lexer) advance() {
	if l.atEOF() {
		return
	}
	l.pos++
	l.column++
}

func ParseInt(tok token.Token) (int64, error) {
	return strconv.ParseInt(tok.Lexeme, 10, 64)
}

func ParseFloat(tok token.Token) (float64, error) {
	return strconv.ParseFloat(tok.Lexeme, 64)
}
