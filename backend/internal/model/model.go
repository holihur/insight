// Package model contains the shared domain types for Code Insight.
package model

import "time"

// RepoStatus describes where a repository is in the analysis pipeline.
type RepoStatus string

const (
	StatusQueued     RepoStatus = "queued"
	StatusCloning    RepoStatus = "cloning"
	StatusIndexing   RepoStatus = "indexing"
	StatusPlanning   RepoStatus = "planning"
	StatusGenerating RepoStatus = "generating"
	StatusReady      RepoStatus = "ready"
	StatusError      RepoStatus = "error"
)

// Active reports whether the repository is still being processed.
func (s RepoStatus) Active() bool {
	switch s {
	case StatusQueued, StatusCloning, StatusIndexing, StatusPlanning, StatusGenerating:
		return true
	}
	return false
}

// Repo is the persistent record for one analyzed repository.
type Repo struct {
	ID        string     `json:"id"`
	URL       string     `json:"url"`
	WebURL    string     `json:"webUrl"`
	Owner     string     `json:"owner"`
	Name      string     `json:"name"`
	Branch    string     `json:"branch"`
	Commit    string     `json:"commit"`
	Status    RepoStatus `json:"status"`
	Progress  int        `json:"progress"`
	Message   string     `json:"message"`
	Error     string     `json:"error,omitempty"`
	Demo      bool       `json:"demo"`
	Provider  string     `json:"provider"`
	Ignore    []string   `json:"ignore,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	Stats     Stats      `json:"stats"`
	Wiki      *Wiki      `json:"wiki,omitempty"`
}

// Stats summarizes the indexed source tree.
type Stats struct {
	Files     int        `json:"files"`
	Lines     int        `json:"lines"`
	Bytes     int64      `json:"bytes"`
	Languages []Language `json:"languages"`
}

// Language is a per-language aggregate.
type Language struct {
	Name  string `json:"name"`
	Files int    `json:"files"`
	Lines int    `json:"lines"`
}

// Wiki is the generated documentation for a repository.
type Wiki struct {
	Title       string    `json:"title"`
	Summary     string    `json:"summary"`
	Pages       []*Page   `json:"pages"`
	GeneratedAt time.Time `json:"generatedAt"`
	Demo        bool      `json:"demo"`
	Provider    string    `json:"provider"`
}

// Page is a single markdown wiki page.
type Page struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Content     string `json:"content"`
	Order       int    `json:"order"`
	Status      string `json:"status"` // pending | writing | ready | error
}

// TreeNode is a serializable file-tree node used by the source browser.
type TreeNode struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"`
	Type     string      `json:"type"` // dir | file
	Size     int64       `json:"size,omitempty"`
	Children []*TreeNode `json:"children,omitempty"`
}

// Index is the static analysis snapshot persisted next to the clone.
type Index struct {
	Tree       *TreeNode `json:"tree"`
	KeyFiles   []string  `json:"keyFiles"`
	EntryFiles []string  `json:"entryFiles"`
	Modules    []Module  `json:"modules"`
	Stats      Stats     `json:"stats"`
	Readme     string    `json:"readme"`
	Truncated  bool      `json:"truncated"`
}

// Module is a top-level source directory.
type Module struct {
	Name  string   `json:"name"`
	Files int      `json:"files"`
	Lines int      `json:"lines"`
	Top   []string `json:"top"`
}

// FileContent is returned by the source-file endpoint.
type FileContent struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	Language string `json:"language"`
	Lines    int    `json:"lines"`
	Start    int    `json:"start"`
	End      int    `json:"end"`
	Total    int    `json:"total"`
}

// Function is a reviewable function or method discovered in a source file.
type Function struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	StartLine int    `json:"startLine"`
	EndLine   int    `json:"endLine"`
	Changed   bool   `json:"changed"`
	Reviewed  bool   `json:"reviewed"`
}

// Coverage summarizes human review progress for the current commit.
type Coverage struct {
	Commit            string     `json:"commit"`
	Parent            string     `json:"parent,omitempty"`
	ChangedAvailable  bool       `json:"changedAvailable"`
	TotalFunctions    int        `json:"totalFunctions"`
	ReviewedFunctions int        `json:"reviewedFunctions"`
	ChangedFunctions  int        `json:"changedFunctions"`
	ReviewedChanged   int        `json:"reviewedChanged"`
	OverallPercent    int        `json:"overallPercent"`
	ChangedPercent    int        `json:"changedPercent"`
	IgnoredFiles      int        `json:"ignoredFiles"`
	Functions         []Function `json:"functions"`
	UpdatedAt         string     `json:"updatedAt"`
}

// CoverageNode aggregates human review progress for a directory or file.
type CoverageNode struct {
	Path            string `json:"path"`
	Name            string `json:"name"`
	Type            string `json:"type"` // dir | file
	Total           int    `json:"total"`
	Reviewed        int    `json:"reviewed"`
	Changed         int    `json:"changed"`
	ReviewedChanged int    `json:"reviewedChanged"`
	Percent         int    `json:"percent"`
	ChangedPercent  int    `json:"changedPercent"`
}

// CoverageTree maps every directory and file to its aggregated coverage.
type CoverageTree struct {
	Commit    string                  `json:"commit"`
	Nodes     map[string]CoverageNode `json:"nodes"`
	Overall   CoverageNode            `json:"overall"`
	UpdatedAt string                  `json:"updatedAt"`
}
