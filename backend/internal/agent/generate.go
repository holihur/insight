package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	adkmodel "google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool"
	"google.golang.org/genai"

	"github.com/codeinsight/code-insight/internal/analysis"
	"github.com/codeinsight/code-insight/internal/config"
	"github.com/codeinsight/code-insight/internal/llm"
	"github.com/codeinsight/code-insight/internal/model"
	"github.com/codeinsight/code-insight/internal/store"
)

// Generator runs the clone → index → plan → write pipeline.
type Generator struct {
	cfg      config.Config
	store    *store.Store
	llm      adkmodel.LLM
	sessions session.Service
	running  sync.Map
}

// NewGenerator creates a Generator. A nil model enables local demo mode.
func NewGenerator(cfg config.Config, st *store.Store, model adkmodel.LLM) *Generator {
	return &Generator{
		cfg:      cfg,
		store:    st,
		llm:      model,
		sessions: session.InMemoryService(),
	}
}

// Analyze clones (when needed), indexes and documents a repository.
func (g *Generator) Analyze(ctx context.Context, id string) error {
	return g.run(ctx, id, true)
}

// Regenerate re-indexes an existing checkout and rewrites the wiki.
func (g *Generator) Regenerate(ctx context.Context, id string) error {
	return g.run(ctx, id, false)
}

func (g *Generator) run(ctx context.Context, id string, allowClone bool) error {
	if _, loaded := g.running.LoadOrStore(id, true); loaded {
		return errors.New("analysis is already running for this repository")
	}
	defer g.running.Delete(id)

	repo, err := g.store.Get(id)
	if err != nil {
		return err
	}
	ref, err := analysis.ParseRepoURL(repo.URL)
	if err != nil {
		g.fail(id, err)
		return err
	}
	sourceDir := g.store.SourceDir(id)

	if _, statErr := os.Stat(sourceDir); statErr != nil {
		allowClone = true
	}

	if allowClone {
		if _, err := os.Stat(sourceDir); err != nil {
			g.update(id, func(r *model.Repo) {
				r.Status = model.StatusCloning
				r.Progress = 3
				r.Message = "Cloning repository…"
				r.Error = ""
			})
			cloneCtx, cancel := context.WithTimeout(ctx, g.cfg.CloneTimeout)
			branch, commit, cloneErr := analysis.Clone(cloneCtx, ref, sourceDir, g.cfg.GitHubToken)
			cancel()
			if cloneErr != nil {
				g.fail(id, cloneErr)
				return cloneErr
			}
			g.update(id, func(r *model.Repo) {
				r.Branch = branch
				r.Commit = commit
				r.Progress = 12
				r.Message = "Repository cloned"
			})
		}
	}

	g.update(id, func(r *model.Repo) {
		r.Status = model.StatusIndexing
		r.Progress = 15
		r.Message = "Indexing source tree…"
		r.Error = ""
	})

	idx, err := analysis.BuildIndex(sourceDir, g.cfg.MaxRepoFiles)
	if err != nil {
		g.fail(id, fmt.Errorf("index repository: %w", err))
		return err
	}
	if err := g.store.SaveIndex(id, idx); err != nil {
		log.Printf("save index %s: %v", id, err)
	}

	g.update(id, func(r *model.Repo) {
		r.Stats = idx.Stats
		r.Progress = 24
		r.Message = fmt.Sprintf("Indexed %d files · %d lines", idx.Stats.Files, idx.Stats.Lines)
	})

	wiki, err := g.generateWiki(ctx, id, ref, idx)
	if err != nil {
		g.fail(id, err)
		return err
	}

	g.update(id, func(r *model.Repo) {
		r.Wiki = wiki
		r.Status = model.StatusReady
		r.Progress = 100
		r.Error = ""
		r.Demo = wiki.Demo
		r.Provider = wiki.Provider
		r.Message = fmt.Sprintf("Wiki ready · %d pages", len(wiki.Pages))
	})
	return nil
}

func (g *Generator) generateWiki(ctx context.Context, id string, ref *analysis.RepoRef, idx *model.Index) (*model.Wiki, error) {
	root := g.store.SourceDir(id)
	provider := llm.Label(g.cfg.Model)

	if g.llm == nil {
		g.update(id, func(r *model.Repo) {
			r.Status = model.StatusGenerating
			r.Progress = 40
			r.Message = "Building local wiki (no model configured)…"
		})
		wiki := analysis.FallbackWiki(ref, idx, root)
		g.publishWiki(id, wiki, 95)
		return wiki, nil
	}

	g.update(id, func(r *model.Repo) {
		r.Status = model.StatusPlanning
		r.Progress = 27
		r.Message = "Agent is planning the wiki…"
	})

	plan, err := g.plan(ctx, id, ref, idx, root)
	if err != nil {
		log.Printf("wiki planner failed for %s: %v", id, err)
		g.update(id, func(r *model.Repo) {
			r.Message = "Planner unavailable — building a local wiki instead"
		})
		wiki := analysis.FallbackWiki(ref, idx, root)
		wiki.Provider = provider
		g.publishWiki(id, wiki, 95)
		return wiki, nil
	}

	title := strings.TrimSpace(plan.Title)
	if title == "" {
		title = analysis.FallbackWiki(ref, idx, root).Title
	}
	summary := strings.TrimSpace(plan.Summary)
	if summary == "" {
		summary = fmt.Sprintf("Documentation for %s/%s.", ref.Owner, ref.Name)
	}

	pages := make([]*model.Page, 0, len(plan.Pages))
	for i, p := range plan.Pages {
		slug := slugify(p.Slug)
		if slug == "" {
			slug = slugify(p.Title)
		}
		if slug == "" {
			slug = fmt.Sprintf("page-%d", i+1)
		}
		pages = append(pages, &model.Page{
			ID:          fmt.Sprintf("page-%d", i+1),
			Title:       strings.TrimSpace(p.Title),
			Slug:        uniqueSlug(slug, pages),
			Description: strings.TrimSpace(p.Description),
			Order:       i,
			Status:      "pending",
			Content:     "",
		})
	}
	if len(pages) == 0 {
		wiki := analysis.FallbackWiki(ref, idx, root)
		wiki.Provider = provider
		g.publishWiki(id, wiki, 95)
		return wiki, nil
	}

	wiki := &model.Wiki{
		Title:       title,
		Summary:     summary,
		Pages:       pages,
		GeneratedAt: time.Now().UTC(),
		Provider:    provider,
	}
	g.update(id, func(r *model.Repo) {
		r.Status = model.StatusGenerating
		r.Progress = 34
		r.Message = fmt.Sprintf("Writing %d pages…", len(pages))
		r.Wiki = wiki
	})

	fallback := analysis.FallbackWiki(ref, idx, root)
	fallbackBySlug := map[string]*model.Page{}
	for _, p := range fallback.Pages {
		fallbackBySlug[p.Slug] = p
	}

	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		completed int
		usedLocal bool
	)
	sem := make(chan struct{}, 2)
	for _, page := range pages {
		page := page
		planPage := findPlanPage(plan.Pages, page.Slug)
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			g.updatePage(id, page.ID, func(p *model.Page) {
				p.Status = "writing"
			})
			pageCtx, cancel := context.WithTimeout(ctx, 4*time.Minute)
			content, err := g.writePage(pageCtx, id, ref, idx, root, title, page, planPage)
			cancel()

			local := false
			if err != nil || strings.TrimSpace(content) == "" {
				if err != nil {
					log.Printf("writer failed for %s/%s: %v", id, page.Slug, err)
				}
				if fb := fallbackBySlug[page.Slug]; fb != nil {
					content = fb.Content
				} else {
					content = fmt.Sprintf("# %s\n\nThe agent did not return content for this page. Try regenerating the wiki.\n", page.Title)
				}
				local = true
			}

			g.updatePage(id, page.ID, func(p *model.Page) {
				p.Content = content
				p.Status = "ready"
			})

			mu.Lock()
			completed++
			usedLocal = usedLocal || local
			done := completed
			mu.Unlock()
			g.update(id, func(r *model.Repo) {
				r.Progress = 34 + int(float64(done)/float64(len(pages))*62)
				r.Message = fmt.Sprintf("Writing %d pages… (%d/%d done)", len(pages), done, len(pages))
			})
		}()
	}
	wg.Wait()

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	repo, _ := g.store.Get(id)
	if repo != nil && repo.Wiki != nil {
		wiki = repo.Wiki
	}
	wiki.Provider = provider
	wiki.Demo = usedLocal
	wiki.GeneratedAt = time.Now().UTC()
	g.publishWiki(id, wiki, 100)
	return wiki, nil
}

func (g *Generator) plan(ctx context.Context, id string, ref *analysis.RepoRef, idx *model.Index, root string) (*wikiPlan, error) {
	a, err := g.newTextAgent(
		"wiki_planner",
		"You are a staff software engineer and technical writer. You map unfamiliar codebases into "+
			"clear, accurate documentation plans. Explore the repository with the provided tools before "+
			"deciding anything. Prefer concrete file paths over guesses. When you are done, return the "+
			"requested JSON and nothing else.",
		id, root, ref, idx, "Planning wiki structure…",
	)
	if err != nil {
		return nil, err
	}

	prompt := fmt.Sprintf(`Create a DeepWiki-style documentation plan for the repository below.

The wiki should let a new engineer understand the project quickly. Requirements:
- Produce 5 to 8 pages.
- The first page MUST be an "overview" page.
- Include a "getting-started" page and an "architecture" page.
- Add pages for the 2-4 most important subsystems or request flows you discover.
- "focus" lists concrete repository paths the writer should read for that page.
- Titles are human friendly, descriptions are one sentence.

Return ONLY valid JSON with exactly this shape (no markdown fences, no commentary):
{
  "title": "Human-friendly wiki title",
  "summary": "Two sentence summary of the project",
  "pages": [
    {
      "title": "Overview",
      "slug": "overview",
      "description": "What the project does and how it is organized.",
      "focus": ["README.md", "cmd/"]
    }
  ]
}

Repository digest:
%s`, analysis.Digest(ref, idx, 9000))

	pageCtx, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()
	text, err := g.runText(pageCtx, a, prompt)
	if err != nil {
		return nil, err
	}
	raw, err := extractJSON(text)
	if err != nil {
		return nil, fmt.Errorf("parse plan: %w", err)
	}
	var plan wikiPlan
	if err := json.Unmarshal([]byte(raw), &plan); err != nil {
		return nil, fmt.Errorf("decode plan: %w", err)
	}
	if len(plan.Pages) == 0 {
		return nil, errors.New("planner returned no pages")
	}
	if len(plan.Pages) > 10 {
		plan.Pages = plan.Pages[:10]
	}
	for i := range plan.Pages {
		if strings.TrimSpace(plan.Pages[i].Title) == "" {
			plan.Pages[i].Title = fmt.Sprintf("Page %d", i+1)
		}
	}
	return &plan, nil
}

func (g *Generator) writePage(ctx context.Context, id string, ref *analysis.RepoRef, idx *model.Index, root, wikiTitle string, page *model.Page, planPage *planPage) (string, error) {
	a, err := g.newTextAgent(
		"wiki_writer",
		"You are a senior engineer writing one page of an internal code wiki. Ground every claim in "+
			"the repository: read the files, follow imports, and cite paths in backticks. Write for an "+
			"engineer who just joined the team. Use markdown with useful headings, short paragraphs, "+
			"tables and code blocks where they help. Include a mermaid diagram only when it clarifies "+
			"real structure. Never invent files, endpoints or configuration. Return only the markdown "+
			"page content, starting with a single level-1 heading.",
		id, root, ref, idx, "Reading "+page.Title+"…",
	)
	if err != nil {
		return "", err
	}

	var focus []string
	description := page.Description
	if planPage != nil {
		focus = planPage.Focus
		if description == "" {
			description = planPage.Description
		}
	}
	prefetched := g.focusContext(root, focus)

	prompt := fmt.Sprintf(`Write the "%s" page of the "%s" wiki for %s/%s.

Page description: %s

%s

Repository digest:
%s

Rules:
- Output markdown only. Start with "# %s".
- Explain the actual design: name real packages, types, functions and files.
- Use relative file paths in backticks (for example %s).
- Prefer depth over generic filler. 500-1100 words.
- If you include a diagram, use a mermaid code block with valid syntax.
`,
		page.Title, wikiTitle, ref.Owner, ref.Name, description,
		prefetched, analysis.Digest(ref, idx, 10000), page.Title, examplePath(idx))

	pageCtx, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()
	text, err := g.runText(pageCtx, a, prompt)
	if err != nil {
		return "", err
	}
	return cleanMarkdown(text, page.Title), nil
}

func (g *Generator) newTextAgent(name, instruction, repoID, root string, ref *analysis.RepoRef, idx *model.Index, idleMessage string) (agent.Agent, error) {
	tools, err := Tools(root, ref, idx)
	if err != nil {
		return nil, err
	}
	activity := func(ctx agent.ToolContext, t tool.Tool, args map[string]any) (map[string]any, error) {
		msg := describeToolCall(t.Name(), args)
		if msg == "" {
			msg = idleMessage
		}
		g.setMessage(repoID, msg)
		return nil, nil
	}
	temp := g.cfg.Model.Temperature
	return llmagent.New(llmagent.Config{
		Name:        name,
		Model:       g.llm,
		Description: "Code Insight wiki agent",
		// A provider bypasses ADK's {state} templating. Repository digests and
		// prompts can contain literal braces (JSON, Go, shell), which must not
		// be interpreted as session-state references.
		InstructionProvider: func(ctx agent.ReadonlyContext) (string, error) {
			return instruction, nil
		},
		Tools: tools,
		GenerateContentConfig: &genai.GenerateContentConfig{
			Temperature:     &temp,
			MaxOutputTokens: int32(g.cfg.Model.MaxTokens),
		},
		BeforeToolCallbacks: []llmagent.BeforeToolCallback{activity},
	})
}

func (g *Generator) runText(ctx context.Context, a agent.Agent, prompt string) (string, error) {
	r, err := runner.New(runner.Config{
		AppName:           g.cfg.AppName,
		Agent:             a,
		SessionService:    g.sessions,
		AutoCreateSession: true,
	})
	if err != nil {
		return "", err
	}
	sessionID := uuid.NewString()
	var text strings.Builder
	for event, err := range r.Run(ctx, "system", sessionID, genai.NewContentFromText(prompt, genai.RoleUser), agent.RunConfig{StreamingMode: agent.StreamingModeNone}) {
		if err != nil {
			return "", err
		}
		if event == nil || event.Content == nil {
			continue
		}
		toolEvent := false
		for _, part := range event.Content.Parts {
			if part == nil {
				continue
			}
			if part.FunctionCall != nil || part.FunctionResponse != nil {
				toolEvent = true
				continue
			}
			if part.Text != "" {
				text.WriteString(part.Text)
			}
		}
		// A model turn that calls tools often starts with a short preamble
		// ("I'll inspect the files..."). The real answer comes after the last
		// tool result, so discard everything accumulated before it.
		if toolEvent {
			text.Reset()
		}
	}
	out := strings.TrimSpace(text.String())
	if out == "" {
		return "", errors.New("agent returned no text")
	}
	return out, nil
}

func (g *Generator) focusContext(root string, focus []string) string {
	if len(focus) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Prefetched context (verify before citing):\n")
	budget := 12000
	for _, rel := range focus {
		if budget <= 0 {
			break
		}
		rel = strings.TrimPrefix(strings.TrimSpace(rel), "./")
		if rel == "" {
			continue
		}
		full, err := analysis.SafeJoin(root, rel)
		if err != nil {
			continue
		}
		info, err := os.Stat(full)
		if err != nil {
			continue
		}
		if info.IsDir() {
			tree, err := analysis.ListTree(root, rel, 2)
			if err != nil {
				continue
			}
			tree = truncateStr(tree, minInt(budget, 4000))
			b.WriteString("\n### " + rel + "/\n```text\n" + tree + "```\n")
			budget -= len(tree)
			continue
		}
		if info.Size() > 512*1024 {
			continue
		}
		content, _, err := analysis.ReadSource(root, rel, 1, 200)
		if err != nil {
			continue
		}
		content = truncateStr(content, minInt(budget, 6000))
		b.WriteString("\n### " + rel + "\n```\n" + content + "```\n")
		budget -= len(content)
	}
	if b.Len() == 0 {
		return ""
	}
	return b.String()
}

func (g *Generator) publishWiki(id string, wiki *model.Wiki, progress int) {
	g.update(id, func(r *model.Repo) {
		r.Wiki = wiki
		r.Progress = progress
		if progress >= 100 {
			r.Status = model.StatusReady
			r.Message = fmt.Sprintf("Wiki ready · %d pages", len(wiki.Pages))
		}
	})
}

func (g *Generator) updatePage(repoID, pageID string, fn func(*model.Page)) {
	_, _ = g.store.Update(repoID, func(r *model.Repo) {
		if r.Wiki == nil {
			return
		}
		for _, p := range r.Wiki.Pages {
			if p.ID == pageID {
				fn(p)
				return
			}
		}
	})
}

func (g *Generator) update(id string, fn func(*model.Repo)) {
	if _, err := g.store.Update(id, fn); err != nil {
		log.Printf("update repo %s: %v", id, err)
	}
}

func (g *Generator) setMessage(id, message string) {
	if message == "" {
		return
	}
	if repo, err := g.store.Get(id); err == nil && repo.Message == message {
		return
	}
	g.update(id, func(r *model.Repo) {
		if !r.Status.Active() {
			return
		}
		r.Message = message
	})
}

func (g *Generator) fail(id string, err error) {
	log.Printf("analysis failed for %s: %v", id, err)
	g.update(id, func(r *model.Repo) {
		r.Status = model.StatusError
		r.Error = err.Error()
		r.Message = "Analysis failed"
	})
}

type wikiPlan struct {
	Title   string     `json:"title"`
	Summary string     `json:"summary"`
	Pages   []planPage `json:"pages"`
}

type planPage struct {
	Title       string   `json:"title"`
	Slug        string   `json:"slug"`
	Description string   `json:"description"`
	Focus       []string `json:"focus"`
}

func findPlanPage(pages []planPage, slug string) *planPage {
	for i := range pages {
		if slugify(pages[i].Slug) == slug || slugify(pages[i].Title) == slug {
			return &pages[i]
		}
	}
	return nil
}

func uniqueSlug(slug string, pages []*model.Page) string {
	base := slug
	for i := 2; ; i++ {
		exists := false
		for _, p := range pages {
			if p.Slug == slug {
				exists = true
				break
			}
		}
		if !exists {
			return slug
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
}

var slugReplacer = strings.NewReplacer("_", "-", " ", "-", ".", "-", "/", "-")

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugReplacer.Replace(s)
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == '-':
			if !lastDash && b.Len() > 0 {
				b.WriteRune('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func extractJSON(text string) (string, error) {
	text = strings.TrimSpace(text)
	if idx := strings.Index(text, "```"); idx >= 0 {
		rest := text[idx+3:]
		rest = strings.TrimPrefix(rest, "json")
		rest = strings.TrimSpace(rest)
		if end := strings.Index(rest, "```"); end >= 0 {
			candidate := strings.TrimSpace(rest[:end])
			if strings.HasPrefix(candidate, "{") {
				return candidate, nil
			}
		}
	}
	start := strings.Index(text, "{")
	if start < 0 {
		return "", errors.New("no JSON object in response")
	}
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(text); i++ {
		c := text[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return text[start : i+1], nil
			}
		}
	}
	return "", errors.New("unbalanced JSON object")
}

func cleanMarkdown(text, title string) string {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "```markdown")
	text = strings.TrimPrefix(text, "```md")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)
	// Models sometimes add a sentence before the requested heading.
	if idx := strings.Index(text, "# "); idx > 0 && idx < 500 {
		text = strings.TrimSpace(text[idx:])
	}
	if !strings.HasPrefix(text, "# ") {
		text = "# " + title + "\n\n" + text
	}
	return text + "\n"
}

func describeToolCall(name string, args map[string]any) string {
	pick := func(keys ...string) string {
		for _, key := range keys {
			if v, ok := args[key].(string); ok && strings.TrimSpace(v) != "" {
				return v
			}
		}
		return ""
	}
	switch name {
	case "read_file":
		return "Reading " + pick("path")
	case "list_files":
		dir := pick("dir")
		if dir == "" {
			dir = "."
		}
		return "Listing " + dir
	case "search_code":
		return "Searching for \"" + pick("query") + "\""
	default:
		return ""
	}
}

func examplePath(idx *model.Index) string {
	if len(idx.EntryFiles) > 0 {
		return idx.EntryFiles[0]
	}
	if len(idx.KeyFiles) > 0 {
		return idx.KeyFiles[0]
	}
	return "README.md"
}

func truncateStr(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "\n… (truncated)"
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
