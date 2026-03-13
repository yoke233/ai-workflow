package llmconfig

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/yoke233/ai-workflow/internal/platform/config"
	"github.com/yoke233/ai-workflow/internal/platform/configruntime"
)

const (
	ProviderOpenAIChatCompletion = "openai_chat_completion"
	ProviderOpenAIResponse       = "openai_response"
	ProviderAnthropic            = "anthropic"
)

var ErrLLMConfigUnavailable = errors.New("llm config runtime unavailable")

type Report struct {
	DefaultConfigID string                         `json:"default_config_id"`
	Configs         []config.RuntimeLLMEntryConfig `json:"configs"`
}

type UpdateRequest struct {
	DefaultConfigID *string                         `json:"default_config_id,omitempty"`
	Configs         *[]config.RuntimeLLMEntryConfig `json:"configs,omitempty"`
}

type Inspector interface {
	Inspect(ctx context.Context) Report
}

type ControlService interface {
	Inspector
	Update(ctx context.Context, req UpdateRequest) (Report, error)
}

type ReadOnlyControlService struct {
	cfg config.RuntimeLLMConfig
}

func NewReadOnlyControlService(cfg config.RuntimeLLMConfig) ReadOnlyControlService {
	return ReadOnlyControlService{cfg: normalizeConfig(cfg)}
}

func (s ReadOnlyControlService) Inspect(_ context.Context) Report {
	return buildReport(s.cfg)
}

func (s ReadOnlyControlService) Update(ctx context.Context, _ UpdateRequest) (Report, error) {
	return s.Inspect(ctx), ErrLLMConfigUnavailable
}

type RuntimeControlService struct {
	manager  *configruntime.Manager
	fallback config.RuntimeLLMConfig
}

func NewRuntimeControlService(manager *configruntime.Manager, fallback config.RuntimeLLMConfig) RuntimeControlService {
	return RuntimeControlService{
		manager:  manager,
		fallback: normalizeConfig(fallback),
	}
}

func (s RuntimeControlService) Inspect(_ context.Context) Report {
	return buildReport(s.currentConfig())
}

func (s RuntimeControlService) Update(ctx context.Context, req UpdateRequest) (Report, error) {
	if s.manager == nil {
		return s.Inspect(ctx), ErrLLMConfigUnavailable
	}
	if req.DefaultConfigID == nil && req.Configs == nil {
		return s.Inspect(ctx), fmt.Errorf("default_config_id or configs is required")
	}

	current := s.manager.GetRuntime()
	next := normalizeConfig(current.LLM)

	if req.Configs != nil {
		next.Configs = cloneEntries(*req.Configs)
	}
	if req.DefaultConfigID != nil {
		next.DefaultConfigID = strings.TrimSpace(*req.DefaultConfigID)
	}

	next = normalizeConfig(next)
	if err := validateConfig(next); err != nil {
		return s.Inspect(ctx), err
	}

	current.LLM = next
	if _, err := s.manager.UpdateRuntime(ctx, current); err != nil {
		return s.Inspect(ctx), err
	}
	return s.Inspect(ctx), nil
}

func (s RuntimeControlService) currentConfig() config.RuntimeLLMConfig {
	if s.manager != nil {
		if snap := s.manager.Current(); snap != nil && snap.Config != nil {
			return normalizeConfig(snap.Config.Runtime.LLM)
		}
	}
	return normalizeConfig(s.fallback)
}

func buildReport(cfg config.RuntimeLLMConfig) Report {
	cfg = normalizeConfig(cfg)
	return Report{
		DefaultConfigID: cfg.DefaultConfigID,
		Configs:         cloneEntries(cfg.Configs),
	}
}

func normalizeConfig(cfg config.RuntimeLLMConfig) config.RuntimeLLMConfig {
	cfg.DefaultConfigID = strings.TrimSpace(cfg.DefaultConfigID)
	cfg.Configs = cloneEntries(cfg.Configs)
	for i := range cfg.Configs {
		cfg.Configs[i].ID = strings.TrimSpace(cfg.Configs[i].ID)
		cfg.Configs[i].Type = normalizeProviderType(cfg.Configs[i].Type)
		cfg.Configs[i].BaseURL = strings.TrimSpace(cfg.Configs[i].BaseURL)
		cfg.Configs[i].APIKey = strings.TrimSpace(cfg.Configs[i].APIKey)
		cfg.Configs[i].Model = strings.TrimSpace(cfg.Configs[i].Model)
	}
	if cfg.DefaultConfigID == "" && len(cfg.Configs) > 0 {
		cfg.DefaultConfigID = cfg.Configs[0].ID
	}
	return cfg
}

func validateConfig(cfg config.RuntimeLLMConfig) error {
	seen := make(map[string]struct{}, len(cfg.Configs))
	for _, item := range cfg.Configs {
		if item.ID == "" {
			return fmt.Errorf("llm config id is required")
		}
		if _, ok := seen[item.ID]; ok {
			return fmt.Errorf("duplicate llm config id %q", item.ID)
		}
		seen[item.ID] = struct{}{}
		if !isKnownProviderType(item.Type) {
			return fmt.Errorf("unknown llm provider type %q", item.Type)
		}
	}
	if cfg.DefaultConfigID != "" {
		if _, ok := seen[cfg.DefaultConfigID]; !ok {
			return fmt.Errorf("default llm config %q not found", cfg.DefaultConfigID)
		}
	}
	return nil
}

func cloneEntries(in []config.RuntimeLLMEntryConfig) []config.RuntimeLLMEntryConfig {
	if in == nil {
		return nil
	}
	out := make([]config.RuntimeLLMEntryConfig, len(in))
	copy(out, in)
	return out
}

func normalizeProviderType(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case ProviderOpenAIChatCompletion:
		return ProviderOpenAIChatCompletion
	case ProviderAnthropic:
		return ProviderAnthropic
	case "", ProviderOpenAIResponse:
		return ProviderOpenAIResponse
	default:
		return strings.ToLower(strings.TrimSpace(provider))
	}
}

func isKnownProviderType(provider string) bool {
	switch provider {
	case ProviderOpenAIChatCompletion, ProviderOpenAIResponse, ProviderAnthropic:
		return true
	default:
		return false
	}
}
