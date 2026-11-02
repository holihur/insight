package callgraph

import "testing"

func TestSearchSymbols(t *testing.T) {
	graph := buildLangFixture(t, map[string]string{
		"app.py": `MAX_RETRIES = 3

class Widget:
    def render(self):
        helper()

def helper():
    pass
`,
	})
	// functions and methods come from the call index
	hits := graph.SearchSymbols("help", 10)
	if len(hits) == 0 || hits[0].Name != "helper" || hits[0].Kind != "function" {
		t.Fatalf("helper search = %+v", hits)
	}
	// classes and variables come from the symbol index
	if hits := graph.SearchSymbols("widget", 10); len(hits) == 0 || hits[0].Kind != "class" {
		t.Fatalf("widget search = %+v", hits)
	}
	if hits := graph.SearchSymbols("retries", 10); len(hits) == 0 || hits[0].Kind != "variable" {
		t.Fatalf("variable search = %+v", hits)
	}
	// ranking: exact name before substring
	hits = graph.SearchSymbols("render", 10)
	if len(hits) == 0 || hits[0].Name != "render" {
		t.Fatalf("render search = %+v", hits)
	}
}
