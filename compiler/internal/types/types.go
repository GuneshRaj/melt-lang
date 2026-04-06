package types

type Kind int

const (
	Invalid Kind = iota
	Void
	Bool
	Int32
	Int64
	Float32
	Float64
	String
	Array
	Struct
	Function
	TypeValue
	Namespace
)

type Type struct {
	Kind   Kind
	Elem   *Type
	Name   string
	Params []Type
	Return *Type
}

func NewArray(elem Type) Type {
	return Type{Kind: Array, Elem: &elem}
}

func NewStruct(name string) Type {
	return Type{Kind: Struct, Name: name}
}

func NewFunction(params []Type, ret Type) Type {
	return Type{Kind: Function, Params: params, Return: &ret}
}

func NewTypeValue(name string) Type {
	return Type{Kind: TypeValue, Name: name}
}

func (t Type) String() string {
	switch t.Kind {
	case Void:
		return "Void"
	case Bool:
		return "Bool"
	case Int32:
		return "Int32"
	case Int64:
		return "Int64"
	case Float32:
		return "Float32"
	case Float64:
		return "Float64"
	case String:
		return "String"
	case Array:
		if t.Elem == nil {
			return "Array[Invalid]"
		}
		return "Array[" + t.Elem.String() + "]"
	case Struct:
		return t.Name
	case Function:
		return "Function"
	case TypeValue:
		return "Type"
	case Namespace:
		return "Namespace(" + t.Name + ")"
	default:
		return "Invalid"
	}
}

func (t Type) Equal(other Type) bool {
	if t.Kind != other.Kind {
		return false
	}
	switch t.Kind {
	case Array:
		if t.Elem == nil || other.Elem == nil {
			return t.Elem == other.Elem
		}
		return t.Elem.Equal(*other.Elem)
	case Struct, Namespace, TypeValue:
		return t.Name == other.Name
	case Function:
		if len(t.Params) != len(other.Params) {
			return false
		}
		for i := range t.Params {
			if !t.Params[i].Equal(other.Params[i]) {
				return false
			}
		}
		if t.Return == nil || other.Return == nil {
			return t.Return == other.Return
		}
		return t.Return.Equal(*other.Return)
	default:
		return true
	}
}

func IsNumeric(t Type) bool {
	return t.Kind == Int32 || t.Kind == Int64 || t.Kind == Float32 || t.Kind == Float64
}

func IsInteger(t Type) bool {
	return t.Kind == Int32 || t.Kind == Int64
}

func CanAssign(dst Type, src Type) bool {
	if dst.Equal(src) {
		return true
	}
	if IsNumeric(dst) && IsNumeric(src) {
		return true
	}
	return false
}

func PromoteNumeric(a Type, b Type) Type {
	if a.Kind == Float64 || b.Kind == Float64 {
		return Type{Kind: Float64}
	}
	if a.Kind == Float32 || b.Kind == Float32 {
		return Type{Kind: Float32}
	}
	if a.Kind == Int64 || b.Kind == Int64 {
		return Type{Kind: Int64}
	}
	if a.Kind == Int32 || b.Kind == Int32 {
		return Type{Kind: Int32}
	}
	return Type{Kind: Invalid}
}
