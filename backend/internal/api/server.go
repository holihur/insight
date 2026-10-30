// Package api exposes the Code Insight HTTP API and serves the web app.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/codeinsight/code-insight/internal/agent"
	"github.com/codeinsight/code-insight/internal/analysis"
	"github.com/codeinsight/code-insight/internal/config"
	covsvc "github.com/codeinsight/code-insight/internal/coverage"
	"github.com/codeinsight/code-insight/internal/llm"
	"github.com/codeinsight/code-insight/internal/model"
	"github.com/codeinsight/code-insight/internal/store"
	webassets "github.com/codeinsight/code-insight/web"
)

// Server handles HTTP requests.
type Server struct {
	cfg      config.Config
	store    *store.Store
	gen      *agent.Generator
	chat     *agent.Chatter
	coverage *covsvc.Service
	mu       sync.Mutex
	running  map[string]bool
}

// New creates the API server.
func New(cfg config.Config, st *store.Store, gen *agent.Generator, chat *agent.Chatter, cov *covsvc.Service) *Server {
	return &Server{cfg: cfg, store: st, gen: gen, chat: chat, coverage: cov, running: map[string]bool{}}
}

// Handler builds the complete HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("GET /api/repos", s.listRepos)
	mux.HandleFunc("POST /api/repos", s.createRepo)
	mux.HandleFunc("GET /api/repos/{id}", s.getRepo)
	mux.HandleFunc("DELETE /api/repos/{id}", s.deleteRepo)
	mux.HandleFunc("POST /api/repos/{id}/regenerate", s.regenerate)
	mux.HandleFunc("GET /api/repos/{id}/index", s.getIndex)
	mux.HandleFunc("GET /api/repos/{id}/file", s.getFile)
	mux.HandleFunc("GET /api/repos/{id}/coverage", s.getCoverage)
	mux.HandleFunc("POST /api/repos/{id}/coverage/review", s.reviewFunction)
	mux.HandleFunc("PUT /api/repos/{id}/ignore", s.updateIgnore)
	mux.HandleFunc("POST /api/repos/{id}/chat", s.chatStream)
	mux.Handle("/", s.staticHandler())

	return s.recoverMW(s.corsMW(s.logMW(mux)))
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"provider": s.cfg.Model.Provider,
		"model":    s.cfg.Model.Model,
		"label":    llm.Label(s.cfg.Model),
		"demo":     s.cfg.Model.Demo(),
		"time":     time.Now().UTC(),
	})
}

func (s *Server) listRepos(w http.ResponseWriter, r *http.Request) {
	repos := s.store.List()
	out := make([]*model.Repo, 0, len(repos))
	for _, repo := range repos {
		out = append(out, repoSummary(repo))
	}
	writeJSON(w, http.StatusOK, map[string]any{"repos": out})
}

type createRepoRequest struct {
	URL   string `json:"url"`
	Force bool   `json:"force"`
}

func (s *Server) createRepo(w http.ResponseWriter, r *http.Request) {
	var req createRepoRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	ref, err := analysis.ParseRepoURL(req.URL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	existing := s.store.FindByURL(ref.URL)
	if existing != nil {
		if s.isRunning(existing.ID) {
			writeError(w, http.StatusConflict, "this repository is already being analyzed")
			return
		}
		if !req.Force {
			writeJSON(w, http.StatusOK, existing)
			return
		}
		if existing.Status == model.StatusReady {
			if !s.startJob(existing.ID, true) {
				writeError(w, http.StatusConflict, "this repository is already being analyzed")
				return
			}
			writeJSON(w, http.StatusAccepted, existing)
			return
		}
		writeJSON(w, http.StatusOK, existing)
		return
	}

	now := time.Now().UTC()
	repo := &model.Repo{
		ID:        ref.ID,
		URL:       ref.URL,
		WebURL:    ref.WebURL,
		Owner:     ref.Owner,
		Name:      ref.Name,
		Status:    model.StatusQueued,
		Progress:  0,
		Message:   "Queued for analysis",
		Provider:  llm.Label(s.cfg.Model),
		Demo:      s.cfg.Model.Demo(),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.store.Create(repo); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if !s.startJob(repo.ID, false) {
		writeError(w, http.StatusConflict, "this repository is already being analyzed")
		return
	}
	writeJSON(w, http.StatusCreated, repo)
}

func (s *Server) getRepo(w http.ResponseWriter, r *http.Request) {
	repo, err := s.store.Get(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "repository not found")
		return
	}
	writeJSON(w, http.StatusOK, repo)
}

func (s *Server) deleteRepo(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if s.isRunning(id) {
		writeError(w, http.StatusConflict, "wait for the current analysis to finish before deleting")
		return
	}
	if err := s.store.Delete(id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "repository not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) regenerate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	repo, err := s.store.Get(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "repository not found")
		return
	}
	if !s.startJob(id, true) {
		writeError(w, http.StatusConflict, "analysis is already running for this repository")
		return
	}
	updated, _ := s.store.Get(id)
	if updated == nil {
		updated = repo
	}
	writeJSON(w, http.StatusAccepted, updated)
}

func (s *Server) getIndex(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.store.Get(id); err != nil {
		writeError(w, http.StatusNotFound, "repository not found")
		return
	}
	idx, err := s.store.LoadIndex(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "index is not ready yet")
		return
	}
	writeJSON(w, http.StatusOK, idx)
}

func (s *Server) getFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.store.Get(id); err != nil {
		writeError(w, http.StatusNotFound, "repository not found")
		return
	}
	rel := r.URL.Query().Get("path")
	if rel == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}
	full, err := analysis.SafeJoin(s.store.SourceDir(id), rel)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	info, err := os.Stat(full)
	if err != nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	if info.IsDir() {
		writeError(w, http.StatusBadRequest, "path is a directory")
		return
	}
	if info.Size() > 2*1024*1024 {
		writeError(w, http.StatusRequestEntityTooLarge, "file is too large to display (max 2 MB)")
		return
	}
	data, err := os.ReadFile(full)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if isBinary(data) {
		writeError(w, http.StatusUnsupportedMediaType, "binary files cannot be displayed")
		return
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if len(lines) > 1 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	total := len(lines)
	start := queryInt(r, "start", 1)
	if start < 1 {
		start = 1
	}
	if start > total {
		start = total
	}
	end := queryInt(r, "end", 0)
	if end <= 0 || end > total {
		end = total
	}
	if end > start+5000 {
		end = start + 5000
	}
	content := strings.Join(lines[start-1:end], "\n")
	writeJSON(w, http.StatusOK, model.FileContent{
		Path:     filepath.ToSlash(rel),
		Content:  content,
		Language: analysis.LanguageFor(rel),
		Lines:    end - start + 1,
		Start:    start,
		End:      end,
		Total:    total,
	})
}

func (s *Server) getCoverage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.store.Get(id); err != nil {
		writeError(w, http.StatusNotFound, "repository not found")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	data, err := s.coverage.Get(ctx, id, r.URL.Query().Get("path"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func (s *Server) reviewFunction(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		FunctionID string `json:"functionId"`
		Reviewed   bool   `json:"reviewed"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.FunctionID) == "" {
		writeError(w, http.StatusBadRequest, "functionId is required")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	data, err := s.coverage.SetReview(ctx, id, req.FunctionID, req.Reviewed)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func (s *Server) updateIgnore(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Patterns []string `json:"patterns"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(req.Patterns) > 200 {
		writeError(w, http.StatusBadRequest, "too many ignore patterns (max 200)")
		return
	}
	cleaned := make([]string, 0, len(req.Patterns))
	for _, pattern := range req.Patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" || len(pattern) > 300 {
			continue
		}
		cleaned = append(cleaned, pattern)
	}
	repo, err := s.store.Update(id, func(repo *model.Repo) {
		repo.Ignore = cleaned
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "repository not found")
		return
	}
	s.coverage.Invalidate(id)
	writeJSON(w, http.StatusOK, repo)
}

func (s *Server) startJob(id string, regenerate bool) bool {
	s.mu.Lock()
	if s.running[id] {
		s.mu.Unlock()
		return false
	}
	s.running[id] = true
	s.mu.Unlock()

	go func() {
		defer func() {
			s.mu.Lock()
			delete(s.running, id)
			s.mu.Unlock()
		}()
		ctx, cancel := context.WithTimeout(context.Background(), s.cfg.RequestTimeout)
		defer cancel()
		var err error
		if regenerate {
			err = s.gen.Regenerate(ctx, id)
		} else {
			err = s.gen.Analyze(ctx, id)
		}
		if err != nil {
			log.Printf("job %s finished with error: %v", id, err)
		}
	}()
	return true
}

func (s *Server) isRunning(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running[id]
}

func (s *Server) chatStream(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	repo, err := s.store.Get(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "repository not found")
		return
	}
	var req struct {
		Message   string `json:"message"`
		SessionID string `json:"sessionId"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.Message) == "" {
		writeError(w, http.StatusBadRequest, "message is required")
		return
	}
	if req.SessionID == "" {
		req.SessionID = uuid.NewString()
	}
	if len(req.SessionID) > 120 {
		writeError(w, http.StatusBadRequest, "invalid session id")
		return
	}
	if _, err := os.Stat(s.store.SourceDir(id)); err != nil {
		writeError(w, http.StatusConflict, "repository is still being prepared")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming is not supported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	emit := func(event agent.ChatEvent) error {
		data, err := json.Marshal(event)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}

	err = s.chat.Stream(r.Context(), repo, req.SessionID, req.Message, emit)
	if err != nil {
		if r.Context().Err() == nil {
			_ = emit(agent.ChatEvent{Type: "error", Message: err.Error()})
		}
		return
	}
	_ = emit(agent.ChatEvent{Type: "done", SessionID: req.SessionID})
}

func (s *Server) staticHandler() http.Handler {
	var webFS fs.FS
	if s.cfg.WebDir != "" {
		if _, err := os.Stat(filepath.Join(s.cfg.WebDir, "index.html")); err == nil {
			webFS = os.DirFS(s.cfg.WebDir)
		}
	}
	if webFS == nil && webassets.Available() {
		webFS = webassets.FS()
	}
	if webFS == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			writeJSON(w, http.StatusOK, map[string]any{
				"name":    "Code Insight API",
				"message": "No frontend build found. Run `mise run build` (or `pnpm --dir frontend build`) to embed it, or use the Vite dev server on :5173.",
				"api":     "/api/health",
			})
		})
	}

	fileServer := http.FileServer(http.FS(webFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := path.Clean("/" + r.URL.Path)
		rel := strings.TrimPrefix(clean, "/")
		if rel == "" {
			rel = "index.html"
		}
		if f, err := webFS.Open(rel); err == nil {
			_ = f.Close()
			if strings.HasPrefix(rel, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				w.Header().Set("Cache-Control", "no-cache")
			}
			fileServer.ServeHTTP(w, r)
			return
		}
		data, err := fs.ReadFile(webFS, "index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(data)
	})
}

func (s *Server) corsMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := s.cfg.CORSOrigin
		if origin == "" {
			origin = "*"
		}
		if origin == "*" {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) logMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(sw, r)
		if !strings.HasPrefix(r.URL.Path, "/api/health") {
			log.Printf("%s %s %d %s", r.Method, r.URL.Path, sw.status, time.Since(start).Round(time.Millisecond))
		}
	})
}

func (s *Server) recoverMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if val := recover(); val != nil {
				log.Printf("panic serving %s: %v", r.URL.Path, val)
				writeError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New("request body is required")
		}
		return fmt.Errorf("invalid request body: %w", err)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func queryInt(r *http.Request, key string, fallback int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func isBinary(data []byte) bool {
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

func repoSummary(repo *model.Repo) *model.Repo {
	cp := *repo
	cp.Stats.Languages = append([]model.Language(nil), repo.Stats.Languages...)
	if repo.Wiki != nil {
		wiki := *repo.Wiki
		wiki.Pages = make([]*model.Page, 0, len(repo.Wiki.Pages))
		for _, page := range repo.Wiki.Pages {
			pageCopy := *page
			pageCopy.Content = ""
			wiki.Pages = append(wiki.Pages, &pageCopy)
		}
		cp.Wiki = &wiki
	}
	return &cp
}
