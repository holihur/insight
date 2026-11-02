package callgraph

import (
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"sort"
	"strings"

	"github.com/codeinsight/code-insight/internal/analysis"
	sitter "github.com/smacker/go-tree-sitter"
)

// SymbolRef is a searchable non-callable definition (class or variable).
type SymbolRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"` // class | variable
	Path string `json:"path"`
	Line int    `json:"line"`
}

var symbolIdentRe = regexp.MustCompile(`^[A-Za-z_$][\w$]*$`)

var classNodeTypes = map[string][]string{
	"TypeScript": {"class_declaration", "interface_declaration", "type_alias_declaration", "enum_declaration"},
	"JavaScript": {"class_declaration"},
	"Python":     {"class_definition"},
	"Rust":       {"struct_item", "enum_item", "trait_item", "type_item", "union_item"},
	"Java":       {"class_declaration", "interface_declaration", "enum_declaration", "record_declaration"},
	"C":          {"struct_specifier", "union_specifier", "enum_specifier", "type_definition"},
	"C++":        {"class_specifier", "struct_specifier", "union_specifier", "enum_specifier", "type_definition"},
	"C#":         {"class_declaration", "interface_declaration", "struct_declaration", "enum_declaration", "record_declaration"},
	"PHP":        {"class_declaration", "interface_declaration", "trait_declaration", "enum_declaration"},
	"Ruby":       {"class", "module"},
	"Kotlin":     {"class_declaration", "object_declaration"},
	"Swift":      {"class_declaration", "struct_declaration", "enum_declaration", "protocol_declaration"},
	"Scala":      {"class_definition", "trait_definition", "object_definition"},
}

var variableNodeTypes = map[string][]string{
	"TypeScript": {"lexical_declaration", "variable_declaration", "public_field_definition", "field_definition"},
	"JavaScript": {"lexical_declaration", "variable_declaration", "public_field_definition", "field_definition"},
	"Python":     {"assignment"},
	"Rust":       {"const_item", "static_item", "field_declaration"},
	"Java":       {"field_declaration", "constant_declaration"},
	"C":          {"declaration", "field_declaration"},
	"C++":        {"declaration", "field_declaration"},
	"C#":         {"field_declaration", "property_declaration", "const_declaration"},
	"PHP":        {"property_declaration"},
	"Ruby":       {"assignment"},
	"Kotlin":     {"property_declaration"},
	"Swift":      {"property_declaration"},
	"Scala":      {"val_definition", "var_definition"},
}

var functionAncestors = map[string]bool{
	"function_declaration": true, "function_definition": true, "function_item": true,
	"method_declaration": true, "method_definition": true, "constructor_declaration": true,
	"arrow_function": true, "function_expression": true, "local_function_statement": true,
	"lambda": true,
}

func symbolRef(filePath, kind, name string, line int) SymbolRef {
	return SymbolRef{ID: filePath + "::" + kind + "::" + name + "#" + itoa(line), Name: name, Kind: kind, Path: filePath, Line: line}
}

func goSymbolRefs(filePath string, content []byte, fset *token.FileSet) []SymbolRef {
	parsed, err := parser.ParseFile(fset, filePath, content, parser.SkipObjectResolution)
	if err != nil {
		return nil
	}
	var out []SymbolRef
	for _, decl := range parsed.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		switch gen.Tok {
		case token.TYPE:
			for _, spec := range gen.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				out = append(out, symbolRef(filePath, "class", ts.Name.Name, fset.Position(ts.Pos()).Line))
				if st, ok := ts.Type.(*ast.StructType); ok {
					for _, field := range st.Fields.List {
						for _, name := range field.Names {
							out = append(out, symbolRef(filePath, "variable", name.Name, fset.Position(name.Pos()).Line))
						}
					}
				}
			}
		case token.VAR, token.CONST:
			for _, spec := range gen.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, name := range vs.Names {
					if name.Name == "_" {
						continue
					}
					out = append(out, symbolRef(filePath, "variable", name.Name, fset.Position(name.Pos()).Line))
				}
			}
		}
	}
	return out
}

func extractSymbolRefs(filePath, lang string, content []byte) []SymbolRef {
	grammar, ok := tsLanguageFor(filePath, lang)
	if !ok {
		return nil
	}
	parser := sitter.NewParser()
	parser.SetLanguage(grammar)
	tree := parser.Parse(nil, content)
	if tree == nil {
		return nil
	}
	classes := setOf(classNodeTypes[lang])
	vars := setOf(variableNodeTypes[lang])
	var out []SymbolRef
	seen := map[string]bool{}
	add := func(name, kind string, node *sitter.Node) {
		if !symbolIdentRe.MatchString(name) || name == "_" {
			return
		}
		line := int(node.StartPoint().Row) + 1
		key := name + "\x00" + itoa(line) + "\x00" + kind
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, symbolRef(filePath, kind, name, line))
	}
	var walk func(node *sitter.Node)
	walk = func(node *sitter.Node) {
		if node == nil {
			return
		}
		t := node.Type()
		switch {
		case classes[t]:
			if name := symbolNodeName(lang, node, content); name != "" {
				add(name, "class", node)
			}
		case vars[t]:
			if !insideFunc(node) {
				for _, name := range symbolVariableNames(node, content) {
					add(name, "variable", node)
				}
			}
		}
		for i := 0; i < int(node.ChildCount()); i++ {
			walk(node.Child(i))
		}
	}
	walk(tree.RootNode())
	return out
}

func symbolNodeName(lang string, node *sitter.Node, src []byte) string {
	if name := node.ChildByFieldName("name"); name != nil {
		return identifierFrom(name.Content(src))
	}
	if lang == "C" || lang == "C++" || lang == "Swift" || lang == "Kotlin" || lang == "Scala" || lang == "Ruby" {
		return firstIdentifier(node, src)
	}
	return ""
}

func symbolVariableNames(node *sitter.Node, src []byte) []string {
	switch node.Type() {
	case "lexical_declaration", "variable_declaration":
		var out []string
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			if child.Type() != "variable_declarator" {
				continue
			}
			if name := child.ChildByFieldName("name"); name != nil {
				if id := identifierFrom(name.Content(src)); id != "" {
					out = append(out, id)
				}
			}
		}
		return out
	case "field_declaration", "property_declaration", "constant_declaration", "const_declaration", "field_definition", "public_field_definition":
		var out []string
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			if child.Type() == "variable_declarator" {
				if name := child.ChildByFieldName("name"); name != nil {
					if id := identifierFrom(name.Content(src)); id != "" {
						out = append(out, id)
					}
				}
				continue
			}
			if strings.Contains(child.Type(), "identifier") {
				if id := firstIdentifier(child, src); id != "" {
					out = append(out, id)
				}
			}
		}
		return out
	case "assignment":
		if left := node.ChildByFieldName("left"); left != nil {
			if id := identifierFrom(left.Content(src)); id != "" {
				return []string{id}
			}
		}
		return nil
	}
	if name := node.ChildByFieldName("name"); name != nil {
		if id := identifierFrom(name.Content(src)); id != "" {
			return []string{id}
		}
	}
	if id := firstIdentifier(node, src); id != "" {
		return []string{id}
	}
	return nil
}

func insideFunc(node *sitter.Node) bool {
	for parent := node.Parent(); parent != nil; parent = parent.Parent() {
		if functionAncestors[parent.Type()] || strings.Contains(parent.Type(), "lambda") {
			return true
		}
	}
	return false
}

// SearchSymbols ranks functions, methods, classes and variables by name.
func (g *Graph) SearchSymbols(query string, limit int) []SymbolRef {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" || limit <= 0 {
		return nil
	}
	type hit struct {
		ref   SymbolRef
		score int
	}
	hits := make([]hit, 0, 64)
	seen := map[string]bool{}
	add := func(ref SymbolRef) {
		key := ref.Path + "\x00" + ref.Name + "\x00" + itoa(ref.Line)
		if seen[key] {
			return
		}
		seen[key] = true
		name := strings.ToLower(ref.Name)
		score := 4
		switch {
		case name == query:
			score = 0
		case strings.HasPrefix(name, query):
			score = 1
		case strings.Contains(name, query):
			score = 2
		case strings.Contains(strings.ToLower(ref.Path), query):
			score = 3
		default:
			return
		}
		hits = append(hits, hit{ref: ref, score: score})
	}
	for id, node := range g.Nodes {
		add(SymbolRef{ID: id, Name: node.Name, Kind: node.Kind, Path: node.Path, Line: node.StartLine})
	}
	for _, symbol := range g.Symbols {
		add(symbol)
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score < hits[j].score
		}
		if len(hits[i].ref.Name) != len(hits[j].ref.Name) {
			return len(hits[i].ref.Name) < len(hits[j].ref.Name)
		}
		if hits[i].ref.Path != hits[j].ref.Path {
			return hits[i].ref.Path < hits[j].ref.Path
		}
		return hits[i].ref.Line < hits[j].ref.Line
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	out := make([]SymbolRef, len(hits))
	for i, h := range hits {
		out[i] = h.ref
	}
	return out
}

// LanguageForPath exposes the analysis language for symbol extraction callers.
func LanguageForPath(filePath string) string { return analysis.LanguageFor(filePath) }
