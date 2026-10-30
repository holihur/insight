// Package callgraph indexes function definitions and call sites per commit.
//
// The current implementation understands Go precisely (go/ast) and is
// deliberately conservative: a call site is only returned when it can be
// resolved to at least one repository function.
package callgraph

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/codeinsight/code-insight/internal/analysis"
	"github.com/codeinsight/code-insight/internal/coverage"
)

// Version is bumped whenever the on-disk graph layout changes.
const Version = 1

// Node is a function or method definition.
type Node struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	Name      string `json:"name"`
	Kind      string `json:"kind"` // function | method
	StartLine int    `json:"startLine"`
	EndLine   int    `json:"endLine"`
	Receiver  string `json:"receiver,omitempty"`
}

// Target is a lightweight reference to a node, cheap to embed in a site.
type Target struct {
	ID   string `json:"id"`
	Path string `json:"path"`
	Name string `json:"name"`
	Line int    `json:"line"`
}

// Site is a clickable token in a source file: a call or a definition.
type Site struct {
	Path       string   `json:"path"`
	Line       int      `json:"line"`
	Col        int      `json:"col"`    // 0-based byte offset within the line
	EndCol     int      `json:"endCol"` // exclusive
	Token      string   `json:"token"`
	Kind       string   `json:"kind"` // call | def
	Confidence string   `json:"confidence,omitempty"`
	FunctionID string   `json:"functionId,omitempty"` // for kind=def
	Targets    []Target `json:"targets,omitempty"`
}

// Edge links a caller to a callee at a specific source position.
type Edge struct {
	CallerID   string `json:"callerId"`
	CalleeID   string `json:"calleeId"`
	Path       string `json:"path"`
	Line       int    `json:"line"`
	Confidence string `json:"confidence"`
}

// Graph is the per-commit index persisted on disk.
type Graph struct {
	Version     int               `json:"version"`
	Commit      string            `json:"commit"`
	Nodes       map[string]Node   `json:"nodes"`
	Edges       []Edge            `json:"edges"`
	Sites       map[string][]Site `json:"sites"`
	GoFiles     int               `json:"goFiles"`
	Files       int               `json:"files"`
	ReusedFiles int               `json:"reusedFiles,omitempty"`
	ParsedFiles int               `json:"parsedFiles,omitempty"`
	Truncated   bool              `json:"truncated"`
	BuiltAt     time.Time         `json:"builtAt"`

	callees map[string][]int
	callers map[string][]int
}

// Result is returned when looking up one function.
type Result struct {
	Function        Node     `json:"function"`
	Callees         []Link   `json:"callees"`
	Callers         []Link   `json:"callers"`
	Implementations []Target `json:"implementations"`
	Commit          string   `json:"commit"`
	Indexed         bool     `json:"indexed"`
	GoFiles         int      `json:"goFiles"`
	Files           int      `json:"files"`
	ReusedFiles     int      `json:"reusedFiles,omitempty"`
	Truncated       bool     `json:"truncated"`
}

// Link is a resolved edge endpoint with display information.
type Link struct {
	Node       Node   `json:"node"`
	Path       string `json:"path"`
	Line       int    `json:"line"`
	Confidence string `json:"confidence,omitempty"`
}

// SitesResult is the clickable tokens for one file.
type SitesResult struct {
	Path        string `json:"path"`
	Commit      string `json:"commit"`
	Sites       []Site `json:"sites"`
	Indexed     bool   `json:"indexed"`
	GoFiles     int    `json:"goFiles"`
	Files       int    `json:"files"`
	ReusedFiles int    `json:"reusedFiles,omitempty"`
	Truncated   bool   `json:"truncated"`
}

type fileInfo struct {
	path    string
	dir     string
	pkg     string
	content []byte
	imports map[string]string
}

type builder struct {
	root       string
	fset       *token.FileSet
	files      map[string]*fileInfo
	types      map[string]map[string]bool
	fields     map[string]map[string]string // qualified struct type -> field -> qualified type
	funcs      map[string]map[string][]string
	methods    map[string][]string
	methodRec  map[string][]string
	nodes      map[string]Node
	sites      map[string][]Site
	edges      []Edge
	seenEdge   map[string]bool
	declIDs    map[string]string
	modules    map[string]moduleInfo
	js         *jsIndex
	fileCount  int
	parseCache *parseCache
	truncated  bool
	maxFiles   int
}

// Build indexes a repository checkout at root for the given commit. An
// optional parse cache makes re-indexing incremental: unchanged files reuse
// their tree-sitter results instead of being parsed again.
func Build(commit, root string, ignore []string, maxFiles int, caches ...*parseCache) (*Graph, error) {
	if maxFiles <= 0 {
		maxFiles = 8000
	}
	ig := coverage.NewIgnorer(root, ignore)
	b := &builder{
		root:      root,
		fset:      token.NewFileSet(),
		files:     map[string]*fileInfo{},
		types:     map[string]map[string]bool{},
		fields:    map[string]map[string]string{},
		funcs:     map[string]map[string][]string{},
		methods:   map[string][]string{},
		methodRec: map[string][]string{},
		nodes:     map[string]Node{},
		sites:     map[string][]Site{},
		seenEdge:  map[string]bool{},
		declIDs:   map[string]string{},
		modules:   map[string]moduleInfo{},
		js:        newJSIndex(),
		maxFiles:  maxFiles,
	}
	if len(caches) > 0 && caches[0] != nil {
		b.parseCache = caches[0]
		b.parseCache.beginBuild()
	}
	if err := b.collect(ig); err != nil {
		return nil, err
	}
	b.resolveAll()
	b.buildJS()
	g := &Graph{
		Version:   Version,
		Commit:    commit,
		Nodes:     b.nodes,
		Edges:     b.edges,
		Sites:     b.sites,
		GoFiles:   len(b.files),
		Files:     len(b.files) + len(b.js.files),
		Truncated: b.truncated,
		BuiltAt:   time.Now().UTC(),
	}
	if b.parseCache != nil {
		g.ReusedFiles, g.ParsedFiles = b.parseCache.stats()
	}
	g.buildIndexes()
	return g, nil
}

func (b *builder) collect(ig interface{ Ignored(string) bool }) error {
	return filepath.WalkDir(b.root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p != b.root && analysis.ShouldSkipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		rel, relErr := filepath.Rel(b.root, p)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if ig.Ignored(rel) {
			return nil
		}
		if strings.HasSuffix(rel, ".go") {
			return b.collectGoFile(p, rel, d, ig)
		}
		lang := analysis.LanguageFor(rel)
		if lang == "" || !tsSupported(rel, lang) {
			return nil
		}
		if b.fileCount >= b.maxFiles {
			b.truncated = true
			return filepath.SkipAll
		}
		info, statErr := d.Info()
		if statErr != nil || info.Size() > 2*1024*1024 {
			return nil
		}
		content, readErr := os.ReadFile(p)
		if readErr != nil {
			return nil
		}
		b.js.addFile(rel, lang, content)
		b.fileCount++
		return nil
	})
}

func (b *builder) collectGoFile(p, rel string, d fs.DirEntry, ig interface{ Ignored(string) bool }) error {
	if b.fileCount >= b.maxFiles {
		b.truncated = true
		return filepath.SkipAll
	}
	info, statErr := d.Info()
	if statErr != nil || info.Size() > 2*1024*1024 {
		return nil
	}
	content, readErr := os.ReadFile(p)
	if readErr != nil {
		return nil
	}
	file, parseErr := parser.ParseFile(b.fset, rel, content, parser.SkipObjectResolution)
	if parseErr != nil || file.Name == nil {
		return nil
	}
	dir := strings.TrimSuffix(path.Dir(rel), ".")
	fi := &fileInfo{
		path:    rel,
		dir:     dir,
		pkg:     file.Name.Name,
		content: content,
		imports: map[string]string{},
	}
	for _, spec := range file.Imports {
		if spec.Path == nil {
			continue
		}
		importPath := strings.Trim(spec.Path.Value, `"`)
		alias := ""
		if spec.Name != nil {
			alias = spec.Name.Name
		} else {
			alias = path.Base(importPath)
		}
		if alias == "_" || alias == "." || alias == "" {
			continue
		}
		fi.imports[alias] = importPath
	}
	b.files[rel] = fi
	b.fileCount++

	if b.types[dir] == nil {
		b.types[dir] = map[string]bool{}
	}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			b.types[dir][ts.Name.Name] = true
			structType, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}
			qualified := dir + "::" + ts.Name.Name
			if b.fields[qualified] == nil {
				b.fields[qualified] = map[string]string{}
			}
			for _, field := range structType.Fields.List {
				typ := b.qualifyType(fi, field.Type)
				if typ == "" {
					continue
				}
				if len(field.Names) == 0 {
					b.fields[qualified][qualifyBare(typ)] = typ
					continue
				}
				for _, name := range field.Names {
					b.fields[qualified][name.Name] = typ
				}
			}
		}
	}
	b.collectDecls(fi, file)
	return nil
}
func (b *builder) collectDecls(fi *fileInfo, file *ast.File) {
	used := map[string]int{}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name == nil {
			continue
		}
		name := fn.Name.Name
		kind := "function"
		recv := ""
		if fn.Recv != nil && len(fn.Recv.List) > 0 {
			recv = receiverType(fn.Recv.List[0].Type)
			if recv != "" {
				name = recv + "." + name
				kind = "method"
			}
		}
		id := analysis.FunctionID(fi.path, name, used)
		start := b.fset.Position(fn.Pos())
		end := b.fset.Position(fn.End())
		node := Node{
			ID: id, Path: fi.path, Name: name, Kind: kind,
			StartLine: start.Line, EndLine: end.Line, Receiver: recv,
		}
		b.nodes[id] = node
		b.declIDs[fi.path+"\x00"+strconv.Itoa(start.Line)] = id

		if kind == "method" {
			b.methods[fn.Name.Name] = append(b.methods[fn.Name.Name], id)
			if recv != "" {
				key := fi.dir + "::" + recv + "." + fn.Name.Name
				b.methodRec[key] = append(b.methodRec[key], id)
			}
		} else {
			if b.funcs[fi.dir] == nil {
				b.funcs[fi.dir] = map[string][]string{}
			}
			b.funcs[fi.dir][fn.Name.Name] = append(b.funcs[fi.dir][fn.Name.Name], id)
		}

		namePos := b.fset.Position(fn.Name.Pos())
		b.sites[fi.path] = append(b.sites[fi.path], Site{
			Path: fi.path, Line: namePos.Line,
			Col: namePos.Column - 1, EndCol: namePos.Column - 1 + len(fn.Name.Name),
			Token: fn.Name.Name, Kind: "def", FunctionID: id,
			Targets: []Target{{ID: id, Path: fi.path, Name: name, Line: start.Line}},
		})
	}
}

func (b *builder) resolveAll() {
	for _, fi := range b.files {
		file, err := parser.ParseFile(b.fset, fi.path, fi.content, parser.SkipObjectResolution)
		if err != nil {
			continue
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			callerID := b.lookupID(fi, fn)
			if callerID == "" {
				continue
			}
			env := b.typeEnv(fi, fn)
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				b.recordCall(fi, callerID, env, call)
				return true
			})
		}
	}
}

func (b *builder) lookupID(fi *fileInfo, fn *ast.FuncDecl) string {
	return b.declIDs[fi.path+"\x00"+strconv.Itoa(b.fset.Position(fn.Pos()).Line)]
}

var builtinFuncs = map[string]bool{
	"append": true, "cap": true, "clear": true, "close": true, "complex": true,
	"copy": true, "delete": true, "imag": true, "len": true, "make": true,
	"max": true, "min": true, "new": true, "panic": true, "print": true,
	"println": true, "real": true, "recover": true,
}

var builtinTypes = map[string]bool{
	"any": true, "bool": true, "byte": true, "complex64": true, "complex128": true,
	"error": true, "float32": true, "float64": true, "int": true, "int8": true,
	"int16": true, "int32": true, "int64": true, "rune": true, "string": true,
	"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
	"uintptr": true,
}

func (b *builder) typeEnv(fi *fileInfo, fn *ast.FuncDecl) map[string]string {
	env := map[string]string{}
	addField := func(field *ast.Field) {
		typ := b.qualifyType(fi, field.Type)
		if typ == "" {
			return
		}
		for _, name := range field.Names {
			env[name.Name] = typ
		}
	}
	if fn.Recv != nil {
		for _, field := range fn.Recv.List {
			addField(field)
		}
	}
	if fn.Type != nil && fn.Type.Params != nil {
		for _, field := range fn.Type.Params.List {
			addField(field)
		}
	}
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.AssignStmt:
			if n.Tok != token.DEFINE {
				return true
			}
			for i, lhs := range n.Lhs {
				if i >= len(n.Rhs) {
					break
				}
				ident, ok := lhs.(*ast.Ident)
				if !ok {
					continue
				}
				if typ := b.qualifyType(fi, exprTypeExpr(n.Rhs[i])); typ != "" {
					env[ident.Name] = typ
				}
			}
		case *ast.ValueSpec:
			typ := b.qualifyType(fi, n.Type)
			if typ != "" {
				for _, name := range n.Names {
					env[name.Name] = typ
				}
			}
		}
		return true
	})
	return env
}

// exprTypeExpr unwraps composite literals so the declared type can be resolved.
func exprTypeExpr(expr ast.Expr) ast.Expr {
	switch e := expr.(type) {
	case *ast.UnaryExpr:
		if e.Op == token.AND {
			return exprTypeExpr(e.X)
		}
	case *ast.CompositeLit:
		return e.Type
	case *ast.ParenExpr:
		return exprTypeExpr(e.X)
	}
	return nil
}

// qualifyType resolves an AST type expression to "<dir>::<Type>" so method
// lookups cannot collide between packages that both define a "Service".
func (b *builder) qualifyType(fi *fileInfo, expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.StarExpr:
		return b.qualifyType(fi, e.X)
	case *ast.ParenExpr:
		return b.qualifyType(fi, e.X)
	case *ast.Ident:
		return fi.dir + "::" + e.Name
	case *ast.SelectorExpr:
		pkg, ok := e.X.(*ast.Ident)
		if !ok {
			return ""
		}
		importPath, ok := fi.imports[pkg.Name]
		if !ok {
			return ""
		}
		dir, ok := b.importDir(fi.dir, importPath)
		if !ok {
			return ""
		}
		return dir + "::" + e.Sel.Name
	}
	return ""
}

func qualifyBare(qualified string) string {
	if idx := strings.LastIndex(qualified, "::"); idx >= 0 {
		return qualified[idx+2:]
	}
	return qualified
}

func (b *builder) recordCall(fi *fileInfo, callerID string, env map[string]string, call *ast.CallExpr) {
	targets, token, confidence := b.resolveCall(fi, env, call)
	if len(targets) == 0 || token == "" {
		return
	}
	pos := b.fset.Position(call.Fun.Pos())
	end := b.fset.Position(call.Fun.End())
	if pos.Line != end.Line {
		return
	}
	site := Site{
		Path: fi.path, Line: pos.Line,
		Col: pos.Column - 1, EndCol: pos.Column - 1 + (end.Offset - pos.Offset),
		Token: token, Kind: "call", Confidence: confidence,
		Targets: targets,
	}
	b.sites[fi.path] = append(b.sites[fi.path], site)

	for _, target := range targets {
		key := callerID + "\x00" + target.ID + "\x00" + fi.path + "\x00" + strconv.Itoa(pos.Line)
		if b.seenEdge[key] {
			continue
		}
		b.seenEdge[key] = true
		b.edges = append(b.edges, Edge{
			CallerID: callerID, CalleeID: target.ID,
			Path: fi.path, Line: pos.Line, Confidence: confidence,
		})
	}
}

func (b *builder) resolveCall(fi *fileInfo, env map[string]string, call *ast.CallExpr) ([]Target, string, string) {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		name := fun.Name
		if builtinFuncs[name] || builtinTypes[name] || b.types[fi.dir][name] {
			return nil, "", ""
		}
		if ids := b.funcs[fi.dir][name]; len(ids) > 0 {
			return b.targets(ids), name, "local"
		}
		return nil, "", ""
	case *ast.SelectorExpr:
		sel := fun.Sel.Name
		if chained, ok := fun.X.(*ast.SelectorExpr); ok {
			if inner, ok := chained.X.(*ast.Ident); ok {
				if typ, ok := env[inner.Name]; ok {
					if fieldType := b.fields[typ][chained.Sel.Name]; fieldType != "" {
						if ids := b.methodRec[fieldType+"."+sel]; len(ids) > 0 {
							return b.targets(ids), inner.Name + "." + chained.Sel.Name + "." + sel, confidenceFor(ids, "receiver")
						}
					}
				}
			}
			// Calls on the result of another call (x.F().M()) are too noisy to
			// resolve by bare name; only keep the typed resolution above.
			return nil, "", ""
		}
		ident, ok := fun.X.(*ast.Ident)
		if !ok {
			return nil, "", ""
		}
		if importPath, ok := fi.imports[ident.Name]; ok {
			if dir, ok := b.importDir(fi.dir, importPath); ok {
				if ids := b.funcs[dir][sel]; len(ids) > 0 {
					return b.targets(ids), ident.Name + "." + sel, "exact"
				}
			}
			return nil, "", ""
		}
		if typ, ok := env[ident.Name]; ok {
			if ids := b.methodRec[typ+"."+sel]; len(ids) > 0 {
				return b.targets(ids), ident.Name + "." + sel, confidenceFor(ids, "receiver")
			}
		}
		if ids := b.methods[sel]; len(ids) > 0 {
			return b.targets(ids), ident.Name + "." + sel, confidenceFor(ids, "name")
		}
		return nil, "", ""
	}
	return nil, "", ""
}

func confidenceFor(ids []string, base string) string {
	if len(ids) > 1 {
		return "multiple"
	}
	return base
}

func (b *builder) targets(ids []string) []Target {
	const maxTargets = 30
	out := make([]Target, 0, len(ids))
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		node, ok := b.nodes[id]
		if !ok {
			continue
		}
		out = append(out, Target{ID: node.ID, Path: node.Path, Name: node.Name, Line: node.StartLine})
		if len(out) >= maxTargets {
			break
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Line < out[j].Line
	})
	return out
}

func (b *builder) importDir(fromDir, importPath string) (string, bool) {
	mod, ok := b.moduleFor(fromDir)
	if !ok {
		return "", false
	}
	rel := ""
	switch {
	case importPath == mod.path:
		rel = "."
	case strings.HasPrefix(importPath, mod.path+"/"):
		rel = strings.TrimPrefix(importPath, mod.path+"/")
	default:
		return "", false
	}
	repoRel, err := filepath.Rel(b.root, mod.root)
	if err != nil {
		return "", false
	}
	dir := path.Clean(path.Join(filepath.ToSlash(repoRel), rel))
	if dir == "." {
		dir = ""
	}
	return dir, true
}

type moduleInfo struct {
	path string
	root string
}

// moduleFor finds the nearest go.mod above dir and caches it.
func (b *builder) moduleFor(dir string) (moduleInfo, bool) {
	if mod, ok := b.modules[dir]; ok {
		return mod, mod.path != ""
	}
	current := dir
	for {
		candidate := filepath.Join(b.root, filepath.FromSlash(current))
		if module := readModule(candidate); module != "" {
			mod := moduleInfo{path: module, root: candidate}
			b.modules[dir] = mod
			return mod, true
		}
		if current == "" || current == "." {
			break
		}
		parent := path.Dir(current)
		if parent == "." || parent == "/" {
			parent = ""
		}
		if parent == current {
			break
		}
		current = parent
	}
	b.modules[dir] = moduleInfo{}
	return moduleInfo{}, false
}

func readModule(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}

func receiverType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return receiverType(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr:
		return receiverType(t.X)
	case *ast.IndexListExpr:
		return receiverType(t.X)
	}
	return ""
}

func (g *Graph) buildIndexes() {
	g.callees = map[string][]int{}
	g.callers = map[string][]int{}
	for i, edge := range g.Edges {
		g.callees[edge.CallerID] = append(g.callees[edge.CallerID], i)
		g.callers[edge.CalleeID] = append(g.callers[edge.CalleeID], i)
	}
}

// Lookup returns callers, callees and same-name implementations of a function.
func (g *Graph) Lookup(functionID string) *Result {
	node, ok := g.Nodes[functionID]
	if !ok {
		return nil
	}
	res := &Result{
		Function:    node,
		Callees:     []Link{},
		Callers:     []Link{},
		Commit:      g.Commit,
		Indexed:     true,
		GoFiles:     g.GoFiles,
		Files:       g.Files,
		ReusedFiles: g.ReusedFiles,
		Truncated:   g.Truncated,
	}
	seen := map[string]bool{}
	for _, idx := range g.callees[functionID] {
		edge := g.Edges[idx]
		callee, ok := g.Nodes[edge.CalleeID]
		if !ok {
			continue
		}
		key := "out\x00" + callee.ID
		if seen[key] {
			continue
		}
		seen[key] = true
		res.Callees = append(res.Callees, Link{Node: callee, Path: callee.Path, Line: callee.StartLine, Confidence: edge.Confidence})
	}
	for _, idx := range g.callers[functionID] {
		edge := g.Edges[idx]
		caller, ok := g.Nodes[edge.CallerID]
		if !ok {
			continue
		}
		key := "in\x00" + caller.ID + "\x00" + edge.Path + "\x00" + strconv.Itoa(edge.Line)
		if seen[key] {
			continue
		}
		seen[key] = true
		res.Callers = append(res.Callers, Link{Node: caller, Path: edge.Path, Line: edge.Line, Confidence: edge.Confidence})
	}
	res.Implementations = g.implementations(node)
	return res
}

func (g *Graph) implementations(node Node) []Target {
	name := node.Name
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		name = name[idx+1:]
	}
	out := []Target{}
	seen := map[string]bool{}
	for id, candidate := range g.Nodes {
		if id == node.ID || seen[id] {
			continue
		}
		base := candidate.Name
		if idx := strings.LastIndex(base, "."); idx >= 0 {
			base = base[idx+1:]
		}
		if base != name {
			continue
		}
		seen[id] = true
		out = append(out, Target{ID: candidate.ID, Path: candidate.Path, Name: candidate.Name, Line: candidate.StartLine})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Line < out[j].Line
	})
	if len(out) > 50 {
		out = out[:50]
	}
	return out
}

// SitesFor returns clickable tokens for one file.
func (g *Graph) SitesFor(filePath string) []Site {
	sites := append([]Site(nil), g.Sites[filePath]...)
	// keep one token per position, prefer call over def
	seen := map[string]bool{}
	out := make([]Site, 0, len(sites))
	for _, site := range sites {
		key := site.Path + "\x00" + strconv.Itoa(site.Line) + "\x00" + strconv.Itoa(site.Col)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, site)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Line != out[j].Line {
			return out[i].Line < out[j].Line
		}
		return out[i].Col < out[j].Col
	})
	return out
}
