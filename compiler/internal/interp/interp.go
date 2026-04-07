package interp

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"

	"meltlang/compiler/internal/mir"
)

type Value struct {
	Kind    string
	Number  float64
	Numbers []float64
	Rows    []map[string]float64
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
			env[inst.Dest] = Value{Kind: inst.Type, Number: inst.Float}
		case "csv_load_struct_array":
			rows, err := loadStructRows(inst.Text)
			if err != nil {
				return Value{}, err
			}
			env[inst.Dest] = Value{Kind: "rows", Rows: rows}
		case "csv_load_column":
			arr, err := loadNumericColumn(inst.Text, inst.Field)
			if err != nil {
				return Value{}, err
			}
			env[inst.Dest] = Value{Kind: inst.Type, Numbers: arr}
		case "csv_load_float64_array":
			arr, err := loadFloat64Array(inst.Text)
			if err != nil {
				return Value{}, err
			}
			env[inst.Dest] = Value{Kind: "Array[Float64]", Numbers: arr}
		case "map_field":
			base := env[inst.Args[0]]
			out := make([]float64, len(base.Rows))
			for idx, row := range base.Rows {
				out[idx] = row[inst.Field]
			}
			env[inst.Dest] = Value{Kind: inst.Type, Numbers: out}
		case "map_mul_const":
			base := env[inst.Args[0]]
			out := make([]float64, len(base.Numbers))
			for idx, v := range base.Numbers {
				out[idx] = v * inst.Float
			}
			env[inst.Dest] = Value{Kind: inst.Type, Numbers: out}
		case "map_mul_var":
			base := env[inst.Args[0]]
			factor := env[inst.Args[1]].Number
			out := make([]float64, len(base.Numbers))
			for idx, v := range base.Numbers {
				out[idx] = v * factor
			}
			env[inst.Dest] = Value{Kind: inst.Type, Numbers: out}
		case "filter_compare_const":
			base := env[inst.Args[0]]
			out := make([]float64, 0, len(base.Numbers))
			for _, v := range base.Numbers {
				if compareNumber(v, inst.Text, inst.Float) {
					out = append(out, v)
				}
			}
			env[inst.Dest] = Value{Kind: inst.Type, Numbers: out}
		case "filter_compare_var":
			base := env[inst.Args[0]]
			rhs := env[inst.Args[1]].Number
			out := make([]float64, 0, len(base.Numbers))
			for _, v := range base.Numbers {
				if compareNumber(v, inst.Text, rhs) {
					out = append(out, v)
				}
			}
			env[inst.Dest] = Value{Kind: inst.Type, Numbers: out}
		case "array_sum":
			base := env[inst.Args[0]]
			sum := 0.0
			for _, v := range base.Numbers {
				sum += v
			}
			env[inst.Dest] = Value{Kind: inst.Type, Number: sum}
		case "array_count":
			base := env[inst.Args[0]]
			env[inst.Dest] = Value{Kind: inst.Type, Number: float64(len(base.Numbers))}
		case "array_mean":
			base := env[inst.Args[0]]
			sum := 0.0
			for _, v := range base.Numbers {
				sum += v
			}
			mean := 0.0
			if len(base.Numbers) > 0 {
				mean = sum / float64(len(base.Numbers))
			}
			env[inst.Dest] = Value{Kind: inst.Type, Number: mean}
		case "array_min":
			base := env[inst.Args[0]]
			env[inst.Dest] = Value{Kind: inst.Type, Number: aggregateExtrema(base.Numbers, true)}
		case "array_max":
			base := env[inst.Args[0]]
			env[inst.Dest] = Value{Kind: inst.Type, Number: aggregateExtrema(base.Numbers, false)}
		case "print":
			fmt.Println(env[inst.Args[0]])
		case "csv_save":
			if err := saveFloat64Array(inst.Text, env[inst.Args[0]].Numbers); err != nil {
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

func loadNumericColumn(path string, column string) ([]float64, error) {
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
	idx := -1
	for i, name := range records[0] {
		if name == column {
			idx = i
			break
		}
	}
	if idx == -1 {
		return nil, fmt.Errorf("missing column %s in %s", column, path)
	}
	out := make([]float64, 0, len(records)-1)
	for _, rec := range records[1:] {
		if idx >= len(rec) {
			return nil, fmt.Errorf("short row in %s", path)
		}
		v, err := strconv.ParseFloat(rec[idx], 64)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func compareNumber(lhs float64, op string, rhs float64) bool {
	switch op {
	case "==":
		return lhs == rhs
	case "!=":
		return lhs != rhs
	case "<":
		return lhs < rhs
	case "<=":
		return lhs <= rhs
	case ">":
		return lhs > rhs
	case ">=":
		return lhs >= rhs
	default:
		return false
	}
}

func aggregateExtrema(values []float64, wantMin bool) float64 {
	if len(values) == 0 {
		return 0
	}
	best := values[0]
	for _, v := range values[1:] {
		if wantMin && v < best {
			best = v
		}
		if !wantMin && v > best {
			best = v
		}
	}
	return best
}

func (v Value) String() string {
	switch v.Kind {
	case "Int64":
		return strconv.FormatInt(int64(v.Number), 10)
	case "Int32":
		return strconv.FormatInt(int64(v.Number), 10)
	case "Float32", "Float64":
		return strconv.FormatFloat(v.Number, 'f', -1, 64)
	default:
		if v.Rows != nil {
			return fmt.Sprintf("%v", v.Rows)
		}
		if v.Numbers != nil {
			return fmt.Sprintf("%v", v.Numbers)
		}
		return v.Kind
	}
}
