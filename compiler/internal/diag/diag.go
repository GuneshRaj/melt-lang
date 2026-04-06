package diag

type Kind string

const (
	LexErrorKind      Kind = "lex"
	ParseErrorKind    Kind = "parse"
	ResolveErrorKind  Kind = "resolve"
	TypeErrorKind     Kind = "type"
	DomainErrorKind   Kind = "domain"
	LoweringErrorKind Kind = "lowering"
	BuildErrorKind    Kind = "build"
	InternalErrorKind Kind = "internal"
)

type Diagnostic struct {
	Kind      Kind
	Message   string
	File      string
	Line      int
	Column    int
	EndLine   int
	EndColumn int
	Note      string
}

func (d Diagnostic) Error() string {
	if d.File == "" {
		return string(d.Kind) + ": " + d.Message
	}
	return d.File + ":" + itoa(d.Line) + ":" + itoa(d.Column) + ": " + string(d.Kind) + ": " + d.Message
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	sign := ""
	if v < 0 {
		sign = "-"
		v = -v
	}
	var digits [20]byte
	i := len(digits)
	for v > 0 {
		i--
		digits[i] = byte('0' + (v % 10))
		v /= 10
	}
	return sign + string(digits[i:])
}
