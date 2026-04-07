package parser

import (
	"testing"

	"meltlang/compiler/internal/lexer"
)

func TestParseCommentedProgram(t *testing.T) {
	src := `struct PriceRow:
    ts: Int64
    /* close column */
    close: Float32

// host scale
fn scale(x: Array[Float32], factor: Float32) -> Array[Float32]:
    return x.map(v -> v * factor) // inline comment

fn main():
    // load data
    rows = csv.load("data/prices_10k.csv", as=Array[PriceRow])
    close = rows.map(r -> r.close)
    out = scale(close, 1.1)
`

	toks, lexDiags := lexer.Lex("test.melt", src)
	if len(lexDiags) > 0 {
		t.Fatalf("unexpected lexer diagnostics: %+v", lexDiags)
	}

	mod, parseDiags := Parse(toks)
	if len(parseDiags) > 0 {
		t.Fatalf("unexpected parser diagnostics: %+v", parseDiags)
	}
	if mod == nil || len(mod.Decls) != 3 {
		t.Fatalf("unexpected module: %#v", mod)
	}
}
