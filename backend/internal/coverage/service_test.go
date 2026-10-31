package coverage

import (
	"testing"

	"github.com/codeinsight/code-insight/internal/analysis"
)

func TestReviewableFuncsExcludeNonCallables(t *testing.T) {
	in := []analysis.FuncInfo{
		{Name: "main", Kind: "function"},
		{Name: "Server.Run", Kind: "method"},
		{Name: "Config", Kind: "class"},
		{Name: "timeout", Kind: "variable"},
	}
	out := reviewableFuncs(in)
	if len(out) != 2 {
		t.Fatalf("got %d funcs: %+v", len(out), out)
	}
	for _, fn := range out {
		if fn.Kind != "function" && fn.Kind != "method" {
			t.Fatalf("non-callable leaked into coverage: %+v", fn)
		}
	}
}
