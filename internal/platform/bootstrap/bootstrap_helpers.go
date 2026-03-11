package bootstrap

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/yoke233/ai-workflow/internal/adapters/http"
	"github.com/yoke233/ai-workflow/internal/adapters/sandbox"
	"github.com/yoke233/ai-workflow/internal/adapters/store/sqlite"
	flowapp "github.com/yoke233/ai-workflow/internal/application/flow"
	"github.com/yoke233/ai-workflow/internal/core"
	"github.com/yoke233/ai-workflow/internal/platform/appdata"
	"github.com/yoke233/ai-workflow/internal/platform/config"
	"github.com/yoke233/ai-workflow/internal/platform/configruntime"
)

func currentPRFlowPrompts(runtimeManager *configruntime.Manager, bootstrapCfg *config.Config) flowapp.PRFlowPrompts {
	var prompts config.RuntimePromptsConfig
	if runtimeManager != nil {
		prompts = runtimeManager.GetRuntime().Prompts
	} else if bootstrapCfg != nil {
		prompts = bootstrapCfg.Runtime.Prompts
	}
	return flowapp.MergePRFlowPrompts(flowapp.PRFlowPrompts{
		Global: flowapp.PRProviderPrompts{
			ImplementObjective:  strings.TrimSpace(prompts.PRImplementObjective),
			GateObjective:       strings.TrimSpace(prompts.PRGateObjective),
			MergeReworkFeedback: strings.TrimSpace(prompts.PRMergeReworkFeedback),
		},
		GitHub: flowapp.PRProviderPrompts{
			ImplementObjective:  strings.TrimSpace(prompts.PRProviders.GitHub.ImplementObjective),
			GateObjective:       strings.TrimSpace(prompts.PRProviders.GitHub.GateObjective),
			MergeReworkFeedback: strings.TrimSpace(prompts.PRProviders.GitHub.MergeReworkFeedback),
			MergeStates: flowapp.PRMergeStatePrompts{
				Default:  strings.TrimSpace(prompts.PRProviders.GitHub.MergeStates.Default),
				Dirty:    strings.TrimSpace(prompts.PRProviders.GitHub.MergeStates.Dirty),
				Blocked:  strings.TrimSpace(prompts.PRProviders.GitHub.MergeStates.Blocked),
				Behind:   strings.TrimSpace(prompts.PRProviders.GitHub.MergeStates.Behind),
				Unstable: strings.TrimSpace(prompts.PRProviders.GitHub.MergeStates.Unstable),
				Draft:    strings.TrimSpace(prompts.PRProviders.GitHub.MergeStates.Draft),
			},
		},
		CodeUp: flowapp.PRProviderPrompts{
			ImplementObjective:  strings.TrimSpace(prompts.PRProviders.CodeUp.ImplementObjective),
			GateObjective:       strings.TrimSpace(prompts.PRProviders.CodeUp.GateObjective),
			MergeReworkFeedback: strings.TrimSpace(prompts.PRProviders.CodeUp.MergeReworkFeedback),
			MergeStates: flowapp.PRMergeStatePrompts{
				Default:  strings.TrimSpace(prompts.PRProviders.CodeUp.MergeStates.Default),
				Dirty:    strings.TrimSpace(prompts.PRProviders.CodeUp.MergeStates.Dirty),
				Blocked:  strings.TrimSpace(prompts.PRProviders.CodeUp.MergeStates.Blocked),
				Behind:   strings.TrimSpace(prompts.PRProviders.CodeUp.MergeStates.Behind),
				Unstable: strings.TrimSpace(prompts.PRProviders.CodeUp.MergeStates.Unstable),
				Draft:    strings.TrimSpace(prompts.PRProviders.CodeUp.MergeStates.Draft),
			},
		},
		GitLab: flowapp.PRProviderPrompts{
			ImplementObjective:  strings.TrimSpace(prompts.PRProviders.GitLab.ImplementObjective),
			GateObjective:       strings.TrimSpace(prompts.PRProviders.GitLab.GateObjective),
			MergeReworkFeedback: strings.TrimSpace(prompts.PRProviders.GitLab.MergeReworkFeedback),
			MergeStates: flowapp.PRMergeStatePrompts{
				Default:  strings.TrimSpace(prompts.PRProviders.GitLab.MergeStates.Default),
				Dirty:    strings.TrimSpace(prompts.PRProviders.GitLab.MergeStates.Dirty),
				Blocked:  strings.TrimSpace(prompts.PRProviders.GitLab.MergeStates.Blocked),
				Behind:   strings.TrimSpace(prompts.PRProviders.GitLab.MergeStates.Behind),
				Unstable: strings.TrimSpace(prompts.PRProviders.GitLab.MergeStates.Unstable),
				Draft:    strings.TrimSpace(prompts.PRProviders.GitLab.MergeStates.Draft),
			},
		},
	})
}

func buildSandbox(cfg *config.Config, dataDir string) sandbox.Sandbox {
	if cfg == nil || !cfg.Runtime.Sandbox.Enabled {
		return sandbox.NoopSandbox{}
	}

	requireAuth := false
	if raw := strings.ToLower(strings.TrimSpace(os.Getenv("AI_WORKFLOW_CODEX_REQUIRE_AUTH"))); raw != "" {
		switch raw {
		case "1", "true", "yes", "on":
			requireAuth = true
		}
	}

	homeSandbox := sandbox.HomeDirSandbox{
		DataDir:          dataDir,
		SkillsRoot:       filepath.Join(dataDir, "skills"),
		RequireCodexAuth: requireAuth,
	}

	switch strings.ToLower(strings.TrimSpace(cfg.Runtime.Sandbox.Provider)) {
	case "", "home_dir":
		return homeSandbox
	case "litebox":
		return sandbox.LiteBoxSandbox{
			Base:          homeSandbox,
			BridgeCommand: strings.TrimSpace(cfg.Runtime.Sandbox.LiteBox.BridgeCommand),
			BridgeArgs:    append([]string(nil), cfg.Runtime.Sandbox.LiteBox.BridgeArgs...),
			RunnerPath:    strings.TrimSpace(cfg.Runtime.Sandbox.LiteBox.RunnerPath),
			RunnerArgs:    append([]string(nil), cfg.Runtime.Sandbox.LiteBox.RunnerArgs...),
		}
	default:
		slog.Warn("sandbox: unknown provider, fallback to home_dir", "provider", cfg.Runtime.Sandbox.Provider)
		return homeSandbox
	}
}

func buildRuntimeManager(store *sqlite.Store) *configruntime.Manager {
	dataDir, err := appdata.ResolveDataDir()
	if err != nil {
		return nil
	}

	cfgPath := filepath.Join(dataDir, "config.toml")
	secretsPath := secretsFilePath(dataDir)
	runtimeManager, err := configruntime.NewManager(cfgPath, secretsPath, configruntime.DisabledMCPEnv(), slog.Default(), func(ctx context.Context, snap *configruntime.Snapshot) error {
		return configruntime.SyncRegistry(ctx, store, snap)
	})
	if err != nil {
		slog.Warn("bootstrap: config runtime disabled", "error", err)
		return nil
	}
	return runtimeManager
}

func buildAPIOptions(
	bootstrapCfg *config.Config,
	runtimeManager *configruntime.Manager,
	leadAgent api.LeadChatService,
	scheduler flowapp.Scheduler,
	registry core.AgentRegistry,
	dagGen api.DAGGenerator,
) []api.HandlerOption {
	enabled := bootstrapCfg != nil && bootstrapCfg.Runtime.Sandbox.Enabled
	provider := ""
	if bootstrapCfg != nil {
		provider = bootstrapCfg.Runtime.Sandbox.Provider
	}
	skillsRoot := ""
	if dataDir, err := appdata.ResolveDataDir(); err == nil {
		skillsRoot = filepath.Join(dataDir, "skills")
	}

	return []api.HandlerOption{
		api.WithLeadAgent(leadAgent),
		api.WithScheduler(scheduler),
		api.WithRegistry(registry),
		api.WithDAGGenerator(dagGen),
		api.WithSandboxInspector(sandbox.NewDefaultSupportInspector(enabled, provider)),
		api.WithSkillsRoot(skillsRoot),
		api.WithPRFlowPromptsProvider(func() flowapp.PRFlowPrompts {
			return currentPRFlowPrompts(runtimeManager, bootstrapCfg)
		}),
	}
}
