package callgraph

import "testing"

func TestTraceFollowsCalls(t *testing.T) {
	graph := buildLangFixture(t, map[string]string{
		"app.py": `def a():
    b()
    c()

def b():
    c()

def c():
    pass
`,
	})
	trace := graph.Trace("app.py::a", "callees", 3, 10)
	if trace.Root == nil || trace.Root.Function.Name != "a" {
		t.Fatalf("root = %+v", trace.Root)
	}
	names := map[string]*TraceNode{}
	for _, child := range trace.Root.Children {
		names[child.Function.Name] = child
	}
	if names["b"] == nil || names["c"] == nil {
		t.Fatalf("children = %+v", trace.Root.Children)
	}
	if len(names["b"].Children) != 1 || names["b"].Children[0].Function.Name != "c" {
		t.Fatalf("b children = %+v", names["b"].Children)
	}
	if len(names["c"].Children) != 0 {
		t.Fatalf("c should have no children: %+v", names["c"].Children)
	}
}

func TestTraceCallers(t *testing.T) {
	graph := buildLangFixture(t, map[string]string{
		"app.py": `def a():
    c()

def b():
    c()

def c():
    pass
`,
	})
	trace := graph.Trace("app.py::c", "callers", 2, 10)
	if trace.Root == nil {
		t.Fatal("missing root")
	}
	callers := map[string]bool{}
	for _, child := range trace.Root.Children {
		callers[child.Function.Name] = true
	}
	if !callers["a"] || !callers["b"] {
		t.Fatalf("callers = %+v", trace.Root.Children)
	}
}

func TestTraceMarksCyclesAndTruncation(t *testing.T) {
	graph := buildLangFixture(t, map[string]string{
		"app.py": `def a():
    a()
    b()
    c()
    d()

def b():
    pass

def c():
    pass

def d():
    pass
`,
	})
	trace := graph.Trace("app.py::a", "callees", 2, 2)
	if trace.Root == nil {
		t.Fatal("missing root")
	}
	if !trace.Root.Truncated {
		t.Fatalf("expected truncation: %+v", trace.Root)
	}
	var cycle bool
	for _, child := range trace.Root.Children {
		if child.Function.Name == "a" && child.Cycle {
			cycle = true
		}
	}
	if !cycle {
		t.Fatalf("expected cycle marker: %+v", trace.Root.Children)
	}
	if trace.Root.Children[0].Cycle {
		// the cycle node must not recurse
		if len(trace.Root.Children[0].Children) != 0 {
			t.Fatalf("cycle node recursed: %+v", trace.Root.Children[0])
		}
	}
}
