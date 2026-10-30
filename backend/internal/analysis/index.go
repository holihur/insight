package analysis

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/codeinsight/code-insight/internal/model"
)

var ignoredDirs = map[string]bool{
	".git": true, ".hg": true, ".svn": true, "node_modules": true, "vendor": true,
	"dist": true, "build": true, "target": true, "out": true, "bin": true, "obj": true,
	".next": true, ".nuxt": true, ".svelte-kit": true, ".turbo": true, ".cache": true,
	"__pycache__": true, ".venv": true, "venv": true, "env": true, ".env": true,
	".idea": true, ".vscode": true, ".gradle": true, "coverage": true, ".coverage": true,
	"Pods": true, "DerivedData": true, ".mypy_cache": true, ".pytest_cache": true,
	".ruff_cache": true, ".terraform": true, "bower_components": true, "jspm_packages": true,
	"third_party": true, "external": true, "deps": true, "tmp": true, "temp": true,
}

var keyFileNames = map[string]bool{
	"readme": true, "readme.md": true, "readme.mdx": true, "readme.rst": true, "readme.txt": true,
	"license": true, "licence": true, "copying": true, "notice": true, "authors": true,
	"changelog": true, "changes": true, "contributing": true,
	"go.mod": true, "package.json": true, "pnpm-workspace.yaml": true, "pyproject.toml": true,
	"requirements.txt": true, "cargo.toml": true, "pom.xml": true, "build.gradle": true,
	"build.gradle.kts": true, "settings.gradle": true, "settings.gradle.kts": true,
	"makefile": true, "dockerfile": true, "docker-compose.yml": true, "docker-compose.yaml": true,
	"compose.yml": true, "compose.yaml": true, "tsconfig.json": true, "vite.config.ts": true,
	"vite.config.js": true, "next.config.js": true, "next.config.mjs": true, "turbo.json": true,
	"gemfile": true, "mix.exs": true, "cmakelists.txt": true, "justfile": true, "taskfile.yml": true,
	".env.example": true, "makefile.am": true, "meson.build": true, "bazel.build": true,
	"workspace": true, "build.sbt": true, "deno.json": true, "composer.json": true,
}

// BuildIndex walks root and returns a static snapshot of the repository.
func BuildIndex(root string, maxFiles int) (*model.Index, error) {
	if maxFiles <= 0 {
		maxFiles = 8000
	}
	idx := &model.Index{
		Tree: &model.TreeNode{Name: filepath.Base(root), Path: "", Type: "dir"},
	}
	dirNodes := map[string]*model.TreeNode{"": idx.Tree}
	langFiles := map[string]int{}
	langLines := map[string]int{}
	moduleFiles := map[string]int{}
	moduleLines := map[string]int{}
	moduleTop := map[string]map[string]int{}
	var allFiles []string
	fileCount := 0

	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		name := d.Name()

		if d.IsDir() {
			if rel == "." {
				return nil
			}
			if ignoredDirs[name] || (strings.HasPrefix(name, ".") && name != ".github") {
				return filepath.SkipDir
			}
			if fileCount >= maxFiles {
				idx.Truncated = true
				return filepath.SkipAll
			}
			parent := path.Dir(rel)
			if parent == "." {
				parent = ""
			}
			if parentNode, ok := dirNodes[parent]; ok {
				node := &model.TreeNode{Name: name, Path: rel, Type: "dir"}
				parentNode.Children = append(parentNode.Children, node)
				dirNodes[rel] = node
			}
			return nil
		}

		if fileCount >= maxFiles {
			idx.Truncated = true
			return filepath.SkipAll
		}
		if binaryExt[strings.ToLower(path.Ext(name))] || ignoredFileName(name) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		fileCount++
		allFiles = append(allFiles, rel)

		parent := path.Dir(rel)
		if parent == "." {
			parent = ""
		}
		if parentNode, ok := dirNodes[parent]; ok {
			parentNode.Children = append(parentNode.Children, &model.TreeNode{
				Name: name, Path: rel, Type: "file", Size: info.Size(),
			})
		}

		idx.Stats.Files++
		idx.Stats.Bytes += info.Size()

		lang := LanguageFor(name)
		isText := lang != "" || isLikelyText(rel)
		if !isText || info.Size() > 2*1024*1024 {
			return nil
		}
		lines := countLines(p)
		idx.Stats.Lines += lines
		if lang != "" {
			langFiles[lang]++
			langLines[lang] += lines
			if rel != "" {
				top := strings.Split(rel, "/")[0]
				if top != rel {
					moduleFiles[top]++
					moduleLines[top] += lines
					if moduleTop[top] == nil {
						moduleTop[top] = map[string]int{}
					}
					moduleTop[top][rel] = lines
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk repository: %w", err)
	}

	sortTree(idx.Tree)
	idx.KeyFiles = detectKeyFiles(allFiles)
	idx.EntryFiles = detectEntryFiles(allFiles)
	idx.Readme = readReadme(root, allFiles)
	idx.Stats.Languages = make([]model.Language, 0, len(langFiles))
	for name, files := range langFiles {
		idx.Stats.Languages = append(idx.Stats.Languages, model.Language{
			Name: name, Files: files, Lines: langLines[name],
		})
	}
	sort.Slice(idx.Stats.Languages, func(i, j int) bool {
		return idx.Stats.Languages[i].Lines > idx.Stats.Languages[j].Lines
	})
	if len(idx.Stats.Languages) > 12 {
		idx.Stats.Languages = idx.Stats.Languages[:12]
	}

	for name, files := range moduleFiles {
		mod := model.Module{Name: name, Files: files, Lines: moduleLines[name]}
		type pair struct {
			path  string
			lines int
		}
		var top []pair
		for p, l := range moduleTop[name] {
			top = append(top, pair{p, l})
		}
		sort.Slice(top, func(i, j int) bool { return top[i].lines > top[j].lines })
		for i, p := range top {
			if i >= 5 {
				break
			}
			mod.Top = append(mod.Top, p.path)
		}
		idx.Modules = append(idx.Modules, mod)
	}
	sort.Slice(idx.Modules, func(i, j int) bool {
		return idx.Modules[i].Lines > idx.Modules[j].Lines
	})
	if len(idx.Modules) > 16 {
		idx.Modules = idx.Modules[:16]
	}
	return idx, nil
}

func ignoredFileName(name string) bool {
	lower := strings.ToLower(name)
	if lower == ".ds_store" || lower == "thumbs.db" {
		return true
	}
	if strings.HasSuffix(lower, ".min.js") || strings.HasSuffix(lower, ".min.css") ||
		strings.HasSuffix(lower, ".map") || strings.HasSuffix(lower, ".lock") {
		return true
	}
	return false
}

func isLikelyText(rel string) bool {
	ext := strings.ToLower(path.Ext(rel))
	if ext == "" {
		base := strings.ToLower(path.Base(rel))
		switch base {
		case "makefile", "dockerfile", "license", "licence", "copying", "notice",
			"readme", "authors", "changelog", "changes", "contributing", "procfile",
			"gemfile", "rakefile", "justfile", "codeowners":
			return true
		}
		return false
	}
	switch ext {
	case ".txt", ".log", ".conf", ".cfg", ".properties", ".env", ".gitignore",
		".dockerignore", ".editorconfig", ".csv", ".tsv", ".lock", ".sum", ".mod",
		".bzl", ".bazel", ".mk", ".cmake", ".asm", ".s", ".v", ".sv", ".vhd",
		".patch", ".diff", ".gitattributes", ".npmrc", ".nvmrc", ".prettierrc",
		".eslintrc", ".stylelintrc", ".csproj", ".sln", ".pbxproj", ".plist", ".manifest":
		return true
	}
	return false
}

func countLines(file string) int {
	data, err := os.ReadFile(file)
	if err != nil {
		return 0
	}
	if len(data) == 0 {
		return 0
	}
	n := strings.Count(string(data), "\n")
	if data[len(data)-1] != '\n' {
		n++
	}
	return n
}

func sortTree(node *model.TreeNode) {
	if node == nil {
		return
	}
	sort.SliceStable(node.Children, func(i, j int) bool {
		a, b := node.Children[i], node.Children[j]
		if a.Type != b.Type {
			return a.Type == "dir"
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
	for _, child := range node.Children {
		sortTree(child)
	}
}

func detectKeyFiles(files []string) []string {
	var out []string
	for _, rel := range files {
		base := strings.ToLower(path.Base(rel))
		depth := strings.Count(rel, "/")
		if keyFileNames[base] && depth <= 2 {
			out = append(out, rel)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		di, dj := strings.Count(out[i], "/"), strings.Count(out[j], "/")
		if di != dj {
			return di < dj
		}
		return out[i] < out[j]
	})
	if len(out) > 24 {
		out = out[:24]
	}
	return out
}

func detectEntryFiles(files []string) []string {
	entryNames := map[string]bool{
		"main.go": true, "main.ts": true, "main.tsx": true, "main.js": true, "main.jsx": true,
		"main.py": true, "app.py": true, "manage.py": true, "main.rs": true, "lib.rs": true,
		"main.c": true, "main.cc": true, "main.cpp": true, "main.java": true, "index.js": true,
		"index.ts": true, "index.tsx": true, "server.go": true, "server.ts": true,
		"app.ts": true, "app.tsx": true, "cli.py": true, "main.rb": true, "config.ru": true,
	}
	var out []string
	for _, rel := range files {
		if strings.Count(rel, "/") > 3 {
			continue
		}
		base := strings.ToLower(path.Base(rel))
		if entryNames[base] {
			out = append(out, rel)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	if len(out) > 16 {
		out = out[:16]
	}
	return out
}

func readReadme(root string, files []string) string {
	candidates := []string{"README.md", "README.mdx", "README.rst", "README.txt", "readme.md", "Readme.md"}
	for _, name := range candidates {
		if data, err := os.ReadFile(filepath.Join(root, name)); err == nil {
			return truncate(string(data), 16000)
		}
	}
	for _, rel := range files {
		base := strings.ToLower(path.Base(rel))
		if strings.HasPrefix(base, "readme") && strings.Count(rel, "/") <= 1 {
			if data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel))); err == nil {
				return truncate(string(data), 16000)
			}
		}
	}
	return ""
}

// Digest renders a compact repository overview suitable for an LLM prompt.
func Digest(ref *RepoRef, idx *model.Index, maxChars int) string {
	if maxChars <= 0 {
		maxChars = 14000
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Repository: %s/%s\n", ref.Owner, ref.Name)
	fmt.Fprintf(&b, "Files: %d · Lines: %d\n", idx.Stats.Files, idx.Stats.Lines)
	b.WriteString("Languages: ")
	for i, lang := range idx.Stats.Languages {
		if i >= 8 {
			break
		}
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%s (%d files, %d lines)", lang.Name, lang.Files, lang.Lines)
	}
	b.WriteString("\n\nTop-level structure:\n")
	b.WriteString(TreeString(idx.Tree, 2, 90))
	b.WriteString("\nModules (top-level directories by lines):\n")
	for i, mod := range idx.Modules {
		if i >= 12 {
			break
		}
		fmt.Fprintf(&b, "- %s/: %d files, %d lines\n", mod.Name, mod.Files, mod.Lines)
	}
	b.WriteString("\nNotable files:\n")
	for _, f := range idx.KeyFiles {
		fmt.Fprintf(&b, "- %s\n", f)
	}
	if len(idx.EntryFiles) > 0 {
		b.WriteString("\nEntry points:\n")
		for _, f := range idx.EntryFiles {
			fmt.Fprintf(&b, "- %s\n", f)
		}
	}
	if idx.Readme != "" {
		b.WriteString("\nREADME excerpt:\n")
		b.WriteString(truncate(idx.Readme, 6000))
		b.WriteString("\n")
	}
	return truncate(b.String(), maxChars)
}

// TreeString renders a tree node as an indented outline.
func TreeString(node *model.TreeNode, maxDepth, maxLines int) string {
	var b strings.Builder
	lines := 0
	var walk func(n *model.TreeNode, prefix string, depth int)
	walk = func(n *model.TreeNode, prefix string, depth int) {
		if lines >= maxLines {
			return
		}
		for _, child := range n.Children {
			if lines >= maxLines {
				b.WriteString(prefix + "…\n")
				lines++
				return
			}
			name := child.Name
			if child.Type == "dir" {
				name += "/"
			}
			b.WriteString(prefix + name + "\n")
			lines++
			if child.Type == "dir" && depth < maxDepth {
				walk(child, prefix+"  ", depth+1)
			}
		}
	}
	walk(node, "", 1)
	return b.String()
}

// ListTree returns an indented listing of one directory.
func ListTree(root, rel string, depth int) (string, error) {
	full, err := SafeJoin(root, rel)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(full)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", rel)
	}
	if depth <= 0 {
		depth = 2
	}
	if depth > 4 {
		depth = 4
	}
	var b strings.Builder
	count := 0
	var walk func(dir, prefix string, level int)
	walk = func(dir, prefix string, level int) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		sort.SliceStable(entries, func(i, j int) bool {
			if entries[i].IsDir() != entries[j].IsDir() {
				return entries[i].IsDir()
			}
			return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
		})
		for _, entry := range entries {
			if count >= 250 {
				b.WriteString(prefix + "…\n")
				return
			}
			name := entry.Name()
			if entry.IsDir() && (ignoredDirs[name] || (strings.HasPrefix(name, ".") && name != ".github")) {
				continue
			}
			if !entry.IsDir() && (binaryExt[strings.ToLower(path.Ext(name))] || ignoredFileName(name)) {
				continue
			}
			if entry.IsDir() {
				b.WriteString(prefix + name + "/\n")
			} else {
				b.WriteString(prefix + name + "\n")
			}
			count++
			if entry.IsDir() && level < depth {
				walk(filepath.Join(dir, name), prefix+"  ", level+1)
			}
		}
	}
	walk(full, "", 1)
	if b.Len() == 0 {
		return "(empty directory)", nil
	}
	return b.String(), nil
}

// ReadSource returns numbered lines from a repository file.
func ReadSource(root, rel string, start, end int) (string, int, error) {
	full, err := SafeJoin(root, rel)
	if err != nil {
		return "", 0, err
	}
	info, err := os.Stat(full)
	if err != nil {
		return "", 0, err
	}
	if info.IsDir() {
		return "", 0, fmt.Errorf("%s is a directory", rel)
	}
	if info.Size() > 2*1024*1024 {
		return "", 0, fmt.Errorf("file is too large to read (%d bytes)", info.Size())
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return "", 0, err
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	total := len(lines)
	if start <= 0 {
		start = 1
	}
	if end <= 0 || end > total {
		end = total
	}
	if end > start+700 {
		end = start + 700
	}
	if start > total {
		return "", total, fmt.Errorf("start line %d is past end of file (%d lines)", start, total)
	}
	var b strings.Builder
	for i := start; i <= end; i++ {
		fmt.Fprintf(&b, "%5d | %s\n", i, lines[i-1])
	}
	if end < total {
		fmt.Fprintf(&b, "… (%d more lines)\n", total-end)
	}
	return b.String(), total, nil
}

// Search performs a literal, case-insensitive search across source files.
func Search(root, query string, maxResults int) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", fmt.Errorf("query is required")
	}
	if maxResults <= 0 {
		maxResults = 40
	}
	if maxResults > 80 {
		maxResults = 80
	}
	lower := strings.ToLower(query)
	var b strings.Builder
	results := 0
	filesScanned := 0

	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if p != root && (ignoredDirs[name] || (strings.HasPrefix(name, ".") && name != ".github")) {
				return filepath.SkipDir
			}
			return nil
		}
		if results >= maxResults || filesScanned >= 15000 {
			return filepath.SkipAll
		}
		if binaryExt[strings.ToLower(path.Ext(name))] || ignoredFileName(name) {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() > 1024*1024 {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil || isBinaryData(data) {
			return nil
		}
		filesScanned++
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		for i, line := range strings.Split(string(data), "\n") {
			if !strings.Contains(strings.ToLower(line), lower) {
				continue
			}
			line = strings.TrimSpace(line)
			if len(line) > 240 {
				line = line[:240] + "…"
			}
			fmt.Fprintf(&b, "%s:%d: %s\n", rel, i+1, line)
			results++
			if results >= maxResults {
				return filepath.SkipAll
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if b.Len() == 0 {
		return "No matches found.", nil
	}
	return b.String(), nil
}

func isBinaryData(data []byte) bool {
	n := len(data)
	if n > 512 {
		n = 512
	}
	for _, c := range data[:n] {
		if c == 0 {
			return true
		}
	}
	return false
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := max
	for cut > 0 && s[cut]&0xC0 == 0x80 {
		cut--
	}
	return s[:cut] + "\n… (truncated)"
}
