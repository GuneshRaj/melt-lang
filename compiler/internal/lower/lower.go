package lower

import (
	"fmt"

	"meltlang/compiler/internal/ast"
	"meltlang/compiler/internal/mir"
	"meltlang/compiler/internal/sema"
	"meltlang/compiler/internal/types"
)

type Lowerer struct {
	info     *sema.Info
	tempSeed int
}

func Lower(module *ast.Module, info *sema.Info) (*mir.Module, error) {
	l := &Lowerer{info: info}
	out := &mir.Module{}

	for _, s := range info.Structs {
		def := mir.StructDef{Name: s.Name}
		for name, ty := range s.Fields {
			def.Fields = append(def.Fields, mir.FieldDef{Name: name, Type: ty.String()})
		}
		out.Structs = append(out.Structs, def)
	}

	for _, decl := range module.Decls {
		fn, ok := decl.(*ast.FunctionDecl)
		if !ok {
			continue
		}
		mfn, err := l.lowerFunction(fn)
		if err != nil {
			return nil, err
		}
		out.Functions = append(out.Functions, mfn)
	}
	return out, nil
}

func (l *Lowerer) lowerFunction(fn *ast.FunctionDecl) (mir.Function, error) {
	info := l.info.Funcs[fn.Name]
	out := mir.Function{
		Name:       fn.Name,
		Domain:     "host",
		ReturnType: info.Return.String(),
	}
	if fn.IsKernel {
		out.Domain = "kernel"
	}
	for i, p := range fn.Params {
		out.Params = append(out.Params, mir.Param{Name: p.Name, Type: info.Params[i].String()})
	}
	for _, stmt := range fn.Body {
		if err := l.lowerStmt(&out, stmt); err != nil {
			return mir.Function{}, err
		}
	}
	return out, nil
}

func (l *Lowerer) lowerStmt(out *mir.Function, stmt ast.Stmt) error {
	switch s := stmt.(type) {
	case *ast.LetStmt:
		instrs, err := l.lowerExprTo(s.Value, s.Name)
		if err != nil {
			return err
		}
		if s.Ty != nil && len(instrs) == 1 && instrs[0].Op == "const_number" && instrs[0].Dest == s.Name {
			instrs[0].Type = typeExprString(s.Ty)
		}
		out.Instrs = append(out.Instrs, instrs...)
	case *ast.ExprStmt:
		instrs, err := l.lowerExprTo(s.Value, "")
		if err != nil {
			return err
		}
		out.Instrs = append(out.Instrs, instrs...)
	case *ast.ReturnStmt:
		if s.Value == nil {
			out.Instrs = append(out.Instrs, mir.Instr{Op: "return"})
			return nil
		}
		name, err := l.referenceOf(s.Value)
		if err != nil {
			tmp := "__return_tmp"
			instrs, lowerErr := l.lowerExprTo(s.Value, tmp)
			if lowerErr != nil {
				return lowerErr
			}
			out.Instrs = append(out.Instrs, instrs...)
			name = tmp
		}
		out.Instrs = append(out.Instrs, mir.Instr{Op: "return", Args: []string{name}})
	default:
		return fmt.Errorf("unsupported statement %T", stmt)
	}
	return nil
}

func (l *Lowerer) lowerExprTo(expr ast.Expr, dest string) ([]mir.Instr, error) {
	switch e := expr.(type) {
	case *ast.CallExpr:
		if field, ok := e.Callee.(*ast.FieldExpr); ok {
			if ns, ok := field.Base.(*ast.NameExpr); ok && ns.Name == "csv" {
				switch field.Name {
				case "col":
					path, _ := e.Args[0].Value.(*ast.StringExpr)
					column, _ := e.Args[1].Value.(*ast.StringExpr)
					return []mir.Instr{{Op: "csv_load_column", Dest: dest, Type: l.info.ExprTypes[expr].String(), Text: path.Value, Field: column.Value}}, nil
				case "load":
					path, _ := e.Args[0].Value.(*ast.StringExpr)
					ty := l.info.ExprTypes[expr]
					if ty.Kind == types.Array && ty.Elem != nil && ty.Elem.Kind == types.Struct {
						return []mir.Instr{{Op: "csv_load_struct_array", Dest: dest, Type: ty.String(), Text: path.Value, Field: ty.Elem.Name}}, nil
					}
					return []mir.Instr{{Op: "csv_load_float64_array", Dest: dest, Type: ty.String(), Text: path.Value}}, nil
				case "save":
					path, _ := e.Args[0].Value.(*ast.StringExpr)
					arg, err := l.referenceOf(e.Args[1].Value)
					if err != nil {
						return nil, err
					}
					return []mir.Instr{{Op: "csv_save", Args: []string{arg}, Text: path.Value, Type: l.info.ExprTypes[e.Args[1].Value].String()}}, nil
				}
			}
			if field.Name == "map" {
				base, err := l.referenceOf(field.Base)
				if err != nil {
					return nil, err
				}
				lambda := e.Args[0].Value.(*ast.LambdaExpr)
				switch body := lambda.Body.(type) {
				case *ast.FieldExpr:
					return []mir.Instr{{Op: "map_field", Dest: dest, Args: []string{base}, Field: body.Name, Type: l.info.ExprTypes[expr].String()}}, nil
				case *ast.BinaryExpr:
					if body.Op == "*" {
						if name, ok := body.Left.(*ast.NameExpr); ok && name.Name == lambda.Params[0] {
							switch rhs := body.Right.(type) {
							case *ast.FloatExpr:
								return []mir.Instr{{Op: "map_mul_const", Dest: dest, Args: []string{base}, Float: rhs.Value, Type: l.info.ExprTypes[expr].String()}}, nil
							case *ast.NameExpr:
								return []mir.Instr{{Op: "map_mul_var", Dest: dest, Args: []string{base, rhs.Name}, Type: l.info.ExprTypes[expr].String()}}, nil
							}
						}
					}
				}
			}
			if field.Name == "filter" {
				base, err := l.referenceOf(field.Base)
				if err != nil {
					return nil, err
				}
				lambda := e.Args[0].Value.(*ast.LambdaExpr)
				op, operandKind, operandValue, operandFloat, err := l.lowerPredicate(lambda)
				if err != nil {
					return nil, err
				}
				inst := mir.Instr{Dest: dest, Args: []string{base}, Text: op, Type: l.info.ExprTypes[expr].String()}
				switch operandKind {
				case "const":
					inst.Op = "filter_compare_const"
					inst.Float = operandFloat
				case "var":
					inst.Op = "filter_compare_var"
					inst.Args = append(inst.Args, operandValue)
				default:
					return nil, fmt.Errorf("unsupported filter operand kind %s", operandKind)
				}
				return []mir.Instr{inst}, nil
			}
			switch field.Name {
			case "sum":
				base, err := l.referenceOf(field.Base)
				if err != nil {
					return nil, err
				}
				return []mir.Instr{{Op: "array_sum", Dest: dest, Args: []string{base}, Type: l.info.ExprTypes[expr].String()}}, nil
			case "count":
				base, err := l.referenceOf(field.Base)
				if err != nil {
					return nil, err
				}
				return []mir.Instr{{Op: "array_count", Dest: dest, Args: []string{base}, Type: l.info.ExprTypes[expr].String()}}, nil
			case "mean":
				base, err := l.referenceOf(field.Base)
				if err != nil {
					return nil, err
				}
				return []mir.Instr{{Op: "array_mean", Dest: dest, Args: []string{base}, Type: l.info.ExprTypes[expr].String()}}, nil
			case "min":
				base, err := l.referenceOf(field.Base)
				if err != nil {
					return nil, err
				}
				return []mir.Instr{{Op: "array_min", Dest: dest, Args: []string{base}, Type: l.info.ExprTypes[expr].String()}}, nil
			case "max":
				base, err := l.referenceOf(field.Base)
				if err != nil {
					return nil, err
				}
				return []mir.Instr{{Op: "array_max", Dest: dest, Args: []string{base}, Type: l.info.ExprTypes[expr].String()}}, nil
			}
		}
		if name, ok := e.Callee.(*ast.NameExpr); ok {
			if name.Name == "print" {
				arg, err := l.referenceOf(e.Args[0].Value)
				if err != nil {
					return nil, err
				}
				return []mir.Instr{{Op: "print", Args: []string{arg}}}, nil
			}
			if fnInfo, ok := l.info.Funcs[name.Name]; ok && fnInfo.IsKernel {
				args := []string{}
				var instrs []mir.Instr
				for idx, arg := range e.Args {
					expected := types.Type{Kind: types.Invalid}
					if idx < len(fnInfo.Params) {
						expected = fnInfo.Params[idx]
					}
					ref, extra, err := l.referenceOrTemp(arg.Value, expected)
					if err != nil {
						return nil, err
					}
					instrs = append(instrs, extra...)
					args = append(args, ref)
				}
				instrs = append(instrs, mir.Instr{Op: "kernel_call", Dest: dest, Args: args, Text: name.Name, Type: fnInfo.Return.String()})
				return instrs, nil
			}
			if fnInfo, ok := l.info.Funcs[name.Name]; ok && !fnInfo.IsKernel {
				args := []string{}
				var instrs []mir.Instr
				for idx, arg := range e.Args {
					expected := types.Type{Kind: types.Invalid}
					if idx < len(fnInfo.Params) {
						expected = fnInfo.Params[idx]
					}
					ref, extra, err := l.referenceOrTemp(arg.Value, expected)
					if err != nil {
						return nil, err
					}
					instrs = append(instrs, extra...)
					args = append(args, ref)
				}
				instrs = append(instrs, mir.Instr{Op: "host_call", Dest: dest, Args: args, Text: name.Name, Type: fnInfo.Return.String()})
				return instrs, nil
			}
		}
		if field, ok := e.Callee.(*ast.FieldExpr); ok {
			return nil, fmt.Errorf("unsupported call expression .%s", field.Name)
		}
		return nil, fmt.Errorf("unsupported call expression with callee %T", e.Callee)
	case *ast.NameExpr:
		return []mir.Instr{{Op: "move", Dest: dest, Args: []string{e.Name}, Type: l.info.ExprTypes[expr].String()}}, nil
	case *ast.FloatExpr:
		return []mir.Instr{{Op: "const_number", Dest: dest, Float: e.Value, Type: l.info.ExprTypes[expr].String()}}, nil
	case *ast.IntExpr:
		return []mir.Instr{{Op: "const_number", Dest: dest, Float: float64(e.Value), Type: l.info.ExprTypes[expr].String()}}, nil
	}
	return nil, fmt.Errorf("unsupported expression %T", expr)
}

func (l *Lowerer) referenceOf(expr ast.Expr) (string, error) {
	if name, ok := expr.(*ast.NameExpr); ok {
		return name.Name, nil
	}
	return "", fmt.Errorf("expression is not a named value: %T", expr)
}

func (l *Lowerer) referenceOrTemp(expr ast.Expr, expected types.Type) (string, []mir.Instr, error) {
	if name, ok := expr.(*ast.NameExpr); ok {
		return name.Name, nil, nil
	}
	switch e := expr.(type) {
	case *ast.FloatExpr:
		tmp := l.nextTemp()
		constType := "Float64"
		if expected.Kind == types.Float32 {
			constType = "Float32"
		}
		return tmp, []mir.Instr{{Op: "const_number", Dest: tmp, Float: e.Value, Type: constType}}, nil
	case *ast.IntExpr:
		tmp := l.nextTemp()
		constType := "Int64"
		if expected.Kind == types.Float32 {
			constType = "Float32"
		} else if expected.Kind == types.Float64 {
			constType = "Float64"
		}
		return tmp, []mir.Instr{{Op: "const_number", Dest: tmp, Float: float64(e.Value), Type: constType}}, nil
	default:
		return "", nil, fmt.Errorf("expression is not a named or constant value: %T", expr)
	}
}

func (l *Lowerer) nextTemp() string {
	l.tempSeed++
	return fmt.Sprintf("__tmp%d", l.tempSeed)
}

func (l *Lowerer) lowerPredicate(lambda *ast.LambdaExpr) (string, string, string, float64, error) {
	body, ok := lambda.Body.(*ast.BinaryExpr)
	if !ok {
		return "", "", "", 0, fmt.Errorf("unsupported filter lambda body %T", lambda.Body)
	}
	if !isCompareOp(body.Op) {
		return "", "", "", 0, fmt.Errorf("unsupported filter operator %s", body.Op)
	}
	if name, ok := body.Left.(*ast.NameExpr); ok && name.Name == lambda.Params[0] {
		switch rhs := body.Right.(type) {
		case *ast.FloatExpr:
			return body.Op, "const", "", rhs.Value, nil
		case *ast.IntExpr:
			return body.Op, "const", "", float64(rhs.Value), nil
		case *ast.NameExpr:
			return body.Op, "var", rhs.Name, 0, nil
		}
	}
	if name, ok := body.Right.(*ast.NameExpr); ok && name.Name == lambda.Params[0] {
		switch lhs := body.Left.(type) {
		case *ast.FloatExpr:
			return flipCompareOp(body.Op), "const", "", lhs.Value, nil
		case *ast.IntExpr:
			return flipCompareOp(body.Op), "const", "", float64(lhs.Value), nil
		case *ast.NameExpr:
			return flipCompareOp(body.Op), "var", lhs.Name, 0, nil
		}
	}
	return "", "", "", 0, fmt.Errorf("filter lambda must compare its parameter to a constant or variable")
}

func isCompareOp(op string) bool {
	switch op {
	case "==", "!=", "<", "<=", ">", ">=":
		return true
	default:
		return false
	}
}

func flipCompareOp(op string) string {
	switch op {
	case "<":
		return ">"
	case "<=":
		return ">="
	case ">":
		return "<"
	case ">=":
		return "<="
	default:
		return op
	}
}

func typeExprString(expr ast.TypeExpr) string {
	if expr == nil {
		return "Void"
	}
	named, ok := expr.(*ast.NamedTypeExpr)
	if !ok {
		return "Invalid"
	}
	if len(named.Args) == 0 {
		return named.Name
	}
	if named.Name == "Array" && len(named.Args) == 1 {
		return "Array[" + typeExprString(named.Args[0]) + "]"
	}
	return named.Name
}
