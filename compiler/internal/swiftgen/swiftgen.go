package swiftgen

import (
	"fmt"
	"strings"

	"meltlang/compiler/internal/mir"
)

type Options struct {
	BenchmarkMode bool
}

func Generate(module *mir.Module, opts Options) (string, error) {
	var b strings.Builder
	b.WriteString("import Foundation\n\n")

	for _, s := range module.Structs {
		b.WriteString("struct " + s.Name + ": Decodable {\n")
		for _, f := range s.Fields {
			b.WriteString("    let " + f.Name + ": " + swiftType(f.Type) + "\n")
		}
		b.WriteString("}\n\n")
	}

	b.WriteString("@main\nstruct MeltProgram {\n")
	for _, fn := range module.Functions {
		if fn.Name == "main" || fn.Domain != "host" {
			continue
		}
		src, err := generateHostHelper(fn)
		if err != nil {
			return "", err
		}
		b.WriteString(src)
		b.WriteString("\n")
	}
	b.WriteString("    static func main() throws {\n")

	var mainFn *mir.Function
	for i := range module.Functions {
		if module.Functions[i].Name == "main" {
			mainFn = &module.Functions[i]
			break
		}
	}
	if mainFn == nil {
		return "", fmt.Errorf("missing main function")
	}

	var mainSrc string
	var err error
	if opts.BenchmarkMode {
		mainSrc, err = generateBenchmarkMain(*mainFn)
	} else {
		mainSrc, err = generateNormalMain(*mainFn)
	}
	if err != nil {
		return "", err
	}
	b.WriteString(mainSrc)
	b.WriteString("    }\n")
	b.WriteString("}\n")
	return b.String(), nil
}

func generateHostHelper(fn mir.Function) (string, error) {
	var b strings.Builder
	b.WriteString("    static func " + fn.Name + "(")
	for i, p := range fn.Params {
		if i > 0 {
			b.WriteString(", ")
		}
		if i == 0 {
			b.WriteString("_ " + p.Name + ": " + swiftType(p.Type))
		} else {
			b.WriteString(p.Name + ": " + swiftType(p.Type))
		}
	}
	b.WriteString(") -> " + swiftType(fn.ReturnType) + " {\n")
	for _, inst := range fn.Instrs {
		line, err := swiftForInstr(inst, false)
		if err != nil {
			return "", err
		}
		if line != "" {
			b.WriteString("        " + line + "\n")
		}
	}
	b.WriteString("    }\n")
	return b.String(), nil
}

func generateNormalMain(fn mir.Function) (string, error) {
	var b strings.Builder
	if functionUsesKernel(fn) {
		b.WriteString("        let metal = try MeltMetal()\n")
		b.WriteString("        let exeDir = URL(fileURLWithPath: CommandLine.arguments[0]).deletingLastPathComponent().path\n")
		b.WriteString("        let metallibPath = exeDir + \"/default.metallib\"\n")
	}
	for _, inst := range fn.Instrs {
		line, err := swiftForInstr(inst, false)
		if err != nil {
			return "", err
		}
		if line != "" {
			b.WriteString("        " + line + "\n")
		}
	}
	return b.String(), nil
}

func generateBenchmarkMain(fn mir.Function) (string, error) {
	var b strings.Builder
	var hasKernel bool
	for _, inst := range fn.Instrs {
		if inst.Op == "kernel_call" {
			hasKernel = true
			break
		}
	}
	if hasKernel {
		b.WriteString("        let metal = try MeltMetal()\n")
		b.WriteString("        let exeDir = URL(fileURLWithPath: CommandLine.arguments[0]).deletingLastPathComponent().path\n")
		b.WriteString("        let metallibPath = exeDir + \"/default.metallib\"\n")
	}

	stage := "load"
	b.WriteString("        let __loadStart = CFAbsoluteTimeGetCurrent()\n")
	for _, inst := range fn.Instrs {
		switch inst.Op {
		case "kernel_call", "host_call":
			if stage == "load" {
				b.WriteString("        let __loadMs = (CFAbsoluteTimeGetCurrent() - __loadStart) * 1000.0\n")
				b.WriteString("        let __computeStart = CFAbsoluteTimeGetCurrent()\n")
				stage = "compute"
			}
		case "csv_save":
			if stage == "compute" {
				if hasKernel {
					b.WriteString("        let __computeMs = __gpuMetrics.dispatchMs\n")
				} else {
					b.WriteString("        let __computeMs = (CFAbsoluteTimeGetCurrent() - __computeStart) * 1000.0\n")
				}
				b.WriteString("        let __saveStart = CFAbsoluteTimeGetCurrent()\n")
				stage = "save"
			}
		}

		line, err := swiftForInstr(inst, true)
		if err != nil {
			return "", err
		}
		if line != "" {
			b.WriteString("        " + line + "\n")
		}
	}
	if stage == "save" {
		b.WriteString("        let __saveMs = (CFAbsoluteTimeGetCurrent() - __saveStart) * 1000.0\n")
		b.WriteString("        let __totalMs = __loadMs + __computeMs + __saveMs\n")
		if hasKernel {
			b.WriteString("        print(\"load_ms: \\(__loadMs)\")\n")
			b.WriteString("        print(\"compute_ms: \\(__computeMs)\")\n")
			b.WriteString("        print(\"gpu_setup_ms: \\(__gpuMetrics.setupMs)\")\n")
			b.WriteString("        print(\"gpu_readback_ms: \\(__gpuMetrics.readbackMs)\")\n")
			b.WriteString("        print(\"save_ms: \\(__saveMs)\")\n")
			b.WriteString("        print(\"total_ms: \\(__totalMs)\")\n")
		} else {
			b.WriteString("        print(\"load_ms: \\(__loadMs)\")\n")
			b.WriteString("        print(\"compute_ms: \\(__computeMs)\")\n")
			b.WriteString("        print(\"save_ms: \\(__saveMs)\")\n")
			b.WriteString("        print(\"total_ms: \\(__totalMs)\")\n")
		}
	}
	return b.String(), nil
}

func swiftForInstr(inst mir.Instr, benchmark bool) (string, error) {
	switch inst.Op {
	case "const_number":
		if inst.Type == "Float32" {
			return fmt.Sprintf("let %s = Float(%v)", inst.Dest, inst.Float), nil
		}
		if inst.Type == "Int64" {
			return fmt.Sprintf("let %s = Int64(%d)", inst.Dest, int64(inst.Float)), nil
		}
		return fmt.Sprintf("let %s = %v", inst.Dest, inst.Float), nil
	case "csv_load_struct_array":
		return fmt.Sprintf("let %s = try MeltCSV.loadRows(%q, as: %s.self)", inst.Dest, inst.Text, inst.Field), nil
	case "csv_load_float64_array":
		return fmt.Sprintf("let %s = try MeltCSV.loadFloat64Array(%q)", inst.Dest, inst.Text), nil
	case "map_field":
		return fmt.Sprintf("let %s = %s.map { $0.%s }", inst.Dest, inst.Args[0], inst.Field), nil
	case "map_mul_const":
		if inst.Type == "Array[Float32]" {
			return fmt.Sprintf("let %s = %s.map { $0 * Float(%v) }", inst.Dest, inst.Args[0], inst.Float), nil
		}
		return fmt.Sprintf("let %s = %s.map { $0 * %v }", inst.Dest, inst.Args[0], inst.Float), nil
	case "map_mul_var":
		if inst.Type == "Array[Float32]" {
			return fmt.Sprintf("let %s = %s.map { $0 * Float(%s) }", inst.Dest, inst.Args[0], inst.Args[1]), nil
		}
		return fmt.Sprintf("let %s = %s.map { $0 * %s }", inst.Dest, inst.Args[0], inst.Args[1]), nil
	case "kernel_call":
		if inst.Type == "Array[Float32]" {
			if benchmark {
				return fmt.Sprintf("let (%s, __gpuMetrics) = try metal.mapScaleF32Timed(%s, factor: Float(%s), metallibPath: metallibPath)", inst.Dest, inst.Args[0], inst.Args[1]), nil
			}
			return fmt.Sprintf("let %s = try metal.mapScaleF32(%s, factor: Float(%s), metallibPath: metallibPath)", inst.Dest, inst.Args[0], inst.Args[1]), nil
		}
		return fmt.Sprintf("let %s = try metal.mapScaleF64(%s, factor: %s, metallibPath: metallibPath)", inst.Dest, inst.Args[0], inst.Args[1]), nil
	case "host_call":
		return fmt.Sprintf("let %s = %s(%s)", inst.Dest, inst.Text, swiftCallArgs(inst.Args)), nil
	case "print":
		return fmt.Sprintf("print(%s)", inst.Args[0]), nil
	case "csv_save":
		if inst.Type == "Array[Float32]" {
			return fmt.Sprintf("try MeltCSV.saveFloat32Array(%q, %s)", inst.Text, inst.Args[0]), nil
		}
		return fmt.Sprintf("try MeltCSV.saveFloat64Array(%q, %s)", inst.Text, inst.Args[0]), nil
	case "return":
		if len(inst.Args) == 0 {
			return "return", nil
		}
		return "return " + inst.Args[0], nil
	case "move":
		return fmt.Sprintf("let %s = %s", inst.Dest, inst.Args[0]), nil
	default:
		return "", fmt.Errorf("unsupported Swift instruction %s", inst.Op)
	}
}

func swiftType(t string) string {
	switch t {
	case "Void":
		return "Void"
	case "Bool":
		return "Bool"
	case "Int64":
		return "Int64"
	case "Float32":
		return "Float"
	case "Float64":
		return "Double"
	case "String":
		return "String"
	case "Array[Float32]":
		return "[Float]"
	case "Array[Float64]":
		return "[Double]"
	default:
		return t
	}
}

func swiftCallArgs(args []string) string {
	if len(args) == 0 {
		return ""
	}
	if len(args) == 1 {
		return args[0]
	}
	out := args[0]
	for i := 1; i < len(args); i++ {
		if i == 1 {
			out += ", factor: " + args[i]
		} else {
			out += ", " + args[i]
		}
	}
	return out
}

func functionUsesKernel(fn mir.Function) bool {
	for _, inst := range fn.Instrs {
		if inst.Op == "kernel_call" {
			return true
		}
	}
	return false
}
