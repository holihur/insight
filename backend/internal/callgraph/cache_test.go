package callgraph

import (
	"path/filepath"
	"testing"
)

func TestIncrementalParseCache(t *testing.T) {
	root := t.TempDir()
	writeSource(t, root, "src/a.py", "def caller():\n    helper()\n")
	writeSource(t, root, "src/b.py", "def helper():\n    pass\n")

	cache := newParseCache()
	first, err := Build("c1", root, nil, 100, cache)
	if err != nil {
		t.Fatal(err)
	}
	hits, misses := cache.stats()
	if hits != 0 || misses != 2 {
		t.Fatalf("first build hits=%d misses=%d, want 0/2", hits, misses)
	}
	if first.ReusedFiles != 0 || first.ParsedFiles != 2 {
		t.Fatalf("graph stats reused=%d parsed=%d", first.ReusedFiles, first.ParsedFiles)
	}

	second, err := Build("c1", root, nil, 100, cache)
	if err != nil {
		t.Fatal(err)
	}
	hits, misses = cache.stats()
	if hits != 2 || misses != 0 {
		t.Fatalf("second build hits=%d misses=%d, want 2/0", hits, misses)
	}
	if second.ReusedFiles != 2 || second.ParsedFiles != 0 {
		t.Fatalf("second stats reused=%d parsed=%d", second.ReusedFiles, second.ParsedFiles)
	}
	if len(second.Edges) != len(first.Edges) || len(second.Nodes) != len(first.Nodes) {
		t.Fatalf("cached rebuild changed graph: edges %d->%d nodes %d->%d",
			len(first.Edges), len(second.Edges), len(first.Nodes), len(second.Nodes))
	}

	// A changed file must be the only one re-parsed, and resolution must see
	// the new duplicate name in the changed file.
	writeSource(t, root, "src/b.py", "def helper():\n    pass\n\ndef helper():\n    pass\n")
	third, err := Build("c2", root, nil, 100, cache)
	if err != nil {
		t.Fatal(err)
	}
	hits, misses = cache.stats()
	if hits != 1 || misses != 1 {
		t.Fatalf("third build hits=%d misses=%d, want 1/1", hits, misses)
	}
	var callerHelper *Site
	for _, site := range third.SitesFor("src/a.py") {
		if site.Kind == "call" && site.Token == "helper" {
			callerHelper = &site
		}
	}
	if callerHelper == nil {
		t.Fatal("missing helper call")
	}
	if callerHelper.Confidence != "multiple" || len(callerHelper.Targets) != 2 {
		t.Fatalf("helper = %+v (%s), want 2 candidates", callerHelper.Targets, callerHelper.Confidence)
	}
}

func TestParseCachePersistence(t *testing.T) {
	root := t.TempDir()
	writeSource(t, root, "app.py", "def run():\n    helper()\n\ndef helper():\n    pass\n")

	cache := newParseCache()
	if _, err := Build("c1", root, nil, 100, cache); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "parse-cache.json")
	if err := cache.save(path); err != nil {
		t.Fatal(err)
	}

	loaded := loadParseCache(path)
	if len(loaded.entries) != 1 {
		t.Fatalf("loaded %d entries, want 1", len(loaded.entries))
	}
	graph, err := Build("c1", root, nil, 100, loaded)
	if err != nil {
		t.Fatal(err)
	}
	hits, misses := loaded.stats()
	if hits != 1 || misses != 0 {
		t.Fatalf("reloaded cache hits=%d misses=%d, want 1/0", hits, misses)
	}
	if len(graph.Edges) == 0 {
		t.Fatal("expected cached graph to keep edges")
	}
}

func TestGoFilesAreNotCachedButStillIndexed(t *testing.T) {
	root := t.TempDir()
	writeSource(t, root, "go.mod", "module example.com/cachetest\n")
	writeSource(t, root, "main.go", "package main\n\nfunc main() { helper() }\n\nfunc helper() {}\n")
	cache := newParseCache()
	graph, err := Build("c1", root, nil, 100, cache)
	if err != nil {
		t.Fatal(err)
	}
	if graph.ReusedFiles != 0 {
		t.Fatalf("expected no reused files, got %d", graph.ReusedFiles)
	}
	if graph.Lookup("main.go::helper") == nil {
		t.Fatal("missing Go function in cached build")
	}
}
