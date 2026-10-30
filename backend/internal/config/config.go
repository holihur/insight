// Package config loads runtime configuration from the environment.
package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config is the complete server configuration.
type Config struct {
	Addr           string
	DataDir        string
	WebDir         string
	AppName        string
	MaxRepoFiles   int
	CloneTimeout   time.Duration
	RequestTimeout time.Duration
	CORSOrigin     string
	GitHubToken    string
	Model          ModelConfig
}

// ModelConfig selects and configures the LLM provider.
type ModelConfig struct {
	Provider    string // gemini | openai | mock
	Model       string
	APIKey      string
	BaseURL     string
	Temperature float32
	MaxTokens   int
}

// Demo reports whether the server runs without an LLM.
func (m ModelConfig) Demo() bool { return m.Provider == "mock" }

// Load reads .env files and the process environment.
func Load() Config {
	loadDotEnv(".env")
	loadDotEnv("../.env")

	cfg := Config{
		Addr:           env("INSIGHT_ADDR", ":"+env("PORT", "8080")),
		DataDir:        env("DATA_DIR", "./data"),
		WebDir:         env("WEB_DIR", ""),
		AppName:        "code-insight",
		MaxRepoFiles:   envInt("MAX_REPO_FILES", 8000),
		CloneTimeout:   time.Duration(envInt("GIT_CLONE_TIMEOUT", 180)) * time.Second,
		RequestTimeout: time.Duration(envInt("ANALYSIS_TIMEOUT", 900)) * time.Second,
		CORSOrigin:     env("CORS_ORIGIN", "*"),
		GitHubToken:    env("GITHUB_TOKEN", ""),
		Model:          detectModel(),
	}
	return cfg
}

func detectModel() ModelConfig {
	provider := strings.ToLower(env("INSIGHT_PROVIDER", "auto"))
	modelName := env("INSIGHT_MODEL", "")
	temp := float32(envFloat("LLM_TEMPERATURE", 0.25))
	maxTokens := envInt("LLM_MAX_TOKENS", 8192)

	geminiKey := firstEnv("GEMINI_API_KEY", "GOOGLE_API_KEY")
	openAIKey := env("OPENAI_API_KEY", "")
	deepSeekKey := env("DEEPSEEK_API_KEY", "")
	openRouterKey := env("OPENROUTER_API_KEY", "")
	ollamaModel := env("OLLAMA_MODEL", "")

	if provider == "auto" {
		switch {
		case geminiKey != "":
			provider = "gemini"
		case openAIKey != "":
			provider = "openai"
		case deepSeekKey != "":
			provider = "openai"
		case openRouterKey != "":
			provider = "openai"
		case ollamaModel != "" || env("OPENAI_BASE_URL", "") != "":
			provider = "openai"
		default:
			provider = "mock"
		}
	}

	m := ModelConfig{Provider: provider, Temperature: temp, MaxTokens: maxTokens}

	switch provider {
	case "gemini":
		m.APIKey = geminiKey
		m.BaseURL = env("GEMINI_BASE_URL", "")
		m.Model = firstNonEmpty(modelName, env("GEMINI_MODEL", ""), "gemini-2.5-flash")
	case "openai":
		m.APIKey = openAIKey
		m.BaseURL = env("OPENAI_BASE_URL", "https://api.openai.com/v1")
		m.Model = firstNonEmpty(modelName, env("OPENAI_MODEL", ""), "gpt-4o-mini")
		if m.APIKey == "" && deepSeekKey != "" {
			m.APIKey = deepSeekKey
			m.BaseURL = "https://api.deepseek.com/v1"
			m.Model = firstNonEmpty(modelName, env("DEEPSEEK_MODEL", ""), "deepseek-flash")
		}
		if m.APIKey == "" && openRouterKey != "" {
			m.APIKey = openRouterKey
			m.BaseURL = "https://openrouter.ai/api/v1"
			m.Model = firstNonEmpty(modelName, env("OPENROUTER_MODEL", ""), "google/gemini-2.5-flash")
		}
		if m.APIKey == "" && ollamaModel != "" {
			m.APIKey = "ollama"
			m.BaseURL = env("OLLAMA_HOST", "http://localhost:11434") + "/v1"
			m.Model = ollamaModel
		}
	default:
		m.Provider = "mock"
	}
	return m
}

func firstEnv(names ...string) string {
	for _, name := range names {
		if v := os.Getenv(name); v != "" {
			return v
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func envFloat(key string, fallback float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return f
}

// loadDotEnv parses a small, dependency-free subset of dotenv syntax.
func loadDotEnv(path string) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return
	}
	f, err := os.Open(abs)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, value)
		}
	}
}
