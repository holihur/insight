package callgraph

import (
	"os"
	"path/filepath"
	"testing"
)

func writeSource(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func jsFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeSource(t, root, "src/util.ts", `
export function helper() {}

export function other() {}

export const arrowHelper = () => {
  helper()
}
`)
	writeSource(t, root, "src/store.ts", `
export class Store {
  get(key: string) {
    return key
  }
}
`)
	writeSource(t, root, "src/other.ts", `
export class Other {
  get(key: string) {
    return key
  }
}
`)
	writeSource(t, root, "src/main.ts", `
import { helper as h, arrowHelper } from "./util"
import * as util from "./util"
import { Store } from "./store"
import { helper } from "@/util"

class Runner {
  start() {
    h()
    util.other()
    arrowHelper()
    helper()
    this.stop()
    // helper() in a comment must be ignored
    const label = "helper()"
    console.log(label)
  }

  stop() {}
}

function loose(s) {
  s.get("x")
}
`)
	return root
}

func buildJSFixture(t *testing.T) *Graph {
	t.Helper()
	graph, err := Build("js123", jsFixture(t), nil, 100)
	if err != nil {
		t.Fatal(err)
	}
	return graph
}

func TestJSIndexesDefinitions(t *testing.T) {
	graph := buildJSFixture(t)
	for _, id := range []string{
		"src/main.ts::start",
		"src/main.ts::stop",
		"src/main.ts::loose",
		"src/util.ts::helper",
		"src/util.ts::arrowHelper",
		"src/store.ts::get",
		"src/other.ts::get",
	} {
		if _, ok := graph.Nodes[id]; !ok {
			t.Fatalf("missing node %s, have %v", id, keys(graph.Nodes))
		}
	}
}

func TestJSResolvesImportsAndQualifiedCalls(t *testing.T) {
	graph := buildJSFixture(t)
	sites := graph.SitesFor("src/main.ts")

	find := func(token, kind string) []Site {
		var out []Site
		for _, site := range sites {
			if site.Token == token && site.Kind == kind {
				out = append(out, site)
			}
		}
		return out
	}

	// h() and helper() both come from the named import.
	for _, token := range []string{"h", "helper"} {
		hits := find(token, "call")
		if len(hits) == 0 {
			t.Fatalf("missing call %s in %+v", token, sites)
		}
		target := hits[0].Targets[0]
		if target.ID != "src/util.ts::helper" || hits[0].Confidence != "exact" {
			t.Fatalf("%s -> %+v (%s)", token, target, hits[0].Confidence)
		}
	}

	// Namespace import.
	ns := find("util.other", "call")
	if len(ns) != 1 || ns[0].Targets[0].ID != "src/util.ts::other" || ns[0].Confidence != "exact" {
		t.Fatalf("util.other = %+v", ns)
	}

	// this.method() resolves inside the same file.
	stop := find("this.stop", "call")
	if len(stop) != 1 || stop[0].Targets[0].ID != "src/main.ts::stop" || stop[0].Confidence != "receiver" {
		t.Fatalf("this.stop = %+v", stop)
	}

	// The comment and the string literal must not become call sites.
	if len(find("helper", "call")) != 1 {
		t.Fatalf("comment/string produced extra helper calls: %+v", find("helper", "call"))
	}
}

func TestJSAmbiguousMethodReturnsCandidates(t *testing.T) {
	graph := buildJSFixture(t)
	for _, site := range graph.SitesFor("src/main.ts") {
		if site.Token != "s.get" {
			continue
		}
		if site.Confidence != "multiple" || len(site.Targets) != 2 {
			t.Fatalf("s.get = %+v (confidence %s)", site.Targets, site.Confidence)
		}
		return
	}
	t.Fatal("missing s.get call site")
}

func TestJSPathAliasResolution(t *testing.T) {
	graph := buildJSFixture(t)
	helper := graph.Lookup("src/util.ts::helper")
	if helper == nil {
		t.Fatal("missing helper")
	}
	var fromMain bool
	for _, caller := range helper.Callers {
		if caller.Node.ID == "src/main.ts::start" {
			fromMain = true
		}
	}
	if !fromMain {
		t.Fatalf("helper callers = %+v", helper.Callers)
	}
	if len(helper.Callers) < 2 {
		t.Fatalf("expected calls from main and arrowHelper, got %+v", helper.Callers)
	}
}

func TestJSIgnoresNonJSFiles(t *testing.T) {
	root := t.TempDir()
	writeSource(t, root, "README.md", "helper()")
	writeSource(t, root, "types.d.ts", "declare function helper(): void")
	graph, err := Build("x", root, nil, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Nodes) != 0 {
		t.Fatalf("expected no nodes, got %d", len(graph.Nodes))
	}
}
