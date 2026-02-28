package config

import "time"

func Defaults() Config {
	return Config{
		Agents: AgentsConfig{
			Claude: &AgentConfig{
				Plugin:   ptrValue("claude"),
				Binary:   ptrValue("claude"),
				MaxTurns: ptrValue(30),
			},
			Codex: &AgentConfig{
				Plugin:    ptrValue("codex"),
				Binary:    ptrValue("codex"),
				Model:     ptrValue("gpt-5.3-codex"),
				Reasoning: ptrValue("high"),
				Sandbox:   ptrValue("workspace-write"),
				Approval:  ptrValue("never"),
			},
			OpenSpec: &AgentConfig{
				Binary: ptrValue("openspec"),
			},
		},
		Runtime: RuntimeConfig{
			Driver: "process",
		},
		Pipeline: PipelineConfig{
			DefaultTemplate:   "standard",
			GlobalTimeout:     2 * time.Hour,
			AutoInferTemplate: true,
			MaxTotalRetries:   5,
		},
		Scheduler: SchedulerConfig{
			MaxGlobalAgents:     3,
			MaxProjectPipelines: 2,
		},
		Store: StoreConfig{
			Driver: "sqlite",
			Path:   "~/.ai-workflow/data.db",
		},
		Log: LogConfig{
			Level:      "info",
			File:       "~/.ai-workflow/logs/app.log",
			MaxSizeMB:  100,
			MaxAgeDays: 30,
		},
	}
}

func ptrValue[T any](v T) *T { return &v }
