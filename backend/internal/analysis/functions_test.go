package analysis

import "testing"

func TestExtractFunctionsDestructuredParamsAndTypes(t *testing.T) {
	src := `export function Widget({
  a,
  b,
}: {
  a: string
  b: (x: number) => void
}) {
  const value = "}"
  return value
}

const arrow = (v: number) => v + 1

function plain() {
  return 1
}
`
	funcs := ExtractFunctions("widget.tsx", []byte(src), "TypeScript")
	byName := map[string]FuncInfo{}
	for _, fn := range funcs {
		byName[fn.Name] = fn
	}
	widget, ok := byName["Widget"]
	if !ok {
		t.Fatalf("Widget not found: %+v", funcs)
	}
	if widget.StartLine != 1 || widget.EndLine != 10 {
		t.Fatalf("Widget = %d-%d, want 1-10", widget.StartLine, widget.EndLine)
	}
	arrow, ok := byName["arrow"]
	if !ok || arrow.StartLine != 12 || arrow.EndLine != 12 {
		t.Fatalf("arrow = %+v", arrow)
	}
	plain, ok := byName["plain"]
	if !ok || plain.StartLine != 14 || plain.EndLine != 16 {
		t.Fatalf("plain = %+v", plain)
	}
}

func TestExtractFunctionsSkipsStringBraces(t *testing.T) {
	src := `const tmpl = ` + "`" + `
value: {
  nested
}
` + "`" + `

export function after() {
  return 1
}
`
	funcs := ExtractFunctions("x.ts", []byte(src), "TypeScript")
	found := false
	for _, fn := range funcs {
		if fn.Name != "after" {
			continue
		}
		found = true
		if fn.StartLine != 7 || fn.EndLine != 9 {
			t.Fatalf("after = %d-%d, want 7-9", fn.StartLine, fn.EndLine)
		}
	}
	if !found {
		t.Fatalf("after not found: %+v", funcs)
	}
}
