package analysis

import (
	"crypto/sha1"
	"encoding/hex"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"regexp"
	"strings"
)

// FuncInfo is a function or method discovered in one source file.
type FuncInfo struct {
	Name      string
	Kind      string
	StartLine int
	EndLine   int
	BodyHash  string
}

type funcPattern struct {
	re   *regexp.Regexp
	kind string
}

var (
	pythonDef   = regexp.MustCompile(`^(\s*)(?:async\s+)?def\s+([A-Za-z_]\w*)\s*\(`)
	jsFunction  = regexp.MustCompile(`^\s*(?:export\s+)?(?:default\s+)?(?:async\s+)?function\s+([A-Za-z_$][\w$]*)\s*\(`)
	jsArrow     = regexp.MustCompile(`^\s*(?:export\s+)?(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*(?::[^=]+)?=\s*(?:async\s*)?(?:\([^)]*\)|[A-Za-z_$][\w$]*)\s*=>`)
	jsMethod    = regexp.MustCompile(`^\s*(?:async\s+)?([A-Za-z_$][\w$]*)\s*\([^;]*\)\s*\{`)
	rustFn      = regexp.MustCompile(`^\s*(?:pub(?:\([^)]*\))?\s+)?(?:async\s+)?(?:unsafe\s+)?(?:const\s+)?fn\s+([A-Za-z_]\w*)`)
	phpFn       = regexp.MustCompile(`^\s*(?:(?:public|private|protected|static|final|abstract)\s+)*function\s+([A-Za-z_]\w*)\s*\(`)
	rubyDef     = regexp.MustCompile(`^\s*def\s+(?:self\.)?([A-Za-z_]\w*[?!]?)`)
	kotlinFun   = regexp.MustCompile(`^\s*(?:(?:public|private|protected|internal|open|override|suspend|inline|operator|tailrec|external|expect|actual)\s+)*fun\s+(?:<[^>]+>\s*)?([A-Za-z_]\w*)`)
	swiftFunc   = regexp.MustCompile(`^\s*(?:(?:public|private|internal|fileprivate|open|static|class|override|mutating|nonmutating|async)\s+)*func\s+([A-Za-z_]\w*)`)
	scalaDef    = regexp.MustCompile(`^\s*(?:(?:private|protected|override|final|implicit|inline)\s+)*def\s+([A-Za-z_]\w*)`)
	elixirDef   = regexp.MustCompile(`^\s*defp?\s+([A-Za-z_]\w*[?!]?)`)
	luaFunc     = regexp.MustCompile(`^\s*(?:local\s+)?function\s+([A-Za-z_][\w.:]*)`)
	genericFn   = regexp.MustCompile(`^\s*(?:(?:public|private|protected|internal|static|final|abstract|synchronized|native|async|override|open|suspend|operator|inline|external|unsafe|mutating|nonmutating|partial|virtual|explicit)\s+)*(?:[A-Za-z_][\w<>\[\],.?&*:]*\s+)+([A-Za-z_]\w*)\s*\([^;]*\)\s*(?::\s*[^{;]+)?(?:throws\s+[^{]+)?\{`)
	constructor = regexp.MustCompile(`^\s*(?:public|private|protected|internal)\s+([A-Z][A-Za-z0-9_]*)\s*\([^;]*\)\s*\{`)
	jsKeywords  = map[string]bool{
		"if": true, "for": true, "while": true, "switch": true, "catch": true,
		"function": true, "return": true, "with": true, "else": true, "do": true,
		"try": true, "finally": true, "new": true, "typeof": true, "await": true,
	}
)

// ExtractFunctions parses a source file and returns its functions/methods.
func ExtractFunctions(filePath string, content []byte, language string) []FuncInfo {
	if language == "" {
		language = LanguageFor(filePath)
	}
	switch language {
	case "Go":
		if funcs := extractGoFunctions(filePath, content); len(funcs) > 0 {
			return funcs
		}
	case "Python":
		return extractIndented(content, pythonDef, "function", 2)
	case "Ruby":
		return extractRuby(content)
	case "Elixir":
		return extractIndented(content, elixirDef, "function", 1)
	case "JavaScript", "TypeScript":
		return extractBraceLanguage(content, []funcPattern{
			{jsFunction, "function"},
			{jsArrow, "function"},
			{jsMethod, "method"},
		})
	case "Rust":
		return extractBraceLanguage(content, []funcPattern{{rustFn, "function"}})
	case "PHP":
		return extractBraceLanguage(content, []funcPattern{{phpFn, "method"}})
	case "Kotlin":
		return extractBraceLanguage(content, []funcPattern{{kotlinFun, "function"}})
	case "Swift":
		return extractBraceLanguage(content, []funcPattern{{swiftFunc, "function"}})
	case "Scala":
		return extractBraceLanguage(content, []funcPattern{{scalaDef, "method"}})
	case "Lua":
		return extractBraceLanguage(content, []funcPattern{{luaFunc, "function"}})
	case "Java", "C", "C++", "C#", "Dart":
		return extractBraceLanguage(content, []funcPattern{
			{genericFn, "method"},
			{constructor, "method"},
		})
	}
	return nil
}

func extractGoFunctions(filePath string, content []byte) []FuncInfo {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, content, parser.SkipObjectResolution)
	if err != nil {
		return nil
	}
	lines := splitLines(content)
	var out []FuncInfo
	ast.Inspect(file, func(node ast.Node) bool {
		fn, ok := node.(*ast.FuncDecl)
		if !ok {
			return true
		}
		name := fn.Name.Name
		kind := "function"
		if fn.Recv != nil && len(fn.Recv.List) > 0 {
			recv := receiverName(fn.Recv.List[0].Type)
			if recv != "" {
				name = recv + "." + name
				kind = "method"
			}
		}
		start := fset.Position(fn.Pos()).Line
		end := fset.Position(fn.End()).Line
		out = append(out, FuncInfo{
			Name: name, Kind: kind, StartLine: start, EndLine: end,
			BodyHash: hashLines(lines, start, end),
		})
		return true
	})
	return out
}

func receiverName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return receiverName(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr:
		return receiverName(t.X)
	case *ast.IndexListExpr:
		return receiverName(t.X)
	case *ast.SelectorExpr:
		return t.Sel.Name
	}
	return ""
}

func extractIndented(content []byte, re *regexp.Regexp, kind string, nameGroup int) []FuncInfo {
	lines := splitLines(content)
	var out []FuncInfo
	for i, line := range lines {
		m := re.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		name := ""
		if nameGroup < len(m) {
			name = m[nameGroup]
		}
		if name == "" {
			continue
		}
		baseIndent := indentWidth(line)
		end := len(lines)
		for j := i + 1; j < len(lines); j++ {
			trimmed := strings.TrimSpace(lines[j])
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			if indentWidth(lines[j]) <= baseIndent {
				end = j
				break
			}
		}
		for end > i+1 && strings.TrimSpace(lines[end-1]) == "" {
			end--
		}
		out = append(out, FuncInfo{
			Name: name, Kind: kind, StartLine: i + 1, EndLine: end,
			BodyHash: hashLines(lines, i+1, end),
		})
	}
	return out
}

func extractRuby(content []byte) []FuncInfo {
	lines := splitLines(content)
	var out []FuncInfo
	for i, line := range lines {
		m := rubyDef.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		baseIndent := indentWidth(line)
		end := len(lines)
		for j := i + 1; j < len(lines); j++ {
			trimmed := strings.TrimSpace(lines[j])
			if trimmed == "end" && indentWidth(lines[j]) <= baseIndent {
				end = j + 1
				break
			}
			if strings.HasPrefix(trimmed, "def ") && indentWidth(lines[j]) <= baseIndent {
				end = j
				break
			}
		}
		out = append(out, FuncInfo{
			Name: m[1], Kind: "method", StartLine: i + 1, EndLine: end,
			BodyHash: hashLines(lines, i+1, end),
		})
	}
	return out
}

func extractBraceLanguage(content []byte, patterns []funcPattern) []FuncInfo {
	lines := splitLines(content)
	var out []FuncInfo
	for i, line := range lines {
		for _, p := range patterns {
			m := p.re.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			name := m[1]
			if name == "" || jsKeywords[name] {
				continue
			}
			end := findBraceEnd(lines, i)
			if end <= i {
				continue
			}
			out = append(out, FuncInfo{
				Name: name, Kind: p.kind, StartLine: i + 1, EndLine: end,
				BodyHash: hashLines(lines, i+1, end),
			})
			break
		}
	}
	return out
}

func findBraceEnd(lines []string, start int) int {
	limit := start + 60
	if limit > len(lines) {
		limit = len(lines)
	}
	inString := byte(0)
	escaped := false
	inBlockComment := false
	parenDepth := 0
	bodyLine, bodyCol := -1, -1

	// Locate the body brace: the first "{" outside parentheses, so object
	// destructuring in parameters (function f({ a, b })) does not end the body.
	for i := start; i < limit && bodyLine < 0; i++ {
		line := lines[i]
		for j := 0; j < len(line); j++ {
			c := line[j]
			if inBlockComment {
				if c == '*' && j+1 < len(line) && line[j+1] == '/' {
					inBlockComment = false
					j++
				}
				continue
			}
			if inString != 0 {
				if escaped {
					escaped = false
					continue
				}
				if c == '\\' {
					escaped = true
					continue
				}
				if c == inString {
					inString = 0
				}
				continue
			}
			switch c {
			case '/':
				if j+1 < len(line) && line[j+1] == '/' {
					j = len(line)
					continue
				}
				if j+1 < len(line) && line[j+1] == '*' {
					inBlockComment = true
					j++
				}
			case '"', '\'', '`':
				inString = c
			case '(':
				parenDepth++
			case ')':
				if parenDepth > 0 {
					parenDepth--
				}
			case '{':
				if parenDepth == 0 {
					bodyLine, bodyCol = i, j
					j = len(line)
				}
			case '=':
				// Expression-bodied arrow at the top level of the signature:
				// "const f = (a) => value". Arrows inside a parameter list
				// (type annotations, default values) must not end the body.
				if parenDepth == 0 && j+1 < len(line) && line[j+1] == '>' && !strings.Contains(line[j+2:], "{") {
					return i + 1
				}
			}
		}
		if inString != '`' {
			inString = 0
		}
		escaped = false
	}
	if bodyLine < 0 {
		return 0
	}

	depth := 0
	for i := bodyLine; i < len(lines); i++ {
		line := lines[i]
		startCol := 0
		if i == bodyLine {
			startCol = bodyCol
		}
		for j := startCol; j < len(line); j++ {
			c := line[j]
			if inBlockComment {
				if c == '*' && j+1 < len(line) && line[j+1] == '/' {
					inBlockComment = false
					j++
				}
				continue
			}
			if inString != 0 {
				if escaped {
					escaped = false
					continue
				}
				if c == '\\' {
					escaped = true
					continue
				}
				if c == inString {
					inString = 0
				}
				continue
			}
			switch c {
			case '/':
				if j+1 < len(line) && line[j+1] == '/' {
					j = len(line)
					continue
				}
				if j+1 < len(line) && line[j+1] == '*' {
					inBlockComment = true
					j++
				}
			case '"', '\'', '`':
				inString = c
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					return i + 1
				}
			}
		}
		if inString != '`' {
			inString = 0
		}
		escaped = false
	}
	return len(lines)
}

func hashLines(lines []string, start, end int) string {
	h := sha1.New()
	for i := start - 1; i < end && i < len(lines); i++ {
		_, _ = h.Write([]byte(strings.TrimSpace(lines[i])))
		_, _ = h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func splitLines(content []byte) []string {
	text := strings.ReplaceAll(string(content), "\r\n", "\n")
	if strings.HasSuffix(text, "\n") {
		text = strings.TrimSuffix(text, "\n")
	}
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

func indentWidth(line string) int {
	width := 0
	for _, r := range line {
		if r == ' ' {
			width++
		} else if r == '\t' {
			width += 4
		} else {
			break
		}
	}
	return width
}

// IsSourceFile reports whether a file should be parsed for functions.
func IsSourceFile(filePath string) bool {
	lang := LanguageFor(filePath)
	if lang == "" {
		return false
	}
	switch lang {
	case "Markdown", "JSON", "YAML", "TOML", "XML", "INI", "CSV", "HTML", "CSS",
		"SCSS", "Less", "SQL", "GraphQL", "Protocol Buffers", "Text":
		return false
	}
	return true
}

func FunctionID(filePath, name string, used map[string]int) string {
	base := path.Clean(filePath) + "::" + name
	n := used[base]
	used[base] = n + 1
	if n == 0 {
		return base
	}
	return base + "#" + strings.Repeat("2", n)
}
