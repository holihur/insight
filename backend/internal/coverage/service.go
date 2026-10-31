package coverage

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/codeinsight/code-insight/internal/analysis"
	"github.com/codeinsight/code-insight/internal/config"
	"github.com/codeinsight/code-insight/internal/model"
	"github.com/codeinsight/code-insight/internal/store"
)

type fileFuncs struct {
	funcs []analysis.FuncInfo
	ids   []string
}

type snapshot struct {
	commit           string
	parent           string
	changedAvailable bool
	files            map[string]fileFuncs
	all              []model.Function
	ignoredFiles     int
	builtAt          time.Time
}

// Service computes and persists human review coverage.
type Service struct {
	cfg   config.Config
	store *store.Store
	mu    sync.Mutex
	cache map[string]*snapshot
}

// New creates a coverage service.
func New(cfg config.Config, st *store.Store) *Service {
	return &Service{cfg: cfg, store: st, cache: map[string]*snapshot{}}
}

// Invalidate drops cached coverage for a repository.
func (s *Service) Invalidate(id string) {
	s.mu.Lock()
	delete(s.cache, id)
	s.mu.Unlock()
}

// Get returns coverage for the current commit, optionally filtered to one file.
func (s *Service) Get(ctx context.Context, repoID, filterPath string) (*model.Coverage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	repo, err := s.store.Get(repoID)
	if err != nil {
		return nil, err
	}
	snap, err := s.snapshotLocked(ctx, repo)
	if err != nil {
		return nil, err
	}
	reviews, err := s.store.LoadReviews(repoID)
	if err != nil {
		return nil, err
	}
	return buildCoverage(repo, snap, reviews, filterPath), nil
}

// Tree returns aggregated coverage for each directory and file.
func (s *Service) Tree(ctx context.Context, repoID string) (*model.CoverageTree, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	repo, err := s.store.Get(repoID)
	if err != nil {
		return nil, err
	}
	snap, err := s.snapshotLocked(ctx, repo)
	if err != nil {
		return nil, err
	}
	reviews, err := s.store.LoadReviews(repoID)
	if err != nil {
		return nil, err
	}
	reviewed := reviews[reviewKey(repo.Commit)]

	tree := &model.CoverageTree{
		Commit:    repo.Commit,
		Nodes:     map[string]model.CoverageNode{},
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	ensure := func(nodePath, nodeType string) model.CoverageNode {
		if node, ok := tree.Nodes[nodePath]; ok {
			return node
		}
		name := nodePath
		if idx := strings.LastIndex(nodePath, "/"); idx >= 0 {
			name = nodePath[idx+1:]
		}
		node := model.CoverageNode{Path: nodePath, Name: name, Type: nodeType}
		tree.Nodes[nodePath] = node
		return node
	}
	apply := func(nodePath string, fn model.Function, isReviewed bool) {
		node := ensure(nodePath, nodeTypeFor(nodePath))
		node.Total++
		if fn.Changed {
			node.Changed++
		}
		if isReviewed {
			node.Reviewed++
			if fn.Changed {
				node.ReviewedChanged++
			}
		}
		tree.Nodes[nodePath] = node
	}
	for _, fn := range snap.all {
		isReviewed := reviewed[fn.ID]
		apply(fn.Path, fn, isReviewed)
		dir := path.Dir(fn.Path)
		for dir != "." && dir != "" {
			apply(dir, fn, isReviewed)
			dir = path.Dir(dir)
		}
		apply("", fn, isReviewed)
	}
	for key, node := range tree.Nodes {
		node.Percent = percent(node.Reviewed, node.Total)
		if node.Changed > 0 {
			node.ChangedPercent = percent(node.ReviewedChanged, node.Changed)
		}
		tree.Nodes[key] = node
	}
	tree.Overall = tree.Nodes[""]
	return tree, nil
}

// reviewableFuncs keeps only callables. Classes, variables and other symbols
// are navigation-only: they never participate in human coverage.
func reviewableFuncs(funcs []analysis.FuncInfo) []analysis.FuncInfo {
	out := make([]analysis.FuncInfo, 0, len(funcs))
	for _, fn := range funcs {
		if fn.Kind == "function" || fn.Kind == "method" {
			out = append(out, fn)
		}
	}
	return out
}

func nodeTypeFor(nodePath string) string {
	if nodePath != "" && path.Ext(nodePath) != "" {
		return "file"
	}
	return "dir"
}

// SetReview marks a function as reviewed or unreviewed for the current commit.
func (s *Service) SetReview(ctx context.Context, repoID, functionID string, reviewed bool) (*model.Coverage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	repo, err := s.store.Get(repoID)
	if err != nil {
		return nil, err
	}
	snap, err := s.snapshotLocked(ctx, repo)
	if err != nil {
		return nil, err
	}
	path := ""
	found := false
	for _, fn := range snap.all {
		if fn.ID == functionID {
			found = true
			path = fn.Path
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("function not found")
	}

	reviews, err := s.store.LoadReviews(repoID)
	if err != nil {
		return nil, err
	}
	commit := reviewKey(repo.Commit)
	if reviews[commit] == nil {
		reviews[commit] = map[string]bool{}
	}
	if reviewed {
		reviews[commit][functionID] = true
	} else {
		delete(reviews[commit], functionID)
	}
	if err := s.store.SaveReviews(repoID, reviews); err != nil {
		return nil, err
	}
	return buildCoverage(repo, snap, reviews, path), nil
}

func (s *Service) snapshotLocked(ctx context.Context, repo *model.Repo) (*snapshot, error) {
	if snap, ok := s.cache[repo.ID]; ok && snap != nil && snap.commit == repo.Commit && time.Since(snap.builtAt) < 10*time.Minute {
		return snap, nil
	}
	ref, err := analysis.ParseRepoURL(repo.URL)
	if err != nil {
		return nil, err
	}
	root := s.store.SourceDir(repo.ID)
	ignorer := NewIgnorer(root, repo.Ignore)
	snap := &snapshot{commit: repo.Commit, files: map[string]fileFuncs{}}

	err = filepath.WalkDir(root, func(p string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if p != root && analysis.ShouldSkipDir(name) {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if ignorer.Ignored(rel) {
			if analysis.IsSourceFile(rel) && !analysis.IsBinaryName(name) {
				snap.ignoredFiles++
			}
			return nil
		}
		if !analysis.IsSourceFile(rel) || analysis.IsBinaryName(name) {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() > 2*1024*1024 {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		lang := analysis.LanguageFor(rel)
		funcs := reviewableFuncs(analysis.ExtractFunctions(rel, data, lang))
		if len(funcs) == 0 {
			return nil
		}
		used := map[string]int{}
		ids := make([]string, len(funcs))
		for i, fn := range funcs {
			ids[i] = analysis.FunctionID(rel, fn.Name, used)
		}
		snap.files[rel] = fileFuncs{funcs: funcs, ids: ids}
		return nil
	})
	if err != nil {
		return nil, err
	}

	changed, parent, available := s.computeChanged(ctx, repo, ref, root, snap.files)
	snap.parent = parent
	snap.changedAvailable = available

	for path, ff := range snap.files {
		for i, fn := range ff.funcs {
			id := ff.ids[i]
			snap.all = append(snap.all, model.Function{
				ID: id, Path: path, Name: fn.Name, Kind: fn.Kind,
				StartLine: fn.StartLine, EndLine: fn.EndLine, Changed: changed[id],
			})
		}
	}
	sort.Slice(snap.all, func(i, j int) bool {
		if snap.all[i].Path != snap.all[j].Path {
			return snap.all[i].Path < snap.all[j].Path
		}
		return snap.all[i].StartLine < snap.all[j].StartLine
	})
	snap.builtAt = time.Now()
	s.cache[repo.ID] = snap
	return snap, nil
}

func (s *Service) computeChanged(ctx context.Context, repo *model.Repo, ref *analysis.RepoRef, root string, files map[string]fileFuncs) (map[string]bool, string, bool) {
	changed := map[string]bool{}
	markFile := func(path string) {
		if ff, ok := files[path]; ok {
			for _, id := range ff.ids {
				changed[id] = true
			}
		}
	}
	markAll := func() {
		for path := range files {
			markFile(path)
		}
	}

	if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
		parent, ok := analysis.GitParent(ctx, root)
		if !ok {
			markAll()
			return changed, "", true
		}
		changedFiles, err := analysis.GitChangedFiles(ctx, root, parent)
		if err != nil {
			markAll()
			return changed, shortSHA(parent), true
		}
		for _, path := range changedFiles {
			current, ok := files[path]
			if !ok {
				continue
			}
			content, err := analysis.GitShowFile(ctx, root, parent, path)
			if err != nil {
				markFile(path)
				continue
			}
			markDifferences(path, current, content, changed)
		}
		return changed, shortSHA(parent), true
	}

	full, parent, err := analysis.GitHubCommitInfo(ctx, ref, repo.Commit, s.cfg.GitHubToken)
	if err != nil {
		return changed, "", false
	}
	if parent == "" {
		markAll()
		return changed, shortSHA(full), true
	}
	changedFiles, err := analysis.GitHubCompareFiles(ctx, ref, parent, full, s.cfg.GitHubToken)
	if err != nil {
		return changed, shortSHA(parent), false
	}
	for _, cf := range changedFiles {
		if cf.Status == "removed" {
			continue
		}
		path := cf.Filename
		current, ok := files[path]
		if !ok {
			continue
		}
		if cf.Status == "added" {
			markFile(path)
			continue
		}
		parentPath := path
		if cf.PreviousFilename != "" {
			parentPath = cf.PreviousFilename
		}
		content, err := analysis.GitHubRawFile(ctx, ref, parent, parentPath, s.cfg.GitHubToken)
		if err != nil || content == nil {
			markFile(path)
			continue
		}
		markDifferences(path, current, content, changed)
	}
	return changed, shortSHA(parent), true
}

func markDifferences(path string, current fileFuncs, parentContent []byte, changed map[string]bool) {
	parentFuncs := analysis.ExtractFunctions(path, parentContent, analysis.LanguageFor(path))
	used := map[string]int{}
	hashes := make(map[string]string, len(parentFuncs))
	for _, fn := range parentFuncs {
		id := analysis.FunctionID(path, fn.Name, used)
		hashes[id] = fn.BodyHash
	}
	for i, id := range current.ids {
		hash, ok := hashes[id]
		if !ok || hash != current.funcs[i].BodyHash {
			changed[id] = true
		}
	}
}

func buildCoverage(repo *model.Repo, snap *snapshot, reviews map[string]map[string]bool, filterPath string) *model.Coverage {
	commitKey := reviewKey(repo.Commit)
	reviewed := reviews[commitKey]
	cov := &model.Coverage{
		Commit:           repo.Commit,
		Parent:           snap.parent,
		ChangedAvailable: snap.changedAvailable,
		IgnoredFiles:     snap.ignoredFiles,
		Functions:        []model.Function{},
		UpdatedAt:        time.Now().UTC().Format(time.RFC3339),
	}
	for _, fn := range snap.all {
		fn.Reviewed = reviewed[fn.ID]
		cov.TotalFunctions++
		if fn.Changed {
			cov.ChangedFunctions++
		}
		if fn.Reviewed {
			cov.ReviewedFunctions++
			if fn.Changed {
				cov.ReviewedChanged++
			}
		}
		if filterPath == "" || fn.Path == filterPath {
			cov.Functions = append(cov.Functions, fn)
		}
	}
	cov.OverallPercent = percent(cov.ReviewedFunctions, cov.TotalFunctions)
	if cov.ChangedAvailable {
		cov.ChangedPercent = percent(cov.ReviewedChanged, cov.ChangedFunctions)
	}
	return cov
}

func percent(part, total int) int {
	if total == 0 {
		return 100
	}
	return int(float64(part)/float64(total)*100 + 0.5)
}

func reviewKey(commit string) string {
	if strings.TrimSpace(commit) == "" {
		return "unknown"
	}
	return commit
}

func shortSHA(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}
