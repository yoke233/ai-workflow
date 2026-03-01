package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/user/ai-workflow/internal/core"
	"github.com/user/ai-workflow/internal/secretary"
)

func TestCreateChatSessionThenGetChatSession(t *testing.T) {
	store := newTestStore(t)
	project := core.Project{
		ID:       "proj-chat-api",
		Name:     "chat-api",
		RepoPath: filepath.Join(t.TempDir(), "repo-chat-api"),
	}
	if err := store.CreateProject(&project); err != nil {
		t.Fatalf("seed project: %v", err)
	}

	srv := NewServer(Config{Store: store})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	rawBody, err := json.Marshal(map[string]any{
		"message": "请帮我拆解一个 OAuth 登录改造计划",
	})
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}

	createResp, err := http.Post(
		ts.URL+"/api/v1/projects/proj-chat-api/chat",
		"application/json",
		bytes.NewReader(rawBody),
	)
	if err != nil {
		t.Fatalf("POST /api/v1/projects/{pid}/chat: %v", err)
	}
	defer createResp.Body.Close()
	if createResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", createResp.StatusCode)
	}

	var created struct {
		SessionID string `json:"session_id"`
		Reply     string `json:"reply"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		t.Fatalf("decode create chat response: %v", err)
	}
	if created.SessionID == "" {
		t.Fatal("expected non-empty session_id")
	}
	if created.Reply == "" {
		t.Fatal("expected non-empty reply")
	}

	getResp, err := http.Get(ts.URL + "/api/v1/projects/proj-chat-api/chat/" + created.SessionID)
	if err != nil {
		t.Fatalf("GET /api/v1/projects/{pid}/chat/{sid}: %v", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", getResp.StatusCode)
	}

	var session core.ChatSession
	if err := json.NewDecoder(getResp.Body).Decode(&session); err != nil {
		t.Fatalf("decode chat session response: %v", err)
	}
	if session.ID != created.SessionID {
		t.Fatalf("expected session id %s, got %s", created.SessionID, session.ID)
	}
	if session.ProjectID != "proj-chat-api" {
		t.Fatalf("expected project id proj-chat-api, got %s", session.ProjectID)
	}
	if len(session.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(session.Messages))
	}
	if session.Messages[0].Role != "user" {
		t.Fatalf("expected first message role=user, got %s", session.Messages[0].Role)
	}
	if session.Messages[1].Role != "assistant" {
		t.Fatalf("expected second message role=assistant, got %s", session.Messages[1].Role)
	}
}

func TestCreateChatSessionRequiresMessage(t *testing.T) {
	store := newTestStore(t)
	project := core.Project{
		ID:       "proj-chat-required",
		Name:     "chat-required",
		RepoPath: filepath.Join(t.TempDir(), "repo-chat-required"),
	}
	if err := store.CreateProject(&project); err != nil {
		t.Fatalf("seed project: %v", err)
	}

	srv := NewServer(Config{Store: store})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	rawBody, err := json.Marshal(map[string]any{
		"message": "   ",
	})
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}

	resp, err := http.Post(
		ts.URL+"/api/v1/projects/proj-chat-required/chat",
		"application/json",
		bytes.NewReader(rawBody),
	)
	if err != nil {
		t.Fatalf("POST /api/v1/projects/{pid}/chat: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	var apiErr apiError
	if err := json.NewDecoder(resp.Body).Decode(&apiErr); err != nil {
		t.Fatalf("decode api error: %v", err)
	}
	if apiErr.Code != "MESSAGE_REQUIRED" {
		t.Fatalf("expected code MESSAGE_REQUIRED, got %s", apiErr.Code)
	}
}

func TestDeleteChatSession(t *testing.T) {
	store := newTestStore(t)
	project := core.Project{
		ID:       "proj-chat-delete",
		Name:     "chat-delete",
		RepoPath: filepath.Join(t.TempDir(), "repo-chat-delete"),
	}
	if err := store.CreateProject(&project); err != nil {
		t.Fatalf("seed project: %v", err)
	}

	session := &core.ChatSession{
		ID:        "chat-20260301-delete01",
		ProjectID: project.ID,
		Messages: []core.ChatMessage{
			{Role: "user", Content: "准备删除会话"},
		},
	}
	if err := store.CreateChatSession(session); err != nil {
		t.Fatalf("seed chat session: %v", err)
	}

	srv := NewServer(Config{Store: store})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	req, err := http.NewRequest(
		http.MethodDelete,
		ts.URL+"/api/v1/projects/proj-chat-delete/chat/"+session.ID,
		nil,
	)
	if err != nil {
		t.Fatalf("create delete request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /api/v1/projects/{pid}/chat/{sid}: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	getResp, err := http.Get(ts.URL + "/api/v1/projects/proj-chat-delete/chat/" + session.ID)
	if err != nil {
		t.Fatalf("GET deleted session: %v", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for deleted session, got %d", getResp.StatusCode)
	}
}

func TestCreateChatSessionTriggersSecretaryDraftWhenPlanManagerConfigured(t *testing.T) {
	store := newTestStore(t)
	project := core.Project{
		ID:       "proj-chat-plan-draft",
		Name:     "chat-plan-draft",
		RepoPath: filepath.Join(t.TempDir(), "repo-chat-plan-draft"),
	}
	if err := store.CreateProject(&project); err != nil {
		t.Fatalf("seed project: %v", err)
	}

	createDraftCalled := false
	planManager := &testPlanManager{
		createDraftFn: func(_ context.Context, input secretary.CreateDraftInput) (*core.TaskPlan, error) {
			createDraftCalled = true
			plan := &core.TaskPlan{
				ID:         core.NewTaskPlanID(),
				ProjectID:  input.ProjectID,
				SessionID:  input.SessionID,
				Name:       "auto-created-from-chat",
				Status:     core.PlanDraft,
				WaitReason: core.WaitNone,
				FailPolicy: core.FailBlock,
				Tasks: []core.TaskItem{
					{
						ID:          "task-auto-chat-1",
						PlanID:      "",
						Title:       "拆解任务",
						Description: "由 chat 自动触发拆解",
						Template:    "standard",
						Status:      core.ItemPending,
					},
				},
			}
			if err := store.CreateTaskPlan(plan); err != nil {
				return nil, err
			}
			return store.GetTaskPlan(plan.ID)
		},
	}

	srv := NewServer(Config{Store: store, PlanManager: planManager})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	rawBody, err := json.Marshal(map[string]any{
		"message": "请拆分一个认证系统改造计划",
	})
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}

	resp, err := http.Post(
		ts.URL+"/api/v1/projects/proj-chat-plan-draft/chat",
		"application/json",
		bytes.NewReader(rawBody),
	)
	if err != nil {
		t.Fatalf("POST /api/v1/projects/{pid}/chat: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var created struct {
		SessionID string `json:"session_id"`
		Reply     string `json:"reply"`
		PlanID    string `json:"plan_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode create chat response: %v", err)
	}
	if created.SessionID == "" {
		t.Fatal("expected non-empty session_id")
	}
	if created.PlanID == "" {
		t.Fatal("expected non-empty plan_id when plan manager is configured")
	}
	if !createDraftCalled {
		t.Fatal("expected chat create to delegate decomposition to plan manager")
	}

	plan, err := store.GetTaskPlan(created.PlanID)
	if err != nil {
		t.Fatalf("load created plan: %v", err)
	}
	if plan.SessionID != created.SessionID {
		t.Fatalf("plan session id = %s, want %s", plan.SessionID, created.SessionID)
	}
}
