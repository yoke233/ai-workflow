// Package llm provides a reusable runtime LLM client.
// It wraps the official OpenAI and Anthropic SDKs and exposes two high-level methods:
//   - Complete: structured JSON output.
//   - CompleteText: free-form text completion.
package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	anthropicoption "github.com/anthropics/anthropic-sdk-go/option"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/responses"
	"github.com/openai/openai-go/shared"
)

// ToolDef describes a JSON schema tool for structured output extraction.
type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

const (
	ProviderOpenAIResponse       = "openai_response"
	ProviderOpenAIChatCompletion = "openai_chat_completion"
	ProviderAnthropic            = "anthropic"

	defaultAnthropicMaxTokens int64 = 4096
)

// Config configures the LLM client.
type Config struct {
	Provider             string
	BaseURL              string
	APIKey               string
	Model                string
	Temperature          float64       // default 0 to preserve deterministic behavior
	MaxOutputTokens      int64         // 0 = provider default
	ReasoningEffort      string        // OpenAI reasoning models: low/medium/high
	ThinkingBudgetTokens int64         // Anthropic extended thinking budget; 0 = disabled
	MaxRetries           int           // 0 = no retry
	MinBackoff           time.Duration // default 200ms
	MaxBackoff           time.Duration // default 2s
}

// Client is a reusable runtime LLM client.
type Client struct {
	openaiClient         openai.Client
	anthropicClient      anthropic.Client
	provider             string
	model                string
	temperature          float64
	maxOutputTokensLimit int64
	reasoningEffort      string
	thinkingBudgetTokens int64
	maxRetries           int
	minBackoff           time.Duration
	maxBackoff           time.Duration
}

// New creates a Client from the given Config.
func New(cfg Config) (*Client, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("llm: api_key is required")
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return nil, fmt.Errorf("llm: model is required")
	}
	provider := normalizeProvider(cfg.Provider)
	if provider == "" {
		return nil, fmt.Errorf("llm: unsupported provider %q", strings.TrimSpace(cfg.Provider))
	}
	if cfg.MaxOutputTokens < 0 {
		return nil, fmt.Errorf("llm: max_output_tokens must be >= 0")
	}
	if cfg.ThinkingBudgetTokens < 0 {
		return nil, fmt.Errorf("llm: thinking_budget_tokens must be >= 0")
	}
	reasoningEffort := normalizeReasoningEffort(cfg.ReasoningEffort)
	if strings.TrimSpace(cfg.ReasoningEffort) != "" && reasoningEffort == "" {
		return nil, fmt.Errorf("llm: unsupported reasoning_effort %q", strings.TrimSpace(cfg.ReasoningEffort))
	}
	if cfg.ThinkingBudgetTokens > 0 && cfg.ThinkingBudgetTokens < 1024 {
		return nil, fmt.Errorf("llm: thinking_budget_tokens must be >= 1024")
	}
	if cfg.MaxOutputTokens > 0 && cfg.ThinkingBudgetTokens >= cfg.MaxOutputTokens {
		return nil, fmt.Errorf("llm: thinking_budget_tokens must be less than max_output_tokens")
	}

	minBackoff := cfg.MinBackoff
	if minBackoff <= 0 {
		minBackoff = 200 * time.Millisecond
	}
	maxBackoff := cfg.MaxBackoff
	if maxBackoff <= 0 {
		maxBackoff = 2 * time.Second
	}

	client := &Client{
		provider:             provider,
		model:                strings.TrimSpace(cfg.Model),
		temperature:          cfg.Temperature,
		maxOutputTokensLimit: max(0, cfg.MaxOutputTokens),
		reasoningEffort:      reasoningEffort,
		thinkingBudgetTokens: max(0, cfg.ThinkingBudgetTokens),
		maxRetries:           max(0, cfg.MaxRetries),
		minBackoff:           minBackoff,
		maxBackoff:           maxBackoff,
	}

	switch provider {
	case ProviderAnthropic:
		opts := []anthropicoption.RequestOption{
			anthropicoption.WithAPIKey(cfg.APIKey),
			anthropicoption.WithMaxRetries(0),
		}
		if baseURL := strings.TrimSpace(cfg.BaseURL); baseURL != "" {
			opts = append(opts, anthropicoption.WithBaseURL(baseURL))
		}
		client.anthropicClient = anthropic.NewClient(opts...)
	default:
		opts := []option.RequestOption{
			option.WithAPIKey(cfg.APIKey),
			option.WithMaxRetries(0),
		}
		if baseURL := strings.TrimSpace(cfg.BaseURL); baseURL != "" {
			opts = append(opts, option.WithBaseURL(baseURL))
		}
		client.openaiClient = openai.NewClient(opts...)
	}

	return client, nil
}

// Complete returns structured JSON output. For Anthropic this is implemented via
// forced single-tool use and reading the tool input JSON from the response.
func (c *Client) Complete(ctx context.Context, prompt string, tools []ToolDef) (json.RawMessage, error) {
	if c == nil {
		return nil, fmt.Errorf("llm: client is nil")
	}
	if strings.TrimSpace(prompt) == "" {
		return nil, fmt.Errorf("llm: prompt is empty")
	}
	if len(tools) == 0 {
		return nil, fmt.Errorf("llm: no json schema tool definitions provided")
	}

	tool := tools[0]
	name := strings.TrimSpace(tool.Name)
	if name == "" {
		name = "extract_metadata"
	}
	schema := tool.InputSchema
	if schema == nil {
		return nil, fmt.Errorf("llm: tool %q schema is nil", name)
	}
	if _, ok := schema["additionalProperties"]; !ok {
		schema = cloneMap(schema)
		schema["additionalProperties"] = false
	}

	return c.doWithRetry(ctx, func(ctx context.Context) (string, error) {
		switch c.provider {
		case ProviderAnthropic:
			return c.completeAnthropic(ctx, prompt, tool, schema)
		case ProviderOpenAIChatCompletion:
			params := openai.ChatCompletionNewParams{
				Model: shared.ChatModel(c.model),
				Messages: []openai.ChatCompletionMessageParamUnion{
					openai.UserMessage(prompt),
				},
				Temperature:     openai.Float(c.temperature),
				ReasoningEffort: shared.ReasoningEffort(c.reasoningEffort),
				ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
					OfJSONSchema: &shared.ResponseFormatJSONSchemaParam{
						JSONSchema: shared.ResponseFormatJSONSchemaJSONSchemaParam{
							Name:        name,
							Schema:      schema,
							Strict:      openai.Bool(true),
							Description: openai.String(strings.TrimSpace(tool.Description)),
						},
					},
				},
			}
			if c.maxOutputTokensLimit > 0 {
				params.MaxTokens = openai.Int(c.maxOutputTokensLimit)
			}
			resp, err := c.openaiClient.Chat.Completions.New(ctx, params)
			if err != nil {
				return "", err
			}
			if len(resp.Choices) == 0 {
				return "", fmt.Errorf("llm: chat completion returned zero choices")
			}
			return resp.Choices[0].Message.Content, nil
		default:
			params := responses.ResponseNewParams{
				Model: shared.ResponsesModel(c.model),
				Input: responses.ResponseNewParamsInputUnion{
					OfString: openai.String(prompt),
				},
				Temperature: openai.Float(c.temperature),
				Reasoning:   c.openAIReasoning(),
				Text: responses.ResponseTextConfigParam{
					Format: responses.ResponseFormatTextConfigUnionParam{
						OfJSONSchema: &responses.ResponseFormatTextJSONSchemaConfigParam{
							Name:        name,
							Schema:      schema,
							Strict:      openai.Bool(true),
							Description: openai.String(strings.TrimSpace(tool.Description)),
						},
					},
				},
			}
			if c.maxOutputTokensLimit > 0 {
				params.MaxOutputTokens = openai.Int(c.maxOutputTokensLimit)
			}
			resp, err := c.openaiClient.Responses.New(ctx, params)
			if err != nil {
				return "", err
			}
			return resp.OutputText(), nil
		}
	})
}

// CompleteText returns free-form text output.
func (c *Client) CompleteText(ctx context.Context, prompt string) (string, error) {
	if c == nil {
		return "", fmt.Errorf("llm: client is nil")
	}
	if strings.TrimSpace(prompt) == "" {
		return "", fmt.Errorf("llm: prompt is empty")
	}

	raw, err := c.doTextWithRetry(ctx, func(ctx context.Context) (string, error) {
		switch c.provider {
		case ProviderAnthropic:
			return c.completeAnthropicText(ctx, prompt)
		case ProviderOpenAIChatCompletion:
			params := openai.ChatCompletionNewParams{
				Model: shared.ChatModel(c.model),
				Messages: []openai.ChatCompletionMessageParamUnion{
					openai.UserMessage(prompt),
				},
				Temperature:     openai.Float(c.temperature),
				ReasoningEffort: shared.ReasoningEffort(c.reasoningEffort),
			}
			if c.maxOutputTokensLimit > 0 {
				params.MaxTokens = openai.Int(c.maxOutputTokensLimit)
			}
			resp, err := c.openaiClient.Chat.Completions.New(ctx, params)
			if err != nil {
				return "", err
			}
			if len(resp.Choices) == 0 {
				return "", fmt.Errorf("llm: chat completion returned zero choices")
			}
			return resp.Choices[0].Message.Content, nil
		default:
			params := responses.ResponseNewParams{
				Model: shared.ResponsesModel(c.model),
				Input: responses.ResponseNewParamsInputUnion{
					OfString: openai.String(prompt),
				},
				Temperature: openai.Float(c.temperature),
				Reasoning:   c.openAIReasoning(),
			}
			if c.maxOutputTokensLimit > 0 {
				params.MaxOutputTokens = openai.Int(c.maxOutputTokensLimit)
			}
			resp, err := c.openaiClient.Responses.New(ctx, params)
			if err != nil {
				return "", err
			}
			return resp.OutputText(), nil
		}
	})
	if err != nil {
		return "", err
	}
	return raw, nil
}

func (c *Client) completeAnthropic(ctx context.Context, prompt string, tool ToolDef, schema map[string]any) (string, error) {
	name := strings.TrimSpace(tool.Name)
	if name == "" {
		name = "extract_metadata"
	}

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(c.model),
		MaxTokens: c.maxOutputTokens(),
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
		Temperature: anthropic.Float(c.temperature),
		Thinking:    c.anthropicThinking(),
		Tools: []anthropic.ToolUnionParam{{
			OfTool: &anthropic.ToolParam{
				Name:        name,
				Description: anthropic.String(strings.TrimSpace(tool.Description)),
				InputSchema: anthropicToolInputSchema(schema),
				Type:        anthropic.ToolTypeCustom,
			},
		}},
		ToolChoice: anthropic.ToolChoiceParamOfTool(name),
	}

	resp, err := c.anthropicClient.Messages.New(ctx, params)
	if err != nil {
		return "", err
	}

	raw, err := extractAnthropicToolInput(resp, name)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func (c *Client) completeAnthropicText(ctx context.Context, prompt string) (string, error) {
	resp, err := c.anthropicClient.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(c.model),
		MaxTokens: c.maxOutputTokens(),
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
		Temperature: anthropic.Float(c.temperature),
		Thinking:    c.anthropicThinking(),
	})
	if err != nil {
		return "", err
	}

	text := extractAnthropicText(resp)
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("llm: anthropic returned empty text output")
	}
	return text, nil
}

func anthropicToolInputSchema(schema map[string]any) anthropic.ToolInputSchemaParam {
	params := anthropic.ToolInputSchemaParam{}

	if properties, ok := schema["properties"]; ok {
		params.Properties = properties
	}
	if required, ok := schema["required"].([]string); ok {
		params.Required = append([]string(nil), required...)
	} else if requiredAny, ok := schema["required"].([]any); ok {
		params.Required = make([]string, 0, len(requiredAny))
		for _, item := range requiredAny {
			if value, ok := item.(string); ok {
				params.Required = append(params.Required, value)
			}
		}
	}

	extras := make(map[string]any)
	for key, value := range schema {
		switch key {
		case "type", "properties", "required":
			continue
		default:
			extras[key] = value
		}
	}
	if len(extras) > 0 {
		params.ExtraFields = extras
	}
	return params
}

func extractAnthropicToolInput(resp *anthropic.Message, toolName string) (json.RawMessage, error) {
	if resp == nil {
		return nil, fmt.Errorf("llm: anthropic returned nil response")
	}
	for _, block := range resp.Content {
		if block.Type != "tool_use" {
			continue
		}
		if toolName != "" && strings.TrimSpace(block.Name) != toolName {
			continue
		}
		raw := trimRawJSON(block.Input)
		if len(raw) == 0 {
			return nil, fmt.Errorf("llm: anthropic tool %q returned empty input", block.Name)
		}
		return raw, nil
	}
	return nil, fmt.Errorf("llm: anthropic response did not include expected tool_use block")
}

func extractAnthropicText(resp *anthropic.Message) string {
	if resp == nil {
		return ""
	}
	parts := make([]string, 0, len(resp.Content))
	for _, block := range resp.Content {
		if block.Type != "text" {
			continue
		}
		text := strings.TrimSpace(block.Text)
		if text == "" {
			continue
		}
		parts = append(parts, text)
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func trimRawJSON(in []byte) []byte {
	return []byte(strings.TrimSpace(string(in)))
}

func (c *Client) maxOutputTokens() int64 {
	if c == nil || c.maxOutputTokensLimit <= 0 {
		return defaultAnthropicMaxTokens
	}
	return c.maxOutputTokensLimit
}

func (c *Client) anthropicThinking() anthropic.ThinkingConfigParamUnion {
	if c == nil || c.thinkingBudgetTokens <= 0 {
		return anthropic.ThinkingConfigParamUnion{}
	}
	return anthropic.ThinkingConfigParamOfEnabled(c.thinkingBudgetTokens)
}

func (c *Client) openAIReasoning() shared.ReasoningParam {
	if c == nil || c.reasoningEffort == "" {
		return shared.ReasoningParam{}
	}
	return shared.ReasoningParam{
		Effort: shared.ReasoningEffort(c.reasoningEffort),
	}
}

func normalizeReasoningEffort(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return ""
	case "low":
		return "low"
	case "medium":
		return "medium"
	case "high":
		return "high"
	default:
		return ""
	}
}

// doWithRetry runs fn with exponential backoff retries.
// It strips code fences and validates JSON for structured output.
func (c *Client) doWithRetry(ctx context.Context, fn func(ctx context.Context) (string, error)) (json.RawMessage, error) {
	maxAttempts := c.maxRetries + 1
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		out, err := fn(ctx)
		if err == nil {
			out = strings.TrimSpace(out)
			out = StripCodeFences(out)
			if out == "" {
				lastErr = fmt.Errorf("llm: returned empty output")
			} else if !json.Valid([]byte(out)) {
				// For CompleteText this check may fail — caller handles raw text via CompleteText.
				// But for structured output (Complete), we validate JSON.
				lastErr = fmt.Errorf("llm: output is not valid json")
			} else {
				return json.RawMessage(out), nil
			}
		} else {
			lastErr = err
		}

		if attempt == maxAttempts || !IsRetryable(lastErr) {
			break
		}
		sleepBackoff(ctx, backoffDelay(attempt, c.minBackoff, c.maxBackoff))
	}
	return nil, fmt.Errorf("llm: failed after %d attempt(s): %w", maxAttempts, lastErr)
}

func (c *Client) doTextWithRetry(ctx context.Context, fn func(ctx context.Context) (string, error)) (string, error) {
	maxAttempts := c.maxRetries + 1
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		out, err := fn(ctx)
		if err == nil {
			out = strings.TrimSpace(out)
			out = StripCodeFences(out)
			if out == "" {
				lastErr = fmt.Errorf("llm: returned empty output")
			} else {
				return out, nil
			}
		} else {
			lastErr = err
		}

		if attempt == maxAttempts || !IsRetryable(lastErr) {
			break
		}
		sleepBackoff(ctx, backoffDelay(attempt, c.minBackoff, c.maxBackoff))
	}
	return "", fmt.Errorf("llm: failed after %d attempt(s): %w", maxAttempts, lastErr)
}

// IsRetryable returns true for errors worth retrying (network, 429, 5xx).
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return false
	}
	var apierr *openai.Error
	if errors.As(err, &apierr) {
		switch apierr.StatusCode {
		case 408, 409, 425, 429, 500, 502, 503, 504:
			return true
		default:
			return false
		}
	}
	var anthropicErr *anthropic.Error
	if errors.As(err, &anthropicErr) {
		switch anthropicErr.StatusCode {
		case 400, 401, 402, 403, 404:
			return false
		case 408, 409, 425, 429, 500, 502, 503, 504:
			return true
		default:
			return anthropicErr.StatusCode >= 500
		}
	}
	return true
}

func backoffDelay(attempt int, minBackoff, maxBackoff time.Duration) time.Duration {
	d := minBackoff << (attempt - 1)
	if d > maxBackoff {
		return maxBackoff
	}
	return d
}

func sleepBackoff(ctx context.Context, d time.Duration) {
	if d <= 0 {
		return
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

// StripCodeFences removes markdown code fences (```...```) from LLM output.
func StripCodeFences(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	lines := strings.Split(s, "\n")
	if len(lines) < 2 {
		return s
	}
	if strings.HasPrefix(strings.TrimSpace(lines[0]), "```") {
		lines = lines[1:]
	}
	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "```" {
		lines = lines[:len(lines)-1]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func normalizeProvider(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "", ProviderOpenAIResponse:
		return ProviderOpenAIResponse
	case ProviderOpenAIChatCompletion:
		return ProviderOpenAIChatCompletion
	case ProviderAnthropic:
		return ProviderAnthropic
	default:
		return ""
	}
}
