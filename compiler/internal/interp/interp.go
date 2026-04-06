package interp

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"

	"meltlang/compiler/internal/mir"
)

type Value struct {
	Kind     string
	Float64  float64
	Float64s []float64
	Rows     []map[string]float64
}

type Interpreter struct {
	module *mir.Module
	funcs  map[string]mir.Function
}

func New(module *mir.Module) *Interpreter {
	funcs := map[string]mir.Function{}
	for _, fn := range module.Functions {
		funcs[fn.Name] = fn
	}
	return &Interpreter{module: module, funcs: funcs}
}

func (i *Interpreter) RunMain() error {
	_, err := i.runFunction("main", nil)
	return err
}

func (i *Interpreter) runFunction(name string, args map[string]Value) (Value, error) {
	fn, ok := i.funcs[name]
	if !ok {
		return Value{}, fmt.Errorf("missing function %s", name)
	}
	env := map[string]Value{}
	for k, v := range args {
		env[k] = v
	}
	for _, inst := range fn.Instrs {
		switch inst.Op {
		case "const_number":
			env[inst.Dest] = Value{Kind: "float64", Float64: inst.Float}
		case "csv_load_struct_array":
			rows, err := loadStructRows(inst.Text)
			if err != nil {
				return Value{}, err
			}
			env[inst.Dest] = Value{Kind: "rows", Rows: rows}
		case "csv_load_float64_array":
			arr, err := loadFloat64Array(inst.Text)
			if err != nil {
				return Value{}, err
			}
			env[inst.Dest] = Value{Kind: "float64_array", Float64s: arr}
		case "map_field":
			base := env[inst.Args[0]]
			out := make([]float64, len(base.Rows))
			for idx, row := range base.Rows {
				out[idx] = row[inst.Field]
			}
			env[inst.Dest] = Value{Kind: "float64_array", Float64s: out}
		case "map_mul_const":
			base := env[inst.Args[0]]
			out := make([]float64, len(base.Float64s))
			for idx, v := range base.Float64s {
				out[idx] = v * inst.Float
			}
			env[inst.Dest] = Value{Kind: "float64_array", Float64s: out}
		case "map_mul_var":
			base := env[inst.Args[0]]
			factor := env[inst.Args[1]].Float64
			out := make([]float64, len(base.Float64s))
			for idx, v := range base.Float64s {
				out[idx] = v * factor
			}
			env[inst.Dest] = Value{Kind: "float64_array", Float64s: out}
		case "print":
			fmt.Println(env[inst.Args[0]])
		case "csv_save":
			if err := saveFloat64Array(inst.Text, env[inst.Args[0]].Float64s); err != nil {
				return Value{}, err
			}
		case "kernel_call":
			callArgs := map[string]Value{}
			kfn := i.funcs[inst.Text]
			for idx, param := range kfn.Params {
				callArgs[param.Name] = env[inst.Args[idx]]
			}
			val, err := i.runFunction(inst.Text, callArgs)
			if err != nil {
				return Value{}, err
			}
			env[inst.Dest] = val
		case "host_call":
			callArgs := map[string]Value{}
			hfn := i.funcs[inst.Text]
			for idx, param := range hfn.Params {
				callArgs[param.Name] = env[inst.Args[idx]]
			}
			val, err := i.runFunction(inst.Text, callArgs)
			if err != nil {
				return Value{}, err
			}
			env[inst.Dest] = val
		case "return":
			if len(inst.Args) == 0 {
				return Value{Kind: "void"}, nil
			}
			return env[inst.Args[0]], nil
		case "move":
			env[inst.Dest] = env[inst.Args[0]]
		default:
			return Value{}, fmt.Errorf("unsupported instruction %s", inst.Op)
		}
	}
	return Value{Kind: "void"}, nil
}

func loadStructRows(path string) ([]map[string]float64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) < 1 {
		return nil, nil
	}
	header := records[0]
	out := make([]map[string]float64, 0, len(records)-1)
	for _, rec := range records[1:] {
		row := map[string]float64{}
		for i, col := range header {
			val, err := strconv.ParseFloat(rec[i], 64)
			if err != nil {
				return nil, err
			}
			row[col] = val
		}
		out = append(out, row)
	}
	return out, nil
}

func loadFloat64Array(path string) ([]float64, error) {
	rows, err := loadStructRows(path)
	if err != nil {
		return nil, err
	}
	out := make([]float64, 0, len(rows))
	for _, row := range rows {
		for _, v := range row {
			out = append(out, v)
			break
		}
	}
	return out, nil
}

func saveFloat64Array(path string, values []float64) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	for _, v := range values {
		if err := w.Write([]string{strconv.FormatFloat(v, 'f', -1, 64)}); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}
