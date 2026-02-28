package config

import "fmt"

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
	if override.DefaultTools != nil {
		out.DefaultTools = cloneStringSlicePtr(override.DefaultTools)
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

func MergeForPipeline(global *Config, project *ConfigLayer, override map[string]any) (*Config, error) {
	merged := Defaults()
	if global != nil {
		merged = *global
	}

	ApplyConfigLayer(&merged, project)

	if len(override) > 0 {
		layer, err := decodeLayerFromMap(override)
		if err != nil {
			return nil, fmt.Errorf("decode pipeline override: %w", err)
		}
		ApplyConfigLayer(&merged, layer)
	}

	if err := ApplyEnvOverrides(&merged); err != nil {
		return nil, err
	}
	return &merged, nil
}

func ApplyConfigLayer(cfg *Config, layer *ConfigLayer) {
	if cfg == nil || layer == nil {
		return
	}

	if agents := layer.Agents; agents != nil {
		cfg.Agents.Claude = MergeAgentConfig(cfg.Agents.Claude, agents.Claude)
		cfg.Agents.Codex = MergeAgentConfig(cfg.Agents.Codex, agents.Codex)
		cfg.Agents.OpenSpec = MergeAgentConfig(cfg.Agents.OpenSpec, agents.OpenSpec)
	}

	if pipeline := layer.Pipeline; pipeline != nil {
		if pipeline.DefaultTemplate != nil {
			cfg.Pipeline.DefaultTemplate = *pipeline.DefaultTemplate
		}
		if pipeline.GlobalTimeout != nil {
			cfg.Pipeline.GlobalTimeout = *pipeline.GlobalTimeout
		}
		if pipeline.AutoInferTemplate != nil {
			cfg.Pipeline.AutoInferTemplate = *pipeline.AutoInferTemplate
		}
		if pipeline.MaxTotalRetries != nil {
			cfg.Pipeline.MaxTotalRetries = *pipeline.MaxTotalRetries
		}
	}

	if scheduler := layer.Scheduler; scheduler != nil {
		if scheduler.MaxGlobalAgents != nil {
			cfg.Scheduler.MaxGlobalAgents = *scheduler.MaxGlobalAgents
		}
		if scheduler.MaxProjectPipelines != nil {
			cfg.Scheduler.MaxProjectPipelines = *scheduler.MaxProjectPipelines
		}
	}

	if server := layer.Server; server != nil {
		if server.Host != nil {
			cfg.Server.Host = *server.Host
		}
		if server.Port != nil {
			cfg.Server.Port = *server.Port
		}
		if server.AuthEnabled != nil {
			cfg.Server.AuthEnabled = *server.AuthEnabled
		}
		if server.AuthToken != nil {
			cfg.Server.AuthToken = *server.AuthToken
		}
	}

	if github := layer.GitHub; github != nil {
		if github.Enabled != nil {
			cfg.GitHub.Enabled = *github.Enabled
		}
		if github.Token != nil {
			cfg.GitHub.Token = *github.Token
		}
		if github.AppID != nil {
			cfg.GitHub.AppID = *github.AppID
		}
		if github.PrivateKeyPath != nil {
			cfg.GitHub.PrivateKeyPath = *github.PrivateKeyPath
		}
		if github.InstallationID != nil {
			cfg.GitHub.InstallationID = *github.InstallationID
		}
		if github.WebhookSecret != nil {
			cfg.GitHub.WebhookSecret = *github.WebhookSecret
		}
	}

	if store := layer.Store; store != nil {
		if store.Driver != nil {
			cfg.Store.Driver = *store.Driver
		}
		if store.Path != nil {
			cfg.Store.Path = *store.Path
		}
	}

	if log := layer.Log; log != nil {
		if log.Level != nil {
			cfg.Log.Level = *log.Level
		}
		if log.File != nil {
			cfg.Log.File = *log.File
		}
		if log.MaxSizeMB != nil {
			cfg.Log.MaxSizeMB = *log.MaxSizeMB
		}
		if log.MaxAgeDays != nil {
			cfg.Log.MaxAgeDays = *log.MaxAgeDays
		}
	}
}

func cloneStringSlicePtr(in *[]string) *[]string {
	if in == nil {
		return nil
	}
	out := append([]string(nil), (*in)...)
	return &out
}
