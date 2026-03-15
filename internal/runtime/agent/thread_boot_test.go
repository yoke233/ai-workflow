package agentruntime

import (
	"strings"
	"testing"
	"time"

	"github.com/yoke233/ai-workflow/internal/core"
)

// helpers to reduce boilerplate in test setup.

func newThread(title string, status core.ThreadStatus) *core.Thread {
	return &core.Thread{
		ID:        1,
		Title:     title,
		Status:    status,
		OwnerID:   "user-owner",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func newProfile(id string, role core.AgentRole) *core.AgentProfile {
	return &core.AgentProfile{
		ID:   id,
		Name: id,
		Role: role,
	}
}

func newMessage(senderID, role, content string) *core.ThreadMessage {
	return &core.ThreadMessage{
		ID:        1,
		ThreadID:  1,
		SenderID:  senderID,
		Role:      role,
		Content:   content,
		CreatedAt: time.Now(),
	}
}

func newParticipant(userID, role string) *core.ThreadMember {
	return &core.ThreadMember{
		ID:       1,
		ThreadID: 1,
		UserID:   userID,
		Role:     role,
		JoinedAt: time.Now(),
	}
}

func newWorkItem(id int64, title string, status core.WorkItemStatus) *core.WorkItem {
	return &core.WorkItem{
		ID:       id,
		Title:    title,
		Status:   status,
		Priority: core.PriorityMedium,
	}
}

func TestBuildBootPrompt_FullContext(t *testing.T) {
	profile := newProfile("agent-worker-1", core.RoleWorker)
	thread := newThread("Design review for API v2", core.ThreadActive)
	core.SetThreadFocusProjectID(thread, 42)
	out := BuildBootPrompt(ThreadBootInput{
		Thread: thread,
		RecentMessages: []*core.ThreadMessage{
			newMessage("alice", "human", "Please review the endpoint changes."),
			newMessage("agent-worker-1", "agent", "I will look into the PR now."),
		},
		Participants: []*core.ThreadMember{
			newParticipant("alice", "owner"),
			newParticipant("agent-worker-1", "agent"),
		},
		WorkItems: []*core.WorkItem{
			newWorkItem(42, "Implement GET /users endpoint", core.WorkItemOpen),
		},
		AgentProfile: profile,
		PriorSummary: "Previous session reviewed auth middleware.",
	})

	checks := map[string]string{
		"thread title":      "Design review for API v2",
		"thread status":     "active",
		"focus project":     "Focus Project ID",
		"profile role":      "worker",
		"profile ID":        "agent-worker-1",
		"message content 1": "Please review the endpoint changes.",
		"message content 2": "I will look into the PR now.",
		"participant alice": "alice",
		"participant agent": "agent-worker-1",
		"you marker":        "← you",
		"work item title":   "Implement GET /users endpoint",
		"work item id":      "#42",
		"work item status":  "open",
		"prior summary":     "Previous session reviewed auth middleware.",
		"prior heading":     "Prior Context",
		"instructions":      "You are joining this thread",
	}

	for label, want := range checks {
		if !strings.Contains(out, want) {
			t.Errorf("%s: output should contain %q but did not.\nOutput:\n%s", label, want, out)
		}
	}
}

func TestBuildBootPrompt_NoPriorSummary(t *testing.T) {
	out := BuildBootPrompt(ThreadBootInput{
		Thread:       newThread("Bug triage", core.ThreadActive),
		AgentProfile: newProfile("agent-1", core.RoleWorker),
		PriorSummary: "",
	})

	if strings.Contains(out, "Prior Context") {
		t.Errorf("output should NOT contain 'Prior Context' when PriorSummary is empty.\nOutput:\n%s", out)
	}
}

func TestBuildBootPrompt_WithPriorSummary(t *testing.T) {
	summary := "Completed initial code review of the auth module."
	out := BuildBootPrompt(ThreadBootInput{
		Thread:       newThread("Auth module review", core.ThreadActive),
		AgentProfile: newProfile("agent-1", core.RoleWorker),
		PriorSummary: summary,
	})

	if !strings.Contains(out, "Prior Context") {
		t.Errorf("output should contain 'Prior Context' heading when PriorSummary is set.\nOutput:\n%s", out)
	}
	if !strings.Contains(out, summary) {
		t.Errorf("output should contain the prior summary text %q.\nOutput:\n%s", summary, out)
	}
}

func TestBuildBootPrompt_NoWorkItems(t *testing.T) {
	out := BuildBootPrompt(ThreadBootInput{
		Thread:       newThread("Quick chat", core.ThreadActive),
		AgentProfile: newProfile("agent-1", core.RoleWorker),
		WorkItems:    nil,
	})

	if strings.Contains(out, "Linked Work Items") {
		t.Errorf("output should NOT contain 'Linked Work Items' when WorkItems is empty.\nOutput:\n%s", out)
	}

	// Also verify with an explicitly empty slice.
	out2 := BuildBootPrompt(ThreadBootInput{
		Thread:       newThread("Quick chat", core.ThreadActive),
		AgentProfile: newProfile("agent-1", core.RoleWorker),
		WorkItems:    []*core.WorkItem{},
	})

	if strings.Contains(out2, "Linked Work Items") {
		t.Errorf("output should NOT contain 'Linked Work Items' when WorkItems is an empty slice.\nOutput:\n%s", out2)
	}
}

func TestBuildBootPrompt_CustomBootTemplate(t *testing.T) {
	customTemplate := "You are Acme Corp's senior code reviewer. Follow our style guide strictly."
	profile := newProfile("reviewer-1", core.RoleGate)
	profile.Session.ThreadBootTemplate = customTemplate

	out := BuildBootPrompt(ThreadBootInput{
		Thread:       newThread("Style review", core.ThreadActive),
		AgentProfile: profile,
	})

	// Custom template should appear at the very top.
	idx := strings.Index(out, customTemplate)
	if idx < 0 {
		t.Fatalf("output should contain custom boot template %q.\nOutput:\n%s", customTemplate, out)
	}
	if idx != 0 {
		t.Errorf("custom boot template should appear at position 0 but found at %d.\nOutput:\n%s", idx, out)
	}

	// Standard sections should still be present after the template.
	if !strings.Contains(out, "## Thread Context") {
		t.Errorf("output should still contain '## Thread Context' after custom template.\nOutput:\n%s", out)
	}
	if !strings.Contains(out, "Style review") {
		t.Errorf("output should contain thread title 'Style review'.\nOutput:\n%s", out)
	}
}

func TestBuildBootPrompt_WorkspaceContext(t *testing.T) {
	out := BuildBootPrompt(ThreadBootInput{
		Thread:       newThread("Workspace thread", core.ThreadActive),
		AgentProfile: newProfile("agent-1", core.RoleWorker),
		Workspace: &core.ThreadWorkspaceContext{
			ThreadID:  1,
			Workspace: ".",
			Archive:   "../archive",
			Archives: []core.ThreadWorkspaceArchiveSnapshot{
				{
					Date:      "2026-03-15",
					Path:      "../archive/2026-03-15",
					Manifest:  "../archive/2026-03-15/.manifest.json",
					FileCount: 3,
				},
			},
			Mounts: map[string]core.ThreadWorkspaceMount{
				"project-alpha": {
					Path:      "../mounts/project-alpha",
					ProjectID: 42,
					Access:    core.ContextAccessCheck,
				},
			},
		},
	})

	for _, want := range []string{"Workspace Context", "Mount project-alpha", "../mounts/project-alpha", "[check]", "Archive Snapshot 2026-03-15", ".manifest.json"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output should contain %q.\nOutput:\n%s", want, out)
		}
	}
}
