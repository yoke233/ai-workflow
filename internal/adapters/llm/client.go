// Package llm provides a reusable OpenAI-compatible LLM client.
// It wraps the openai-go SDK and exposes two high-level methods:
//   - Complete: structured JSON output via JSON Schema (Structured Outputs).
//   - CompleteText: free-form text completion.
package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

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

// Config configures the LLM client.
type Config struct {
	BaseURL    string
	APIKey     string
	Model      string
	MaxRetries int           // 0 = no retry
	MinBackoff time.Duration // default 200ms
	MaxBackoff time.Duration // default 2s
}

// Client is a reusable OpenAI-compatible LLM client.
type Client struct {
	client     openai.Client
	model      shared.ResponsesModel
	maxRetries int
	minBackoff time.Duration
	maxBackoff time.Duration
}

// New creates a Client from the given Config.
func New(cfg Config) (*Client, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("llm: api_key is required")
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return nil, fmt.Errorf("llm: model is required")
	}

	opts := []option.RequestOption{option.WithAPIKey(cfg.APIKey)}
	if baseURL := strings.TrimSpace(cfg.BaseURL); baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}

	minBackoff := cfg.MinBackoff
	if minBackoff <= 0 {
		minBackoff = 200 * time.Millisecond
	}
	maxBackoff := cfg.MaxBackoff
	if maxBackoff <= 0 {
		maxBackoff = 2 * time.Second
	}

	return &Client{
		client:     openai.NewClient(opts...),
		model:      shared.ResponsesModel(strings.TrimSpace(cfg.Model)),
		maxRetries: max(0, cfg.MaxRetries),
		minBackoff: minBackoff,
		maxBackoff: maxBackoff,
	}, nil
}

// Complete calls the OpenAI Responses API with Structured Outputs (JSON Schema)
// and returns the raw JSON. The first tool's schema is used as the response format.
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
		resp, err := c.client.Responses.New(ctx, responses.ResponseNewParams{
			Model: c.model,
			Input: responses.ResponseNewParamsInputUnion{
				OfString: openai.String(prompt),
			},
			Temperature: openai.Float(0),
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
		})
		if err != nil {
			return "", err
		}
		return resp.OutputText(), nil
	})
}

// CompleteText calls the OpenAI Responses API for free-form text completion.
// Returns the raw text output.
func (c *Client) CompleteText(ctx context.Context, prompt string) (string, error) {
	if c == nil {
		return "", fmt.Errorf("llm: client is nil")
	}
	if strings.TrimSpace(prompt) == "" {
		return "", fmt.Errorf("llm: prompt is empty")
	}

	raw, err := c.doWithRetry(ctx, func(ctx context.Context) (string, error) {
		resp, err := c.client.Responses.New(ctx, responses.ResponseNewParams{
			Model: c.model,
			Input: responses.ResponseNewParamsInputUnion{
				OfString: openai.String(prompt),
			},
			Temperature: openai.Float(0),
		})
		if err != nil {
			return "", err
		}
		return resp.OutputText(), nil
	})
	if err != nil {
		return "", err
	}
	return string(raw), nil
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
