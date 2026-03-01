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
	notifierdesktop "github.com/user/ai-workflow/internal/plugins/notifier-desktop"
	reviewaipanel "github.com/user/ai-workflow/internal/plugins/review-ai-panel"
	reviewlocal "github.com/user/ai-workflow/internal/plugins/review-local"
	runtimeprocess "github.com/user/ai-workflow/internal/plugins/runtime-process"
	scmlocalgit "github.com/user/ai-workflow/internal/plugins/scm-local-git"
	storesqlite "github.com/user/ai-workflow/internal/plugins/store-sqlite"
	trackerlocal "github.com/user/ai-workflow/internal/plugins/tracker-local"
	"github.com/user/ai-workflow/internal/secretary"
)

// BootstrapSet contains initialized plugins required by engine bootstrap.
type BootstrapSet struct {
	Agents     map[string]core.AgentPlugin
	Runtime    core.RuntimePlugin
	Store      core.Store
	ReviewGate core.ReviewGate
	Tracker    core.Tracker
	SCM        core.SCM
	Notifier   core.Notifier
}

const (
	defaultReviewGatePlugin = "review-ai-panel"
	localReviewGatePlugin   = "review-local"
	defaultTrackerPlugin    = "tracker-local"
	defaultSCMPlugin        = "local-git"
	defaultNotifierPlugin   = "desktop"
)

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

	reviewGateName := strings.TrimSpace(effective.Secretary.ReviewGatePlugin)
	if reviewGateName == "" {
		reviewGateName = defaultReviewGatePlugin
	}
	reviewGateModule, ok := registry.Get(core.SlotReviewGate, reviewGateName)
	if !ok {
		return nil, fmt.Errorf("unknown plugin: slot=%s name=%s", core.SlotReviewGate, reviewGateName)
	}
	reviewGateRaw, err := reviewGateModule.Factory(map[string]any{
		"store":      storePlugin.Store(),
		"max_rounds": effective.Secretary.ReviewPanel.MaxRounds,
	})
	if err != nil {
		return nil, fmt.Errorf("build review gate plugin %q: %w", reviewGateName, err)
	}
	reviewGatePlugin, ok := reviewGateRaw.(core.ReviewGate)
	if !ok {
		return nil, fmt.Errorf("plugin is not a review gate plugin: slot=%s name=%s", core.SlotReviewGate, reviewGateName)
	}

	trackerModule, ok := registry.Get(core.SlotTracker, defaultTrackerPlugin)
	if !ok {
		return nil, fmt.Errorf("unknown plugin: slot=%s name=%s", core.SlotTracker, defaultTrackerPlugin)
	}
	trackerRaw, err := trackerModule.Factory(nil)
	if err != nil {
		return nil, fmt.Errorf("build tracker plugin %q: %w", defaultTrackerPlugin, err)
	}
	trackerPlugin, ok := trackerRaw.(core.Tracker)
	if !ok {
		return nil, fmt.Errorf("plugin is not a tracker plugin: slot=%s name=%s", core.SlotTracker, defaultTrackerPlugin)
	}

	scmModule, ok := registry.Get(core.SlotSCM, defaultSCMPlugin)
	if !ok {
		return nil, fmt.Errorf("unknown plugin: slot=%s name=%s", core.SlotSCM, defaultSCMPlugin)
	}
	scmRaw, err := scmModule.Factory(nil)
	if err != nil {
		return nil, fmt.Errorf("build scm plugin %q: %w", defaultSCMPlugin, err)
	}
	scmPlugin, ok := scmRaw.(core.SCM)
	if !ok {
		return nil, fmt.Errorf("plugin is not a scm plugin: slot=%s name=%s", core.SlotSCM, defaultSCMPlugin)
	}

	notifierModule, ok := registry.Get(core.SlotNotifier, defaultNotifierPlugin)
	if !ok {
		return nil, fmt.Errorf("unknown plugin: slot=%s name=%s", core.SlotNotifier, defaultNotifierPlugin)
	}
	notifierRaw, err := notifierModule.Factory(nil)
	if err != nil {
		return nil, fmt.Errorf("build notifier plugin %q: %w", defaultNotifierPlugin, err)
	}
	notifierPlugin, ok := notifierRaw.(core.Notifier)
	if !ok {
		return nil, fmt.Errorf("plugin is not a notifier plugin: slot=%s name=%s", core.SlotNotifier, defaultNotifierPlugin)
	}

	return &BootstrapSet{
		Agents:     agents,
		Runtime:    runtimePlugin,
		Store:      storePlugin.Store(),
		ReviewGate: reviewGatePlugin,
		Tracker:    trackerPlugin,
		SCM:        scmPlugin,
		Notifier:   notifierPlugin,
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
		{
			Name: defaultReviewGatePlugin,
			Slot: core.SlotReviewGate,
			Factory: func(cfg map[string]any) (core.Plugin, error) {
				if cfg == nil {
					return nil, fmt.Errorf("%s requires store dependency", defaultReviewGatePlugin)
				}
				rawStore, ok := cfg["store"]
				if !ok {
					return nil, fmt.Errorf("%s requires store dependency", defaultReviewGatePlugin)
				}
				store, ok := rawStore.(core.Store)
				if !ok || store == nil {
					return nil, fmt.Errorf("%s requires valid store dependency", defaultReviewGatePlugin)
				}

				panel := secretary.NewDefaultReviewPanel(store)
				if maxRounds, ok := cfg["max_rounds"].(int); ok && maxRounds > 0 {
					panel.MaxRounds = maxRounds
				}
				return reviewaipanel.New(store, panel), nil
			},
		},
		{
			Name: localReviewGatePlugin,
			Slot: core.SlotReviewGate,
			Factory: func(cfg map[string]any) (core.Plugin, error) {
				if cfg == nil {
					return nil, fmt.Errorf("%s requires store dependency", localReviewGatePlugin)
				}
				rawStore, ok := cfg["store"]
				if !ok {
					return nil, fmt.Errorf("%s requires store dependency", localReviewGatePlugin)
				}
				store, ok := rawStore.(core.Store)
				if !ok || store == nil {
					return nil, fmt.Errorf("%s requires valid store dependency", localReviewGatePlugin)
				}
				return reviewlocal.New(store), nil
			},
		},
		{
			Name: defaultTrackerPlugin,
			Slot: core.SlotTracker,
			Factory: func(map[string]any) (core.Plugin, error) {
				return trackerlocal.New(), nil
			},
		},
		{
			Name: defaultSCMPlugin,
			Slot: core.SlotSCM,
			Factory: func(cfg map[string]any) (core.Plugin, error) {
				repoDir := stringFromMap(cfg, "repo_dir", ".")
				return scmlocalgit.New(repoDir), nil
			},
		},
		{
			Name: defaultNotifierPlugin,
			Slot: core.SlotNotifier,
			Factory: func(map[string]any) (core.Plugin, error) {
				return notifierdesktop.New(), nil
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
