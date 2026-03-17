package configruntime

import (
	"github.com/yoke233/ai-workflow/internal/core"
	"github.com/yoke233/ai-workflow/internal/platform/config"
	"github.com/yoke233/ai-workflow/internal/platform/profilellm"
)

func BuildAgents(cfg *config.Config) []*core.AgentProfile {
	if cfg == nil {
		return nil
	}

	return convertProfiles(cfg.Runtime.Agents.Drivers, cfg.Runtime.Agents.Profiles, cfg.Runtime.LLM.Configs)
}

func convertProfiles(driverCfgs []config.RuntimeDriverConfig, profileCfgs []config.RuntimeProfileConfig, llmCfgs []config.RuntimeLLMEntryConfig) []*core.AgentProfile {
	// Build driver lookup map.
	driverMap := make(map[string]config.RuntimeDriverConfig, len(driverCfgs))
	for _, d := range driverCfgs {
		driverMap[d.ID] = d
	}
	llmMap := make(map[string]config.RuntimeLLMEntryConfig, len(llmCfgs))
	for _, item := range llmCfgs {
		llmMap[item.ID] = item
	}

	out := make([]*core.AgentProfile, len(profileCfgs))
	for i, c := range profileCfgs {
		actions := make([]core.AgentAction, len(c.ActionsAllowed))
		for j, action := range c.ActionsAllowed {
			actions[j] = core.AgentAction(action)
		}
		var driverCfg core.DriverConfig
		if d, ok := driverMap[c.Driver]; ok {
			driverCfg = core.DriverConfig{
				ID:            d.ID,
				LaunchCommand: d.LaunchCommand,
				LaunchArgs:    append([]string(nil), d.LaunchArgs...),
				SandboxArgs:   append([]string(nil), d.SandboxArgs...),
				Env:           cloneStringMap(d.Env),
				CapabilitiesMax: core.DriverCapabilities{
					FSRead:   d.CapabilitiesMax.FSRead,
					FSWrite:  d.CapabilitiesMax.FSWrite,
					Terminal: d.CapabilitiesMax.Terminal,
				},
			}
			if llmCfg, ok := llmMap[c.LLMConfigID]; ok {
				if profilellm.ValidateDriverProviderCompatibility(d.ID, d.LaunchCommand, d.LaunchArgs, llmCfg.Type) == nil {
					driverCfg.Env = profilellm.MergeEnv(profilellm.BuildEnv(profilellm.ProviderConfig{
						ID:                   llmCfg.ID,
						Type:                 llmCfg.Type,
						BaseURL:              llmCfg.BaseURL,
						APIKey:               llmCfg.APIKey,
						Model:                llmCfg.Model,
						Temperature:          llmCfg.Temperature,
						MaxOutputTokens:      llmCfg.MaxOutputTokens,
						ReasoningEffort:      llmCfg.ReasoningEffort,
						ThinkingBudgetTokens: llmCfg.ThinkingBudgetTokens,
					}), driverCfg.Env)
				}
			}
		}
		out[i] = &core.AgentProfile{
			ID:             c.ID,
			Name:           c.Name,
			DriverID:       c.Driver,
			LLMConfigID:    c.LLMConfigID,
			Driver:         driverCfg,
			Role:           core.AgentRole(c.Role),
			Capabilities:   append([]string(nil), c.Capabilities...),
			ActionsAllowed: actions,
			PromptTemplate: c.PromptTemplate,
			Skills:         append([]string(nil), c.Skills...),
			Session: core.ProfileSession{
				Reuse:              c.Session.Reuse,
				MaxTurns:           c.Session.MaxTurns,
				IdleTTL:            c.Session.IdleTTL.Duration,
				ThreadBootTemplate: c.Session.ThreadBootTemplate,
				MaxContextTokens:   c.Session.MaxContextTokens,
				ContextWarnRatio:   c.Session.ContextWarnRatio,
			},
			MCP: core.ProfileMCP{
				Enabled: c.MCP.Enabled,
				Tools:   append([]string(nil), c.MCP.Tools...),
			},
		}
	}
	return out
}
