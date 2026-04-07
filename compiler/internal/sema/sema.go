package sema

import (
	"meltlang/compiler/internal/ast"
	"meltlang/compiler/internal/diag"
	"meltlang/compiler/internal/types"
)

type StructInfo struct {
	Name   string
	Fields map[string]types.Type
}

type FuncInfo struct {
	Name     string
	IsKernel bool
	Decl     *ast.FunctionDecl
	Params   []types.Type
	Return   types.Type
}

type Info struct {
	Structs   map[string]StructInfo
	Funcs     map[string]FuncInfo
	ExprTypes map[ast.Expr]types.Type
}

type Analyzer struct {
	module *ast.Module
	info   *Info
	diags  []diag.Diagnostic
}

func Analyze(module *ast.Module) (*Info, []diag.Diagnostic) {
	a := &Analyzer{
		module: module,
		info: &Info{
			Structs:   map[string]StructInfo{},
			Funcs:     map[string]FuncInfo{},
			ExprTypes: map[ast.Expr]types.Type{},
		},
	}
	a.collectTopDecls()
	a.checkFunctions()
	return a.info, a.diags
}

func (a *Analyzer) collectTopDecls() {
	mainCount := 0
	for _, decl := range a.module.Decls {
		switch d := decl.(type) {
		case *ast.StructDecl:
			if _, exists := a.info.Structs[d.Name]; exists {
				a.errorSpan(d.Span, diag.ResolveErrorKind, "duplicate struct "+d.Name)
				continue
			}
			fields := map[string]types.Type{}
			for _, f := range d.Fields {
				if _, exists := fields[f.Name]; exists {
					a.errorSpan(f.Span, diag.ResolveErrorKind, "duplicate field "+f.Name)
					continue
				}
				fields[f.Name] = a.resolveTypeExpr(f.Ty)
			}
			a.info.Structs[d.Name] = StructInfo{Name: d.Name, Fields: fields}
		case *ast.FunctionDecl:
			if _, exists := a.info.Funcs[d.Name]; exists {
				a.errorSpan(d.Span, diag.ResolveErrorKind, "duplicate function "+d.Name)
				continue
			}
			params := make([]types.Type, 0, len(d.Params))
			for _, p := range d.Params {
				params = append(params, a.resolveTypeExpr(p.Ty))
			}
			ret := types.Type{Kind: types.Void}
			if d.ReturnType != nil {
				ret = a.resolveTypeExpr(d.ReturnType)
			}
			a.info.Funcs[d.Name] = FuncInfo{
				Name:     d.Name,
				IsKernel: d.IsKernel,
				Decl:     d,
				Params:   params,
				Return:   ret,
			}
			if d.Name == "main" {
				mainCount++
				if d.IsKernel {
					a.errorSpan(d.Span, diag.DomainErrorKind, "main must be a host function")
				}
				if len(d.Params) != 0 {
					a.errorSpan(d.Span, diag.TypeErrorKind, "main must not accept parameters")
				}
				if d.ReturnType != nil && ret.Kind != types.Void {
					a.errorSpan(d.Span, diag.TypeErrorKind, "main must return Void")
				}
			}
		}
	}
	if mainCount != 1 {
		a.errorSpan(ast.Span{}, diag.ResolveErrorKind, "program must define exactly one fn main()")
	}
}

func (a *Analyzer) checkFunctions() {
	for _, fn := range a.info.Funcs {
		scope := map[string]types.Type{
			"csv":   {Kind: types.Namespace, Name: "csv"},
			"print": types.NewFunction([]types.Type{}, types.Type{Kind: types.Void}),
		}
		for i, p := range fn.Decl.Params {
			scope[p.Name] = fn.Params[i]
		}
		hasReturn := false
		for _, stmt := range fn.Decl.Body {
			if a.checkStmt(stmt, scope, fn) {
				hasReturn = true
			}
		}
		if fn.Return.Kind != types.Void && !hasReturn {
			a.errorSpan(fn.Decl.Span, diag.TypeErrorKind, "missing return in function "+fn.Name)
		}
	}
}

func (a *Analyzer) checkStmt(stmt ast.Stmt, scope map[string]types.Type, fn FuncInfo) bool {
	switch s := stmt.(type) {
	case *ast.LetStmt:
		valTy := a.checkExpr(s.Value, scope, fn)
		if s.Ty != nil {
			declTy := a.resolveTypeExpr(s.Ty)
			if !types.CanAssign(declTy, valTy) {
				a.errorSpan(s.Span, diag.TypeErrorKind, "cannot assign "+valTy.String()+" to "+declTy.String())
			}
			scope[s.Name] = declTy
		} else {
			scope[s.Name] = valTy
		}
	case *ast.AssignStmt:
		targetTy := a.checkLValue(s.Target, scope, fn)
		valTy := a.checkExpr(s.Value, scope, fn)
		if !types.CanAssign(targetTy, valTy) {
			a.errorSpan(s.Span, diag.TypeErrorKind, "cannot assign "+valTy.String()+" to "+targetTy.String())
		}
	case *ast.ReturnStmt:
		retTy := types.Type{Kind: types.Void}
		if s.Value != nil {
			retTy = a.checkExpr(s.Value, scope, fn)
		}
		if !types.CanAssign(fn.Return, retTy) {
			a.errorSpan(s.Span, diag.TypeErrorKind, "bad return type: expected "+fn.Return.String()+", got "+retTy.String())
		}
		return true
	case *ast.ExprStmt:
		a.checkExpr(s.Value, scope, fn)
	case *ast.IfStmt:
		condTy := a.checkExpr(s.Cond, scope, fn)
		if condTy.Kind != types.Bool {
			a.errorSpan(s.Cond.GetSpan(), diag.TypeErrorKind, "if condition must be Bool")
		}
		for _, child := range s.ThenBody {
			a.checkStmt(child, cloneScope(scope), fn)
		}
		for _, br := range s.Elifs {
			condTy = a.checkExpr(br.Cond, scope, fn)
			if condTy.Kind != types.Bool {
				a.errorSpan(br.Cond.GetSpan(), diag.TypeErrorKind, "elif condition must be Bool")
			}
			for _, child := range br.Body {
				a.checkStmt(child, cloneScope(scope), fn)
			}
		}
		for _, child := range s.ElseBody {
			a.checkStmt(child, cloneScope(scope), fn)
		}
	}
	return false
}

func (a *Analyzer) checkLValue(lv ast.LValue, scope map[string]types.Type, fn FuncInfo) types.Type {
	switch v := lv.(type) {
	case *ast.NameLValue:
		ty, ok := scope[v.Name]
		if !ok {
			a.errorSpan(v.Span, diag.ResolveErrorKind, "unknown name "+v.Name)
			return types.Type{Kind: types.Invalid}
		}
		return ty
	case *ast.FieldLValue:
		baseTy := a.checkExpr(v.Base, scope, fn)
		return a.lookupField(baseTy, v.Name, v.Span)
	case *ast.IndexLValue:
		baseTy := a.checkExpr(v.Base, scope, fn)
		a.checkExpr(v.Index, scope, fn)
		if baseTy.Kind == types.Array && baseTy.Elem != nil {
			return *baseTy.Elem
		}
		a.errorSpan(v.Span, diag.TypeErrorKind, "index assignment requires array")
	}
	return types.Type{Kind: types.Invalid}
}

func (a *Analyzer) checkExpr(expr ast.Expr, scope map[string]types.Type, fn FuncInfo) types.Type {
	if expr == nil {
		return types.Type{Kind: types.Void}
	}
	if ty, ok := a.info.ExprTypes[expr]; ok {
		return ty
	}

	var ty types.Type
	switch e := expr.(type) {
	case *ast.NameExpr:
		if fnInfo, ok := a.info.Funcs[e.Name]; ok {
			ty = types.NewFunction(fnInfo.Params, fnInfo.Return)
			break
		}
		if t, ok := scope[e.Name]; ok {
			ty = t
			break
		}
		a.errorSpan(e.Span, diag.ResolveErrorKind, "unknown name "+e.Name)
		ty = types.Type{Kind: types.Invalid}
	case *ast.IntExpr:
		ty = types.Type{Kind: types.Int64}
	case *ast.FloatExpr:
		ty = types.Type{Kind: types.Float64}
	case *ast.StringExpr:
		if fn.IsKernel {
			a.errorSpan(e.Span, diag.DomainErrorKind, "string is not allowed in kernels")
		}
		ty = types.Type{Kind: types.String}
	case *ast.BoolExpr:
		ty = types.Type{Kind: types.Bool}
	case *ast.ArrayExpr:
		if len(e.Elements) == 0 {
			ty = types.NewArray(types.Type{Kind: types.Invalid})
			break
		}
		elemTy := a.checkExpr(e.Elements[0], scope, fn)
		for _, item := range e.Elements[1:] {
			itemTy := a.checkExpr(item, scope, fn)
			if !elemTy.Equal(itemTy) {
				a.errorSpan(item.GetSpan(), diag.TypeErrorKind, "array literals must be homogeneous")
			}
		}
		ty = types.NewArray(elemTy)
	case *ast.TypeExprValue:
		ty = types.NewTypeValue(a.resolveTypeExpr(e.Value).String())
	case *ast.FieldExpr:
		baseTy := a.checkExpr(e.Base, scope, fn)
		if baseTy.Kind == types.Namespace {
			ty = types.NewFunction(nil, types.Type{Kind: types.Invalid})
			ty.Name = baseTy.Name + "." + e.Name
			break
		}
		ty = a.lookupField(baseTy, e.Name, e.Span)
	case *ast.IndexExpr:
		baseTy := a.checkExpr(e.Base, scope, fn)
		a.checkExpr(e.Index, scope, fn)
		if baseTy.Kind == types.Array && baseTy.Elem != nil {
			ty = *baseTy.Elem
		} else {
			a.errorSpan(e.Span, diag.TypeErrorKind, "indexing requires array")
			ty = types.Type{Kind: types.Invalid}
		}
	case *ast.SliceExpr:
		baseTy := a.checkExpr(e.Base, scope, fn)
		if baseTy.Kind != types.Array {
			a.errorSpan(e.Span, diag.TypeErrorKind, "slice requires array")
			ty = types.Type{Kind: types.Invalid}
		} else {
			ty = baseTy
		}
	case *ast.UnaryExpr:
		valueTy := a.checkExpr(e.Expr, scope, fn)
		switch e.Op {
		case "-":
			if !types.IsNumeric(valueTy) {
				a.errorSpan(e.Span, diag.TypeErrorKind, "unary '-' requires numeric operand")
			}
			ty = valueTy
		case "not":
			if valueTy.Kind != types.Bool {
				a.errorSpan(e.Span, diag.TypeErrorKind, "'not' requires Bool operand")
			}
			ty = types.Type{Kind: types.Bool}
		}
	case *ast.BinaryExpr:
		leftTy := a.checkExpr(e.Left, scope, fn)
		rightTy := a.checkExpr(e.Right, scope, fn)
		switch e.Op {
		case "+", "-", "*", "/", "%":
			if !types.IsNumeric(leftTy) || !types.IsNumeric(rightTy) {
				a.errorSpan(e.Span, diag.TypeErrorKind, "arithmetic requires numeric operands")
				ty = types.Type{Kind: types.Invalid}
			} else {
				ty = types.PromoteNumeric(leftTy, rightTy)
			}
		case "==", "!=", "<", "<=", ">", ">=":
			ty = types.Type{Kind: types.Bool}
		case "and", "or":
			if leftTy.Kind != types.Bool || rightTy.Kind != types.Bool {
				a.errorSpan(e.Span, diag.TypeErrorKind, "logical operators require Bool operands")
			}
			ty = types.Type{Kind: types.Bool}
		}
	case *ast.LambdaExpr:
		ty = types.Type{Kind: types.Invalid}
	case *ast.CallExpr:
		ty = a.checkCallExpr(e, scope, fn)
	}

	a.info.ExprTypes[expr] = ty
	return ty
}

func (a *Analyzer) checkCallExpr(call *ast.CallExpr, scope map[string]types.Type, fn FuncInfo) types.Type {
	if field, ok := call.Callee.(*ast.FieldExpr); ok {
		if ns, ok := field.Base.(*ast.NameExpr); ok && ns.Name == "csv" {
			return a.checkCSVCall(field.Name, call, scope, fn)
		}
		methodTargetTy := a.checkExpr(field.Base, scope, fn)
		switch field.Name {
		case "map":
			return a.checkMapCall(call, methodTargetTy, scope, fn)
		case "filter":
			return a.checkFilterCall(call, methodTargetTy, scope, fn)
		case "sum", "count", "mean", "min", "max":
			return a.checkAggregateCall(field.Name, call, methodTargetTy, scope, fn)
		}
	}

	if name, ok := call.Callee.(*ast.NameExpr); ok && name.Name == "print" {
		if fn.IsKernel {
			a.errorSpan(call.Span, diag.DomainErrorKind, "print is not allowed in kernels")
		}
		for _, arg := range call.Args {
			a.checkExpr(arg.Value, scope, fn)
		}
		return types.Type{Kind: types.Void}
	}

	calleeTy := a.checkExpr(call.Callee, scope, fn)
	if calleeTy.Kind != types.Function {
		a.errorSpan(call.Span, diag.TypeErrorKind, "call target is not callable")
		return types.Type{Kind: types.Invalid}
	}
	name, _ := call.Callee.(*ast.NameExpr)
	if name != nil {
		if fnInfo, ok := a.info.Funcs[name.Name]; ok {
			if len(call.Args) != len(fnInfo.Params) {
				a.errorSpan(call.Span, diag.TypeErrorKind, "wrong number of arguments in call to "+name.Name)
			}
			if !fn.IsKernel && fnInfo.IsKernel {
				for i, arg := range call.Args {
					argTy := a.checkExpr(arg.Value, scope, fn)
					if i < len(fnInfo.Params) && !types.CanAssign(fnInfo.Params[i], argTy) {
						a.errorSpan(arg.Span, diag.TypeErrorKind, "bad argument type for "+name.Name)
					}
				}
				return fnInfo.Return
			}
			if fn.IsKernel && !fnInfo.IsKernel {
				a.errorSpan(call.Span, diag.DomainErrorKind, "host function call is not allowed in kernels")
			}
		}
	}
	if calleeTy.Return == nil {
		return types.Type{Kind: types.Invalid}
	}
	return *calleeTy.Return
}

func (a *Analyzer) checkCSVCall(name string, call *ast.CallExpr, scope map[string]types.Type, fn FuncInfo) types.Type {
	if fn.IsKernel {
		a.errorSpan(call.Span, diag.DomainErrorKind, "csv APIs are not allowed in kernels")
	}
	switch name {
	case "col":
		if len(call.Args) != 3 || call.Args[2].Name != "as" {
			a.errorSpan(call.Span, diag.TypeErrorKind, "csv.col expects (path, column, as=Type)")
			return types.Type{Kind: types.Invalid}
		}
		pathTy := a.checkExpr(call.Args[0].Value, scope, fn)
		colTy := a.checkExpr(call.Args[1].Value, scope, fn)
		if pathTy.Kind != types.String {
			a.errorSpan(call.Args[0].Span, diag.TypeErrorKind, "csv.col path must be String")
		}
		if colTy.Kind != types.String {
			a.errorSpan(call.Args[1].Span, diag.TypeErrorKind, "csv.col column must be String")
		}
		asExpr, ok := call.Args[2].Value.(*ast.TypeExprValue)
		if !ok {
			a.errorSpan(call.Args[2].Span, diag.TypeErrorKind, "csv.col as= must be a type")
			return types.Type{Kind: types.Invalid}
		}
		elemTy := a.resolveTypeExpr(asExpr.Value)
		if !types.IsGPUScalar(elemTy) {
			a.errorSpan(call.Args[2].Span, diag.TypeErrorKind, "csv.col supports only Float32 and Int32 in v1")
			return types.Type{Kind: types.Invalid}
		}
		return types.NewArray(elemTy)
	case "load":
		if len(call.Args) != 2 || call.Args[1].Name != "as" {
			a.errorSpan(call.Span, diag.TypeErrorKind, "csv.load expects (path, as=Type)")
			return types.Type{Kind: types.Invalid}
		}
		pathTy := a.checkExpr(call.Args[0].Value, scope, fn)
		if pathTy.Kind != types.String {
			a.errorSpan(call.Args[0].Span, diag.TypeErrorKind, "csv.load path must be String")
		}
		asExpr, ok := call.Args[1].Value.(*ast.TypeExprValue)
		if !ok {
			a.errorSpan(call.Args[1].Span, diag.TypeErrorKind, "csv.load as= must be a type")
			return types.Type{Kind: types.Invalid}
		}
		return a.resolveTypeExpr(asExpr.Value)
	case "save":
		if len(call.Args) != 2 {
			a.errorSpan(call.Span, diag.TypeErrorKind, "csv.save expects (path, data)")
			return types.Type{Kind: types.Invalid}
		}
		pathTy := a.checkExpr(call.Args[0].Value, scope, fn)
		dataTy := a.checkExpr(call.Args[1].Value, scope, fn)
		if pathTy.Kind != types.String {
			a.errorSpan(call.Args[0].Span, diag.TypeErrorKind, "csv.save path must be String")
		}
		if dataTy.Kind != types.Array || dataTy.Elem == nil || !types.IsNumeric(*dataTy.Elem) {
			a.errorSpan(call.Args[1].Span, diag.TypeErrorKind, "csv.save supports only numeric arrays in v1")
		}
		return types.Type{Kind: types.Void}
	default:
		a.errorSpan(call.Span, diag.ResolveErrorKind, "unknown csv API csv."+name)
		return types.Type{Kind: types.Invalid}
	}
}

func (a *Analyzer) checkMapCall(call *ast.CallExpr, targetTy types.Type, scope map[string]types.Type, fn FuncInfo) types.Type {
	if targetTy.Kind != types.Array || targetTy.Elem == nil {
		a.errorSpan(call.Span, diag.TypeErrorKind, "map target must be Array")
		return types.Type{Kind: types.Invalid}
	}
	if len(call.Args) != 1 {
		a.errorSpan(call.Span, diag.TypeErrorKind, "map expects one lambda argument")
		return types.Type{Kind: types.Invalid}
	}
	lambda, ok := call.Args[0].Value.(*ast.LambdaExpr)
	if !ok {
		a.errorSpan(call.Args[0].Span, diag.TypeErrorKind, "map expects a lambda")
		return types.Type{Kind: types.Invalid}
	}
	if len(lambda.Params) != 1 {
		a.errorSpan(lambda.Span, diag.TypeErrorKind, "v1 map lambdas must have exactly one parameter")
		return types.Type{Kind: types.Invalid}
	}
	lambdaScope := cloneScope(scope)
	lambdaScope[lambda.Params[0]] = *targetTy.Elem
	bodyTy := a.checkExpr(lambda.Body, lambdaScope, fn)
	return types.NewArray(bodyTy)
}

func (a *Analyzer) checkFilterCall(call *ast.CallExpr, targetTy types.Type, scope map[string]types.Type, fn FuncInfo) types.Type {
	if targetTy.Kind != types.Array || targetTy.Elem == nil {
		a.errorSpan(call.Span, diag.TypeErrorKind, "filter target must be Array")
		return types.Type{Kind: types.Invalid}
	}
	if !types.IsGPUScalar(*targetTy.Elem) {
		a.errorSpan(call.Span, diag.TypeErrorKind, "filter supports only Array[Float32] and Array[Int32] in v1")
		return types.Type{Kind: types.Invalid}
	}
	if len(call.Args) != 1 {
		a.errorSpan(call.Span, diag.TypeErrorKind, "filter expects one lambda argument")
		return types.Type{Kind: types.Invalid}
	}
	lambda, ok := call.Args[0].Value.(*ast.LambdaExpr)
	if !ok {
		a.errorSpan(call.Args[0].Span, diag.TypeErrorKind, "filter expects a lambda")
		return types.Type{Kind: types.Invalid}
	}
	if len(lambda.Params) != 1 {
		a.errorSpan(lambda.Span, diag.TypeErrorKind, "v1 filter lambdas must have exactly one parameter")
		return types.Type{Kind: types.Invalid}
	}
	lambdaScope := cloneScope(scope)
	lambdaScope[lambda.Params[0]] = *targetTy.Elem
	bodyTy := a.checkExpr(lambda.Body, lambdaScope, fn)
	if bodyTy.Kind != types.Bool {
		a.errorSpan(lambda.Body.GetSpan(), diag.TypeErrorKind, "filter lambda must return Bool")
	}
	return targetTy
}

func (a *Analyzer) checkAggregateCall(name string, call *ast.CallExpr, targetTy types.Type, scope map[string]types.Type, fn FuncInfo) types.Type {
	if targetTy.Kind != types.Array || targetTy.Elem == nil {
		a.errorSpan(call.Span, diag.TypeErrorKind, name+" target must be Array")
		return types.Type{Kind: types.Invalid}
	}
	if !types.IsGPUScalar(*targetTy.Elem) {
		a.errorSpan(call.Span, diag.TypeErrorKind, name+" supports only Array[Float32] and Array[Int32] in v1")
		return types.Type{Kind: types.Invalid}
	}
	if len(call.Args) != 0 {
		a.errorSpan(call.Span, diag.TypeErrorKind, name+" does not accept arguments")
		return types.Type{Kind: types.Invalid}
	}
	switch name {
	case "count":
		return types.Type{Kind: types.Int64}
	case "mean":
		if targetTy.Elem.Kind == types.Int32 {
			return types.Type{Kind: types.Float32}
		}
		return *targetTy.Elem
	default:
		return *targetTy.Elem
	}
}

func (a *Analyzer) resolveTypeExpr(expr ast.TypeExpr) types.Type {
	if expr == nil {
		return types.Type{Kind: types.Void}
	}
	named, ok := expr.(*ast.NamedTypeExpr)
	if !ok {
		return types.Type{Kind: types.Invalid}
	}
	switch named.Name {
	case "Void":
		return types.Type{Kind: types.Void}
	case "Bool":
		return types.Type{Kind: types.Bool}
	case "Int32":
		return types.Type{Kind: types.Int32}
	case "Int64":
		return types.Type{Kind: types.Int64}
	case "Float32":
		return types.Type{Kind: types.Float32}
	case "Float64":
		return types.Type{Kind: types.Float64}
	case "String":
		return types.Type{Kind: types.String}
	case "Array":
		if len(named.Args) != 1 {
			a.errorSpan(named.Span, diag.TypeErrorKind, "Array expects one type argument")
			return types.Type{Kind: types.Invalid}
		}
		elem := a.resolveTypeExpr(named.Args[0])
		return types.NewArray(elem)
	default:
		if _, ok := a.info.Structs[named.Name]; ok {
			return types.NewStruct(named.Name)
		}
		return types.NewStruct(named.Name)
	}
}

func (a *Analyzer) lookupField(baseTy types.Type, fieldName string, span ast.Span) types.Type {
	if baseTy.Kind != types.Struct {
		a.errorSpan(span, diag.TypeErrorKind, "field access requires struct value")
		return types.Type{Kind: types.Invalid}
	}
	info, ok := a.info.Structs[baseTy.Name]
	if !ok {
		a.errorSpan(span, diag.ResolveErrorKind, "unknown struct "+baseTy.Name)
		return types.Type{Kind: types.Invalid}
	}
	fieldTy, ok := info.Fields[fieldName]
	if !ok {
		a.errorSpan(span, diag.ResolveErrorKind, "unknown field "+fieldName+" on "+baseTy.Name)
		return types.Type{Kind: types.Invalid}
	}
	return fieldTy
}

func (a *Analyzer) errorSpan(span ast.Span, kind diag.Kind, msg string) {
	a.diags = append(a.diags, diag.Diagnostic{
		Kind:      kind,
		Message:   msg,
		File:      span.File,
		Line:      span.Line,
		Column:    span.Column,
		EndLine:   span.EndLine,
		EndColumn: span.EndColumn,
	})
}

func cloneScope(in map[string]types.Type) map[string]types.Type {
	out := make(map[string]types.Type, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
