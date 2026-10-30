package analysis

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/codeinsight/code-insight/internal/model"
)

// FallbackWiki builds a useful, fully local wiki without an LLM. It is used
// when no model is configured and as a safety net when an agent call fails.
func FallbackWiki(ref *RepoRef, idx *model.Index, root string) *model.Wiki {
	title := displayName(ref)
	pages := []*model.Page{
		{Title: "Overview", Slug: "overview", Description: "What this project is and how it is organized.", Status: "ready"},
		{Title: "Getting Started", Slug: "getting-started", Description: "Toolchain, setup commands and configuration.", Status: "ready"},
		{Title: "Architecture", Slug: "architecture", Description: "Major subsystems and how they fit together.", Status: "ready"},
	}

	for i, mod := range idx.Modules {
		if i >= 5 {
			break
		}
		pages = append(pages, &model.Page{
			Title:       humanize(mod.Name) + " module",
			Slug:        "module-" + slugify(mod.Name),
			Description: fmt.Sprintf("Files, responsibilities and entry points in %s/.", mod.Name),
			Status:      "ready",
		})
	}
	pages = append(pages, &model.Page{
		Title: "Key Files", Slug: "key-files", Description: "The files worth reading first.", Status: "ready",
	})

	summary := repoSummary(ref, idx)
	for i, page := range pages {
		page.ID = fmt.Sprintf("page-%d", i+1)
		page.Order = i
	}
	pages[0].Content = overviewPage(ref, idx, summary)
	pages[1].Content = gettingStartedPage(ref, idx, root)
	pages[2].Content = architecturePage(ref, idx)
	modulePageIndex := 3
	for i, mod := range idx.Modules {
		if i >= 5 {
			break
		}
		pages[modulePageIndex].Content = modulePage(root, mod)
		modulePageIndex++
	}
	pages[len(pages)-1].Content = keyFilesPage(root, idx)

	return &model.Wiki{
		Title:       title,
		Summary:     summary,
		Pages:       pages,
		GeneratedAt: nowUTC(),
		Demo:        true,
		Provider:    "local",
	}
}

// FallbackPage generates one wiki page locally, used when an agent page write fails.
func FallbackPage(ref *RepoRef, idx *model.Index, root, slug string) *model.Page {
	wiki := FallbackWiki(ref, idx, root)
	for _, page := range wiki.Pages {
		if page.Slug == slug {
			return page
		}
	}
	return nil
}

func displayName(ref *RepoRef) string {
	name := strings.ReplaceAll(ref.Name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")
	words := strings.Fields(name)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}
	return strings.Join(words, " ")
}

func repoSummary(ref *RepoRef, idx *model.Index) string {
	readme := stripMarkdown(idx.Readme)
	if readme != "" {
		paragraphs := strings.Split(readme, "\n\n")
		for _, p := range paragraphs {
			p = strings.TrimSpace(p)
			if len(p) < 40 {
				continue
			}
			if len(p) > 260 {
				p = p[:260]
				if i := strings.LastIndex(p, " "); i > 180 {
					p = p[:i]
				}
				p += "…"
			}
			return p
		}
	}
	lang := "polyglot"
	if len(idx.Stats.Languages) > 0 {
		lang = idx.Stats.Languages[0].Name
	}
	filesWord := "files"
	if idx.Stats.Files == 1 {
		filesWord = "file"
	}
	linesWord := "lines"
	if idx.Stats.Lines == 1 {
		linesWord = "line"
	}
	return fmt.Sprintf("A %s codebase with %d source %s and %d %s of code.", lang, idx.Stats.Files, filesWord, idx.Stats.Lines, linesWord)
}

func overviewPage(ref *RepoRef, idx *model.Index, summary string) string {
	var b strings.Builder
	b.WriteString("# Overview\n\n")
	b.WriteString(summary + "\n\n")
	b.WriteString("## At a glance\n\n")
	b.WriteString("| | |\n|---|---|\n")
	fmt.Fprintf(&b, "| Repository | `%s/%s` |\n", ref.Owner, ref.Name)
	fmt.Fprintf(&b, "| Files indexed | %d |\n", idx.Stats.Files)
	fmt.Fprintf(&b, "| Lines of code | %d |\n", idx.Stats.Lines)
	if len(idx.Stats.Languages) > 0 {
		fmt.Fprintf(&b, "| Primary language | %s |\n", idx.Stats.Languages[0].Name)
	}
	if len(idx.Modules) > 0 {
		fmt.Fprintf(&b, "| Largest module | `%s/` |\n", idx.Modules[0].Name)
	}
	if len(idx.Stats.Languages) > 0 {
		b.WriteString("\n## Languages\n\n| Language | Files | Lines |\n|---|---:|---:|\n")
		for _, lang := range idx.Stats.Languages {
			fmt.Fprintf(&b, "| %s | %d | %d |\n", lang.Name, lang.Files, lang.Lines)
		}
	}
	b.WriteString("\n## Repository layout\n\n```text\n")
	b.WriteString(TreeString(idx.Tree, 2, 70))
	b.WriteString("```\n\n")
	b.WriteString("## Where to start\n\n")
	if len(idx.EntryFiles) > 0 {
		b.WriteString("Likely entry points:\n\n")
		for _, f := range idx.EntryFiles {
			fmt.Fprintf(&b, "- `%s`\n", f)
		}
		b.WriteString("\n")
	}
	b.WriteString("Configuration and project manifests:\n\n")
	for _, f := range idx.KeyFiles {
		if isManifest(f) {
			fmt.Fprintf(&b, "- `%s`\n", f)
		}
	}
	return b.String()
}

func gettingStartedPage(ref *RepoRef, idx *model.Index, root string) string {
	var b strings.Builder
	b.WriteString("# Getting Started\n\n")
	b.WriteString("This guide is assembled from the project's manifests and build files. Verify commands against the README when in doubt.\n\n")

	b.WriteString("## Toolchain\n\n")
	toolchain := detectToolchain(root, idx)
	if len(toolchain) == 0 {
		b.WriteString("No standard toolchain manifest was detected at the repository root.\n")
	} else {
		for _, item := range toolchain {
			fmt.Fprintf(&b, "- %s\n", item)
		}
		b.WriteString("\n")
	}

	b.WriteString("## Common commands\n\n")
	commands := detectCommands(root, idx)
	if len(commands) == 0 {
		b.WriteString("```bash\ngit clone " + ref.WebURL + ".git\ncd " + ref.Name + "\n```\n")
	} else {
		b.WriteString("| Task | Command |\n|---|---|\n")
		for _, cmd := range commands {
			fmt.Fprintf(&b, "| %s | `%s` |\n", cmd.label, cmd.command)
		}
		b.WriteString("\n")
	}

	b.WriteString("## Configuration\n\n")
	envFiles := findFiles(root, idx, func(rel string) bool {
		base := strings.ToLower(path.Base(rel))
		return base == ".env.example" || base == ".env.sample" || base == "config.example.yaml" || base == "config.example.yml"
	}, 6)
	if len(envFiles) == 0 {
		b.WriteString("No example environment file was found. Check the README or deployment manifests for required settings.\n")
	} else {
		for _, f := range envFiles {
			fmt.Fprintf(&b, "- `%s`\n", f)
		}
	}

	b.WriteString("\n## First steps\n\n")
	b.WriteString("1. Read the root `README` and the manifest file for your language.\n")
	b.WriteString("2. Run the project's test command before making changes.\n")
	b.WriteString("3. Start with the entry points listed in the **Overview** page, then follow the code into the largest module.\n")
	return b.String()
}

func architecturePage(ref *RepoRef, idx *model.Index) string {
	var b strings.Builder
	b.WriteString("# Architecture\n\n")
	if len(idx.Modules) == 0 {
		b.WriteString("The repository is mostly flat; there are no substantial top-level modules to map.\n")
		return b.String()
	}

	b.WriteString("The repository is organized into the following top-level subsystems. Sizes are measured in lines of source code.\n\n")
	b.WriteString("```mermaid\ngraph TD\n  root[\"" + escapeMermaid(ref.Name+" repository") + "\"]\n")
	for i, mod := range idx.Modules {
		if i >= 10 {
			break
		}
		fmt.Fprintf(&b, "  root --> m%d[\"%s\"]\n", i, escapeMermaid(mod.Name+"/ ("+fmt.Sprint(mod.Files)+" files)"))
	}
	b.WriteString("```\n\n")
	b.WriteString("## Subsystems\n\n")
	b.WriteString("| Module | Files | Lines | Notable files |\n|---|---:|---:|---|\n")
	for _, mod := range idx.Modules {
		var top []string
		for i, f := range mod.Top {
			if i >= 3 {
				break
			}
			top = append(top, "`"+f+"`")
		}
		fmt.Fprintf(&b, "| [`%s/`](#%s-module) | %d | %d | %s |\n", mod.Name, slugify(mod.Name), mod.Files, mod.Lines, strings.Join(top, ", "))
	}
	b.WriteString("\n")
	if len(idx.EntryFiles) > 0 {
		b.WriteString("## Entry points\n\n")
		for _, f := range idx.EntryFiles {
			fmt.Fprintf(&b, "- `%s`\n", f)
		}
		b.WriteString("\n")
	}
	b.WriteString("## Reading the codebase\n\n")
	b.WriteString("Start from the entry points above, then follow imports into the largest modules. Cross-cutting concerns such as configuration, logging, HTTP handlers and persistence usually live at module boundaries.\n")
	return b.String()
}

func modulePage(root string, mod model.Module) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s module\n\n", humanize(mod.Name))
	fmt.Fprintf(&b, "`%s/` contains **%d source files** and roughly **%d lines** of code.\n\n", mod.Name, mod.Files, mod.Lines)

	files := listModuleFiles(root, mod.Name, 80)
	if len(files) > 0 {
		b.WriteString("## Files\n\n| File | Language | Lines |\n|---|---|---:|\n")
		shown := 0
		for _, f := range files {
			if shown >= 35 {
				break
			}
			fmt.Fprintf(&b, "| `%s` | %s | %d |\n", f.Path, f.Language, f.Lines)
			shown++
		}
		b.WriteString("\n")
	}

	if len(mod.Top) > 0 {
		b.WriteString("## Start here\n\n")
		for _, f := range mod.Top {
			fmt.Fprintf(&b, "- `%s`\n", f)
		}
		b.WriteString("\n")
	}

	b.WriteString("## Responsibilities\n\n")
	b.WriteString("This module is one of the repository's larger units. Inspect its exported symbols and entry points to map responsibilities; the files above are ordered by size and are a good starting point.\n\n")

	if len(files) > 0 {
		b.WriteString("## Structure\n\n```text\n")
		b.WriteString(moduleTree(files, mod.Name))
		b.WriteString("```\n")
	}
	return b.String()
}

func keyFilesPage(root string, idx *model.Index) string {
	var b strings.Builder
	b.WriteString("# Key Files\n\n")
	b.WriteString("A tour of the files that define how this project is built, configured and run.\n\n")
	b.WriteString("| File | Purpose |\n|---|---|\n")
	for _, f := range idx.KeyFiles {
		fmt.Fprintf(&b, "| `%s` | %s |\n", f, describeFile(f))
	}
	if len(idx.EntryFiles) > 0 {
		b.WriteString("\n## Entry points\n\n| File | Purpose |\n|---|---|\n")
		for _, f := range idx.EntryFiles {
			fmt.Fprintf(&b, "| `%s` | Application entry point |\n", f)
		}
	}
	b.WriteString("\n## Largest source files\n\n")
	type fileLines struct {
		path  string
		lines int
	}
	var largest []fileLines
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		base := strings.ToLower(d.Name())
		if binaryExt[path.Ext(base)] {
			return nil
		}
		lang := LanguageFor(base)
		if lang == "" || lang == "JSON" || lang == "Markdown" {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() > 1024*1024 {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		largest = append(largest, fileLines{filepath.ToSlash(rel), countLines(p)})
		return nil
	})
	sort.Slice(largest, func(i, j int) bool { return largest[i].lines > largest[j].lines })
	b.WriteString("| File | Lines |\n|---|---:|\n")
	for i, f := range largest {
		if i >= 15 {
			break
		}
		fmt.Fprintf(&b, "| `%s` | %d |\n", f.path, f.lines)
	}
	return b.String()
}

// AnswerLocally answers a question with repository search results when no LLM
// is available. It is intentionally explicit about being a local fallback.
func AnswerLocally(root string, idx *model.Index, ref *RepoRef, question string) string {
	q := strings.ToLower(question)
	var b strings.Builder
	b.WriteString("> **Demo mode** — no LLM is configured, so this answer is assembled from local code search. ")
	b.WriteString("Add a `DEEPSEEK_API_KEY` to `.env` for full agent answers.\n\n")

	switch {
	case containsAny(q, "architecture", "structure", "overview", "organized", "how does it work"):
		b.WriteString("## Repository structure\n\n")
		for _, mod := range idx.Modules {
			fmt.Fprintf(&b, "- **`%s/`** — %d files, %d lines\n", mod.Name, mod.Files, mod.Lines)
		}
		if len(idx.EntryFiles) > 0 {
			b.WriteString("\n### Entry points\n\n")
			for _, f := range idx.EntryFiles {
				fmt.Fprintf(&b, "- `%s`\n", f)
			}
		}
		return b.String()
	case containsAny(q, "run", "build", "start", "install", "setup", "test", "develop"):
		b.WriteString("## Commands found in the repository\n\n")
		commands := detectCommands(root, idx)
		if len(commands) == 0 {
			b.WriteString("No standard build commands were detected. Check the README and project manifest.\n")
		}
		for _, cmd := range commands {
			fmt.Fprintf(&b, "- **%s**: `%s`\n", cmd.label, cmd.command)
		}
		return b.String()
	}

	keywords := keywordsFrom(question)
	if len(keywords) == 0 {
		b.WriteString("Try asking about a specific symbol, file, feature or subsystem.\n")
		return b.String()
	}
	b.WriteString("## Code matches\n\n")
	seen := map[string]bool{}
	total := 0
	for _, kw := range keywords {
		if total >= 24 {
			break
		}
		res, err := Search(root, kw, 8)
		if err != nil || res == "No matches found." {
			continue
		}
		for _, line := range strings.Split(strings.TrimSpace(res), "\n") {
			if line == "" || seen[line] {
				continue
			}
			seen[line] = true
			b.WriteString("- `" + line + "`\n")
			total++
			if total >= 24 {
				break
			}
		}
	}
	if total == 0 {
		b.WriteString("No matching code was found. The feature may live outside this repository or use different terminology.\n")
	}
	if len(idx.KeyFiles) > 0 {
		b.WriteString("\n### Useful context\n\n")
		for i, f := range idx.KeyFiles {
			if i >= 6 {
				break
			}
			b.WriteString("- `" + f + "`\n")
		}
	}
	return b.String()
}

func keywordsFrom(question string) []string {
	stop := map[string]bool{
		"what": true, "where": true, "which": true, "how": true, "does": true, "this": true,
		"that": true, "with": true, "from": true, "into": true, "about": true, "there": true,
		"their": true, "have": true, "has": true, "the": true, "and": true, "for": true,
		"are": true, "you": true, "can": true, "tell": true, "explain": true, "code": true,
		"work": true, "works": true, "project": true, "repository": true, "repo": true,
	}
	re := regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_.-]{2,}`)
	words := re.FindAllString(question, -1)
	var out []string
	seen := map[string]bool{}
	for _, w := range words {
		lw := strings.ToLower(w)
		if stop[lw] || seen[lw] {
			continue
		}
		seen[lw] = true
		out = append(out, w)
	}
	return out
}

func containsAny(s string, values ...string) bool {
	for _, v := range values {
		if strings.Contains(s, v) {
			return true
		}
	}
	return false
}

type command struct{ label, command string }

func detectCommands(root string, idx *model.Index) []command {
	var out []command

	if data, err := os.ReadFile(filepath.Join(root, "package.json")); err == nil {
		var pkg struct {
			Scripts        map[string]string `json:"scripts"`
			PackageManager string            `json:"packageManager"`
		}
		if json.Unmarshal(data, &pkg) == nil {
			manager := "npm"
			if strings.HasPrefix(pkg.PackageManager, "pnpm") {
				manager = "pnpm"
			} else if strings.HasPrefix(pkg.PackageManager, "yarn") {
				manager = "yarn"
			} else if strings.HasPrefix(pkg.PackageManager, "bun") {
				manager = "bun"
			} else if _, err := os.Stat(filepath.Join(root, "pnpm-lock.yaml")); err == nil {
				manager = "pnpm"
			} else if _, err := os.Stat(filepath.Join(root, "yarn.lock")); err == nil {
				manager = "yarn"
			}
			out = append(out, command{"Install dependencies", manager + " install"})
			keys := make([]string, 0, len(pkg.Scripts))
			for k := range pkg.Scripts {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			priority := []string{"dev", "start", "build", "test", "lint", "check", "typecheck"}
			added := map[string]bool{}
			for _, key := range priority {
				if _, ok := pkg.Scripts[key]; ok {
					out = append(out, command{"Run " + key, manager + " run " + key})
					added[key] = true
				}
			}
			for _, key := range keys {
				if added[key] || len(out) > 10 {
					continue
				}
				out = append(out, command{"Run " + key, manager + " run " + key})
			}
		}
	}

	if data, err := os.ReadFile(filepath.Join(root, "go.mod")); err == nil {
		out = append(out, command{"Download Go modules", "go mod download"})
		out = append(out, command{"Run tests", "go test ./..."})
		out = append(out, command{"Build all packages", "go build ./..."})
		if strings.Contains(string(data), "cmd/") {
			out = append(out, command{"Run a command (replace path)", "go run ./cmd/<name>"})
		}
	}

	if _, err := os.Stat(filepath.Join(root, "pyproject.toml")); err == nil {
		out = append(out, command{"Install project", "pip install -e ."})
		out = append(out, command{"Run tests", "pytest"})
	} else if _, err := os.Stat(filepath.Join(root, "requirements.txt")); err == nil {
		out = append(out, command{"Install dependencies", "pip install -r requirements.txt"})
	}

	if _, err := os.Stat(filepath.Join(root, "Cargo.toml")); err == nil {
		out = append(out, command{"Build", "cargo build"})
		out = append(out, command{"Run tests", "cargo test"})
	}

	if data, err := os.ReadFile(filepath.Join(root, "Makefile")); err == nil {
		targets := parseMakeTargets(string(data))
		for _, target := range targets {
			if len(out) > 14 {
				break
			}
			if target == "all" || target == "clean" || strings.HasPrefix(target, ".") {
				continue
			}
			out = append(out, command{"Make " + target, "make " + target})
		}
	}

	if _, err := os.Stat(filepath.Join(root, "docker-compose.yml")); err == nil {
		out = append(out, command{"Start services", "docker compose up -d"})
	} else if _, err := os.Stat(filepath.Join(root, "compose.yaml")); err == nil {
		out = append(out, command{"Start services", "docker compose up -d"})
	}
	return out
}

func parseMakeTargets(content string) []string {
	re := regexp.MustCompile(`(?m)^([A-Za-z0-9_.-]+):(?!=)`)
	matches := re.FindAllStringSubmatch(content, -1)
	var out []string
	for _, m := range matches {
		out = append(out, m[1])
	}
	sort.Strings(out)
	return out
}

func detectToolchain(root string, idx *model.Index) []string {
	var out []string
	if data, err := os.ReadFile(filepath.Join(root, "go.mod")); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "go ") || strings.HasPrefix(line, "module ") {
				out = append(out, "`go.mod`: "+line)
			}
		}
	}
	if data, err := os.ReadFile(filepath.Join(root, "package.json")); err == nil {
		var pkg struct {
			Engines        map[string]string `json:"engines"`
			PackageManager string            `json:"packageManager"`
		}
		if json.Unmarshal(data, &pkg) == nil {
			if pkg.PackageManager != "" {
				out = append(out, "Package manager: `"+pkg.PackageManager+"`")
			}
			for k, v := range pkg.Engines {
				out = append(out, fmt.Sprintf("%s: `%s`", k, v))
			}
		}
	}
	if data, err := os.ReadFile(filepath.Join(root, "pyproject.toml")); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "requires-python") {
				out = append(out, "`pyproject.toml`: "+strings.TrimSpace(line))
			}
		}
	}
	if _, err := os.Stat(filepath.Join(root, "Dockerfile")); err == nil {
		out = append(out, "Dockerfile present — a container build is supported.")
	}
	if len(out) == 0 && len(idx.Stats.Languages) > 0 {
		out = append(out, "Primary language: "+idx.Stats.Languages[0].Name)
	}
	return out
}

func isManifest(rel string) bool {
	base := strings.ToLower(path.Base(rel))
	switch base {
	case "go.mod", "package.json", "pyproject.toml", "requirements.txt", "cargo.toml",
		"pom.xml", "build.gradle", "build.gradle.kts", "gemfile", "deno.json", "composer.json":
		return true
	}
	return false
}

func describeFile(rel string) string {
	base := strings.ToLower(path.Base(rel))
	switch base {
	case "go.mod":
		return "Go module definition and dependency graph"
	case "package.json":
		return "JavaScript package manifest, scripts and dependencies"
	case "pnpm-workspace.yaml":
		return "pnpm workspace configuration"
	case "pyproject.toml":
		return "Python project metadata, dependencies and tooling"
	case "requirements.txt":
		return "Python dependency list"
	case "cargo.toml":
		return "Rust crate manifest"
	case "dockerfile":
		return "Container image definition"
	case "docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml":
		return "Multi-service local runtime"
	case "makefile":
		return "Build and development tasks"
	case "tsconfig.json":
		return "TypeScript compiler configuration"
	case "vite.config.ts", "vite.config.js":
		return "Vite bundler configuration"
	case "next.config.js", "next.config.mjs":
		return "Next.js framework configuration"
	}
	if strings.HasPrefix(base, "readme") {
		return "Project introduction, setup and usage"
	}
	if strings.Contains(base, ".env") {
		return "Example environment configuration"
	}
	return "Project configuration file"
}

type moduleFile struct {
	Path     string
	Language string
	Lines    int
}

func listModuleFiles(root, module string, max int) []moduleFile {
	dir := filepath.Join(root, filepath.FromSlash(module))
	var out []moduleFile
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p != dir && ignoredDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if len(out) >= max {
			return filepath.SkipAll
		}
		lang := LanguageFor(d.Name())
		if lang == "" || lang == "Markdown" || lang == "JSON" || lang == "YAML" {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() > 1024*1024 {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		out = append(out, moduleFile{Path: filepath.ToSlash(rel), Language: lang, Lines: countLines(p)})
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Lines > out[j].Lines })
	return out
}

func moduleTree(files []moduleFile, module string) string {
	dirs := map[string]bool{module: true}
	for _, f := range files {
		parts := strings.Split(f.Path, "/")
		for i := 1; i < len(parts); i++ {
			dirs[strings.Join(parts[:i], "/")] = true
		}
	}
	names := make([]string, 0, len(dirs))
	for d := range dirs {
		names = append(names, d)
	}
	sort.Strings(names)
	var b strings.Builder
	for _, d := range names {
		depth := strings.Count(d, "/")
		if d == module {
			b.WriteString(module + "/\n")
			continue
		}
		b.WriteString(strings.Repeat("  ", depth) + path.Base(d) + "/\n")
	}
	return b.String()
}

func findFiles(root string, idx *model.Index, pred func(string) bool, max int) []string {
	var out []string
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p != root && ignoredDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if pred(rel) {
			out = append(out, rel)
			if len(out) >= max {
				return filepath.SkipAll
			}
		}
		return nil
	})
	return out
}

func stripMarkdown(s string) string {
	if s == "" {
		return ""
	}
	s = regexp.MustCompile(`(?s)<!--.*?-->`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`\[!\[[^\]]*\]\([^)]*\)\]\([^)]*\)`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`(?m)^#{1,6}\s*`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`\[([^\]]+)\]\([^)]*\)`).ReplaceAllString(s, "$1")
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "__", "")
	s = strings.ReplaceAll(s, "`", "")
	return strings.TrimSpace(s)
}

func humanize(s string) string {
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.ReplaceAll(s, "_", " ")
	words := strings.Fields(s)
	for i, w := range words {
		if w == "" {
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

var slugNonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = slugNonAlnum.ReplaceAllString(strings.ToLower(s), "-")
	return strings.Trim(s, "-")
}

func escapeMermaid(s string) string {
	s = strings.ReplaceAll(s, `"`, "'")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

func nowUTC() (t time.Time) { return time.Now().UTC() }
