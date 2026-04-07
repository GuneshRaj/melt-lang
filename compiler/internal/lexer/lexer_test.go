package lexer

import (
	"testing"

	"meltlang/compiler/internal/token"
)

func TestLexSkipsLineAndBlockComments(t *testing.T) {
	src := `struct PriceRow:
    ts: Int64
    /* close column */
    close: Float32

// host scale
fn scale(x: Array[Float32], factor: Float32) -> Array[Float32]:
    return x.map(v -> v * factor) // inline comment
`

	toks, diags := Lex("test.melt", src)
	if len(diags) > 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}

	for _, tok := range toks {
		if tok.Kind == token.SLASH {
			t.Fatalf("comment slash token leaked into stream: %+v", tok)
		}
	}

	if len(toks) == 0 || toks[len(toks)-1].Kind != token.EOF {
		t.Fatalf("expected EOF token, got %#v", toks)
	}
}
