// Package llm provides model.LLM adapters used by the agents.
package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"net/http"
	"strings"
	"time"

	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

// OpenAI implements model.LLM for OpenAI-compatible chat completion APIs.
// It works with OpenAI, DeepSeek, OpenRouter, Ollama, vLLM and similar servers.
type OpenAI struct {
	name        string
	apiKey      string
	baseURL     string
	temperature float32
	maxTokens   int
	client      *http.Client
}

// OpenAIConfig configures the adapter.
type OpenAIConfig struct {
	Model       string
	APIKey      string
	BaseURL     string
	Temperature float32
	MaxTokens   int
}

// NewOpenAI returns an OpenAI-compatible model adapter.
func NewOpenAI(cfg OpenAIConfig) *OpenAI {
	base := strings.TrimRight(cfg.BaseURL, "/")
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	return &OpenAI{
		name:        cfg.Model,
		apiKey:      cfg.APIKey,
		baseURL:     base,
		temperature: cfg.Temperature,
		maxTokens:   cfg.MaxTokens,
		client: &http.Client{
			Timeout: 0,
			Transport: &http.Transport{
				Proxy:                 http.ProxyFromEnvironment,
				MaxIdleConns:          50,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   20 * time.Second,
				ResponseHeaderTimeout: 120 * time.Second,
			},
		},
	}
}

// Name implements model.LLM.
func (m *OpenAI) Name() string { return m.name }

type chatMessage struct {
	Role       string         `json:"role"`
	Content    any            `json:"content,omitempty"`
	ToolCalls  []toolCall     `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	Name       string         `json:"name,omitempty"`
	Extra      map[string]any `json:"-"`
}

type toolCall struct {
	ID       string       `json:"id,omitempty"`
	Type     string       `json:"type"`
	Function functionCall `json:"function"`
}

type functionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAITool struct {
	Type     string         `json:"type"`
	Function openAIFunction `json:"function"`
}

type openAIFunction struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Tools       []openAITool  `json:"tools,omitempty"`
	ToolChoice  string        `json:"tool_choice,omitempty"`
	Temperature *float32      `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content          string     `json:"content"`
			ReasoningContent string     `json:"reasoning_content"`
			ToolCalls        []toolCall `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int32 `json:"prompt_tokens"`
		CompletionTokens int32 `json:"completion_tokens"`
		TotalTokens      int32 `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
			ToolCalls        []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int32 `json:"prompt_tokens"`
		CompletionTokens int32 `json:"completion_tokens"`
		TotalTokens      int32 `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// GenerateContent implements model.LLM.
func (m *OpenAI) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		payload, err := m.buildRequest(req, stream)
		if err != nil {
			yield(nil, err)
			return
		}
		body, err := json.Marshal(payload)
		if err != nil {
			yield(nil, fmt.Errorf("encode request: %w", err))
			return
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseURL+"/chat/completions", bytes.NewReader(body))
		if err != nil {
			yield(nil, err)
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Accept", "text/event-stream")
		if m.apiKey != "" {
			httpReq.Header.Set("Authorization", "Bearer "+m.apiKey)
		}

		resp, err := m.client.Do(httpReq)
		if err != nil {
			yield(nil, fmt.Errorf("call model: %w", err))
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			data, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
			yield(nil, fmt.Errorf("model returned HTTP %d: %s", resp.StatusCode, providerError(data)))
			return
		}

		if !stream {
			m.handleResponse(resp.Body, yield)
			return
		}
		m.handleStream(resp.Body, yield)
	}
}

func (m *OpenAI) buildRequest(req *model.LLMRequest, stream bool) (*chatRequest, error) {
	messages, err := m.convertMessages(req)
	if err != nil {
		return nil, err
	}
	modelName := req.Model
	if modelName == "" {
		modelName = m.name
	}
	payload := &chatRequest{
		Model:    modelName,
		Messages: messages,
		Stream:   stream,
	}
	if m.maxTokens > 0 {
		payload.MaxTokens = m.maxTokens
	}
	temperature := m.temperature
	if req.Config != nil && req.Config.Temperature != nil {
		temperature = *req.Config.Temperature
	}
	if temperature > 0 {
		payload.Temperature = &temperature
	}
	if req.Config != nil {
		for _, tool := range req.Config.Tools {
			if tool == nil {
				continue
			}
			for _, decl := range tool.FunctionDeclarations {
				if decl == nil || decl.Name == "" {
					continue
				}
				fn := openAIFunction{Name: decl.Name, Description: decl.Description}
				if decl.ParametersJsonSchema != nil {
					fn.Parameters = normalizeSchema(decl.ParametersJsonSchema)
				} else if decl.Parameters != nil {
					raw, _ := json.Marshal(decl.Parameters)
					var schema map[string]any
					_ = json.Unmarshal(raw, &schema)
					fn.Parameters = normalizeSchema(schema)
				}
				if fn.Parameters == nil {
					fn.Parameters = map[string]any{"type": "object", "properties": map[string]any{}}
				}
				payload.Tools = append(payload.Tools, openAITool{Type: "function", Function: fn})
			}
		}
		if len(payload.Tools) > 0 {
			payload.ToolChoice = "auto"
		}
	}
	return payload, nil
}

func (m *OpenAI) convertMessages(req *model.LLMRequest) ([]chatMessage, error) {
	var out []chatMessage
	if req.Config != nil && req.Config.SystemInstruction != nil {
		if text := contentText(req.Config.SystemInstruction); text != "" {
			out = append(out, chatMessage{Role: "system", Content: text})
		}
	}
	for _, content := range req.Contents {
		if content == nil {
			continue
		}
		var text strings.Builder
		var calls []toolCall
		var responses []chatMessage
		for _, part := range content.Parts {
			if part == nil {
				continue
			}
			switch {
			case part.FunctionCall != nil:
				args, err := json.Marshal(part.FunctionCall.Args)
				if err != nil {
					args = []byte("{}")
				}
				calls = append(calls, toolCall{
					ID:   part.FunctionCall.ID,
					Type: "function",
					Function: functionCall{
						Name:      part.FunctionCall.Name,
						Arguments: string(args),
					},
				})
			case part.FunctionResponse != nil:
				data, err := json.Marshal(part.FunctionResponse.Response)
				if err != nil {
					data = []byte("{}")
				}
				responses = append(responses, chatMessage{
					Role:       "tool",
					ToolCallID: part.FunctionResponse.ID,
					Name:       part.FunctionResponse.Name,
					Content:    string(data),
				})
			case part.Text != "":
				text.WriteString(part.Text)
			}
		}
		role := content.Role
		if role == "model" || role == "assistant" {
			role = "assistant"
		} else {
			role = "user"
		}
		if text.Len() > 0 || len(calls) > 0 {
			msg := chatMessage{Role: role}
			if text.Len() > 0 {
				msg.Content = text.String()
			}
			if len(calls) > 0 {
				if msg.Content == nil {
					msg.Content = nil
				}
				msg.ToolCalls = calls
			}
			out = append(out, msg)
		}
		out = append(out, responses...)
	}
	return out, nil
}

func contentText(content *genai.Content) string {
	if content == nil {
		return ""
	}
	var b strings.Builder
	for _, part := range content.Parts {
		if part != nil && part.Text != "" {
			b.WriteString(part.Text)
		}
	}
	return b.String()
}

func (m *OpenAI) handleResponse(body io.Reader, yield func(*model.LLMResponse, error) bool) {
	var resp chatResponse
	if err := json.NewDecoder(body).Decode(&resp); err != nil {
		yield(nil, fmt.Errorf("decode model response: %w", err))
		return
	}
	if resp.Error != nil {
		yield(nil, errors.New(resp.Error.Message))
		return
	}
	if len(resp.Choices) == 0 {
		yield(nil, errors.New("model returned no choices"))
		return
	}
	choice := resp.Choices[0]
	parts := make([]*genai.Part, 0, len(choice.Message.ToolCalls)+1)
	if choice.Message.Content != "" {
		parts = append(parts, &genai.Part{Text: choice.Message.Content})
	}
	for i, call := range choice.Message.ToolCalls {
		parts = append(parts, functionCallPart(call, i))
	}
	result := &model.LLMResponse{
		Content:      &genai.Content{Role: "model", Parts: parts},
		TurnComplete: true,
		FinishReason: finishReason(choice.FinishReason),
	}
	if resp.Usage != nil {
		result.UsageMetadata = usageMetadata(resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens)
	}
	yield(result, nil)
}

func (m *OpenAI) handleStream(body io.Reader, yield func(*model.LLMResponse, error) bool) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)

	var text strings.Builder
	type callState struct {
		id   string
		name string
		args strings.Builder
	}
	calls := map[int]*callState{}
	var callOrder []int
	var finish string
	var usage *struct {
		PromptTokens     int32 `json:"prompt_tokens"`
		CompletionTokens int32 `json:"completion_tokens"`
		TotalTokens      int32 `json:"total_tokens"`
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			// Some providers emit keep-alive or non-JSON lines; ignore them.
			continue
		}
		if chunk.Error != nil {
			yield(nil, errors.New(chunk.Error.Message))
			return
		}
		if chunk.Usage != nil {
			usage = chunk.Usage
		}
		for _, choice := range chunk.Choices {
			if choice.FinishReason != "" {
				finish = choice.FinishReason
			}
			if choice.Delta.Content != "" {
				text.WriteString(choice.Delta.Content)
				if !yield(&model.LLMResponse{
					Content: &genai.Content{Role: "model", Parts: []*genai.Part{{Text: choice.Delta.Content}}},
					Partial: true,
				}, nil) {
					return
				}
			}
			for _, delta := range choice.Delta.ToolCalls {
				state, ok := calls[delta.Index]
				if !ok {
					state = &callState{}
					calls[delta.Index] = state
					callOrder = append(callOrder, delta.Index)
				}
				if delta.ID != "" {
					state.id = delta.ID
				}
				if delta.Function.Name != "" {
					state.name = delta.Function.Name
				}
				state.args.WriteString(delta.Function.Arguments)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		yield(nil, fmt.Errorf("read model stream: %w", err))
		return
	}

	parts := make([]*genai.Part, 0, len(callOrder)+1)
	if text.Len() > 0 {
		parts = append(parts, &genai.Part{Text: text.String()})
	}
	for i, idx := range callOrder {
		state := calls[idx]
		if state == nil || state.name == "" {
			continue
		}
		call := toolCall{
			ID:   state.id,
			Type: "function",
			Function: functionCall{
				Name:      state.name,
				Arguments: state.args.String(),
			},
		}
		parts = append(parts, functionCallPart(call, i))
	}
	if len(parts) == 0 {
		yield(nil, errors.New("model returned an empty response"))
		return
	}
	result := &model.LLMResponse{
		Content:      &genai.Content{Role: "model", Parts: parts},
		TurnComplete: true,
		FinishReason: finishReason(finish),
	}
	if usage != nil {
		result.UsageMetadata = usageMetadata(usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens)
	}
	yield(result, nil)
}

func functionCallPart(call toolCall, index int) *genai.Part {
	args := map[string]any{}
	if strings.TrimSpace(call.Function.Arguments) != "" {
		if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
			args = map[string]any{"_raw": call.Function.Arguments}
		}
	}
	id := call.ID
	if id == "" {
		id = fmt.Sprintf("call_%d", index)
	}
	return &genai.Part{FunctionCall: &genai.FunctionCall{
		ID:   id,
		Name: call.Function.Name,
		Args: args,
	}}
}

func finishReason(reason string) genai.FinishReason {
	switch reason {
	case "length":
		return genai.FinishReasonMaxTokens
	case "content_filter":
		return genai.FinishReasonSafety
	case "tool_calls", "function_call", "stop", "":
		return genai.FinishReasonStop
	default:
		return genai.FinishReason(reason)
	}
}

func usageMetadata(prompt, completion, total int32) *genai.GenerateContentResponseUsageMetadata {
	return &genai.GenerateContentResponseUsageMetadata{
		PromptTokenCount:     prompt,
		CandidatesTokenCount: completion,
		TotalTokenCount:      total,
	}
}

func providerError(data []byte) string {
	var payload struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(data, &payload) == nil && payload.Error.Message != "" {
		return payload.Error.Message
	}
	text := strings.TrimSpace(string(data))
	if len(text) > 500 {
		text = text[:500]
	}
	return text
}

func normalizeSchema(schema any) any {
	raw, err := json.Marshal(schema)
	if err != nil {
		return nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil
	}
	return cleanSchema(value)
}

func cleanSchema(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			switch key {
			case "propertyOrdering", "$schema", "$defs", "definitions", "$id", "title":
				continue
			case "type":
				if s, ok := item.(string); ok {
					out[key] = strings.ToLower(s)
					continue
				}
			case "anyOf":
				if list, ok := item.([]any); ok {
					// OpenAI function schemas do not reliably support null unions.
					var nonNull []any
					for _, entry := range list {
						if m, ok := entry.(map[string]any); ok {
							if t, _ := m["type"].(string); t == "null" {
								continue
							}
						}
						nonNull = append(nonNull, cleanSchema(entry))
					}
					if len(nonNull) == 1 {
						if m, ok := nonNull[0].(map[string]any); ok {
							for k, val := range m {
								out[k] = val
							}
							continue
						}
					}
					out[key] = nonNull
					continue
				}
			}
			out[key] = cleanSchema(item)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = cleanSchema(item)
		}
		return out
	default:
		return value
	}
}
