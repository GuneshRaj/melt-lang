package lower

import (
	"testing"

	"meltlang/compiler/internal/lexer"
	"meltlang/compiler/internal/parser"
	"meltlang/compiler/internal/sema"
)

func TestLowerParquetAnalyticsOps(t *testing.T) {
	src := `fn main():
    close = parquet.col("data/prices_10k.parquet", "close", as=Float32)
    movers = close.filter(v -> v > 105.0)
    total = movers.sum()
    avg = movers.mean()
`

	toks, lexDiags := lexer.Lex("test.melt", src)
	if len(lexDiags) > 0 {
		t.Fatalf("unexpected lexer diagnostics: %+v", lexDiags)
	}
	mod, parseDiags := parser.Parse(toks)
	if len(parseDiags) > 0 {
		t.Fatalf("unexpected parser diagnostics: %+v", parseDiags)
	}
	info, semaDiags := sema.Analyze(mod)
	if len(semaDiags) > 0 {
		t.Fatalf("unexpected sema diagnostics: %+v", semaDiags)
	}

	mirMod, err := Lower(mod, info)
	if err != nil {
		t.Fatalf("lower failed: %v", err)
	}
	ops := []string{}
	for _, inst := range mirMod.Functions[0].Instrs {
		ops = append(ops, inst.Op)
	}
	want := []string{"parquet_load_column", "filter_compare_const", "array_sum", "array_mean"}
	if len(ops) != len(want) {
		t.Fatalf("unexpected op count: got %v want %v", ops, want)
	}
	for i := range want {
		if ops[i] != want[i] {
			t.Fatalf("unexpected ops: got %v want %v", ops, want)
		}
	}
}
