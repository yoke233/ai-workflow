package factory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/user/ai-workflow/internal/config"
	"github.com/user/ai-workflow/internal/core"
	agentclaude "github.com/user/ai-workflow/internal/plugins/agent-claude"
	agentcodex "github.com/user/ai-workflow/internal/plugins/agent-codex"
	runtimeprocess "github.com/user/ai-workflow/internal/plugins/runtime-process"
	storesqlite "github.com/user/ai-workflow/internal/plugins/store-sqlite"
)

// BootstrapSet contains initialized plugins required by engine bootstrap.
type BootstrapSet struct {
	Agents  map[string]core.AgentPlugin
	Runtime core.RuntimePlugin
	Store   core.Store
}

type storeProvider interface {
	core.Plugin
	Store() core.Store
}

type storeAdapter struct {
	name  string
	store core.Store
}

func (s *storeAdapter) Name() string               { return s.name }
func (s *storeAdapter) Init(context.Context) error { return nil }
func (s *storeAdapter) Close() error               { return s.store.Close() }
func (s *storeAdapter) Store() core.Store          { return s.store }

func BuildFromConfig(cfg config.Config) (*BootstrapSet, error) {
	registry, err := newDefaultRegistry()
	if err != nil {
		return nil, err
	}
	return buildWithRegistry(registry, cfg)
}

func buildWithRegistry(registry *core.Registry, cfg config.Config) (*BootstrapSet, error) {
	effective := withDefaults(cfg)

	storeName := strings.TrimSpace(effective.Store.Driver)
	storeModule, ok := registry.Get(core.SlotStore, storeName)
	if !ok {
		return nil, fmt.Errorf("unknown plugin: slot=%s name=%s", core.SlotStore, storeName)
	}

	storePluginRaw, err := storeModule.Factory(map[string]any{
		"path": effective.Store.Path,
	})
	if err != nil {
		return nil, fmt.Errorf("build store plugin %q: %w", storeName, err)
	}
	storePlugin, ok := storePluginRaw.(storeProvider)
	if !ok {
		return nil, fmt.Errorf("plugin is not a store provider: slot=%s name=%s", core.SlotStore, storeName)
	}

	runtimeName := strings.TrimSpace(effective.Runtime.Driver)
	runtimeModule, ok := registry.Get(core.SlotRuntime, runtimeName)
	if !ok {
		return nil, fmt.Errorf("unknown plugin: slot=%s name=%s", core.SlotRuntime, runtimeName)
	}
	runtimeRaw, err := runtimeModule.Factory(nil)
	if err != nil {
		return nil, fmt.Errorf("build runtime plugin %q: %w", runtimeName, err)
	}
	runtimePlugin, ok := runtimeRaw.(core.RuntimePlugin)
	if !ok {
		return nil, fmt.Errorf("plugin is not a runtime plugin: slot=%s name=%s", core.SlotRuntime, runtimeName)
	}

	agentConfigs := map[string]*config.AgentConfig{
		"claude": effective.Agents.Claude,
		"codex":  effective.Agents.Codex,
	}
	agents := make(map[string]core.AgentPlugin, len(agentConfigs))
	for agentName, agentCfg := range agentConfigs {
		if agentCfg == nil {
			continue
		}
		moduleName := agentName
		if agentCfg.Plugin != nil && strings.TrimSpace(*agentCfg.Plugin) != "" {
			moduleName = strings.TrimSpace(*agentCfg.Plugin)
		}

		module, ok := registry.Get(core.SlotAgent, moduleName)
		if !ok {
			return nil, fmt.Errorf("unknown plugin: slot=%s name=%s", core.SlotAgent, moduleName)
		}
		raw, err := module.Factory(agentConfigToMap(agentCfg))
		if err != nil {
			return nil, fmt.Errorf("build agent plugin %q: %w", moduleName, err)
		}
		agentPlugin, ok := raw.(core.AgentPlugin)
		if !ok {
			return nil, fmt.Errorf("plugin is not an agent plugin: slot=%s name=%s", core.SlotAgent, moduleName)
		}
		agents[agentName] = agentPlugin
	}
	if len(agents) == 0 {
		return nil, fmt.Errorf("no agent plugins configured")
	}

	return &BootstrapSet{
		Agents:  agents,
		Runtime: runtimePlugin,
		Store:   storePlugin.Store(),
	}, nil
}

func newDefaultRegistry() (*core.Registry, error) {
	registry := core.NewRegistry()
	modules := []core.PluginModule{
		{
			Name: "claude",
			Slot: core.SlotAgent,
			Factory: func(cfg map[string]any) (core.Plugin, error) {
				binary := stringFromMap(cfg, "binary", "claude")
				return agentclaude.New(binary), nil
			},
		},
		{
			Name: "codex",
			Slot: core.SlotAgent,
			Factory: func(cfg map[string]any) (core.Plugin, error) {
				binary := stringFromMap(cfg, "binary", "codex")
				model := stringFromMap(cfg, "model", "gpt-5.3-codex")
				reasoning := stringFromMap(cfg, "reasoning", "high")
				return agentcodex.New(binary, model, reasoning), nil
			},
		},
		{
			Name: "process",
			Slot: core.SlotRuntime,
			Factory: func(map[string]any) (core.Plugin, error) {
				return runtimeprocess.New(), nil
			},
		},
		{
			Name: "sqlite",
			Slot: core.SlotStore,
			Factory: func(cfg map[string]any) (core.Plugin, error) {
				storePath := expandPath(stringFromMap(cfg, "path", "~/.ai-workflow/data.db"))
				if storePath != ":memory:" {
					if err := os.MkdirAll(filepath.Dir(storePath), 0o755); err != nil {
						return nil, fmt.Errorf("ensure sqlite dir: %w", err)
					}
				}
				store, err := storesqlite.New(storePath)
				if err != nil {
					return nil, err
				}
				return &storeAdapter{name: "sqlite", store: store}, nil
			},
		},
	}

	for _, module := range modules {
		if err := registry.Register(module); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

func withDefaults(cfg config.Config) config.Config {
	def := config.Defaults()

	if cfg.Agents.Claude == nil {
		cfg.Agents.Claude = def.Agents.Claude
	}
	if cfg.Agents.Codex == nil {
		cfg.Agents.Codex = def.Agents.Codex
	}
	if cfg.Runtime.Driver == "" {
		cfg.Runtime.Driver = def.Runtime.Driver
	}
	if cfg.Store.Driver == "" {
		cfg.Store.Driver = def.Store.Driver
	}
	if cfg.Store.Path == "" {
		cfg.Store.Path = def.Store.Path
	}
	return cfg
}

func agentConfigToMap(agent *config.AgentConfig) map[string]any {
	out := map[string]any{}
	if agent == nil {
		return out
	}
	if agent.Binary != nil {
		out["binary"] = *agent.Binary
	}
	if agent.Model != nil {
		out["model"] = *agent.Model
	}
	if agent.Reasoning != nil {
		out["reasoning"] = *agent.Reasoning
	}
	return out
}

func stringFromMap(cfg map[string]any, key, fallback string) string {
	if cfg != nil {
		if raw, ok := cfg[key]; ok {
			if value, ok := raw.(string); ok && strings.TrimSpace(value) != "" {
				return value
			}
		}
	}
	return fallback
}

func expandPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return trimmed
	}
	if trimmed == "~" {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return home
		}
		return trimmed
	}
	if strings.HasPrefix(trimmed, "~/") || strings.HasPrefix(trimmed, "~\\") {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return trimmed
		}
		suffix := strings.TrimPrefix(strings.TrimPrefix(trimmed, "~/"), "~\\")
		return filepath.Join(home, filepath.FromSlash(strings.ReplaceAll(suffix, "\\", "/")))
	}
	return trimmed
}
