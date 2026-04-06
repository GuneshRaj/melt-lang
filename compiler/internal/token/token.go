package token

type Kind string

const (
	EOF      Kind = "EOF"
	NEWLINE  Kind = "NEWLINE"
	INDENT   Kind = "INDENT"
	DEDENT   Kind = "DEDENT"
	IDENT    Kind = "IDENT"
	INT      Kind = "INT"
	FLOAT    Kind = "FLOAT"
	STRING   Kind = "STRING"
	TRUE     Kind = "TRUE"
	FALSE    Kind = "FALSE"
	STRUCT   Kind = "STRUCT"
	FN       Kind = "FN"
	KERNEL   Kind = "KERNEL"
	RETURN   Kind = "RETURN"
	IF       Kind = "IF"
	ELIF     Kind = "ELIF"
	ELSE     Kind = "ELSE"
	AND      Kind = "AND"
	OR       Kind = "OR"
	NOT      Kind = "NOT"
	ARROW    Kind = "ARROW"
	COLON    Kind = ":"
	COMMA    Kind = ","
	DOT      Kind = "."
	LPAREN   Kind = "("
	RPAREN   Kind = ")"
	LBRACKET Kind = "["
	RBRACKET Kind = "]"
	PLUS     Kind = "+"
	MINUS    Kind = "-"
	STAR     Kind = "*"
	SLASH    Kind = "/"
	PERCENT  Kind = "%"
	ASSIGN   Kind = "="
	EQ       Kind = "=="
	NEQ      Kind = "!="
	LT       Kind = "<"
	LTE      Kind = "<="
	GT       Kind = ">"
	GTE      Kind = ">="
)

type Token struct {
	Kind      Kind
	Lexeme    string
	File      string
	Line      int
	Column    int
	EndLine   int
	EndColumn int
}
