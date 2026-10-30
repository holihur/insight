// Package coverage tracks human review coverage for repository functions.
package coverage

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type ignoreRule struct {
	kind    string // dir | path | package | glob
	pattern string
	re      *regexp.Regexp
}

type ignorer struct {
	rules  []ignoreRule
	module string
}

// NewIgnorer loads ignore rules from repo settings and .insightignore.
func NewIgnorer(root string, patterns []string) *ignorer {
	all := append([]string(nil), patterns...)
	if data, err := os.ReadFile(filepath.Join(root, ".insightignore")); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			all = append(all, line)
		}
	}
	ig := &ignorer{module: readModule(root)}
	for _, raw := range all {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		rule := ignoreRule{}
		switch {
		case strings.HasPrefix(line, "package:"):
			rule.kind = "package"
			rule.pattern = strings.TrimSpace(strings.TrimPrefix(line, "package:"))
		case strings.HasPrefix(line, "dir:"):
			rule.kind = "dir"
			rule.pattern = strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "dir:")), "/")
		case strings.HasSuffix(line, "/"):
			rule.kind = "dir"
			rule.pattern = strings.Trim(line, "/")
		case strings.ContainsAny(line, "*?"):
			rule.kind = "glob"
			rule.pattern = strings.TrimPrefix(line, "./")
			rule.re = globToRegex(rule.pattern)
		default:
			rule.kind = "path"
			rule.pattern = strings.TrimPrefix(strings.TrimPrefix(line, "./"), "/")
		}
		if rule.pattern == "" {
			continue
		}
		if rule.kind == "package" && rule.re == nil && strings.ContainsAny(rule.pattern, "*?") {
			rule.re = globToRegex(rule.pattern)
		}
		ig.rules = append(ig.rules, rule)
	}
	return ig
}

// Ignored reports whether a repository-relative file path should be excluded.
func (ig *ignorer) Ignored(filePath string) bool {
	filePath = strings.TrimPrefix(strings.ReplaceAll(filePath, "\\", "/"), "./")
	dir := strings.TrimSuffix(filepath.ToSlash(filepath.Dir(filePath)), ".")
	if dir == "." {
		dir = ""
	}
	for _, rule := range ig.rules {
		switch rule.kind {
		case "dir":
			if filePath == rule.pattern || strings.HasPrefix(filePath, rule.pattern+"/") {
				return true
			}
		case "path":
			if filePath == rule.pattern || strings.HasPrefix(filePath, rule.pattern+"/") {
				return true
			}
		case "glob":
			if rule.re != nil && (rule.re.MatchString(filePath) || (dir != "" && rule.re.MatchString(dir))) {
				return true
			}
		case "package":
			pkgDir := dir
			if pkgDir == "" {
				pkgDir = "."
			}
			if rule.re != nil {
				if rule.re.MatchString(pkgDir) {
					return true
				}
				if ig.module != "" && rule.re.MatchString(strings.TrimPrefix(ig.module+"/"+pkgDir, "./")) {
					return true
				}
				continue
			}
			pattern := strings.Trim(rule.pattern, "/")
			if pkgDir == pattern || strings.HasSuffix(pkgDir, "/"+pattern) {
				return true
			}
			if ig.module != "" {
				importPath := strings.TrimSuffix(ig.module+"/"+pkgDir, "/.")
				if importPath == pattern || strings.HasSuffix(importPath, "/"+pattern) {
					return true
				}
			}
		}
	}
	return false
}

func readModule(root string) string {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
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

func globToRegex(pattern string) *regexp.Regexp {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch c {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				i++
				if i+1 < len(pattern) && pattern[i+1] == '/' {
					i++
					b.WriteString("(?:.*/)?")
				} else {
					b.WriteString(".*")
				}
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		case '.', '+', '(', ')', '|', '^', '$', '{', '}', '[', ']', '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteString("$")
	re, err := regexp.Compile(b.String())
	if err != nil {
		return nil
	}
	return re
}
