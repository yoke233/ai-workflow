package teamleader

import (
	"context"
	"testing"
)

func TestParseDecomposeResponse(t *testing.T) {
	raw := `{
		"summary": "用户注册系统",
		"issues": [
			{"temp_id": "A", "title": "设计DB schema", "body": "设计用户表", "depends_on": [], "labels": ["backend"]},
			{"temp_id": "B", "title": "注册API", "body": "POST /register", "depends_on": ["A"], "labels": ["backend"]}
		]
	}`
	proposal, err := parseDecomposeResponse("proj-1", "做用户注册", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(proposal.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(proposal.Items))
	}
	if proposal.Items[1].DependsOn[0] != "A" {
		t.Fatalf("expected B depends on A")
	}
	if proposal.Summary != "用户注册系统" {
		t.Fatalf("summary = %q", proposal.Summary)
	}
}

func TestParseDecomposeResponse_ExtractJSON(t *testing.T) {
	raw := "这是我的分析：\n```json\n{\"summary\":\"test\",\"issues\":[{\"temp_id\":\"A\",\"title\":\"t\",\"body\":\"b\",\"depends_on\":[]}]}\n```\n以上是方案。"
	proposal, err := parseDecomposeResponse("proj-1", "prompt", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(proposal.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(proposal.Items))
	}
}

func TestParseDecomposeResponse_Invalid(t *testing.T) {
	_, err := parseDecomposeResponse("proj-1", "prompt", "not json at all")
	if err == nil {
		t.Fatal("expected error for invalid response")
	}
}

func TestDecomposePlannerPlan(t *testing.T) {
	planner := NewDecomposePlanner(func(_ context.Context, systemPrompt, userMessage string) (string, error) {
		if systemPrompt == "" {
			t.Fatal("system prompt should not be empty")
		}
		if userMessage != "做用户注册" {
			t.Fatalf("userMessage = %q", userMessage)
		}
		return `{"summary":"注册系统","issues":[{"temp_id":"A","title":"设计 schema","body":"设计用户表","depends_on":[]}]}`, nil
	})
	proposal, err := planner.Plan(context.Background(), "proj-1", "做用户注册")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proposal.ProjectID != "proj-1" {
		t.Fatalf("project_id = %q", proposal.ProjectID)
	}
	if proposal.Prompt != "做用户注册" {
		t.Fatalf("prompt = %q", proposal.Prompt)
	}
	if proposal.ID == "" {
		t.Fatal("expected generated proposal id")
	}
}
