package llmconfig

import (
	"context"
	"testing"

	"github.com/yoke233/ai-workflow/internal/platform/config"
)

func TestBuildReportHidesConnectionFields(t *testing.T) {
	t.Parallel()

	report := buildReport(config.RuntimeLLMConfig{
		DefaultConfigID: "openai-prod",
		Configs: []config.RuntimeLLMEntryConfig{{
			ID:              "openai-prod",
			Type:            ProviderOpenAIResponse,
			BaseURL:         "https://api.example.com/v1",
			APIKey:          "sk-secret",
			Model:           "gpt-4.1-mini",
			Temperature:     0.3,
			ReasoningEffort: "medium",
		}},
	})

	if len(report.Configs) != 1 {
		t.Fatalf("configs len = %d, want 1", len(report.Configs))
	}
	if report.Configs[0].BaseURL != "" || report.Configs[0].APIKey != "" {
		t.Fatalf("connection fields should be hidden, got %+v", report.Configs[0])
	}
	if report.Configs[0].Model != "gpt-4.1-mini" {
		t.Fatalf("model = %q, want gpt-4.1-mini", report.Configs[0].Model)
	}
	if report.Configs[0].Temperature != 0.3 || report.Configs[0].ReasoningEffort != "medium" {
		t.Fatalf("non-secret tuning should remain visible, got %+v", report.Configs[0])
	}
}

func TestMergeEntriesPreservesExistingConnectionFields(t *testing.T) {
	t.Parallel()

	got := mergeEntries(
		[]config.RuntimeLLMEntryConfig{{
			ID:                   "openai-prod",
			Type:                 ProviderOpenAIResponse,
			BaseURL:              "https://api.example.com/v1",
			APIKey:               "sk-secret",
			Model:                "gpt-4.1-mini",
			ThinkingBudgetTokens: 2048,
		}},
		[]config.RuntimeLLMEntryConfig{{
			ID:                   "openai-prod",
			Type:                 ProviderOpenAIResponse,
			Model:                "gpt-4.1",
			ThinkingBudgetTokens: 0,
		}},
	)

	if len(got) != 1 {
		t.Fatalf("configs len = %d, want 1", len(got))
	}
	if got[0].BaseURL != "https://api.example.com/v1" {
		t.Fatalf("BaseURL = %q, want preserved value", got[0].BaseURL)
	}
	if got[0].APIKey != "sk-secret" {
		t.Fatalf("APIKey = %q, want preserved value", got[0].APIKey)
	}
	if got[0].Model != "gpt-4.1" {
		t.Fatalf("Model = %q, want updated value", got[0].Model)
	}
	if got[0].ThinkingBudgetTokens != 0 {
		t.Fatalf("ThinkingBudgetTokens = %d, want explicit reset to 0", got[0].ThinkingBudgetTokens)
	}
}

func TestNormalizeConfigFillsProviderDefaultBaseURL(t *testing.T) {
	t.Parallel()

	cfg := normalizeConfig(config.RuntimeLLMConfig{
		Configs: []config.RuntimeLLMEntryConfig{{
			ID:    "anthropic-default",
			Type:  ProviderAnthropic,
			Model: "claude-sonnet",
		}},
	})

	if len(cfg.Configs) != 1 {
		t.Fatalf("configs len = %d, want 1", len(cfg.Configs))
	}
	if cfg.Configs[0].BaseURL != "https://api.anthropic.com" {
		t.Fatalf("BaseURL = %q, want anthropic default", cfg.Configs[0].BaseURL)
	}
}

func TestValidateConfigRejectsInvalidThinkingBudget(t *testing.T) {
	t.Parallel()

	err := validateConfig(config.RuntimeLLMConfig{
		DefaultConfigID: "anthropic-default",
		Configs: []config.RuntimeLLMEntryConfig{{
			ID:                   "anthropic-default",
			Type:                 ProviderAnthropic,
			Model:                "claude-sonnet",
			ThinkingBudgetTokens: 512,
		}},
	})
	if err == nil {
		t.Fatal("validateConfig() should reject thinking_budget_tokens < 1024")
	}
}

func TestReadOnlyControlServiceUpdateReturnsSanitizedReport(t *testing.T) {
	t.Parallel()

	service := NewReadOnlyControlService(config.RuntimeLLMConfig{
		DefaultConfigID: "openai-prod",
		Configs: []config.RuntimeLLMEntryConfig{{
			ID:      "openai-prod",
			Type:    ProviderOpenAIChatCompletion,
			BaseURL: "https://api.openai.com/v1",
			APIKey:  "sk-secret",
			Model:   "gpt-4.1",
		}},
	})

	report, err := service.Update(context.Background(), UpdateRequest{})
	if err != ErrLLMConfigUnavailable {
		t.Fatalf("Update() error = %v, want %v", err, ErrLLMConfigUnavailable)
	}
	if len(report.Configs) != 1 || report.Configs[0].APIKey != "" || report.Configs[0].BaseURL != "" {
		t.Fatalf("Update() report should be sanitized, got %+v", report.Configs)
	}
}
