package mir

type Module struct {
	Structs   []StructDef `json:"structs"`
	Functions []Function  `json:"functions"`
}

type StructDef struct {
	Name   string     `json:"name"`
	Fields []FieldDef `json:"fields"`
}

type FieldDef struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type Function struct {
	Name       string  `json:"name"`
	Domain     string  `json:"domain"`
	Params     []Param `json:"params"`
	ReturnType string  `json:"return_type"`
	Instrs     []Instr `json:"instrs"`
}

type Param struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type Instr struct {
	Op    string   `json:"op"`
	Dest  string   `json:"dest,omitempty"`
	Args  []string `json:"args,omitempty"`
	Type  string   `json:"type,omitempty"`
	Text  string   `json:"text,omitempty"`
	Field string   `json:"field,omitempty"`
	Float float64  `json:"float,omitempty"`
}
