package bootstrap

import (
	"path/filepath"

	"github.com/yoke233/ai-workflow/internal/adapters/http"
	"github.com/yoke233/ai-workflow/internal/adapters/sandbox"
	flowapp "github.com/yoke233/ai-workflow/internal/application/flow"
	"github.com/yoke233/ai-workflow/internal/core"
	"github.com/yoke233/ai-workflow/internal/platform/appdata"
	"github.com/yoke233/ai-workflow/internal/platform/config"
	"github.com/yoke233/ai-workflow/internal/platform/configruntime"
)

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
