package callgraph

import (
	"path"
	"regexp"
	"sort"
	"strings"
)

var parentheticalRe = regexp.MustCompile(`\s*\([^)]*\)`)

// ResolveCandidate is a function (or file) that matches a diagram label.
type ResolveCandidate struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	Line       int    `json:"line"`
	Confidence string `json:"confidence,omitempty"`
}

// ResolveResult groups candidates for one label from a sequence/flow diagram.
type ResolveResult struct {
	Query      string             `json:"query"`
	Candidates []ResolveCandidate `json:"candidates"`
}

// ResolveQueries matches free-form diagram labels against indexed functions,
// preferring exact names, then bare method names, then file paths.
func (g *Graph) ResolveQueries(queries []string) []ResolveResult {
	results := make([]ResolveResult, 0, len(queries))
	for _, raw := range queries {
		query := normalizeQuery(raw)
		if query == "" {
			continue
		}
		result := ResolveResult{Query: raw, Candidates: []ResolveCandidate{}}
		lower := strings.ToLower(query)

		exact := []ResolveCandidate{}
		bare := []ResolveCandidate{}
		for _, node := range g.Nodes {
			name := strings.ToLower(node.Name)
			base := name
			if idx := strings.LastIndex(base, "."); idx >= 0 {
				base = base[idx+1:]
			}
			switch {
			case name == lower:
				exact = append(exact, candidateFor(node, "exact"))
			case base == lower:
				bare = append(bare, candidateFor(node, "exact"))
			}
		}
		switch {
		case len(exact) > 0:
			result.Candidates = capCandidates(exact, "exact")
		case len(bare) > 0:
			confidence := "exact"
			if len(bare) > 1 {
				confidence = "multiple"
			}
			result.Candidates = capCandidates(bare, confidence)
		default:
			// File-path style labels ("backend/internal/agent/generate.go",
			// "generate.go", "generate") map to the functions in that file.
			file := []ResolveCandidate{}
			for _, node := range g.Nodes {
				if matchesPath(node.Path, lower) {
					file = append(file, candidateFor(node, "file"))
				}
			}
			if len(file) > 0 {
				result.Candidates = capCandidates(file, "file")
			}
		}
		if len(result.Candidates) > 0 {
			results = append(results, result)
		}
	}
	return results
}

func matchesPath(filePath, query string) bool {
	lowerPath := strings.ToLower(filePath)
	if strings.ContainsAny(query, "/.") {
		return strings.Contains(lowerPath, query)
	}
	for _, segment := range strings.Split(lowerPath, "/") {
		if segment == query {
			return true
		}
	}
	base := strings.ToLower(path.Base(filePath))
	ext := path.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	return stem == query || base == query
}

func candidateFor(node Node, confidence string) ResolveCandidate {
	return ResolveCandidate{
		ID: node.ID, Name: node.Name, Path: node.Path, Kind: node.Kind,
		Line: node.StartLine, Confidence: confidence,
	}
}

func capCandidates(in []ResolveCandidate, confidence string) []ResolveCandidate {
	sort.Slice(in, func(i, j int) bool {
		if in[i].Path != in[j].Path {
			return in[i].Path < in[j].Path
		}
		return in[i].Line < in[j].Line
	})
	seen := map[string]bool{}
	out := make([]ResolveCandidate, 0, len(in))
	for _, candidate := range in {
		if seen[candidate.ID] {
			continue
		}
		seen[candidate.ID] = true
		candidate.Confidence = confidence
		out = append(out, candidate)
		if len(out) >= 12 {
			break
		}
	}
	return out
}

func normalizeQuery(query string) string {
	query = strings.TrimSpace(query)
	query = strings.Trim(query, "`'\"*_ ")
	query = strings.TrimSuffix(query, "()")
	query = parentheticalRe.ReplaceAllString(query, "")
	query = strings.TrimSpace(query)
	query = strings.TrimSuffix(query, "/")
	query = strings.TrimSpace(query)
	if idx := strings.Index(query, " as "); idx >= 0 {
		query = strings.TrimSpace(query[idx+4:])
	}
	return query
}
