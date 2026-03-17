package config

import (
	"strings"
	"testing"
)

func TestValidateRuntimeAgentBindingsAcceptsCompatibleProfileLLM(t *testing.T) {
	cfg := &Config{}
	cfg.Runtime.LLM.Configs = []RuntimeLLMEntryConfig{{
		ID:    "openai-response-default",
		Type:  "openai_response",
		Model: "gpt-4.1-mini",
	}}
	cfg.Runtime.Agents.Drivers = []RuntimeDriverConfig{{
		ID:            "codex-acp",
		LaunchCommand: "npx",
		LaunchArgs:    []string{"-y", "@zed-industries/codex-acp"},
	}}
	cfg.Runtime.Agents.Profiles = []RuntimeProfileConfig{{
		ID:          "worker",
		Driver:      "codex-acp",
		LLMConfigID: "openai-response-default",
		Role:        "worker",
	}}

	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateRuntimeAgentBindingsRejectsMissingLLMConfig(t *testing.T) {
	cfg := &Config{}
	cfg.Runtime.LLM.Configs = []RuntimeLLMEntryConfig{{
		ID:    "anthropic-default",
		Type:  "anthropic",
		Model: "claude-3-7-sonnet-latest",
	}}
	cfg.Runtime.Agents.Drivers = []RuntimeDriverConfig{{
		ID:            "claude-acp",
		LaunchCommand: "npx",
		LaunchArgs:    []string{"-y", "@zed-industries/claude-agent-acp"},
	}}
	cfg.Runtime.Agents.Profiles = []RuntimeProfileConfig{{
		ID:          "lead",
		Driver:      "claude-acp",
		LLMConfigID: "missing",
		Role:        "lead",
	}}

	err := Validate(cfg)
	if err == nil || !strings.Contains(err.Error(), `llm_config_id "missing" not found`) {
		t.Fatalf("Validate() error = %v, want missing llm_config_id", err)
	}
}

func TestValidateRuntimeAgentBindingsRejectsIncompatibleProvider(t *testing.T) {
	cfg := &Config{}
	cfg.Runtime.LLM.Configs = []RuntimeLLMEntryConfig{{
		ID:    "anthropic-default",
		Type:  "anthropic",
		Model: "claude-3-7-sonnet-latest",
	}}
	cfg.Runtime.Agents.Drivers = []RuntimeDriverConfig{{
		ID:            "codex-acp",
		LaunchCommand: "npx",
		LaunchArgs:    []string{"-y", "@zed-industries/codex-acp"},
	}}
	cfg.Runtime.Agents.Profiles = []RuntimeProfileConfig{{
		ID:          "worker",
		Driver:      "codex-acp",
		LLMConfigID: "anthropic-default",
		Role:        "worker",
	}}

	err := Validate(cfg)
	if err == nil || !strings.Contains(err.Error(), `only supports provider "openai_response"`) {
		t.Fatalf("Validate() error = %v, want incompatible provider", err)
	}
}
