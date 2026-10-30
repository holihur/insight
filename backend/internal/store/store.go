// Package store persists repository metadata and analysis indexes on disk.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/codeinsight/code-insight/internal/model"
)

// ErrNotFound is returned when a repository id is unknown.
var ErrNotFound = errors.New("repository not found")

// Store is a small, file-backed repository store. Metadata lives in
// data/<id>/meta.json and the analysis snapshot in data/<id>/index.json.
type Store struct {
	root  string
	mu    sync.RWMutex
	repos map[string]*model.Repo
}

// New loads existing repositories from root.
func New(root string) (*Store, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	s := &Store{root: root, repos: make(map[string]*model.Repo)}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		repo, err := readMeta(filepath.Join(root, entry.Name(), "meta.json"))
		if err != nil {
			continue
		}
		if repo.Status.Active() {
			repo.Status = model.StatusError
			repo.Error = "Analysis was interrupted by a server restart. Regenerate the wiki to continue."
			repo.Message = "Interrupted"
			repo.UpdatedAt = time.Now().UTC()
			_ = writeJSON(filepath.Join(root, repo.ID, "meta.json"), repo)
		}
		s.repos[repo.ID] = repo
	}
	return s, nil
}

// Root returns the data directory.
func (s *Store) Root() string { return s.root }

// RepoDir returns the on-disk directory for a repository.
func (s *Store) RepoDir(id string) string { return filepath.Join(s.root, id) }

// SourceDir returns the clone directory for a repository.
func (s *Store) SourceDir(id string) string { return filepath.Join(s.root, id, "repo") }

// List returns all repositories, newest first.
func (s *Store) List() []*model.Repo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*model.Repo, 0, len(s.repos))
	for _, repo := range s.repos {
		out = append(out, cloneRepo(repo))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out
}

// Get returns a copy of a repository.
func (s *Store) Get(id string) (*model.Repo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	repo, ok := s.repos[id]
	if !ok {
		return nil, ErrNotFound
	}
	return cloneRepo(repo), nil
}

// FindByURL returns a repository matching a normalized URL.
func (s *Store) FindByURL(url string) *model.Repo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, repo := range s.repos {
		if repo.URL == url {
			return cloneRepo(repo)
		}
	}
	return nil
}

// Create persists a new repository.
func (s *Store) Create(repo *model.Repo) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.repos[repo.ID]; exists {
		return fmt.Errorf("repository %q already exists", repo.ID)
	}
	if err := os.MkdirAll(s.RepoDir(repo.ID), 0o755); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(s.RepoDir(repo.ID), "meta.json"), repo); err != nil {
		return err
	}
	s.repos[repo.ID] = cloneRepo(repo)
	return nil
}

// Update applies fn to a repository under a write lock and persists the result.
func (s *Store) Update(id string, fn func(*model.Repo)) (*model.Repo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	repo, ok := s.repos[id]
	if !ok {
		return nil, ErrNotFound
	}
	fn(repo)
	repo.UpdatedAt = time.Now().UTC()
	if err := writeJSON(filepath.Join(s.RepoDir(id), "meta.json"), repo); err != nil {
		return nil, err
	}
	return cloneRepo(repo), nil
}

// Delete removes a repository and all of its files.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.repos[id]; !ok {
		return ErrNotFound
	}
	if err := os.RemoveAll(s.RepoDir(id)); err != nil {
		return err
	}
	delete(s.repos, id)
	return nil
}

// LoadReviews returns human-review marks keyed by commit and function id.
func (s *Store) LoadReviews(id string) (map[string]map[string]bool, error) {
	var reviews map[string]map[string]bool
	err := readJSON(filepath.Join(s.RepoDir(id), "reviews.json"), &reviews)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]map[string]bool{}, nil
		}
		return nil, err
	}
	if reviews == nil {
		reviews = map[string]map[string]bool{}
	}
	return reviews, nil
}

// SaveReviews persists human-review marks.
func (s *Store) SaveReviews(id string, reviews map[string]map[string]bool) error {
	if reviews == nil {
		reviews = map[string]map[string]bool{}
	}
	return writeJSON(filepath.Join(s.RepoDir(id), "reviews.json"), reviews)
}

// SaveIndex stores the static analysis snapshot.
func (s *Store) SaveIndex(id string, idx *model.Index) error {
	return writeJSON(filepath.Join(s.RepoDir(id), "index.json"), idx)
}

// LoadIndex reads the static analysis snapshot.
func (s *Store) LoadIndex(id string) (*model.Index, error) {
	var idx model.Index
	if err := readJSON(filepath.Join(s.RepoDir(id), "index.json"), &idx); err != nil {
		return nil, err
	}
	return &idx, nil
}

func readMeta(path string) (*model.Repo, error) {
	var repo model.Repo
	if err := readJSON(path, &repo); err != nil {
		return nil, err
	}
	if repo.ID == "" {
		return nil, errors.New("invalid metadata")
	}
	return &repo, nil
}

func readJSON(path string, dst any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dst)
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func cloneRepo(repo *model.Repo) *model.Repo {
	if repo == nil {
		return nil
	}
	cp := *repo
	cp.Stats.Languages = append([]model.Language(nil), repo.Stats.Languages...)
	cp.Ignore = append([]string(nil), repo.Ignore...)
	if repo.Wiki != nil {
		wiki := *repo.Wiki
		wiki.Pages = make([]*model.Page, 0, len(repo.Wiki.Pages))
		for _, page := range repo.Wiki.Pages {
			pageCopy := *page
			wiki.Pages = append(wiki.Pages, &pageCopy)
		}
		cp.Wiki = &wiki
	}
	return &cp
}
