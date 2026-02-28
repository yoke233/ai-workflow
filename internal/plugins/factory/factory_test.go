package factory

import (
	"strings"
	"testing"

	"github.com/user/ai-workflow/internal/config"
)

func TestFactoryBuildKnownPlugin(t *testing.T) {
	cfg := config.Defaults()
	cfg.Store.Path = ":memory:"

	set, err := BuildFromConfig(cfg)
	if err != nil {
		t.Fatalf("BuildFromConfig returned error: %v", err)
	}
	defer set.Store.Close()

	if set.Store == nil {
		t.Fatal("expected store to be initialized")
	}
	if set.Runtime == nil {
		t.Fatal("expected runtime to be initialized")
	}
	if _, ok := set.Agents["claude"]; !ok {
		t.Fatal("expected claude agent to be initialized")
	}
	if _, ok := set.Agents["codex"]; !ok {
		t.Fatal("expected codex agent to be initialized")
	}
}

func TestFactoryBuildUnknownPlugin(t *testing.T) {
	cfg := config.Defaults()
	cfg.Store.Driver = "unknown-driver"
	cfg.Store.Path = ":memory:"

	_, err := BuildFromConfig(cfg)
	if err == nil {
		t.Fatal("expected BuildFromConfig to fail for unknown plugin")
	}
	if !strings.Contains(err.Error(), "unknown plugin") {
		t.Fatalf("expected unknown plugin error, got %v", err)
	}
}

func TestFactoryBuildUnknownRuntimePlugin(t *testing.T) {
	cfg := config.Defaults()
	cfg.Store.Path = ":memory:"
	cfg.Runtime.Driver = "unknown-runtime"

	_, err := BuildFromConfig(cfg)
	if err == nil {
		t.Fatal("expected BuildFromConfig to fail for unknown runtime plugin")
	}
	if !strings.Contains(err.Error(), "unknown plugin") {
		t.Fatalf("expected unknown plugin error, got %v", err)
	}
}

func TestFactoryBuildUnknownAgentPlugin(t *testing.T) {
	cfg := config.Defaults()
	cfg.Store.Path = ":memory:"
	cfg.Agents.Codex.Plugin = stringPtr("unknown-agent")

	_, err := BuildFromConfig(cfg)
	if err == nil {
		t.Fatal("expected BuildFromConfig to fail for unknown agent plugin")
	}
	if !strings.Contains(err.Error(), "unknown plugin") {
		t.Fatalf("expected unknown plugin error, got %v", err)
	}
}

func stringPtr(v string) *string { return &v }
