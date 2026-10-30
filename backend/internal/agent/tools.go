// Package agent wires repository tools and ADK agents together.
package agent

import (
	"fmt"
	"strings"
	"sync/atomic"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"

	"github.com/codeinsight/code-insight/internal/analysis"
	"github.com/codeinsight/code-insight/internal/model"
)

type listFilesArgs struct {
	Dir   string `json:"dir,omitempty" jsonschema:"Directory relative to the repository root. Use '.' or leave empty for the root."`
	Depth int    `json:"depth,omitempty" jsonschema:"How many levels to recurse, 1-4. Defaults to 2."`
}

type listFilesResult struct {
	Tree string `json:"tree"`
}

type readFileArgs struct {
	Path  string `json:"path" jsonschema:"File path relative to the repository root."`
	Start int    `json:"start,omitempty" jsonschema:"First line to read (1-based). Optional."`
	End   int    `json:"end,omitempty" jsonschema:"Last line to read (inclusive). Optional; at most 700 lines are returned."`
}

type readFileResult struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Total   int    `json:"totalLines"`
}

type searchArgs struct {
	Query      string `json:"query" jsonschema:"Literal text to search for. Case-insensitive."`
	MaxResults int    `json:"maxResults,omitempty" jsonschema:"Maximum number of matches, default 40, max 80."`
}

type searchResult struct {
	Matches string `json:"matches"`
}

type statsArgs struct{}

type statsResult struct {
	Summary string `json:"summary"`
}

type toolBudget struct {
	calls atomic.Int64
}

func (b *toolBudget) use(name string) error {
	if b.calls.Add(1) > 120 {
		return fmt.Errorf("tool budget exhausted (120 calls); stop exploring and answer with what you have")
	}
	return nil
}

// Tools returns filesystem tools scoped to a repository checkout.
func Tools(root string, ref *analysis.RepoRef, idx *model.Index) ([]tool.Tool, error) {
	budget := &toolBudget{}

	list, err := functiontool.New(functiontool.Config{
		Name: "list_files",
		Description: "List files and directories in the repository. Use this to understand the project layout. " +
			"Paths are relative to the repository root.",
	}, func(ctx agent.ToolContext, args listFilesArgs) (listFilesResult, error) {
		if err := budget.use("list_files"); err != nil {
			return listFilesResult{}, err
		}
		dir := strings.TrimSpace(args.Dir)
		if dir == "." || dir == "/" {
			dir = ""
		}
		tree, err := analysis.ListTree(root, dir, args.Depth)
		if err != nil {
			return listFilesResult{}, err
		}
		if len(tree) > 14000 {
			tree = tree[:14000] + "\n… (truncated)"
		}
		return listFilesResult{Tree: tree}, nil
	})
	if err != nil {
		return nil, err
	}

	read, err := functiontool.New(functiontool.Config{
		Name: "read_file",
		Description: "Read a source file (with line numbers) from the repository. Use small ranges for large files. " +
			"Always read the most relevant files before making claims about the code.",
	}, func(ctx agent.ToolContext, args readFileArgs) (readFileResult, error) {
		if err := budget.use("read_file"); err != nil {
			return readFileResult{}, err
		}
		if strings.TrimSpace(args.Path) == "" {
			return readFileResult{}, fmt.Errorf("path is required")
		}
		content, total, err := analysis.ReadSource(root, args.Path, args.Start, args.End)
		if err != nil {
			return readFileResult{}, err
		}
		return readFileResult{Path: args.Path, Content: content, Total: total}, nil
	})
	if err != nil {
		return nil, err
	}

	search, err := functiontool.New(functiontool.Config{
		Name: "search_code",
		Description: "Search for a literal string across the repository. Returns file:line snippets. " +
			"Use this to locate symbols, routes, config keys and error messages.",
	}, func(ctx agent.ToolContext, args searchArgs) (searchResult, error) {
		if err := budget.use("search_code"); err != nil {
			return searchResult{}, err
		}
		matches, err := analysis.Search(root, args.Query, args.MaxResults)
		if err != nil {
			return searchResult{}, err
		}
		if len(matches) > 18000 {
			matches = matches[:18000] + "\n… (truncated)"
		}
		return searchResult{Matches: matches}, nil
	})
	if err != nil {
		return nil, err
	}

	stats, err := functiontool.New(functiontool.Config{
		Name:        "repo_stats",
		Description: "Get repository statistics: languages, modules, entry points and notable files.",
	}, func(ctx agent.ToolContext, args statsArgs) (statsResult, error) {
		return statsResult{Summary: analysis.Digest(ref, idx, 9000)}, nil
	})
	if err != nil {
		return nil, err
	}

	return []tool.Tool{list, read, search, stats}, nil
}
