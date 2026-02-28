package config

import "time"

type Config struct {
	Agents    AgentsConfig    `yaml:"agents"`
	Pipeline  PipelineConfig  `yaml:"pipeline"`
	Scheduler SchedulerConfig `yaml:"scheduler"`
	Store     StoreConfig     `yaml:"store"`
	Log       LogConfig       `yaml:"log"`
}

type AgentsConfig struct {
	Claude   *AgentConfig `yaml:"claude"`
	Codex    *AgentConfig `yaml:"codex"`
	OpenSpec *AgentConfig `yaml:"openspec"`
}

type AgentConfig struct {
	Binary    *string `yaml:"binary"`
	MaxTurns  *int    `yaml:"default_max_turns"`
	Model     *string `yaml:"model"`
	Reasoning *string `yaml:"reasoning"`
	Sandbox   *string `yaml:"sandbox"`
	Approval  *string `yaml:"approval"`
}

type PipelineConfig struct {
	DefaultTemplate   string        `yaml:"default_template"`
	GlobalTimeout     time.Duration `yaml:"global_timeout"`
	AutoInferTemplate bool          `yaml:"auto_infer_template"`
	MaxTotalRetries   int           `yaml:"max_total_retries"`
}

type SchedulerConfig struct {
	MaxGlobalAgents     int `yaml:"max_global_agents"`
	MaxProjectPipelines int `yaml:"max_project_pipelines"`
}

type StoreConfig struct {
	Driver string `yaml:"driver"`
	Path   string `yaml:"path"`
}

type LogConfig struct {
	Level      string `yaml:"level"`
	File       string `yaml:"file"`
	MaxSizeMB  int    `yaml:"max_size_mb"`
	MaxAgeDays int    `yaml:"max_age_days"`
}
