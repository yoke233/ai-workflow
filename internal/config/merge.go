package config

func MergeAgentConfig(base, override *AgentConfig) *AgentConfig {
	if override == nil {
		return base
	}
	if base == nil {
		return override
	}

	out := *base
	if override.Binary != nil {
		out.Binary = override.Binary
	}
	if override.MaxTurns != nil {
		out.MaxTurns = override.MaxTurns
	}
	if override.Model != nil {
		out.Model = override.Model
	}
	if override.Reasoning != nil {
		out.Reasoning = override.Reasoning
	}
	if override.Sandbox != nil {
		out.Sandbox = override.Sandbox
	}
	if override.Approval != nil {
		out.Approval = override.Approval
	}
	return &out
}
