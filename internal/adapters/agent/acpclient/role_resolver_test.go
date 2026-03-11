package acpclient

import (
	"strings"
	"testing"
)

func TestResolveRoleReturnsProfiles(t *testing.T) {
	resolver := NewRoleResolver(
		[]AgentProfile{
			{
				ID:            "codex",
				LaunchCommand: "npx",
				CapabilitiesMax: ClientCapabilities{
					FSRead:   true,
					FSWrite:  true,
					Terminal: true,
				},
			},
		},
		[]RoleProfile{
			{
				ID:             "worker",
				AgentID:        "codex",
				PromptTemplate: "implement",
				Capabilities: ClientCapabilities{
					FSRead:   true,
					FSWrite:  true,
					Terminal: true,
				},
			},
		},
	)

	agent, role, err := resolver.Resolve("worker")
	if err != nil {
		t.Fatalf("expected resolve success, got err: %v", err)
	}

	if agent.ID != "codex" {
		t.Fatalf("expected agent codex, got %q", agent.ID)
	}
	if role.ID != "worker" {
		t.Fatalf("expected role worker, got %q", role.ID)
	}
	if role.PromptTemplate != "implement" {
		t.Fatalf("expected prompt template implement, got %q", role.PromptTemplate)
	}
}

func TestResolveRoleValidatesCapabilitySubset(t *testing.T) {
	resolver := NewRoleResolver(
		[]AgentProfile{
			{
				ID:            "claude",
				LaunchCommand: "claude-agent-acp",
				CapabilitiesMax: ClientCapabilities{
					FSRead:   true,
					FSWrite:  false,
					Terminal: false,
				},
			},
		},
		[]RoleProfile{
			{
				ID:      "reviewer",
				AgentID: "claude",
				Capabilities: ClientCapabilities{
					FSRead:   true,
					FSWrite:  true,
					Terminal: false,
				},
			},
		},
	)

	_, _, err := resolver.Resolve("reviewer")
	if err == nil {
		t.Fatal("expected subset validation error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "reviewer") || !strings.Contains(msg, "claude") {
		t.Fatalf("expected diagnostic error containing role and agent, got %q", msg)
	}
	if !strings.Contains(msg, "capabilities_max") || !strings.Contains(msg, "fs_write") {
		t.Fatalf("expected overflow details, got %q", msg)
	}
}

func TestResolveRoleMissingRoleReturnsDiagnostic(t *testing.T) {
	resolver := NewRoleResolver(nil, nil)

	_, _, err := resolver.Resolve("worker")
	if err == nil {
		t.Fatal("expected missing role error, got nil")
	}
	if !strings.Contains(err.Error(), "role") || !strings.Contains(err.Error(), "worker") {
		t.Fatalf("expected missing role diagnostic, got %q", err.Error())
	}
}

func TestRoleResolver_ListAgents(t *testing.T) {
	agents := []AgentProfile{
		{ID: "claude", LaunchCommand: "npx", LaunchArgs: []string{"-y", "@zed-industries/claude-agent-acp"}},
		{ID: "codex", LaunchCommand: "npx", LaunchArgs: []string{"-y", "@zed-industries/codex-acp"}},
	}
	roles := []RoleProfile{{ID: "team_leader", AgentID: "claude"}}
	resolver := NewRoleResolver(agents, roles)

	result := resolver.ListAgents()
	if len(result) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(result))
	}
	names := map[string]bool{}
	for _, a := range result {
		names[a.ID] = true
	}
	if !names["claude"] || !names["codex"] {
		t.Fatalf("expected claude and codex, got %v", names)
	}
}

func TestRoleResolver_GetAgent(t *testing.T) {
	agents := []AgentProfile{
		{ID: "claude", LaunchCommand: "npx"},
	}
	resolver := NewRoleResolver(agents, nil)

	a, err := resolver.GetAgent("claude")
	if err != nil {
		t.Fatal(err)
	}
	if a.ID != "claude" {
		t.Fatalf("expected claude, got %s", a.ID)
	}

	_, err = resolver.GetAgent("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent agent")
	}
}

func TestResolveRoleMissingAgentReturnsDiagnostic(t *testing.T) {
	resolver := NewRoleResolver(
		nil,
		[]RoleProfile{
			{
				ID:      "worker",
				AgentID: "codex",
			},
		},
	)

	_, _, err := resolver.Resolve("worker")
	if err == nil {
		t.Fatal("expected missing agent error, got nil")
	}
	if !strings.Contains(err.Error(), "agent") || !strings.Contains(err.Error(), "codex") {
		t.Fatalf("expected missing agent diagnostic, got %q", err.Error())
	}
}
