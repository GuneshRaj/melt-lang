package ast

type Span struct {
	File      string
	Line      int
	Column    int
	EndLine   int
	EndColumn int
}

type Module struct {
	Decls []TopDecl `json:"decls"`
}

type TopDecl interface {
	topDecl()
}

type StructDecl struct {
	Name   string      `json:"name"`
	Fields []FieldDecl `json:"fields"`
	Span   Span        `json:"span"`
}

func (*StructDecl) topDecl() {}

type FieldDecl struct {
	Name string   `json:"name"`
	Ty   TypeExpr `json:"ty"`
	Span Span     `json:"span"`
}

type FunctionDecl struct {
	Name       string   `json:"name"`
	IsKernel   bool     `json:"is_kernel"`
	Params     []Param  `json:"params"`
	ReturnType TypeExpr `json:"return_type,omitempty"`
	Body       []Stmt   `json:"body"`
	Span       Span     `json:"span"`
}

func (*FunctionDecl) topDecl() {}

type Param struct {
	Name string   `json:"name"`
	Ty   TypeExpr `json:"ty"`
	Span Span     `json:"span"`
}

type TypeExpr interface {
	typeExpr()
	GetSpan() Span
}

type NamedTypeExpr struct {
	Name string     `json:"name"`
	Args []TypeExpr `json:"args,omitempty"`
	Span Span       `json:"span"`
}

func (*NamedTypeExpr) typeExpr()       {}
func (t *NamedTypeExpr) GetSpan() Span { return t.Span }

type Stmt interface {
	stmt()
	GetSpan() Span
}

type LetStmt struct {
	Name  string   `json:"name"`
	Ty    TypeExpr `json:"ty,omitempty"`
	Value Expr     `json:"value"`
	Span  Span     `json:"span"`
}

func (*LetStmt) stmt()           {}
func (s *LetStmt) GetSpan() Span { return s.Span }

type AssignStmt struct {
	Target LValue `json:"target"`
	Value  Expr   `json:"value"`
	Span   Span   `json:"span"`
}

func (*AssignStmt) stmt()           {}
func (s *AssignStmt) GetSpan() Span { return s.Span }

type ReturnStmt struct {
	Value Expr `json:"value,omitempty"`
	Span  Span `json:"span"`
}

func (*ReturnStmt) stmt()           {}
func (s *ReturnStmt) GetSpan() Span { return s.Span }

type IfStmt struct {
	Cond     Expr       `json:"cond"`
	ThenBody []Stmt     `json:"then_body"`
	Elifs    []IfBranch `json:"elifs,omitempty"`
	ElseBody []Stmt     `json:"else_body,omitempty"`
	Span     Span       `json:"span"`
}

func (*IfStmt) stmt()           {}
func (s *IfStmt) GetSpan() Span { return s.Span }

type ExprStmt struct {
	Value Expr `json:"value"`
	Span  Span `json:"span"`
}

func (*ExprStmt) stmt()           {}
func (s *ExprStmt) GetSpan() Span { return s.Span }

type IfBranch struct {
	Cond Expr   `json:"cond"`
	Body []Stmt `json:"body"`
	Span Span   `json:"span"`
}

type LValue interface {
	lvalue()
	GetSpan() Span
}

type NameLValue struct {
	Name string `json:"name"`
	Span Span   `json:"span"`
}

func (*NameLValue) lvalue()         {}
func (v *NameLValue) GetSpan() Span { return v.Span }

type FieldLValue struct {
	Base Expr   `json:"base"`
	Name string `json:"name"`
	Span Span   `json:"span"`
}

func (*FieldLValue) lvalue()         {}
func (v *FieldLValue) GetSpan() Span { return v.Span }

type IndexLValue struct {
	Base  Expr `json:"base"`
	Index Expr `json:"index"`
	Span  Span `json:"span"`
}

func (*IndexLValue) lvalue()         {}
func (v *IndexLValue) GetSpan() Span { return v.Span }

type Expr interface {
	expr()
	GetSpan() Span
}

type NameExpr struct {
	Name string `json:"name"`
	Span Span   `json:"span"`
}

func (*NameExpr) expr()           {}
func (e *NameExpr) GetSpan() Span { return e.Span }

type IntExpr struct {
	Value int64 `json:"value"`
	Span  Span  `json:"span"`
}

func (*IntExpr) expr()           {}
func (e *IntExpr) GetSpan() Span { return e.Span }

type FloatExpr struct {
	Value float64 `json:"value"`
	Span  Span    `json:"span"`
}

func (*FloatExpr) expr()           {}
func (e *FloatExpr) GetSpan() Span { return e.Span }

type StringExpr struct {
	Value string `json:"value"`
	Span  Span   `json:"span"`
}

func (*StringExpr) expr()           {}
func (e *StringExpr) GetSpan() Span { return e.Span }

type BoolExpr struct {
	Value bool `json:"value"`
	Span  Span `json:"span"`
}

func (*BoolExpr) expr()           {}
func (e *BoolExpr) GetSpan() Span { return e.Span }

type ArrayExpr struct {
	Elements []Expr `json:"elements"`
	Span     Span   `json:"span"`
}

func (*ArrayExpr) expr()           {}
func (e *ArrayExpr) GetSpan() Span { return e.Span }

type TypeExprValue struct {
	Value TypeExpr `json:"value"`
	Span  Span     `json:"span"`
}

func (*TypeExprValue) expr()           {}
func (e *TypeExprValue) GetSpan() Span { return e.Span }

type CallExpr struct {
	Callee Expr  `json:"callee"`
	Args   []Arg `json:"args"`
	Span   Span  `json:"span"`
}

func (*CallExpr) expr()           {}
func (e *CallExpr) GetSpan() Span { return e.Span }

type Arg struct {
	Name  string `json:"name,omitempty"`
	Value Expr   `json:"value"`
	Span  Span   `json:"span"`
}

type FieldExpr struct {
	Base Expr   `json:"base"`
	Name string `json:"name"`
	Span Span   `json:"span"`
}

func (*FieldExpr) expr()           {}
func (e *FieldExpr) GetSpan() Span { return e.Span }

type IndexExpr struct {
	Base  Expr `json:"base"`
	Index Expr `json:"index"`
	Span  Span `json:"span"`
}

func (*IndexExpr) expr()           {}
func (e *IndexExpr) GetSpan() Span { return e.Span }

type SliceExpr struct {
	Base  Expr `json:"base"`
	Start Expr `json:"start,omitempty"`
	End   Expr `json:"end,omitempty"`
	Span  Span `json:"span"`
}

func (*SliceExpr) expr()           {}
func (e *SliceExpr) GetSpan() Span { return e.Span }

type UnaryExpr struct {
	Op   string `json:"op"`
	Expr Expr   `json:"expr"`
	Span Span   `json:"span"`
}

func (*UnaryExpr) expr()           {}
func (e *UnaryExpr) GetSpan() Span { return e.Span }

type BinaryExpr struct {
	Left  Expr   `json:"left"`
	Op    string `json:"op"`
	Right Expr   `json:"right"`
	Span  Span   `json:"span"`
}

func (*BinaryExpr) expr()           {}
func (e *BinaryExpr) GetSpan() Span { return e.Span }

type LambdaExpr struct {
	Params []string `json:"params"`
	Body   Expr     `json:"body"`
	Span   Span     `json:"span"`
}

func (*LambdaExpr) expr()           {}
func (e *LambdaExpr) GetSpan() Span { return e.Span }
