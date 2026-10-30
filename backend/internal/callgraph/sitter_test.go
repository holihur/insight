package callgraph

import (
	"path/filepath"
	"testing"
)

func buildLangFixture(t *testing.T, files map[string]string) *Graph {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		writeSource(t, root, filepath.ToSlash(rel), content)
	}
	graph, err := Build("tree123", root, nil, 200)
	if err != nil {
		t.Fatal(err)
	}
	return graph
}

func siteFor(t *testing.T, graph *Graph, path, token string) Site {
	t.Helper()
	for _, site := range graph.SitesFor(path) {
		if site.Kind == "call" && site.Token == token {
			return site
		}
	}
	t.Fatalf("missing call %s in %s", token, path)
	return Site{}
}

func TestPythonTreeSitterCalls(t *testing.T) {
	graph := buildLangFixture(t, map[string]string{
		"app.py": `def caller():
    callee()
    obj.method()
    self.stop()
    # callee() in a comment
    text = "callee()"

def callee():
    pass

class A:
    def method(self):
        pass

    def stop(self):
        pass
`,
	})

	local := siteFor(t, graph, "app.py", "callee")
	if len(local.Targets) != 1 || local.Targets[0].ID != "app.py::callee" || local.Confidence != "local" {
		t.Fatalf("callee = %+v (%s)", local.Targets, local.Confidence)
	}
	calls := 0
	for _, site := range graph.SitesFor("app.py") {
		if site.Kind == "call" {
			calls++
		}
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls (comment and string ignored), got %d: %+v", calls, graph.SitesFor("app.py"))
	}
	selfStop := siteFor(t, graph, "app.py", "self.stop")
	if selfStop.Targets[0].ID != "app.py::stop" {
		t.Fatalf("self.stop = %+v", selfStop.Targets)
	}
	method := siteFor(t, graph, "app.py", "obj.method")
	if method.Targets[0].ID != "app.py::method" {
		t.Fatalf("obj.method = %+v", method.Targets)
	}
}

func TestRustTreeSitterCalls(t *testing.T) {
	graph := buildLangFixture(t, map[string]string{
		"src/main.rs": `fn caller() {
    callee();
    obj.method();
    self.stop();
}

fn callee() {}

struct A;

impl A {
    fn method(&self) {}
    fn stop(&self) {}
}
`,
	})
	local := siteFor(t, graph, "src/main.rs", "callee")
	if len(local.Targets) != 1 || local.Targets[0].ID != "src/main.rs::callee" {
		t.Fatalf("callee = %+v", local.Targets)
	}
	selfStop := siteFor(t, graph, "src/main.rs", "self.stop")
	if selfStop.Targets[0].ID != "src/main.rs::stop" || selfStop.Confidence != "receiver" {
		t.Fatalf("self.stop = %+v (%s)", selfStop.Targets, selfStop.Confidence)
	}
}

func TestJavaTreeSitterCalls(t *testing.T) {
	graph := buildLangFixture(t, map[string]string{
		"src/A.java": `class A {
  void caller() {
    callee();
    obj.method();
    this.stop();
  }
  void callee() {}
  void method() {}
  void stop() {}
}
`,
	})
	local := siteFor(t, graph, "src/A.java", "callee")
	if len(local.Targets) != 1 || local.Targets[0].ID != "src/A.java::callee" || local.Confidence != "local" {
		t.Fatalf("callee = %+v (%s)", local.Targets, local.Confidence)
	}
	thisStop := siteFor(t, graph, "src/A.java", "this.stop")
	if thisStop.Targets[0].ID != "src/A.java::stop" || thisStop.Confidence != "receiver" {
		t.Fatalf("this.stop = %+v (%s)", thisStop.Targets, thisStop.Confidence)
	}
}

func TestAmbigousAcrossLanguages(t *testing.T) {
	graph := buildLangFixture(t, map[string]string{
		"src/a.py": `def run(obj):
    obj.save()
`,
		"src/b.py": `class A:
    def save(self):
        pass

class B:
    def save(self):
        pass
`,
	})
	site := siteFor(t, graph, "src/a.py", "obj.save")
	if site.Confidence != "multiple" || len(site.Targets) != 2 {
		t.Fatalf("obj.save = %+v (%s)", site.Targets, site.Confidence)
	}
}

func TestCommentOnlyCallsAreIgnored(t *testing.T) {
	graph := buildLangFixture(t, map[string]string{
		"src/main.rs": `// callee();
fn caller() {
    /* callee(); */
}
fn callee() {}
`,
	})
	for _, site := range graph.SitesFor("src/main.rs") {
		if site.Kind == "call" {
			t.Fatalf("unexpected call site: %+v", site)
		}
	}
}

func TestTreeSitterSmokePerLanguage(t *testing.T) {
	cases := []struct {
		path string
		src  string
	}{
		{"a.c", "void caller() { callee(); }\nvoid callee() {}\n"},
		{"a.cpp", "void caller() { callee(); }\nvoid callee() {}\n"},
		{"A.cs", "class A {\n  void Caller() {\n    Callee();\n  }\n  void Callee() {}\n}\n"},
		{"a.php", "<?php\nfunction caller() {\n  callee();\n}\nfunction callee() {}\n"},
		{"a.rb", "def caller\n  callee()\nend\n\ndef callee\nend\n"},
		{"a.kt", "fun caller() { callee() }\nfun callee() {}\n"},
		{"a.swift", "func caller() { callee() }\nfunc callee() {}\n"},
		{"a.scala", "def caller() = { callee() }\ndef callee() = {}\n"},
		{"a.lua", "function caller() callee() end\nfunction callee() end\n"},
		{"a.ex", "defmodule A do\n  def caller, do: callee()\n  def callee, do: :ok\nend\n"},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			graph := buildLangFixture(t, map[string]string{tc.path: tc.src})
			token := "callee"
			name := "callee"
			if tc.path == "A.cs" {
				token, name = "Callee", "Callee"
			}
			callee := siteFor(t, graph, tc.path, token)
			if len(callee.Targets) == 0 || callee.Targets[0].ID != tc.path+"::"+name {
				t.Fatalf("%s callee -> %+v", tc.path, callee.Targets)
			}
		})
	}
}
