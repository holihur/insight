package callgraph

import (
	"sort"
	"strings"
)

// TraceNode is one entry in a call trace tree.
type TraceNode struct {
	Function   Node         `json:"function"`
	Path       string       `json:"path"`
	Line       int          `json:"line"`
	Confidence string       `json:"confidence,omitempty"`
	Children   []*TraceNode `json:"children,omitempty"`
	Truncated  bool         `json:"truncated,omitempty"`
	Cycle      bool         `json:"cycle,omitempty"`
}

// TraceResult is a depth-limited call tree (outgoing or incoming).
type TraceResult struct {
	Root        *TraceNode `json:"root"`
	Direction   string     `json:"direction"`
	Commit      string     `json:"commit"`
	Indexed     bool       `json:"indexed"`
	ReusedFiles int        `json:"reusedFiles,omitempty"`
}

// Trace walks the call graph from functionID in the given direction. Depth
// counts levels including the root (depth 2 = root plus direct neighbours).
func (g *Graph) Trace(functionID, direction string, depth, limit int) *TraceResult {
	if direction != "callers" {
		direction = "callees"
	}
	if depth < 1 {
		depth = 1
	}
	if depth > 4 {
		depth = 4
	}
	if limit < 1 {
		limit = 10
	}
	if limit > 25 {
		limit = 25
	}
	root, ok := g.Nodes[functionID]
	if !ok {
		return &TraceResult{Commit: g.Commit, Indexed: len(g.Nodes) > 0, Direction: direction, ReusedFiles: g.ReusedFiles}
	}
	ancestors := map[string]bool{functionID: true}
	tree := &TraceNode{Function: root, Path: root.Path, Line: root.StartLine}
	g.walkTrace(tree, direction, depth, limit, ancestors)
	return &TraceResult{
		Root: tree, Direction: direction, Commit: g.Commit,
		Indexed: true, ReusedFiles: g.ReusedFiles,
	}
}

func (g *Graph) walkTrace(node *TraceNode, direction string, depth, limit int, ancestors map[string]bool) {
	if depth <= 1 {
		return
	}
	type item struct {
		id         string
		path       string
		line       int
		confidence string
	}
	items := []item{}
	seen := map[string]bool{}
	var edgeIndexes []int
	if direction == "callees" {
		edgeIndexes = g.callees[node.Function.ID]
	} else {
		edgeIndexes = g.callers[node.Function.ID]
	}
	for _, idx := range edgeIndexes {
		edge := g.Edges[idx]
		targetID := edge.CalleeID
		path := ""
		line := 0
		if direction == "callees" {
			if target, ok := g.Nodes[edge.CalleeID]; ok {
				line = target.StartLine
				path = target.Path
			}
		} else {
			targetID = edge.CallerID
			path = edge.Path
			line = edge.Line
		}
		if _, ok := g.Nodes[targetID]; !ok || path == "" {
			continue
		}
		key := targetID
		if direction == "callers" {
			key += "\x00" + path + "\x00" + itoa(line)
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		items = append(items, item{id: targetID, path: path, line: line, confidence: edge.Confidence})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].path != items[j].path {
			return items[i].path < items[j].path
		}
		if items[i].line != items[j].line {
			return items[i].line < items[j].line
		}
		return items[i].id < items[j].id
	})
	if len(items) > limit {
		items = items[:limit]
		node.Truncated = true
	}
	for _, it := range items {
		child := g.Nodes[it.id]
		trace := &TraceNode{
			Function: child, Path: it.path, Line: it.line, Confidence: it.confidence,
		}
		if ancestors[it.id] {
			trace.Cycle = true
			node.Children = append(node.Children, trace)
			continue
		}
		ancestors[it.id] = true
		g.walkTrace(trace, direction, depth-1, limit, ancestors)
		delete(ancestors, it.id)
		node.Children = append(node.Children, trace)
	}
}

// itoa keeps trace.go free of strconv noise in hot loops.
func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	var digits [20]byte
	i := len(digits)
	for value > 0 {
		i--
		digits[i] = byte('0' + value%10)
		value /= 10
	}
	if negative {
		i--
		digits[i] = '-'
	}
	return strings.TrimSpace(string(digits[i:]))
}
