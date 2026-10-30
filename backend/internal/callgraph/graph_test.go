package callgraph

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.com/fx\n\ngo 1.22\n")
	writeFile(t, root, "main.go", `package main

import "example.com/fx/util"

type server struct{}

func (s *server) start() {
	s.stop()
	util.Hello()
	helper()
}

func (s *server) stop() {}

func helper() {}

type holder struct {
	s *server
}

func (h *holder) run() {
	h.s.stop()
}

func main() {
	s := &server{}
	s.start()
}
`)
	writeFile(t, root, "util/util.go", `package util

type Writer interface{ Write(string) }

func Hello() {}

type fileWriter struct{}

func (f *fileWriter) Write(s string) {}

type bufWriter struct{}

func (b *bufWriter) Write(s string) {}

func Use(w Writer) {
	w.Write("x")
}
`)
	return root
}

func buildFixture(t *testing.T) *Graph {
	t.Helper()
	graph, err := Build("abc123", fixture(t), nil, 100)
	if err != nil {
		t.Fatal(err)
	}
	return graph
}

func TestIndexesDefinitionsAndCalls(t *testing.T) {
	graph := buildFixture(t)

	for _, id := range []string{
		"main.go::main",
		"main.go::helper",
		"main.go::server.start",
		"main.go::server.stop",
		"util/util.go::Hello",
		"util/util.go::fileWriter.Write",
	} {
		if _, ok := graph.Nodes[id]; !ok {
			t.Fatalf("missing node %s, have %v", id, keys(graph.Nodes))
		}
	}

	sites := graph.SitesFor("main.go")
	if len(sites) == 0 {
		t.Fatal("expected sites in main.go")
	}
	var hello *Site
	for i := range sites {
		if sites[i].Kind == "call" && sites[i].Token == "util.Hello" {
			hello = &sites[i]
		}
	}
	if hello == nil {
		t.Fatalf("missing util.Hello call site: %+v", sites)
	}
	if len(hello.Targets) != 1 || hello.Targets[0].ID != "util/util.go::Hello" {
		t.Fatalf("bad target: %+v", hello.Targets)
	}
	if hello.Line != 9 {
		t.Fatalf("unexpected line %d", hello.Line)
	}
}

func TestResolvesReceiverAndLocalCalls(t *testing.T) {
	graph := buildFixture(t)

	start := graph.Lookup("main.go::server.start")
	if start == nil {
		t.Fatal("missing start node")
	}
	var stop, hello bool
	for _, link := range start.Callees {
		switch link.Node.ID {
		case "main.go::server.stop":
			stop = true
		case "util/util.go::Hello":
			hello = true
		}
	}
	if !stop || !hello {
		t.Fatalf("start callees = %+v", start.Callees)
	}

	stopLookup := graph.Lookup("main.go::server.stop")
	if stopLookup == nil {
		t.Fatal("missing stop lookup")
	}
	var startLine int
	for _, link := range stopLookup.Callers {
		if link.Node.ID == "main.go::server.start" {
			startLine = link.Line
		}
	}
	if startLine != 8 {
		t.Fatalf("stop callers = %+v", stopLookup.Callers)
	}
}

func TestResolvesMethodThroughStructField(t *testing.T) {
	graph := buildFixture(t)

	for _, site := range graph.SitesFor("main.go") {
		if site.Kind == "call" && site.Token == "h.s.stop" {
			if len(site.Targets) != 1 || site.Targets[0].ID != "main.go::server.stop" {
				t.Fatalf("targets = %+v", site.Targets)
			}
			if site.Confidence != "receiver" {
				t.Fatalf("confidence = %q", site.Confidence)
			}
			return
		}
	}
	t.Fatal("missing h.s.stop site")
}

func TestAmbiguousInterfaceCallReturnsCandidates(t *testing.T) {
	graph := buildFixture(t)

	sites := graph.SitesFor("util/util.go")
	var write *Site
	for i := range sites {
		if sites[i].Kind == "call" && sites[i].Token == "w.Write" {
			write = &sites[i]
		}
	}
	if write == nil {
		t.Fatalf("missing w.Write site: %+v", sites)
	}
	if write.Confidence != "multiple" {
		t.Fatalf("confidence = %q", write.Confidence)
	}
	if len(write.Targets) != 2 {
		t.Fatalf("targets = %+v", write.Targets)
	}
	seen := map[string]bool{}
	for _, target := range write.Targets {
		seen[target.ID] = true
	}
	if !seen["util/util.go::fileWriter.Write"] || !seen["util/util.go::bufWriter.Write"] {
		t.Fatalf("targets = %+v", write.Targets)
	}
}

func TestSitesExposeDefinitionTokens(t *testing.T) {
	graph := buildFixture(t)
	sites := graph.SitesFor("main.go")
	found := false
	for _, site := range sites {
		if site.Kind == "def" && site.Token == "helper" && site.FunctionID == "main.go::helper" {
			found = true
			if site.Col != 5 {
				t.Fatalf("expected col 5, got %d", site.Col)
			}
		}
	}
	if !found {
		t.Fatalf("missing def token: %+v", sites)
	}
}

func TestEmptyRepositoryIsNotIndexed(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "README.md", "hello")
	graph, err := Build("c0ffee", root, nil, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Nodes) != 0 {
		t.Fatalf("expected no nodes, got %d", len(graph.Nodes))
	}
	if res := graph.Lookup("nope"); res != nil {
		t.Fatalf("expected nil lookup, got %+v", res)
	}
}

func keys(m map[string]Node) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
