package llm

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientCompleteSupportsResponsesAndChatCompletions(t *testing.T) {
	tests := []struct {
		name         string
		provider     string
		wantPath     string
		responseBody string
	}{
		{
			name:     "responses",
			provider: ProviderOpenAIResponse,
			wantPath: "/responses",
			responseBody: `{
				"id":"resp_123",
				"object":"response",
				"created_at":1742000000,
				"model":"gpt-4.1-mini",
				"output":[{"id":"msg_1","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"{\"ok\":true}"}]}]
			}`,
		},
		{
			name:     "chat completions",
			provider: ProviderOpenAIChatCompletion,
			wantPath: "/chat/completions",
			responseBody: `{
				"id":"chatcmpl_123",
				"object":"chat.completion",
				"created":1742000000,
				"model":"gpt-4.1-mini",
				"choices":[{"index":0,"finish_reason":"stop","message":{"role":"assistant","content":"{\"ok\":true}","refusal":""}}]
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedPath string
			var capturedBody string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedPath = r.URL.Path
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("read body: %v", err)
				}
				capturedBody = string(body)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer srv.Close()

			client, err := New(Config{
				Provider: tt.provider,
				BaseURL:  srv.URL,
				APIKey:   "test-key",
				Model:    "gpt-4.1-mini",
			})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			raw, err := client.Complete(context.Background(), "say hi", []ToolDef{{
				Name:        "generate_dag",
				Description: "desc",
				InputSchema: map[string]any{"type": "object"},
			}})
			if err != nil {
				t.Fatalf("Complete() error = %v", err)
			}
			if string(raw) != `{"ok":true}` {
				t.Fatalf("Complete() = %s", string(raw))
			}
			if capturedPath != tt.wantPath {
				t.Fatalf("path = %q, want %q", capturedPath, tt.wantPath)
			}
			if !strings.Contains(capturedBody, "say hi") {
				t.Fatalf("body missing prompt: %s", capturedBody)
			}
			if tt.provider == ProviderOpenAIChatCompletion && !strings.Contains(capturedBody, "response_format") {
				t.Fatalf("chat completion body missing response_format: %s", capturedBody)
			}
		})
	}
}

func TestClientChatCompletionUsesMaxTokensParameter(t *testing.T) {
	tests := []struct {
		name string
		call func(*Client) error
	}{
		{
			name: "complete",
			call: func(client *Client) error {
				_, err := client.Complete(context.Background(), "say hi", []ToolDef{{
					Name:        "generate_dag",
					Description: "desc",
					InputSchema: map[string]any{"type": "object"},
				}})
				return err
			},
		},
		{
			name: "complete text",
			call: func(client *Client) error {
				_, err := client.CompleteText(context.Background(), "say hi")
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedBody string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/chat/completions" {
					t.Fatalf("path = %q, want /chat/completions", r.URL.Path)
				}
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("read body: %v", err)
				}
				capturedBody = string(body)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{
					"id":"chatcmpl_123",
					"object":"chat.completion",
					"created":1742000000,
					"model":"gpt-4.1-mini",
					"choices":[{"index":0,"finish_reason":"stop","message":{"role":"assistant","content":"{\"ok\":true}","refusal":""}}]
				}`))
			}))
			defer srv.Close()

			client, err := New(Config{
				Provider:        ProviderOpenAIChatCompletion,
				BaseURL:         srv.URL,
				APIKey:          "test-key",
				Model:           "gpt-4.1-mini",
				MaxOutputTokens: 321,
			})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			if err := tt.call(client); err != nil {
				t.Fatalf("call error = %v", err)
			}
			if !strings.Contains(capturedBody, "\"max_tokens\":321") {
				t.Fatalf("body missing max_tokens: %s", capturedBody)
			}
			if strings.Contains(capturedBody, "\"max_completion_tokens\"") {
				t.Fatalf("body unexpectedly contains max_completion_tokens: %s", capturedBody)
			}
		})
	}
}

func TestClientCompleteSupportsAnthropicMessages(t *testing.T) {
	var capturedPath string
	var capturedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		capturedBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"msg_123",
			"type":"message",
			"role":"assistant",
			"model":"claude-3-5-sonnet-latest",
			"stop_reason":"tool_use",
			"stop_sequence":"",
			"usage":{"input_tokens":10,"output_tokens":5},
			"content":[{"type":"tool_use","id":"toolu_123","name":"generate_dag","input":{"ok":true}}]
		}`))
	}))
	defer srv.Close()

	client, err := New(Config{
		Provider:             ProviderAnthropic,
		BaseURL:              srv.URL,
		APIKey:               "test-key",
		Model:                "claude-3-5-sonnet-latest",
		Temperature:          0.2,
		MaxOutputTokens:      2048,
		ThinkingBudgetTokens: 1024,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	raw, err := client.Complete(context.Background(), "say hi", []ToolDef{{
		Name:        "generate_dag",
		Description: "desc",
		InputSchema: map[string]any{"type": "object"},
	}})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if string(raw) != `{"ok":true}` {
		t.Fatalf("Complete() = %s", string(raw))
	}
	if capturedPath != "/v1/messages" {
		t.Fatalf("path = %q, want /v1/messages", capturedPath)
	}
	if !strings.Contains(capturedBody, "say hi") {
		t.Fatalf("body missing prompt: %s", capturedBody)
	}
	if !strings.Contains(capturedBody, "\"tool_choice\"") {
		t.Fatalf("body missing tool_choice: %s", capturedBody)
	}
	if !strings.Contains(capturedBody, "\"tools\"") {
		t.Fatalf("body missing tools: %s", capturedBody)
	}
	if !strings.Contains(capturedBody, "\"temperature\":0.2") {
		t.Fatalf("body missing temperature: %s", capturedBody)
	}
	if !strings.Contains(capturedBody, "\"max_tokens\":2048") {
		t.Fatalf("body missing max_tokens: %s", capturedBody)
	}
	if !strings.Contains(capturedBody, "\"thinking\":{\"budget_tokens\":1024") {
		t.Fatalf("body missing thinking config: %s", capturedBody)
	}
}

func TestClientCompleteTextSupportsChatCompletions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("path = %q, want /chat/completions", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"chatcmpl_123",
			"object":"chat.completion",
			"created":1742000000,
			"model":"gpt-4.1-mini",
			"choices":[{"index":0,"finish_reason":"stop","message":{"role":"assistant","content":"plain text","refusal":""}}]
		}`))
	}))
	defer srv.Close()

	client, err := New(Config{
		Provider: ProviderOpenAIChatCompletion,
		BaseURL:  srv.URL,
		APIKey:   "test-key",
		Model:    "gpt-4.1-mini",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	text, err := client.CompleteText(context.Background(), "say hi")
	if err != nil {
		t.Fatalf("CompleteText() error = %v", err)
	}
	if text != "plain text" {
		t.Fatalf("CompleteText() = %q, want plain text", text)
	}
}

func TestClientCompleteTextSupportsAnthropicMessages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Fatalf("path = %q, want /v1/messages", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"msg_123",
			"type":"message",
			"role":"assistant",
			"model":"claude-3-5-sonnet-latest",
			"stop_reason":"end_turn",
			"stop_sequence":"",
			"usage":{"input_tokens":10,"output_tokens":5},
			"content":[
				{"type":"text","text":"plain"},
				{"type":"text","text":"text"}
			]
		}`))
	}))
	defer srv.Close()

	client, err := New(Config{
		Provider:             ProviderAnthropic,
		BaseURL:              srv.URL,
		APIKey:               "test-key",
		Model:                "claude-3-5-sonnet-latest",
		Temperature:          0.1,
		MaxOutputTokens:      1536,
		ThinkingBudgetTokens: 1024,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	text, err := client.CompleteText(context.Background(), "say hi")
	if err != nil {
		t.Fatalf("CompleteText() error = %v", err)
	}
	if text != "plain\ntext" {
		t.Fatalf("CompleteText() = %q, want plain\\ntext", text)
	}
}

func TestNewRejectsUnknownProvider(t *testing.T) {
	if _, err := New(Config{
		Provider: "bogus",
		APIKey:   "test-key",
		Model:    "gpt-4.1-mini",
	}); err == nil {
		t.Fatal("New() should reject unknown provider")
	}
}

func TestNewRejectsInvalidThinkingBudget(t *testing.T) {
	if _, err := New(Config{
		Provider:             ProviderAnthropic,
		APIKey:               "test-key",
		Model:                "claude-3-5-sonnet-latest",
		MaxOutputTokens:      2048,
		ThinkingBudgetTokens: 512,
	}); err == nil {
		t.Fatal("New() should reject too-small thinking budget")
	}
}
