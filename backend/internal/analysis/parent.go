package analysis

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"path"
	"strings"
	"time"
)

// ChangedFile is a file reported by the GitHub compare API.
type ChangedFile struct {
	Filename         string `json:"filename"`
	PreviousFilename string `json:"previous_filename"`
	Status           string `json:"status"`
}

// GitHubCommitInfo resolves a commit and returns its full SHA and first parent.
func GitHubCommitInfo(ctx context.Context, ref *RepoRef, commit, token string) (string, string, error) {
	if refHost(ref) != "github.com" {
		return "", "", fmt.Errorf("commit lookup is only supported for GitHub")
	}
	if commit == "" {
		commit = "HEAD"
	}
	endpoint := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits/%s", ref.Owner, ref.Name, url.PathEscape(commit))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "code-insight")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("GitHub commit lookup returned HTTP %d", resp.StatusCode)
	}
	var payload struct {
		SHA     string `json:"sha"`
		Parents []struct {
			SHA string `json:"sha"`
		} `json:"parents"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return "", "", err
	}
	if len(payload.Parents) == 0 {
		return payload.SHA, "", nil
	}
	return payload.SHA, payload.Parents[0].SHA, nil
}

// GitHubCompareFiles lists files changed between two commits.
func GitHubCompareFiles(ctx context.Context, ref *RepoRef, base, head, token string) ([]ChangedFile, error) {
	if refHost(ref) != "github.com" {
		return nil, fmt.Errorf("comparison is only supported for GitHub")
	}
	endpoint := fmt.Sprintf("https://api.github.com/repos/%s/%s/compare/%s...%s",
		ref.Owner, ref.Name, url.PathEscape(base), url.PathEscape(head))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "code-insight")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub compare returned HTTP %d", resp.StatusCode)
	}
	var payload struct {
		Files []ChangedFile `json:"files"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Files, nil
}

// GitHubRawFile returns a file at a specific commit.
func GitHubRawFile(ctx context.Context, ref *RepoRef, commit, filePath, token string) ([]byte, error) {
	if refHost(ref) != "github.com" {
		return nil, fmt.Errorf("raw file lookup is only supported for GitHub")
	}
	clean := strings.TrimPrefix(path.Clean("/"+filePath), "/")
	parts := strings.Split(clean, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	endpoint := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s",
		ref.Owner, ref.Name, url.PathEscape(commit), strings.Join(parts, "/"))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "code-insight")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			return nil, nil
		}
		if resp.StatusCode == http.StatusOK {
			return io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		}
	}

	// Some networks block raw.githubusercontent.com. The contents API lives on
	// api.github.com and returns the same file, so use it as a fallback.
	apiPath := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s?ref=%s",
		ref.Owner, ref.Name, strings.Join(parts, "/"), url.QueryEscape(commit))
	apiReq, err := http.NewRequestWithContext(ctx, http.MethodGet, apiPath, nil)
	if err != nil {
		return nil, err
	}
	apiReq.Header.Set("Accept", "application/vnd.github+json")
	apiReq.Header.Set("User-Agent", "code-insight")
	if token != "" {
		apiReq.Header.Set("Authorization", "Bearer "+token)
	}
	apiResp, err := client.Do(apiReq)
	if err != nil {
		return nil, err
	}
	defer apiResp.Body.Close()
	if apiResp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if apiResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("contents API returned HTTP %d", apiResp.StatusCode)
	}
	var payload struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.NewDecoder(io.LimitReader(apiResp.Body, 8<<20)).Decode(&payload); err != nil {
		return nil, err
	}
	if payload.Encoding != "base64" {
		return nil, fmt.Errorf("unexpected contents encoding %q", payload.Encoding)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(payload.Content, "\n", ""))
	if err != nil {
		return nil, err
	}
	return decoded, nil
}

// GitParent returns HEAD~1 for a local checkout.
func GitParent(ctx context.Context, dir string) (string, bool) {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--verify", "HEAD~1")
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}

// GitChangedFiles lists files changed between parent and HEAD.
func GitChangedFiles(ctx context.Context, dir, parent string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "diff", "--name-only", parent, "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// GitShowFile returns a file from a git revision.
func GitShowFile(ctx context.Context, dir, rev, filePath string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "show", rev+":"+filePath)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ShouldSkipDir reports whether a directory is excluded from source analysis.
func ShouldSkipDir(name string) bool {
	return ignoredDirs[name] || (strings.HasPrefix(name, ".") && name != ".github")
}

// IsBinaryName reports whether a filename is known to be binary.
func IsBinaryName(name string) bool {
	return binaryExt[strings.ToLower(path.Ext(name))] || ignoredFileName(name)
}
