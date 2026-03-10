package configruntime

import (
	"strings"

	"github.com/yoke233/ai-workflow/internal/config"
	v2core "github.com/yoke233/ai-workflow/internal/v2/core"
)

func BuildV2Agents(cfg *config.Config) ([]*v2core.AgentDriver, []*v2core.AgentProfile) {
	if cfg == nil {
		return nil, nil
	}

	var drivers []*v2core.AgentDriver
	var profiles []*v2core.AgentProfile

	if len(cfg.V2.Agents.Drivers) > 0 {
		drivers = convertDrivers(cfg.V2.Agents.Drivers)
		profiles = convertProfiles(cfg.V2.Agents.Profiles)
	} else if len(cfg.EffectiveAgentProfiles()) > 0 {
		for _, ap := range cfg.EffectiveAgentProfiles() {
			drivers = append(drivers, &v2core.AgentDriver{
				ID:            ap.Name,
				LaunchCommand: ap.LaunchCommand,
				LaunchArgs:    append([]string(nil), ap.LaunchArgs...),
				Env:           cloneStringMap(ap.Env),
				CapabilitiesMax: v2core.DriverCapabilities{
					FSRead:   ap.CapabilitiesMax.FSRead,
					FSWrite:  ap.CapabilitiesMax.FSWrite,
					Terminal: ap.CapabilitiesMax.Terminal,
				},
			})
		}
		for _, rc := range cfg.Roles {
			var actions []v2core.Action
			if rc.Capabilities.FSRead {
				actions = append(actions, v2core.ActionReadContext, v2core.ActionSearchFiles)
			}
			if rc.Capabilities.FSWrite {
				actions = append(actions, v2core.ActionFSWrite)
			}
			if rc.Capabilities.Terminal {
				actions = append(actions, v2core.ActionTerminal)
			}
			profiles = append(profiles, &v2core.AgentProfile{
				ID:             rc.Name,
				Name:           rc.Name,
				DriverID:       rc.Agent,
				Role:           inferV2Role(rc.Name),
				ActionsAllowed: actions,
				PromptTemplate: rc.PromptTemplate,
				Session: v2core.ProfileSession{
					Reuse:    rc.Session.Reuse,
					MaxTurns: rc.Session.MaxTurns,
				},
				MCP: v2core.ProfileMCP{
					Enabled: rc.MCP.Enabled,
					Tools:   append([]string(nil), rc.MCP.Tools...),
				},
			})
		}
	}

	return ensurePRReviewer(drivers, profiles)
}

func convertDrivers(cfgs []config.V2DriverConfig) []*v2core.AgentDriver {
	out := make([]*v2core.AgentDriver, len(cfgs))
	for i, c := range cfgs {
		out[i] = &v2core.AgentDriver{
			ID:            c.ID,
			LaunchCommand: c.LaunchCommand,
			LaunchArgs:    append([]string(nil), c.LaunchArgs...),
			Env:           cloneStringMap(c.Env),
			CapabilitiesMax: v2core.DriverCapabilities{
				FSRead:   c.CapabilitiesMax.FSRead,
				FSWrite:  c.CapabilitiesMax.FSWrite,
				Terminal: c.CapabilitiesMax.Terminal,
			},
		}
	}
	return out
}

func convertProfiles(cfgs []config.V2ProfileConfig) []*v2core.AgentProfile {
	out := make([]*v2core.AgentProfile, len(cfgs))
	for i, c := range cfgs {
		actions := make([]v2core.Action, len(c.ActionsAllowed))
		for j, action := range c.ActionsAllowed {
			actions[j] = v2core.Action(action)
		}
		out[i] = &v2core.AgentProfile{
			ID:             c.ID,
			Name:           c.Name,
			DriverID:       c.Driver,
			Role:           v2core.AgentRole(c.Role),
			Capabilities:   append([]string(nil), c.Capabilities...),
			ActionsAllowed: actions,
			PromptTemplate: c.PromptTemplate,
			Skills:         append([]string(nil), c.Skills...),
			Session: v2core.ProfileSession{
				Reuse:    c.Session.Reuse,
				MaxTurns: c.Session.MaxTurns,
				IdleTTL:  c.Session.IdleTTL.Duration,
			},
			MCP: v2core.ProfileMCP{
				Enabled: c.MCP.Enabled,
				Tools:   append([]string(nil), c.MCP.Tools...),
			},
		}
	}
	return out
}

func ensurePRReviewer(drivers []*v2core.AgentDriver, profiles []*v2core.AgentProfile) ([]*v2core.AgentDriver, []*v2core.AgentProfile) {
	const reviewerID = "pr-reviewer"
	for _, profile := range profiles {
		if profile != nil && profile.ID == reviewerID {
			return drivers, profiles
		}
	}

	hasCodex := false
	for _, driver := range drivers {
		if driver != nil && strings.TrimSpace(driver.ID) == "codex" {
			hasCodex = true
			break
		}
	}
	if !hasCodex {
		drivers = append(drivers, &v2core.AgentDriver{
			ID:            "codex",
			LaunchCommand: "npx",
			LaunchArgs:    []string{"-y", "@zed-industries/codex-acp"},
			CapabilitiesMax: v2core.DriverCapabilities{
				FSRead:   true,
				FSWrite:  true,
				Terminal: true,
			},
		})
	}

	profiles = append(profiles, &v2core.AgentProfile{
		ID:           reviewerID,
		Name:         "PR Reviewer (Codex)",
		DriverID:     "codex",
		Role:         v2core.RoleGate,
		Capabilities: []string{"pr.review"},
		ActionsAllowed: []v2core.Action{
			v2core.ActionReadContext,
			v2core.ActionSearchFiles,
			v2core.ActionTerminal,
			v2core.ActionApprove,
			v2core.ActionReject,
			v2core.ActionSubmit,
		},
		PromptTemplate: "review",
		Session: v2core.ProfileSession{
			Reuse:    true,
			MaxTurns: 12,
		},
	})
	return drivers, profiles
}

func inferV2Role(name string) v2core.AgentRole {
	switch name {
	case "team_leader":
		return v2core.RoleLead
	case "reviewer", "aggregator":
		return v2core.RoleGate
	case "worker":
		return v2core.RoleWorker
	case "plan_parser":
		return v2core.RoleSupport
	default:
		return v2core.RoleWorker
	}
}
