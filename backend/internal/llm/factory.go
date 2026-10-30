package llm

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/adk/model"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/genai"

	"github.com/codeinsight/code-insight/internal/config"
)

// New creates the configured model. A nil model means local demo mode.
func New(ctx context.Context, cfg config.ModelConfig) (model.LLM, error) {
	switch cfg.Provider {
	case "gemini":
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("GEMINI_API_KEY or GOOGLE_API_KEY is not set")
		}
		clientCfg := &genai.ClientConfig{APIKey: cfg.APIKey}
		if cfg.BaseURL != "" {
			clientCfg.HTTPOptions.BaseURL = cfg.BaseURL
		}
		return gemini.NewModel(ctx, cfg.Model, clientCfg)
	case "openai":
		if cfg.APIKey == "" && !isLocalBaseURL(cfg.BaseURL) {
			return nil, fmt.Errorf("an API key is required for %s", cfg.BaseURL)
		}
		return NewOpenAI(OpenAIConfig{
			Model:       cfg.Model,
			APIKey:      cfg.APIKey,
			BaseURL:     cfg.BaseURL,
			Temperature: cfg.Temperature,
			MaxTokens:   cfg.MaxTokens,
		}), nil
	default:
		return nil, nil
	}
}

// Label returns a short human-readable provider/model label.
func Label(cfg config.ModelConfig) string {
	switch cfg.Provider {
	case "gemini":
		return "gemini/" + cfg.Model
	case "openai":
		host := cfg.BaseURL
		switch {
		case strings.Contains(host, "deepseek"):
			return "deepseek/" + cfg.Model
		case strings.Contains(host, "openrouter"):
			return "openrouter/" + cfg.Model
		case strings.Contains(host, "localhost"), strings.Contains(host, "127.0.0.1"):
			return "local/" + cfg.Model
		default:
			return "openai/" + cfg.Model
		}
	default:
		return "local"
	}
}

func isLocalBaseURL(base string) bool {
	return strings.Contains(base, "localhost") || strings.Contains(base, "127.0.0.1") ||
		strings.Contains(base, "0.0.0.0") || strings.HasPrefix(base, "http://")
}
