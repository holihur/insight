package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	adkmodel "google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"

	"github.com/codeinsight/code-insight/internal/analysis"
	"github.com/codeinsight/code-insight/internal/config"
	"github.com/codeinsight/code-insight/internal/model"
	"github.com/codeinsight/code-insight/internal/store"
)

// ChatEvent is one server-sent event in a repository conversation.
type ChatEvent struct {
	Type      string         `json:"type"`
	Text      string         `json:"text,omitempty"`
	ID        string         `json:"id,omitempty"`
	Name      string         `json:"name,omitempty"`
	Args      map[string]any `json:"args,omitempty"`
	Result    string         `json:"result,omitempty"`
	SessionID string         `json:"sessionId,omitempty"`
	Message   string         `json:"message,omitempty"`
}

// Chatter answers questions about one repository with streaming ADK events.
type Chatter struct {
	cfg      config.Config
	store    *store.Store
	llm      adkmodel.LLM
	sessions session.Service
}

// NewChatter creates a Chatter.
func NewChatter(cfg config.Config, st *store.Store, model adkmodel.LLM) *Chatter {
	return &Chatter{
		cfg:      cfg,
		store:    st,
		llm:      model,
		sessions: session.InMemoryService(),
	}
}

// Stream runs one chat turn and emits events as they arrive.
func (c *Chatter) Stream(ctx context.Context, repo *model.Repo, clientSessionID, message string, emit func(ChatEvent) error) error {
	message = strings.TrimSpace(message)
	if message == "" {
		return fmt.Errorf("message is required")
	}
	if len(message) > 8000 {
		return fmt.Errorf("message is too long (max 8000 characters)")
	}
	sessionID := repo.ID + ":" + clientSessionID
	if err := emit(ChatEvent{Type: "meta", SessionID: clientSessionID}); err != nil {
		return err
	}

	ref, err := analysis.ParseRepoURL(repo.URL)
	if err != nil {
		return err
	}
	idx, err := c.store.LoadIndex(repo.ID)
	if err != nil || idx == nil {
		idx = &model.Index{Stats: repo.Stats}
	}

	if c.llm == nil {
		answer := analysis.AnswerLocally(c.store.SourceDir(repo.ID), idx, ref, message)
		for _, chunk := range chunkText(answer, 48) {
			if err := emit(ChatEvent{Type: "token", Text: chunk}); err != nil {
				return err
			}
		}
		return emit(ChatEvent{Type: "text", Text: answer})
	}

	root := c.store.SourceDir(repo.ID)
	tools, err := Tools(root, ref, idx)
	if err != nil {
		return err
	}

	instruction := c.chatInstruction(repo, ref, idx)
	a, err := llmagent.New(llmagent.Config{
		Name:        "repo_chat",
		Model:       c.llm,
		Description: "Answers questions about a source repository",
		Instruction: instruction,
		Tools:       tools,
		GenerateContentConfig: &genai.GenerateContentConfig{
			Temperature:     ptrFloat32(0.2),
			MaxOutputTokens: int32(c.cfg.Model.MaxTokens),
		},
	})
	if err != nil {
		return err
	}

	r, err := runner.New(runner.Config{
		AppName:           c.cfg.AppName,
		Agent:             a,
		SessionService:    c.sessions,
		AutoCreateSession: true,
	})
	if err != nil {
		return err
	}

	runCtx, cancel := context.WithTimeout(ctx, 5*60*1e9)
	defer cancel()
	for event, err := range r.Run(runCtx, "web", sessionID, genai.NewContentFromText(message, genai.RoleUser), agent.RunConfig{StreamingMode: agent.StreamingModeSSE}) {
		if err != nil {
			return err
		}
		if event == nil || event.Content == nil {
			continue
		}
		for _, part := range event.Content.Parts {
			if part == nil {
				continue
			}
			switch {
			case part.FunctionCall != nil:
				if err := emit(ChatEvent{
					Type: "tool_call",
					ID:   part.FunctionCall.ID,
					Name: part.FunctionCall.Name,
					Args: part.FunctionCall.Args,
				}); err != nil {
					return err
				}
			case part.FunctionResponse != nil:
				if err := emit(ChatEvent{
					Type:   "tool_result",
					ID:     part.FunctionResponse.ID,
					Name:   part.FunctionResponse.Name,
					Result: summarizeToolResult(part.FunctionResponse.Response),
				}); err != nil {
					return err
				}
			case part.Text != "":
				typ := "text"
				if event.Partial {
					typ = "token"
				}
				if err := emit(ChatEvent{Type: typ, Text: part.Text}); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (c *Chatter) chatInstruction(repo *model.Repo, ref *analysis.RepoRef, idx *model.Index) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You are Code Insight, a senior engineer who has read every line of %s/%s (%s).\n", ref.Owner, ref.Name, repo.Commit)
	b.WriteString("Answer questions about this repository accurately and concretely.\n\n")
	b.WriteString("Ground rules:\n")
	b.WriteString("- Use the provided tools to inspect files before answering. Never invent files or behavior.\n")
	b.WriteString("- Cite repository paths in backticks, with line numbers when you have them.\n")
	b.WriteString("- Explain how things actually work: follow the call chain and name the real functions/types.\n")
	b.WriteString("- Use markdown. Use fenced code blocks with a language tag. Use mermaid only for real structures.\n")
	b.WriteString("- If something is not in the repository, say so explicitly instead of guessing.\n")
	b.WriteString("- Keep answers focused; a typical answer is 3-8 short paragraphs.\n\n")
	if repo.Wiki != nil {
		b.WriteString("Wiki sections available to the reader:\n")
		for _, page := range repo.Wiki.Pages {
			fmt.Fprintf(&b, "- %s (`#%s`): %s\n", page.Title, page.Slug, page.Description)
		}
		b.WriteString("\n")
	}
	b.WriteString("Repository digest:\n")
	b.WriteString(analysis.Digest(ref, idx, 9000))
	return b.String()
}

func summarizeToolResult(result map[string]any) string {
	if result == nil {
		return ""
	}
	data, err := json.Marshal(result)
	if err != nil {
		return ""
	}
	s := string(data)
	if len(s) > 700 {
		s = s[:700] + "…"
	}
	return s
}

func chunkText(s string, size int) []string {
	var out []string
	for len(s) > 0 {
		n := size
		if n > len(s) {
			n = len(s)
		}
		out = append(out, s[:n])
		s = s[n:]
	}
	return out
}

func ptrFloat32(v float32) *float32 { return &v }
