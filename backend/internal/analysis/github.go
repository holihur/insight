// Package analysis clones repositories and builds a static index of their code.
package analysis

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// RepoRef is a normalized repository reference.
type RepoRef struct {
	URL      string // canonical URL without credentials
	WebURL   string // human-facing URL
	Owner    string
	Name     string
	CloneURL string // URL passed to git
	ID       string
}

var slugRe = regexp.MustCompile(`[^a-z0-9._-]+`)

// ParseRepoURL accepts GitHub/GitLab-style URLs, git@ SSH URLs and owner/repo
// shorthand, returning a normalized reference.
func ParseRepoURL(raw string) (*RepoRef, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("repository URL is required")
	}
	if len(raw) > 500 {
		return nil, errors.New("repository URL is too long")
	}

	var host, repoPath, cloneURL, webURL string

	switch {
	case strings.HasPrefix(raw, "git@"):
		// git@github.com:owner/repo.git
		rest := strings.TrimPrefix(raw, "git@")
		h, p, ok := strings.Cut(rest, ":")
		if !ok || h == "" || p == "" {
			return nil, errors.New("invalid SSH repository URL")
		}
		host = h
		repoPath = p
		webURL = "https://" + host + "/" + strings.TrimSuffix(p, ".git")
		cloneURL = raw
	case strings.HasPrefix(raw, "http://"), strings.HasPrefix(raw, "https://"):
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" {
			return nil, errors.New("invalid repository URL")
		}
		host = u.Hostname()
		repoPath = strings.TrimPrefix(u.Path, "/")
		webURL = "https://" + u.Host + "/" + strings.TrimSuffix(repoPath, ".git")
		cloneURL = webURL + ".git"
	case strings.HasPrefix(raw, "ssh://"):
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" || u.Path == "" {
			return nil, errors.New("invalid SSH repository URL")
		}
		host = u.Hostname()
		repoPath = strings.TrimPrefix(u.Path, "/")
		webURL = "https://" + host + "/" + strings.TrimSuffix(repoPath, ".git")
		cloneURL = raw
	default:
		// owner/repo shorthand, GitHub by default.
		if strings.Contains(raw, " ") || !strings.Contains(raw, "/") {
			return nil, errors.New("use a full URL or owner/repo, e.g. gin-gonic/gin")
		}
		host = "github.com"
		repoPath = raw
		webURL = "https://github.com/" + strings.TrimSuffix(raw, ".git")
		cloneURL = webURL + ".git"
	}

	repoPath = strings.Trim(strings.TrimSuffix(repoPath, ".git"), "/")
	parts := strings.Split(repoPath, "/")
	if len(parts) < 2 {
		return nil, errors.New("repository URL must include an owner and a name")
	}
	name := parts[len(parts)-1]
	owner := strings.Join(parts[:len(parts)-1], "/")
	if name == "" || owner == "" || strings.Contains(name, "..") || strings.Contains(owner, "..") {
		return nil, errors.New("invalid repository path")
	}
	if !strings.Contains(host, ".") {
		return nil, errors.New("invalid repository host")
	}

	canonical := strings.ToLower(host + "/" + owner + "/" + name)
	idSlug := slugRe.ReplaceAllString(strings.ToLower(owner+"-"+name), "-")
	idSlug = strings.Trim(idSlug, "-.")
	if len(idSlug) > 64 {
		idSlug = idSlug[:64]
	}
	sum := sha1.Sum([]byte(canonical))
	id := idSlug + "-" + hex.EncodeToString(sum[:])[:7]

	return &RepoRef{
		URL:      "https://" + host + "/" + owner + "/" + name,
		WebURL:   webURL,
		Owner:    owner,
		Name:     name,
		CloneURL: cloneURL,
		ID:       id,
	}, nil
}

// Clone performs a shallow clone and returns the current branch and commit.
func Clone(ctx context.Context, ref *RepoRef, dest, token string) (branch, commit string, err error) {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", "", err
	}
	if _, err := os.Stat(dest); err == nil {
		if err := os.RemoveAll(dest); err != nil {
			return "", "", err
		}
	}

	cloneURL := ref.CloneURL
	if token != "" && strings.HasPrefix(cloneURL, "https://") && strings.Contains(cloneURL, "github.com") {
		cloneURL = strings.Replace(cloneURL, "https://", "https://x-access-token:"+token+"@", 1)
	}

	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "--single-branch", "--no-tags", cloneURL, dest)
	configureProcess(cmd)
	cmd.WaitDelay = 5 * time.Second
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_LFS_SKIP_SMUDGE=1",
		"GIT_ASKPASS=/bin/true",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if token != "" {
			msg = strings.ReplaceAll(msg, token, "***")
		}
		if len(msg) > 800 {
			msg = msg[len(msg)-800:]
		}
		gitErr := fmt.Errorf("git clone failed: %w: %s", err, msg)

		// Some networks block the git smart protocol but allow codeload
		// tarballs. Fall back to an archive for public GitHub repositories.
		if strings.EqualFold(refHost(ref), "github.com") {
			archiveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Minute)
			branch, commit, archiveErr := downloadGitHubArchive(archiveCtx, ref, dest, token)
			cancel()
			if archiveErr == nil {
				return branch, commit, nil
			}
			return "", "", fmt.Errorf("%v; archive fallback failed: %w", gitErr, archiveErr)
		}
		return "", "", gitErr
	}

	branch = gitOutput(ctx, dest, "rev-parse", "--abbrev-ref", "HEAD")
	commit = gitOutput(ctx, dest, "rev-parse", "--short=8", "HEAD")
	return branch, commit, nil
}

func refHost(ref *RepoRef) string {
	u, err := url.Parse(ref.URL)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

type githubRepoMeta struct {
	DefaultBranch string `json:"default_branch"`
}

func downloadGitHubArchive(ctx context.Context, ref *RepoRef, dest, token string) (string, string, error) {
	if err := os.RemoveAll(dest); err != nil {
		return "", "", err
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return "", "", err
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	branch, commit := githubDefaults(ctx, client, ref, token)
	branches := []string{branch, "main", "master"}
	seen := map[string]bool{}
	var lastErr error

	for _, candidate := range branches {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" || seen[candidate] {
			continue
		}
		seen[candidate] = true
		archiveURL := fmt.Sprintf("https://codeload.github.com/%s/%s/tar.gz/refs/heads/%s", ref.Owner, ref.Name, candidate)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, archiveURL, nil)
		if err != nil {
			return "", "", err
		}
		req.Header.Set("User-Agent", "code-insight")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			lastErr = fmt.Errorf("archive for %s returned HTTP %d", candidate, resp.StatusCode)
			continue
		}
		err = extractTarball(resp.Body, dest)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		return candidate, commit, nil
	}
	if lastErr == nil {
		lastErr = errors.New("no downloadable branch found")
	}
	return "", "", lastErr
}

func githubDefaults(ctx context.Context, client *http.Client, ref *RepoRef, token string) (branch, commit string) {
	branch = "main"
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s", ref.Owner, ref.Name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return branch, ""
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "code-insight")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return branch, ""
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		var meta githubRepoMeta
		if json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&meta) == nil && meta.DefaultBranch != "" {
			branch = meta.DefaultBranch
		}
	}

	commitURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits/%s", ref.Owner, ref.Name, url.PathEscape(branch))
	commitReq, err := http.NewRequestWithContext(ctx, http.MethodGet, commitURL, nil)
	if err != nil {
		return branch, ""
	}
	commitReq.Header.Set("Accept", "application/vnd.github+json")
	commitReq.Header.Set("User-Agent", "code-insight")
	if token != "" {
		commitReq.Header.Set("Authorization", "Bearer "+token)
	}
	commitResp, err := client.Do(commitReq)
	if err != nil {
		return branch, ""
	}
	defer commitResp.Body.Close()
	if commitResp.StatusCode == http.StatusOK {
		var payload struct {
			SHA string `json:"sha"`
		}
		if json.NewDecoder(io.LimitReader(commitResp.Body, 1<<20)).Decode(&payload) == nil && len(payload.SHA) >= 8 {
			commit = payload.SHA[:8]
		}
	}
	return branch, commit
}

func extractTarball(body io.Reader, dest string) error {
	gz, err := gzip.NewReader(body)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read archive: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeDir {
			continue
		}
		if hdr.Size > 64*1024*1024 {
			continue
		}
		name := path.Clean(strings.TrimPrefix(hdr.Name, "./"))
		parts := strings.Split(name, "/")
		if len(parts) < 2 {
			continue // strip the single top-level archive directory
		}
		rel := path.Join(parts[1:]...)
		if rel == "" || rel == "." || strings.HasPrefix(rel, "../") {
			continue
		}
		target, err := SafeJoin(dest, rel)
		if err != nil {
			continue
		}
		if hdr.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		f, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		_, copyErr := io.CopyN(f, tr, hdr.Size)
		closeErr := f.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func gitOutput(ctx context.Context, dir string, args ...string) string {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// SafeJoin resolves rel inside root, rejecting traversal and absolute paths.
func SafeJoin(root, rel string) (string, error) {
	rel = filepath.FromSlash(strings.TrimSpace(rel))
	if rel == "" || rel == "." {
		return root, nil
	}
	if filepath.IsAbs(rel) {
		return "", errors.New("absolute paths are not allowed")
	}
	clean := filepath.Clean(rel)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errors.New("path escapes the repository")
	}
	full := filepath.Join(root, clean)

	rootReal, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	// Resolve the nearest existing ancestor to catch symlink escapes.
	probe := full
	for {
		real, err := filepath.EvalSymlinks(probe)
		if err == nil {
			if real != rootReal && !strings.HasPrefix(real, rootReal+string(filepath.Separator)) {
				return "", errors.New("path escapes the repository")
			}
			break
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			break
		}
		probe = parent
	}
	return full, nil
}

// LanguageFor returns the display name for a file extension.
func LanguageFor(name string) string {
	ext := strings.ToLower(path.Ext(name))
	if lang, ok := languageByExt[ext]; ok {
		return lang
	}
	base := strings.ToLower(path.Base(name))
	if lang, ok := languageByFile[base]; ok {
		return lang
	}
	return ""
}

var languageByExt = map[string]string{
	".go": "Go", ".js": "JavaScript", ".mjs": "JavaScript", ".cjs": "JavaScript",
	".jsx": "JavaScript", ".ts": "TypeScript", ".tsx": "TypeScript", ".mts": "TypeScript",
	".py": "Python", ".rb": "Ruby", ".php": "PHP", ".java": "Java", ".kt": "Kotlin",
	".kts": "Kotlin", ".scala": "Scala", ".cs": "C#", ".fs": "F#", ".c": "C", ".h": "C",
	".cc": "C++", ".cpp": "C++", ".cxx": "C++", ".hpp": "C++", ".hh": "C++", ".m": "Objective-C",
	".mm": "Objective-C", ".swift": "Swift", ".rs": "Rust", ".dart": "Dart", ".ex": "Elixir",
	".exs": "Elixir", ".erl": "Erlang", ".hs": "Haskell", ".lua": "Lua", ".pl": "Perl",
	".r": "R", ".jl": "Julia", ".sh": "Shell", ".bash": "Shell", ".zsh": "Shell",
	".fish": "Shell", ".ps1": "PowerShell", ".sql": "SQL", ".proto": "Protocol Buffers",
	".graphql": "GraphQL", ".vue": "Vue", ".svelte": "Svelte", ".html": "HTML", ".htm": "HTML",
	".css": "CSS", ".scss": "SCSS", ".sass": "SCSS", ".less": "Less", ".md": "Markdown",
	".mdx": "Markdown", ".rst": "reStructuredText", ".json": "JSON", ".yaml": "YAML",
	".yml": "YAML", ".toml": "TOML", ".xml": "XML", ".ini": "INI", ".tf": "Terraform",
	".hcl": "HCL", ".sol": "Solidity", ".zig": "Zig", ".nim": "Nim", ".clj": "Clojure",
	".groovy": "Groovy", ".gradle": "Gradle", ".tex": "LaTeX",
}

var languageByFile = map[string]string{
	"dockerfile": "Dockerfile", "makefile": "Makefile", "cmakelists.txt": "CMake",
	"jenkinsfile": "Groovy", "gemfile": "Ruby", "rakefile": "Ruby", "procfile": "Procfile",
	"go.mod": "Go module", "go.sum": "Go checksums", "package.json": "JSON",
	"tsconfig.json": "JSON", "cargo.toml": "TOML", "pyproject.toml": "TOML",
	"readme": "Markdown", "readme.md": "Markdown", "readme.rst": "Markdown",
	"license": "Text", "licence": "Text", "copying": "Text", "notice": "Text",
	"authors": "Text", "changelog": "Text", "changes": "Text", "contributing": "Text",
	"makefile.am": "Makefile", "justfile": "Makefile",
}

var binaryExt = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".ico": true,
	".pdf": true, ".zip": true, ".gz": true, ".tgz": true, ".bz2": true, ".xz": true,
	".7z": true, ".rar": true, ".exe": true, ".dll": true, ".so": true, ".dylib": true,
	".bin": true, ".o": true, ".a": true, ".class": true, ".jar": true, ".war": true,
	".woff": true, ".woff2": true, ".ttf": true, ".otf": true, ".eot": true, ".mp3": true,
	".mp4": true, ".mov": true, ".avi": true, ".mkv": true, ".wav": true, ".sqlite": true,
	".db": true, ".lockb": true, ".pyc": true, ".wasm": true,
}
