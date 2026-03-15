package bootstrap

import (
	"context"
	"path/filepath"
	"testing"

	membus "github.com/yoke233/ai-workflow/internal/adapters/events/memory"
	"github.com/yoke233/ai-workflow/internal/adapters/llm"
	"github.com/yoke233/ai-workflow/internal/adapters/store/sqlite"
	flowapp "github.com/yoke233/ai-workflow/internal/application/flow"
	"github.com/yoke233/ai-workflow/internal/core"
	"github.com/yoke233/ai-workflow/internal/platform/config"
)

func TestResolveWorkItemSchedulerConfigUsesConfiguredProjectRuns(t *testing.T) {
	t.Parallel()

	cfg := config.Defaults()
	cfg.Scheduler.MaxProjectRuns = 5

	got := resolveWorkItemSchedulerConfig(&cfg)
	if got.MaxConcurrentWorkItems != 5 {
		t.Fatalf("MaxConcurrentWorkItems = %d, want 5", got.MaxConcurrentWorkItems)
	}
}

func TestResolveWorkItemSchedulerConfigDefaults(t *testing.T) {
	t.Parallel()

	got := resolveWorkItemSchedulerConfig(nil)
	if got.MaxConcurrentWorkItems != 2 {
		t.Fatalf("MaxConcurrentWorkItems = %d, want 2", got.MaxConcurrentWorkItems)
	}
}

func TestBuildWorkItemEngineAppliesConfiguredAgentConcurrency(t *testing.T) {
	t.Parallel()

	store := newBootstrapTestStore(t)
	bus := membus.NewBus()
	cfg := config.Defaults()
	cfg.Scheduler.MaxGlobalAgents = 6

	engine := buildWorkItemEngine(store, bus, noopActionExecutor, nil, nil, &cfg, "", SCMTokens{}, nil)
	if got := engine.MaxConcurrency(); got != 6 {
		t.Fatalf("engine.MaxConcurrency() = %d, want 6", got)
	}
}

func TestBuildWorkItemEngineUsesDefaultConcurrencyWhenUnset(t *testing.T) {
	t.Parallel()

	store := newBootstrapTestStore(t)
	bus := membus.NewBus()
	cfg := config.Defaults()
	cfg.Scheduler.MaxGlobalAgents = 0

	engine := buildWorkItemEngine(store, bus, noopActionExecutor, nil, nil, &cfg, "", SCMTokens{}, nil)
	if got := engine.MaxConcurrency(); got != 4 {
		t.Fatalf("engine.MaxConcurrency() = %d, want 4", got)
	}
}

func TestResolveFlowLLMConfigPrefersRuntimeLLMDefault(t *testing.T) {
	t.Parallel()

	cfg := config.Defaults()
	cfg.Runtime.Collector.MaxRetries = 3
	cfg.Runtime.LLM.DefaultConfigID = "openai-chat-default"
	cfg.Runtime.LLM.Configs = []config.RuntimeLLMEntryConfig{
		{
			ID:      "openai-chat-default",
			Type:    llm.ProviderOpenAIChatCompletion,
			BaseURL: "https://example.test/v1",
			APIKey:  "chat-key",
			Model:   "chat-model",
		},
	}

	got, source, ok := resolveFlowLLMConfig(&cfg)
	if !ok {
		t.Fatalf("resolveFlowLLMConfig() ok = false, want true")
	}
	if source != "runtime.llm" {
		t.Fatalf("source = %q, want runtime.llm", source)
	}
	if got.Provider != llm.ProviderOpenAIChatCompletion {
		t.Fatalf("Provider = %q, want %q", got.Provider, llm.ProviderOpenAIChatCompletion)
	}
	if got.APIKey != "chat-key" || got.Model != "chat-model" {
		t.Fatalf("got = %#v, want runtime.llm credentials", got)
	}
	if got.MaxRetries != 3 {
		t.Fatalf("MaxRetries = %d, want 3", got.MaxRetries)
	}
}

func TestResolveFlowLLMConfigSupportsAnthropicDefault(t *testing.T) {
	t.Parallel()

	cfg := config.Defaults()
	cfg.Runtime.Collector.MaxRetries = 2
	cfg.Runtime.LLM.DefaultConfigID = "anthropic-default"
	cfg.Runtime.LLM.Configs = []config.RuntimeLLMEntryConfig{
		{
			ID:                   "anthropic-default",
			Type:                 llm.ProviderAnthropic,
			BaseURL:              "https://api.anthropic.com",
			APIKey:               "anthropic-key",
			Model:                "claude-3-5-sonnet-latest",
			Temperature:          0.25,
			MaxOutputTokens:      3000,
			ThinkingBudgetTokens: 2048,
		},
	}

	got, source, ok := resolveFlowLLMConfig(&cfg)
	if !ok {
		t.Fatalf("resolveFlowLLMConfig() ok = false, want true")
	}
	if source != "runtime.llm" {
		t.Fatalf("source = %q, want runtime.llm", source)
	}
	if got.Provider != llm.ProviderAnthropic {
		t.Fatalf("Provider = %q, want %q", got.Provider, llm.ProviderAnthropic)
	}
	if got.APIKey != "anthropic-key" || got.Model != "claude-3-5-sonnet-latest" {
		t.Fatalf("got = %#v, want runtime.llm anthropic credentials", got)
	}
	if got.Temperature != 0.25 || got.MaxOutputTokens != 3000 || got.ThinkingBudgetTokens != 2048 {
		t.Fatalf("got provider tuning = %#v, want anthropic runtime tuning", got)
	}
	if got.MaxRetries != 2 {
		t.Fatalf("MaxRetries = %d, want 2", got.MaxRetries)
	}
}

func TestResolveFlowLLMConfigReturnsFalseWhenNoUsableConfig(t *testing.T) {
	t.Parallel()

	cfg := config.Defaults()
	cfg.Runtime.LLM.DefaultConfigID = "openai-chat-default"
	cfg.Runtime.LLM.Configs = []config.RuntimeLLMEntryConfig{
		{
			ID:      "openai-chat-default",
			Type:    llm.ProviderOpenAIChatCompletion,
			BaseURL: "https://example.test/v1",
			APIKey:  "",
			Model:   "chat-model",
		},
	}

	_, _, ok := resolveFlowLLMConfig(&cfg)
	if ok {
		t.Fatalf("resolveFlowLLMConfig() ok = true, want false")
	}
}

func newBootstrapTestStore(t *testing.T) *sqlite.Store {
	t.Helper()

	store, err := sqlite.New(filepath.Join(t.TempDir(), "bootstrap-test.db"))
	if err != nil {
		t.Fatalf("sqlite.New() error = %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	return store
}

func noopActionExecutor(context.Context, *core.Action, *core.Run) error {
	return nil
}

var _ flowapp.ActionExecutor = noopActionExecutor
