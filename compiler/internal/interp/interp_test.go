package interp

import (
	"os"
	"path/filepath"
	"testing"

	"meltlang/compiler/internal/mir"
)

func TestAnalyticsOps(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "values.csv")
	if err := os.WriteFile(path, []byte("value\n1\n2\n3\n4\n"), 0o644); err != nil {
		t.Fatalf("write csv: %v", err)
	}

	module := &mir.Module{
		Functions: []mir.Function{
			{
				Name:       "main",
				Domain:     "host",
				ReturnType: "Float32",
				Instrs: []mir.Instr{
					{Op: "csv_load_column", Dest: "values", Type: "Array[Float32]", Text: path, Field: "value"},
					{Op: "filter_compare_const", Dest: "filtered", Args: []string{"values"}, Type: "Array[Float32]", Text: ">", Float: 2},
					{Op: "array_sum", Dest: "sum", Args: []string{"filtered"}, Type: "Float32"},
					{Op: "array_mean", Dest: "mean", Args: []string{"filtered"}, Type: "Float32"},
					{Op: "array_min", Dest: "min", Args: []string{"filtered"}, Type: "Float32"},
					{Op: "array_max", Dest: "max", Args: []string{"filtered"}, Type: "Float32"},
					{Op: "array_count", Dest: "count", Args: []string{"filtered"}, Type: "Int64"},
					{Op: "return", Args: []string{"mean"}},
				},
			},
		},
	}

	i := New(module)
	val, err := i.runFunction("main", nil)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if val.Kind != "Float32" || val.Number != 3.5 {
		t.Fatalf("unexpected return: %#v", val)
	}
}
