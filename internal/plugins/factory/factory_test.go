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
	if set.ReviewGate == nil {
		t.Fatal("expected review gate to be initialized")
	}
	if set.ReviewGate.Name() != "ai-panel" {
		t.Fatalf("expected review gate name ai-panel, got %q", set.ReviewGate.Name())
	}
	if set.Tracker == nil {
		t.Fatal("expected tracker to be initialized")
	}
	if set.Tracker.Name() != "local" {
		t.Fatalf("expected tracker name local, got %q", set.Tracker.Name())
	}
	if set.SCM == nil {
		t.Fatal("expected scm to be initialized")
	}
	if set.SCM.Name() != "local-git" {
		t.Fatalf("expected scm name local-git, got %q", set.SCM.Name())
	}
	if set.Notifier == nil {
		t.Fatal("expected notifier to be initialized")
	}
	if set.Notifier.Name() != "desktop" {
		t.Fatalf("expected notifier name desktop, got %q", set.Notifier.Name())
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

func TestFactoryBuildReviewGateCanSwitchToLocal(t *testing.T) {
	cfg := config.Defaults()
	cfg.Store.Path = ":memory:"
	cfg.Secretary.ReviewGatePlugin = "review-local"

	set, err := BuildFromConfig(cfg)
	if err != nil {
		t.Fatalf("BuildFromConfig returned error: %v", err)
	}
	defer set.Store.Close()

	if set.ReviewGate == nil {
		t.Fatal("expected review gate to be initialized")
	}
	if set.ReviewGate.Name() != "local" {
		t.Fatalf("expected review gate name local, got %q", set.ReviewGate.Name())
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
