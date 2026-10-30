package callgraph

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/codeinsight/code-insight/internal/config"
	"github.com/codeinsight/code-insight/internal/model"
	"github.com/codeinsight/code-insight/internal/store"
)

// Service builds, caches and persists per-commit call graphs.
type Service struct {
	cfg         config.Config
	store       *store.Store
	mu          sync.Mutex
	cache       map[string]*Graph
	parseCaches map[string]*parseCache
}

// New creates a call-graph service.
func New(cfg config.Config, st *store.Store) *Service {
	return &Service{
		cfg: cfg, store: st,
		cache:       map[string]*Graph{},
		parseCaches: map[string]*parseCache{},
	}
}

// Invalidate drops cached and persisted call graphs for a repository.
func (s *Service) Invalidate(repoID string) {
	s.mu.Lock()
	delete(s.cache, repoID)
	s.mu.Unlock()
	if repoID == "" {
		return
	}
	// Keep the content-addressed parse cache: unchanged files stay valid and
	// the next build only parses what actually changed.
	dir := filepath.Join(s.store.RepoDir(repoID), "callgraph")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "parse-cache.json" {
			continue
		}
		_ = os.Remove(filepath.Join(dir, entry.Name()))
	}
}

// Get returns callers, callees and implementations for one function.
func (s *Service) Get(_ context.Context, repoID, functionID string) (*Result, error) {
	repo, err := s.store.Get(repoID)
	if err != nil {
		return nil, err
	}
	graph, err := s.graphFor(repo)
	if err != nil {
		return nil, err
	}
	res := graph.Lookup(functionID)
	if res == nil {
		if len(graph.Nodes) == 0 {
			return &Result{Commit: repo.Commit, Indexed: false, Callees: []Link{}, Callers: []Link{}, Implementations: []Target{}}, nil
		}
		return nil, fmt.Errorf("function not found in the call index")
	}
	return res, nil
}

// Resolve maps diagram labels to indexed functions for a repository.
func (s *Service) Resolve(_ context.Context, repoID string, queries []string) ([]ResolveResult, error) {
	repo, err := s.store.Get(repoID)
	if err != nil {
		return nil, err
	}
	graph, err := s.graphFor(repo)
	if err != nil {
		return nil, err
	}
	return graph.ResolveQueries(queries), nil
}

// Sites returns clickable tokens for one file.
func (s *Service) Sites(_ context.Context, repoID, filePath string) (*SitesResult, error) {
	repo, err := s.store.Get(repoID)
	if err != nil {
		return nil, err
	}
	graph, err := s.graphFor(repo)
	if err != nil {
		return nil, err
	}
	return &SitesResult{
		Path:        filePath,
		Commit:      repo.Commit,
		Sites:       graph.SitesFor(filePath),
		Indexed:     len(graph.Nodes) > 0,
		GoFiles:     graph.GoFiles,
		Files:       graph.Files,
		ReusedFiles: graph.ReusedFiles,
		Truncated:   graph.Truncated,
	}, nil
}

func (s *Service) graphFor(repo *model.Repo) (*Graph, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if graph, ok := s.cache[repo.ID]; ok && graph.Commit == repo.Commit && time.Since(graph.BuiltAt) < 30*time.Minute {
		return graph, nil
	}

	root := s.store.SourceDir(repo.ID)
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("repository source is not available")
	}

	dir := filepath.Join(s.store.RepoDir(repo.ID), "callgraph")
	file := filepath.Join(dir, safeCommit(repo.Commit)+".json")
	if graph := loadGraph(file, repo.Commit); graph != nil {
		s.cache[repo.ID] = graph
		return graph, nil
	}

	parseCache := s.parseCaches[repo.ID]
	if parseCache == nil {
		parseCache = loadParseCache(filepath.Join(dir, "parse-cache.json"))
		s.parseCaches[repo.ID] = parseCache
	}

	graph, err := Build(repo.Commit, root, repo.Ignore, s.cfg.MaxRepoFiles, parseCache)
	if err != nil {
		return nil, err
	}
	s.cache[repo.ID] = graph
	if parseCache.takeDirty() {
		_ = parseCache.save(filepath.Join(dir, "parse-cache.json"))
	}
	if err := saveGraph(dir, file, graph); err != nil {
		// Persisting is best-effort: the in-memory index is still usable.
		_ = err
	}
	return graph, nil
}

func loadGraph(file, commit string) *Graph {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var graph Graph
	if err := json.Unmarshal(data, &graph); err != nil || graph.Version != Version || graph.Commit != commit {
		return nil
	}
	if graph.Nodes == nil {
		graph.Nodes = map[string]Node{}
	}
	if graph.Sites == nil {
		graph.Sites = map[string][]Site{}
	}
	graph.buildIndexes()
	return &graph
}

func saveGraph(dir, file string, graph *Graph) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	payload, err := json.Marshal(graph)
	if err != nil {
		return err
	}
	tmp := file + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, file); err != nil {
		return err
	}
	prune(dir, filepath.Base(file))
	return nil
}

func prune(dir, keep string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == keep || entry.Name() == "parse-cache.json" {
			continue
		}
		_ = os.Remove(filepath.Join(dir, entry.Name()))
	}
}

func safeCommit(commit string) string {
	commit = strings.TrimSpace(commit)
	if commit == "" {
		return "unknown"
	}
	var out strings.Builder
	for _, r := range commit {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			out.WriteRune(r)
		default:
			out.WriteByte('_')
		}
		if out.Len() >= 64 {
			break
		}
	}
	return out.String()
}
