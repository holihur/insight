package callgraph

import (
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/codeinsight/code-insight/internal/analysis"
)

// This file adds TypeScript/JavaScript support. Definitions reuse the same
// extractor (and therefore the same function IDs) as the coverage feature, so
// the two views always agree. Calls are found with a masked-source scanner:
// comments and string literals are blanked out first, then identifiers that
// look like calls are resolved through imports, same-file names and finally
// repo-wide unique names. Dynamic dispatch can only be a best effort, so
// ambiguous names are returned as candidates instead of a single guess.

var jsCallRe = regexp.MustCompile(`(?:([A-Za-z_$][\w$]*)\s*\??\.\s*)?\b([A-Za-z_$][\w$]*)\s*\(`)
var jsFromRe = regexp.MustCompile(`(?m)\bimport\s+(?:type\s+)?([^;]+?)\s+from\s+["']([^"']+)["']`)
var jsRequireRe = regexp.MustCompile(`\b(?:const|let|var)\s+([A-Za-z_$][\w$]*|\{[^}]*\})\s*=\s*require\(\s*["']([^"']+)["']\s*\)`)
var jsNamedRe = regexp.MustCompile(`([A-Za-z_$][\w$]*)(?:\s+as\s+([A-Za-z_$][\w$]*))?`)

var jsExts = map[string]bool{
	".ts": true, ".tsx": true, ".js": true, ".jsx": true, ".mjs": true, ".cjs": true,
}

var jsExtList = []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"}

var jsSkipNames = map[string]bool{
	"if": true, "for": true, "while": true, "switch": true, "catch": true,
	"function": true, "return": true, "typeof": true, "new": true, "await": true,
	"else": true, "do": true, "try": true, "finally": true, "throw": true,
	"delete": true, "void": true, "yield": true, "import": true, "export": true,
	"constructor": true, "super": true, "this": true, "class": true, "extends": true,
}

var jsBuiltinObjects = map[string]bool{
	"console": true, "JSON": true, "Math": true, "Object": true, "Array": true,
	"String": true, "Number": true, "Boolean": true, "Promise": true, "Date": true,
	"RegExp": true, "Error": true, "Map": true, "Set": true, "WeakMap": true,
	"WeakSet": true, "Reflect": true, "Intl": true, "globalThis": true,
	"window": true, "document": true, "navigator": true, "process": true,
	"localStorage": true, "sessionStorage": true, "Symbol": true, "BigInt": true,
	"Proxy": true, "Atomics": true, "structuredClone": true, "fetch": true,
}

type jsFunc struct {
	ID        string
	Name      string
	Kind      string
	StartLine int
	EndLine   int
}

type jsImport struct {
	module    string // resolved repository path, or the raw specifier
	name      string // imported binding when the alias maps to a named export
	namespace bool   // import * as ns / require
	fallback  bool   // default import
}

type jsFile struct {
	path       string
	lang       string
	content    string
	funcs      []jsFunc
	imports    map[string]jsImport
	lineStarts []int
	defs       map[int]bool // absolute offsets of definition tokens
	tsDefs     []tsDef
	tsCalls    []tsCall
	tsParsed   bool
}

type jsIndex struct {
	files       map[string]*jsFile
	byFile      map[string]map[string][]string
	byName      map[string][]string
	paths       []string
	moduleCache map[string]string
}

func newJSIndex() *jsIndex {
	return &jsIndex{
		files:       map[string]*jsFile{},
		byFile:      map[string]map[string][]string{},
		byName:      map[string][]string{},
		moduleCache: map[string]string{},
	}
}

func isJSFile(filePath string) bool {
	lower := strings.ToLower(filePath)
	if strings.HasSuffix(lower, ".d.ts") {
		return false
	}
	return jsExts[path.Ext(lower)]
}

func (idx *jsIndex) addFile(filePath, lang string, content []byte) {
	idx.files[filePath] = &jsFile{
		path: filePath, lang: lang, content: string(content), defs: map[int]bool{},
	}
}

// buildJS indexes TS/JS definitions, imports and calls.
func (b *builder) buildJS() {
	idx := b.js
	idx.paths = make([]string, 0, len(idx.files))
	for filePath := range idx.files {
		idx.paths = append(idx.paths, filePath)
	}
	sort.Strings(idx.paths)

	for _, filePath := range idx.paths {
		file := idx.files[filePath]
		file.lineStarts = lineStartsOf(file.content)
		lang := analysis.LanguageFor(filePath)
		funcs := analysis.ExtractFunctions(filePath, []byte(file.content), lang)
		var defs []tsDef
		var calls []tsCall
		parsed := true
		usedCache := false
		cacheKey := ""
		if b.parseCache != nil {
			cacheKey = b.parseCache.key(lang, []byte(file.content))
			if entry, ok := b.parseCache.get(cacheKey); ok {
				defs, calls = entry.Defs, entry.Calls
				usedCache = true
			}
		}
		if !usedCache {
			defs, calls, parsed = extractTS(filePath, lang, []byte(file.content))
			if b.parseCache != nil && parsed {
				b.parseCache.put(cacheKey, &cachedParse{Lang: lang, Defs: defs, Calls: calls})
			}
		}
		file.tsDefs, file.tsCalls, file.tsParsed = defs, calls, parsed
		if parsed && len(funcs) == 0 && len(defs) > 0 {
			// Some languages (for example Lua) are not covered by the
			// brace-based coverage extractor. Fall back to tree-sitter
			// definitions so the call graph still has nodes to attach to.
			funcs = tsFuncInfos(defs)
		}
		used := map[string]int{}
		if idx.byFile[filePath] == nil {
			idx.byFile[filePath] = map[string][]string{}
		}
		for _, fn := range funcs {
			id := analysis.FunctionID(filePath, fn.Name, used)
			node := Node{
				ID: id, Path: filePath, Name: fn.Name, Kind: fn.Kind,
				StartLine: fn.StartLine, EndLine: fn.EndLine,
			}
			b.nodes[id] = node
			file.funcs = append(file.funcs, jsFunc{ID: id, Name: fn.Name, Kind: fn.Kind, StartLine: fn.StartLine, EndLine: fn.EndLine})
			idx.byFile[filePath][fn.Name] = append(idx.byFile[filePath][fn.Name], id)
			idx.byName[fn.Name] = append(idx.byName[fn.Name], id)

			line := lineAt(file.content, file.lineStarts, fn.StartLine)
			col := strings.Index(line, fn.Name)
			if col < 0 {
				continue
			}
			offset := file.lineStarts[fn.StartLine-1] + col
			file.defs[offset] = true
			b.sites[filePath] = append(b.sites[filePath], Site{
				Path: filePath, Line: fn.StartLine, Col: col, EndCol: col + len(fn.Name),
				Token: fn.Name, Kind: "def", FunctionID: id,
				Targets: []Target{{ID: id, Path: filePath, Name: fn.Name, Line: fn.StartLine}},
			})
		}
	}

	b.jsResolveImports()
	for _, filePath := range idx.paths {
		file := idx.files[filePath]
		if !file.tsParsed {
			b.jsResolveCallsFallback(file)
			continue
		}
		b.jsResolveCalls(file, file.tsDefs, file.tsCalls)
	}
}

func tsFuncInfos(defs []tsDef) []analysis.FuncInfo {
	out := make([]analysis.FuncInfo, 0, len(defs))
	seen := map[string]bool{}
	for _, def := range defs {
		key := def.Name + "\x00" + strconv.Itoa(def.StartLine)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, analysis.FuncInfo{
			Name: def.Name, Kind: "function",
			StartLine: def.StartLine, EndLine: def.EndLine,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartLine < out[j].StartLine })
	return out
}

// jsResolveCalls attributes tree-sitter calls to the enclosing function and
// resolves their names.
func (b *builder) jsResolveCalls(file *jsFile, defs []tsDef, calls []tsCall) {
	if len(file.funcs) == 0 || len(calls) == 0 {
		return
	}
	ranges := b.matchTSDefs(file, defs)
	for _, call := range calls {
		callerID := ""
		var best *tsDef
		for i := range ranges {
			def := &ranges[i]
			if call.StartByte >= def.StartByte && call.StartByte < def.EndByte {
				if best == nil || (def.EndByte-def.StartByte) < (best.EndByte-best.StartByte) {
					best = def
				}
			}
		}
		if best != nil {
			callerID = best.ID
		} else {
			callerID = enclosingFuncID(file, call.Line)
		}
		if callerID == "" {
			continue
		}
		var targets []Target
		var confidence string
		if file.lang == "TypeScript" || file.lang == "JavaScript" {
			targets, _, confidence = b.resolveJSCall(file, call.Receiver, call.Name)
		} else {
			targets, confidence = b.resolveGenericCall(file, call.Receiver, call.Name)
		}
		if len(targets) == 0 {
			continue
		}
		b.sites[file.path] = append(b.sites[file.path], Site{
			Path: file.path, Line: call.Line, Col: call.Col, EndCol: call.EndCol,
			Token: call.Token, Kind: "call", Confidence: confidence, Targets: targets,
		})
		for _, target := range targets {
			key := callerID + "\x00" + target.ID + "\x00" + file.path + "\x00" + strconv.Itoa(call.Line)
			if b.seenEdge[key] {
				continue
			}
			b.seenEdge[key] = true
			b.edges = append(b.edges, Edge{
				CallerID: callerID, CalleeID: target.ID,
				Path: file.path, Line: call.Line, Confidence: confidence,
			})
		}
	}
}

func (b *builder) matchTSDefs(file *jsFile, defs []tsDef) []tsDef {
	byKey := make(map[string]string, len(file.funcs))
	for _, fn := range file.funcs {
		byKey[fn.Name+"\x00"+strconv.Itoa(fn.StartLine)] = fn.ID
	}
	out := make([]tsDef, 0, len(defs))
	for _, def := range defs {
		id := byKey[def.Name+"\x00"+strconv.Itoa(def.StartLine)]
		if id == "" {
			for delta := 1; delta <= 2 && id == ""; delta++ {
				id = byKey[def.Name+"\x00"+strconv.Itoa(def.StartLine-delta)]
				if id == "" {
					id = byKey[def.Name+"\x00"+strconv.Itoa(def.StartLine+delta)]
				}
			}
		}
		if id == "" {
			continue
		}
		def.ID = id
		out = append(out, def)
	}
	return out
}

func enclosingFuncID(file *jsFile, line int) string {
	for _, fn := range file.funcs {
		if line >= fn.StartLine && line <= fn.EndLine {
			return fn.ID
		}
	}
	return ""
}

// resolveGenericCall resolves calls for languages without import-aware
// analysis: self/this prefers the same file, then repo-wide unique names.
func (b *builder) resolveGenericCall(file *jsFile, receiver, name string) ([]Target, string) {
	byFile := b.js.byFile[file.path]
	pick := func(ids []string, confidence string) ([]Target, string) {
		return b.targets(ids), confidenceFor(ids, confidence)
	}
	switch receiver {
	case "this", "self", "$this":
		if ids := byFile[name]; len(ids) > 0 {
			return pick(ids, "receiver")
		}
	}
	if ids := byFile[name]; len(ids) > 0 {
		return pick(ids, "local")
	}
	if ids := b.js.byName[name]; len(ids) > 0 {
		return pick(ids, "name")
	}
	return nil, ""
}

func (b *builder) jsResolveImports() {
	for _, filePath := range b.js.paths {
		file := b.js.files[filePath]
		file.imports = parseJSImports(file.content)
		for alias, imp := range file.imports {
			if resolved, ok := b.resolveJSModule(filePath, imp.module); ok {
				imp.module = resolved
				file.imports[alias] = imp
			}
		}
	}
}

func (b *builder) jsResolveCallsFallback(file *jsFile) {
	if len(file.funcs) == 0 {
		return
	}
	masked := maskJS(file.content)
	matches := jsCallRe.FindAllStringSubmatchIndex(masked, -1)
	if len(matches) == 0 {
		return
	}
	for _, fn := range file.funcs {
		startLine, endLine := fn.StartLine, fn.EndLine
		if startLine < 1 || endLine > len(file.lineStarts) {
			continue
		}
		start := file.lineStarts[startLine-1]
		end := len(masked)
		if endLine < len(file.lineStarts) {
			end = file.lineStarts[endLine]
		}
		for _, match := range matches {
			nameStart, nameEnd := match[4], match[5]
			if nameStart < start || nameStart >= end {
				continue
			}
			if file.defs[nameStart] {
				continue
			}
			if match[2] < 0 && nameStart > 0 && masked[nameStart-1] == '.' {
				// chained call such as a.b.c(): the carrier type is unknown
				continue
			}
			name := masked[nameStart:nameEnd]
			if jsSkipNames[name] {
				continue
			}
			if prevWord(masked, nameStart) == "new" {
				continue
			}
			qualifier := ""
			if match[2] >= 0 {
				qualifier = masked[match[2]:match[3]]
			}
			targets, token, confidence := b.resolveJSCall(file, qualifier, name)
			if len(targets) == 0 {
				continue
			}
			line, col := lineColAt(file.lineStarts, nameStart)
			b.sites[file.path] = append(b.sites[file.path], Site{
				Path: file.path, Line: line, Col: col, EndCol: col + len(token),
				Token: token, Kind: "call", Confidence: confidence, Targets: targets,
			})
			for _, target := range targets {
				key := fn.ID + "\x00" + target.ID + "\x00" + file.path + "\x00" + strconv.Itoa(line)
				if b.seenEdge[key] {
					continue
				}
				b.seenEdge[key] = true
				b.edges = append(b.edges, Edge{
					CallerID: fn.ID, CalleeID: target.ID,
					Path: file.path, Line: line, Confidence: confidence,
				})
			}
		}
	}
}

func (b *builder) resolveJSCall(file *jsFile, qualifier, name string) ([]Target, string, string) {
	display := name
	if qualifier != "" {
		display = qualifier + "." + name
	}
	byFile := b.js.byFile[file.path]
	pick := func(ids []string, confidence string) ([]Target, string, string) {
		return b.targets(ids), display, confidenceFor(ids, confidence)
	}

	if qualifier == "" {
		if imp, ok := file.imports[name]; ok && imp.name != "" {
			if ids := b.importedFuncs(imp); len(ids) > 0 {
				return pick(ids, "exact")
			}
		}
		if ids := byFile[name]; len(ids) > 0 {
			return pick(ids, "local")
		}
		if ids := b.js.byName[name]; len(ids) > 0 {
			return pick(ids, "name")
		}
		return nil, "", ""
	}
	if qualifier == "this" {
		if ids := byFile[name]; len(ids) > 0 {
			return pick(ids, "receiver")
		}
		if ids := b.js.byName[name]; len(ids) > 0 {
			return pick(ids, "name")
		}
		return nil, "", ""
	}
	if jsBuiltinObjects[qualifier] {
		return nil, "", ""
	}
	if imp, ok := file.imports[qualifier]; ok {
		// The qualifier is an imported binding (namespace, default or named
		// object). Only resolve inside that module; guessing by bare name here
		// produces wrong edges such as api.regenerate -> local regenerate.
		if ids := b.importedFuncs(imp); len(ids) > 0 {
			return pick(ids, "exact")
		}
		if imp.module != "" {
			if ids := b.js.byFile[imp.module][name]; len(ids) > 0 {
				return pick(ids, "exact")
			}
		}
		return nil, "", ""
	}
	if ids := byFile[name]; len(ids) > 0 {
		return pick(ids, "local")
	}
	if ids := b.js.byName[name]; len(ids) > 0 {
		return pick(ids, "name")
	}
	return nil, "", ""
}

func (b *builder) importedFuncs(imp jsImport) []string {
	if imp.module == "" || imp.name == "" {
		return nil
	}
	return b.js.byFile[imp.module][imp.name]
}

func parseJSImports(content string) map[string]jsImport {
	out := map[string]jsImport{}
	for _, match := range jsFromRe.FindAllStringSubmatch(content, -1) {
		parseImportClause(strings.TrimSpace(match[1]), match[2], out)
	}
	for _, match := range jsRequireRe.FindAllStringSubmatch(content, -1) {
		binding := strings.TrimSpace(match[1])
		module := match[2]
		if strings.HasPrefix(binding, "*") {
			continue
		}
		if strings.HasPrefix(binding, "{") {
			parseRequireDestructure(binding, module, out)
			continue
		}
		out[binding] = jsImport{module: module, namespace: true}
	}
	return out
}

func parseImportClause(clause, module string, out map[string]jsImport) {
	clause = strings.TrimSpace(strings.TrimPrefix(clause, "type "))
	switch {
	case strings.HasPrefix(clause, "{"):
		addNamedImports(clause, module, out, "as")
	case strings.HasPrefix(clause, "*"):
		alias := lastWord(clause)
		if alias != "" && alias != "*" {
			out[alias] = jsImport{module: module, namespace: true}
		}
	default:
		if comma := strings.Index(clause, ","); comma >= 0 {
			def := strings.TrimSpace(clause[:comma])
			if def != "" {
				out[def] = jsImport{module: module, fallback: true}
			}
			rest := strings.TrimSpace(clause[comma+1:])
			if strings.HasPrefix(rest, "{") {
				addNamedImports(rest, module, out, "as")
			} else if strings.HasPrefix(rest, "*") {
				if alias := lastWord(rest); alias != "" && alias != "*" {
					out[alias] = jsImport{module: module, namespace: true}
				}
			}
			return
		}
		first := strings.Fields(clause)
		if len(first) > 0 {
			out[first[0]] = jsImport{module: module, fallback: true}
		}
	}
}

func addNamedImports(block, module string, out map[string]jsImport, separator string) {
	block = strings.TrimSpace(block)
	block = strings.TrimPrefix(block, "{")
	if idx := strings.Index(block, "}"); idx >= 0 {
		block = block[:idx]
	}
	for _, item := range strings.Split(block, ",") {
		item = strings.TrimSpace(item)
		if item == "" || strings.HasPrefix(item, "type ") {
			continue
		}
		item = strings.TrimPrefix(item, "type ")
		if separator == "as" {
			for _, match := range jsNamedRe.FindAllStringSubmatch(item, -1) {
				name, alias := match[1], match[2]
				if alias == "" {
					alias = name
				}
				out[alias] = jsImport{module: module, name: name}
			}
		}
	}
}

func parseRequireDestructure(block, module string, out map[string]jsImport) {
	block = strings.TrimPrefix(strings.TrimSpace(block), "{")
	if idx := strings.Index(block, "}"); idx >= 0 {
		block = block[:idx]
	}
	for _, item := range strings.Split(block, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		name, alias := item, item
		if idx := strings.Index(item, ":"); idx >= 0 {
			name = strings.TrimSpace(item[:idx])
			alias = strings.TrimSpace(item[idx+1:])
		}
		if name == "" || alias == "" {
			continue
		}
		out[alias] = jsImport{module: module, name: name}
	}
}

func (b *builder) resolveJSModule(fromPath, module string) (string, bool) {
	if module == "" {
		return "", false
	}
	cacheKey := fromPath + "\x00" + module
	if cached, ok := b.js.moduleCache[cacheKey]; ok {
		return cached, cached != ""
	}
	resolved := ""
	if strings.HasPrefix(module, ".") {
		base := path.Clean(path.Join(path.Dir(fromPath), module))
		if found, ok := b.findJSFile(base); ok {
			resolved = found
		}
	} else {
		resolved = b.findJSBySuffix(module)
	}
	b.js.moduleCache[cacheKey] = resolved
	return resolved, resolved != ""
}

func (b *builder) findJSFile(base string) (string, bool) {
	candidates := []string{base}
	for _, ext := range jsExtList {
		candidates = append(candidates, base+ext, base+"/index"+ext)
	}
	for _, candidate := range candidates {
		if _, ok := b.js.files[candidate]; ok {
			return candidate, true
		}
	}
	return "", false
}

func (b *builder) findJSBySuffix(module string) string {
	base := strings.TrimPrefix(module, "/")
	variant := base
	if strings.HasPrefix(variant, "@") || strings.HasPrefix(variant, "~") {
		variant = variant[1:]
	}
	variant = strings.TrimPrefix(variant, "/")
	variants := []string{variant}
	if strings.HasPrefix(variant, "src/") {
		variants = append(variants, strings.TrimPrefix(variant, "src/"))
	} else if !strings.Contains(variant, "/") {
		variants = append(variants, "src/"+variant)
	}
	best := ""
	for _, pathCandidate := range b.js.paths {
		for _, v := range variants {
			for _, ext := range jsExtList {
				suffix := v + ext
				if pathCandidate == suffix || strings.HasSuffix(pathCandidate, "/"+suffix) {
					if best == "" || len(pathCandidate) < len(best) {
						best = pathCandidate
					}
				}
				indexSuffix := v + "/index" + ext
				if pathCandidate == indexSuffix || strings.HasSuffix(pathCandidate, "/"+indexSuffix) {
					if best == "" || len(pathCandidate) < len(best) {
						best = pathCandidate
					}
				}
			}
		}
	}
	return best
}

func lineStartsOf(content string) []int {
	starts := []int{0}
	for i := 0; i < len(content); i++ {
		if content[i] == '\n' {
			starts = append(starts, i+1)
		}
	}
	return starts
}

func lineAt(content string, starts []int, line int) string {
	if line < 1 || line > len(starts) {
		return ""
	}
	start := starts[line-1]
	end := len(content)
	if line < len(starts) {
		end = starts[line] - 1
	}
	if end < start {
		return ""
	}
	return content[start:end]
}

func lineColAt(starts []int, offset int) (int, int) {
	idx := sort.Search(len(starts), func(i int) bool { return starts[i] > offset }) - 1
	if idx < 0 {
		idx = 0
	}
	return idx + 1, offset - starts[idx]
}

func prevWord(src string, offset int) string {
	end := offset
	for end > 0 && !isIdentByte(src[end-1]) {
		end--
		if offset-end > 2 {
			break
		}
	}
	start := end
	for start > 0 && isIdentByte(src[start-1]) {
		start--
	}
	return src[start:end]
}

func lastWord(src string) string {
	fields := strings.Fields(src)
	if len(fields) == 0 {
		return ""
	}
	word := fields[len(fields)-1]
	return strings.Trim(word, ";{}")
}

func isIdentByte(c byte) bool {
	return c == '_' || c == '$' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// maskJS blanks comments and string/template literals while preserving length
// and newlines, so regex offsets still map to the original source.
func maskJS(src string) string {
	out := []byte(src)
	const (
		normal = iota
		lineComment
		blockComment
		single
		double
		template
	)
	state := normal
	blank := func(i int) {
		if out[i] != '\n' {
			out[i] = ' '
		}
	}
	for i := 0; i < len(out); i++ {
		c := out[i]
		switch state {
		case normal:
			switch {
			case c == '/' && i+1 < len(out) && out[i+1] == '/':
				blank(i)
				i++
				blank(i)
				state = lineComment
			case c == '/' && i+1 < len(out) && out[i+1] == '*':
				blank(i)
				i++
				blank(i)
				state = blockComment
			case c == '\'':
				blank(i)
				state = single
			case c == '"':
				blank(i)
				state = double
			case c == '`':
				blank(i)
				state = template
			}
		case lineComment:
			if c == '\n' {
				state = normal
				continue
			}
			blank(i)
		case blockComment:
			if c == '*' && i+1 < len(out) && out[i+1] == '/' {
				blank(i)
				i++
				blank(i)
				state = normal
				continue
			}
			blank(i)
		case single, double, template:
			if c == '\\' {
				blank(i)
				if i+1 < len(out) {
					i++
					blank(i)
				}
				continue
			}
			quote := byte('\'')
			if state == double {
				quote = '"'
			} else if state == template {
				quote = '`'
			}
			if c == quote {
				blank(i)
				state = normal
				continue
			}
			blank(i)
		}
	}
	return string(out)
}
