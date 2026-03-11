package bootstrap

import (
	"strings"

	flowapp "github.com/yoke233/ai-workflow/internal/application/flow"
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
