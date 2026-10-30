package callgraph

import (
	"regexp"
	"sort"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/c"
	"github.com/smacker/go-tree-sitter/cpp"
	"github.com/smacker/go-tree-sitter/csharp"
	"github.com/smacker/go-tree-sitter/elixir"
	"github.com/smacker/go-tree-sitter/java"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/kotlin"
	"github.com/smacker/go-tree-sitter/lua"
	"github.com/smacker/go-tree-sitter/php"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/ruby"
	"github.com/smacker/go-tree-sitter/rust"
	"github.com/smacker/go-tree-sitter/scala"
	"github.com/smacker/go-tree-sitter/swift"
	"github.com/smacker/go-tree-sitter/typescript/tsx"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

// tree-sitter grammars are keyed by the language names used in analysis.
// Go keeps its dedicated go/ast path; these cover the rest.
var tsGrammars = map[string]func() *sitter.Language{
	"TypeScript": typescript.GetLanguage,
	"JavaScript": javascript.GetLanguage,
	"Python":     python.GetLanguage,
	"Rust":       rust.GetLanguage,
	"Java":       java.GetLanguage,
	"C":          c.GetLanguage,
	"C++":        cpp.GetLanguage,
	"C#":         csharp.GetLanguage,
	"PHP":        php.GetLanguage,
	"Ruby":       ruby.GetLanguage,
	"Kotlin":     kotlin.GetLanguage,
	"Swift":      swift.GetLanguage,
	"Scala":      scala.GetLanguage,
	"Lua":        lua.GetLanguage,
	"Elixir":     elixir.GetLanguage,
}

var tsDefTypes = map[string][]string{
	"TypeScript": {"function_declaration", "generator_function_declaration", "method_definition", "function_expression", "arrow_function"},
	"JavaScript": {"function_declaration", "generator_function_declaration", "method_definition", "function_expression", "arrow_function"},
	"Python":     {"function_definition"},
	"Rust":       {"function_item"},
	"Java":       {"method_declaration", "constructor_declaration"},
	"C":          {"function_definition"},
	"C++":        {"function_definition"},
	"C#":         {"method_declaration", "constructor_declaration", "local_function_statement"},
	"PHP":        {"function_definition", "method_declaration"},
	"Ruby":       {"method", "singleton_method"},
	"Kotlin":     {"function_declaration"},
	"Swift":      {"function_declaration"},
	"Scala":      {"function_definition", "function_declaration"},
	"Lua":        {"function_statement", "function_definition"},
}

var tsCallTypes = map[string][]string{
	"TypeScript": {"call_expression"},
	"JavaScript": {"call_expression"},
	"Python":     {"call"},
	"Rust":       {"call_expression"},
	"Java":       {"method_invocation"},
	"C":          {"call_expression"},
	"C++":        {"call_expression"},
	"C#":         {"invocation_expression"},
	"PHP":        {"function_call_expression", "member_call_expression", "scoped_call_expression"},
	"Ruby":       {"call", "command", "command_call"},
	"Kotlin":     {"call_expression"},
	"Swift":      {"call_expression"},
	"Scala":      {"call_expression"},
	"Lua":        {"function_call"},
	"Elixir":     {"call"},
}

var tsIdentRe = regexp.MustCompile(`^[A-Za-z_$][\w$]*$`)

// tsDef is a function/method definition found by tree-sitter.
type tsDef struct {
	ID        string
	Name      string
	StartLine int
	EndLine   int
	StartByte uint32
	EndByte   uint32
}

// tsCall is a call expression found by tree-sitter.
type tsCall struct {
	Name      string
	Receiver  string
	Token     string
	Line      int
	Col       int
	EndCol    int
	StartByte uint32
}

func tsLanguageFor(filePath, lang string) (*sitter.Language, bool) {
	if lang == "TypeScript" && strings.HasSuffix(strings.ToLower(filePath), ".tsx") {
		return tsx.GetLanguage(), true
	}
	grammar, ok := tsGrammars[lang]
	if !ok {
		return nil, false
	}
	return grammar(), true
}

func tsSupported(filePath, lang string) bool {
	_, ok := tsLanguageFor(filePath, lang)
	return ok
}

// extractTS parses content and returns definitions and calls. It never
// panics: on parse errors the tree is still walked for partial results.
func extractTS(filePath, lang string, content []byte) ([]tsDef, []tsCall, bool) {
	grammar, ok := tsLanguageFor(filePath, lang)
	if !ok {
		return nil, nil, false
	}
	parser := sitter.NewParser()
	parser.SetLanguage(grammar)
	tree := parser.Parse(nil, content)
	if tree == nil {
		return nil, nil, false
	}
	root := tree.RootNode()
	if root == nil {
		return nil, nil, false
	}

	defTypes := setOf(tsDefTypes[lang])
	callTypes := setOf(tsCallTypes[lang])
	starts := lineStartsOf(string(content))
	var defs []tsDef
	var calls []tsCall

	var walk func(node *sitter.Node)
	walk = func(node *sitter.Node) {
		if node == nil {
			return
		}
		nodeType := node.Type()
		if defTypes[nodeType] {
			if name := tsDefName(lang, node, content); name != "" {
				startByte := node.StartByte()
				for int(startByte) < len(content) && isSpaceByte(content[startByte]) {
					startByte++
				}
				startLine, _ := lineColFromStarts(starts, int(startByte))
				endLine, _ := lineColFromStarts(starts, int(node.EndByte()))
				defs = append(defs, tsDef{
					Name:      name,
					StartLine: startLine,
					EndLine:   endLine,
					StartByte: startByte,
					EndByte:   node.EndByte(),
				})
			}
		}
		if callTypes[nodeType] {
			if call, ok := tsCallInfo(lang, node, content, starts); ok {
				calls = append(calls, call)
			}
		}
		for i := 0; i < int(node.ChildCount()); i++ {
			walk(node.Child(i))
		}
	}
	walk(root)
	return defs, calls, true
}

func isSpaceByte(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

func setOf(values []string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		out[value] = true
	}
	return out
}

func tsDefName(lang string, node *sitter.Node, src []byte) string {
	if name := node.ChildByFieldName("name"); name != nil {
		return identifierFrom(name.Content(src))
	}
	switch lang {
	case "TypeScript", "JavaScript":
		if node.Type() == "arrow_function" || node.Type() == "function_expression" {
			if parent := node.Parent(); parent != nil {
				if name := parent.ChildByFieldName("name"); name != nil {
					declarator := name.Content(src)
					if idx := strings.LastIndex(declarator, "="); idx >= 0 {
						declarator = strings.TrimSpace(declarator[:idx])
					}
					return identifierFrom(declarator)
				}
			}
		}
	case "C", "C++":
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			if child.Type() == "function_declarator" {
				return firstIdentifier(child, src)
			}
		}
	case "Kotlin", "Swift", "Scala":
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			if strings.Contains(child.Type(), "identifier") {
				return identifierFrom(child.Content(src))
			}
		}
	case "Lua":
		if name := node.ChildByFieldName("name"); name != nil {
			return identifierFrom(name.Content(src))
		}
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			if child.Type() == "identifier" {
				return child.Content(src)
			}
		}
	case "Elixir":
		return ""
	}
	return ""
}

func firstIdentifier(node *sitter.Node, src []byte) string {
	if node == nil {
		return ""
	}
	if strings.Contains(node.Type(), "identifier") {
		return identifierFrom(node.Content(src))
	}
	for i := 0; i < int(node.NamedChildCount()); i++ {
		if name := firstIdentifier(node.NamedChild(i), src); name != "" {
			return name
		}
	}
	return ""
}

// identifierFrom keeps the last identifier of a textual name such as
// "A::run" or "pkg.fn" and drops decorations.
func identifierFrom(text string) string {
	text = strings.TrimSpace(text)
	if idx := strings.LastIndexAny(text, ":."); idx >= 0 {
		text = text[idx+1:]
	}
	text = strings.TrimSpace(text)
	if !tsIdentRe.MatchString(text) {
		return ""
	}
	return text
}

func tsCallInfo(lang string, node *sitter.Node, src []byte, starts []int) (tsCall, bool) {
	switch lang {
	case "Java":
		nameNode := node.ChildByFieldName("name")
		if nameNode == nil {
			return tsCall{}, false
		}
		name := nameNode.Content(src)
		if !tsIdentRe.MatchString(name) {
			return tsCall{}, false
		}
		receiver := ""
		if object := node.ChildByFieldName("object"); object != nil {
			receiver = simpleReceiver(object.Content(src))
		}
		return makeTSCall(name, receiver, nameNode.StartByte(), src, starts), true
	case "Ruby":
		methodNode := node.ChildByFieldName("method")
		if methodNode == nil {
			return tsCall{}, false
		}
		name := methodNode.Content(src)
		if !tsIdentRe.MatchString(name) {
			return tsCall{}, false
		}
		receiver := ""
		if recv := node.ChildByFieldName("receiver"); recv != nil {
			receiver = simpleReceiver(recv.Content(src))
		}
		return makeTSCall(name, receiver, methodNode.StartByte(), src, starts), true
	case "PHP":
		if nameNode := node.ChildByFieldName("name"); nameNode != nil {
			name := nameNode.Content(src)
			if !tsIdentRe.MatchString(name) {
				return tsCall{}, false
			}
			receiver := ""
			if obj := node.ChildByFieldName("object"); obj != nil {
				receiver = simpleReceiver(obj.Content(src))
			} else if scope := node.ChildByFieldName("scope"); scope != nil {
				receiver = simpleReceiver(scope.Content(src))
			}
			return makeTSCall(name, receiver, nameNode.StartByte(), src, starts), true
		}
	case "Elixir":
		target := node.ChildByFieldName("target")
		if target == nil {
			return tsCall{}, false
		}
		name := target.Content(src)
		if name == "" || tsElixirKeywords[name] {
			return tsCall{}, false
		}
		if tsElixirDefWrapper(node, src) {
			return tsCall{}, false
		}
		if target.Type() == "identifier" && tsIdentRe.MatchString(name) {
			return makeTSCall(name, "", target.StartByte(), src, starts), true
		}
		return tsCallFromText(name, node.StartByte(), src, starts)
	}

	end := callCalleeEnd(lang, node)
	if end <= node.StartByte() || end > node.EndByte() {
		return tsCall{}, false
	}
	text := string(src[node.StartByte():end])
	return tsCallFromText(text, node.StartByte(), src, starts)
}

func callCalleeEnd(lang string, node *sitter.Node) uint32 {
	for _, field := range []string{"arguments", "argument_list"} {
		if child := node.ChildByFieldName(field); child != nil {
			return child.StartByte()
		}
	}
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		nodeType := child.Type()
		if nodeType == "call_suffix" || (strings.Contains(nodeType, "argument") && !strings.Contains(nodeType, "type_argument")) {
			return child.StartByte()
		}
	}
	return node.EndByte()
}

func tsCallFromText(text string, startByte uint32, src []byte, starts []int) (tsCall, bool) {
	raw := strings.TrimRight(text, " \t\r\n")
	if idx := strings.Index(raw, "("); idx >= 0 {
		raw = raw[:idx]
	}
	raw = strings.TrimSpace(raw)
	name, receiver, offset := splitCallee(raw)
	if name == "" {
		return tsCall{}, false
	}
	return makeTSCall(name, receiver, startByte+uint32(offset), src, starts), true
}

func makeTSCall(name, receiver string, nameStart uint32, src []byte, starts []int) tsCall {
	tokenStart := nameStart
	if receiver != "" {
		if candidate, ok := tokenStartFor(src, nameStart, receiver); ok {
			tokenStart = candidate
		}
	}
	nameEnd := nameStart + uint32(len(name))
	if int(nameEnd) > len(src) {
		nameEnd = uint32(len(src))
	}
	token := string(src[tokenStart:nameEnd])
	if strings.ContainsAny(token, "\n\r") {
		token = name
		tokenStart = nameStart
	}
	line, col := lineColFromStarts(starts, int(tokenStart))
	return tsCall{
		Name: name, Receiver: receiver, Token: token,
		Line: line, Col: col, EndCol: col + len(token),
		StartByte: nameStart,
	}
}

// tokenStartFor walks backwards over call separators to include the receiver
// in the clickable token (A::run, obj?.method, obj->method, obj:method).
func tokenStartFor(src []byte, nameStart uint32, receiver string) (uint32, bool) {
	if nameStart == 0 || nameStart > uint32(len(src)) {
		return nameStart, false
	}
	i := int(nameStart) - 1
	for i >= 0 && isCallSep(src[i]) {
		i--
	}
	j := i - len(receiver) + 1
	if j < 0 || string(src[j:i+1]) != receiver {
		return nameStart, false
	}
	return uint32(j), true
}

func isCallSep(c byte) bool {
	switch c {
	case '.', ':', '-', '>', '?', '&', ' ', '\t':
		return true
	}
	return false
}

func tsElixirDefWrapper(node *sitter.Node, src []byte) bool {
	parent := node.Parent()
	if parent == nil || parent.Type() != "arguments" {
		return false
	}
	call := parent.Parent()
	if call == nil || call.Type() != "call" {
		return false
	}
	target := call.ChildByFieldName("target")
	if target == nil {
		return false
	}
	return tsElixirKeywords[target.Content(src)]
}

func lineColFromStarts(starts []int, offset int) (int, int) {
	if offset < 0 {
		offset = 0
	}
	idx := sort.Search(len(starts), func(i int) bool { return starts[i] > offset }) - 1
	if idx < 0 {
		idx = 0
	}
	return idx + 1, offset - starts[idx]
}

// splitCallee splits "obj.method", "a::b", "obj->m", "obj:method" or "name".
// It returns the offset of the name within text.
func splitCallee(text string) (name, receiver string, nameOffset int) {
	text = strings.TrimSpace(text)
	text = stripGenerics(text)
	text = strings.TrimRight(text, "!?")
	bestEnd := -1
	bestSep := ""
	for _, sep := range []string{"->", "?.", "&.", "::", ".", ":"} {
		if idx := strings.LastIndex(text, sep); idx >= 0 {
			end := idx + len(sep)
			if end > bestEnd || (end == bestEnd && len(sep) > len(bestSep)) {
				bestEnd = end
				bestSep = sep
			}
		}
	}
	if bestEnd >= 0 {
		name = strings.TrimSpace(text[bestEnd:])
		receiver = simpleReceiver(strings.TrimSpace(text[:bestEnd-len(bestSep)]))
		nameOffset = bestEnd
	} else {
		name = strings.TrimSpace(text)
		nameOffset = 0
	}
	if !tsIdentRe.MatchString(name) {
		return "", "", 0
	}
	return name, receiver, nameOffset
}

func stripGenerics(text string) string {
	if !strings.HasSuffix(text, ">") {
		return text
	}
	depth := 0
	for i := len(text) - 1; i >= 0; i-- {
		switch text[i] {
		case '>':
			depth++
		case '<':
			depth--
			if depth == 0 {
				return strings.TrimSpace(text[:i])
			}
		}
	}
	return text
}

func simpleReceiver(text string) string {
	text = strings.TrimSpace(text)
	if text == "this" || text == "self" || text == "$this" || text == "static" || text == "cls" {
		return text
	}
	if tsIdentRe.MatchString(text) {
		return text
	}
	return ""
}

var tsElixirKeywords = map[string]bool{
	"def": true, "defp": true, "defmodule": true, "defmacrop": true, "defmacro": true,
	"defstruct": true, "defimpl": true, "defprotocol": true, "defdelegate": true,
	"import": true, "use": true, "alias": true, "require": true, "quote": true,
	"unquote": true, "case": true, "cond": true, "fn": true,
}
