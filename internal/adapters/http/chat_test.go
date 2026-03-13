package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	membus "github.com/yoke233/ai-workflow/internal/adapters/events/memory"
	"github.com/yoke233/ai-workflow/internal/adapters/store/sqlite"
	chatapp "github.com/yoke233/ai-workflow/internal/application/chat"
	flowapp "github.com/yoke233/ai-workflow/internal/application/flow"
	"github.com/yoke233/ai-workflow/internal/core"
)

type stubLeadChatService struct {
	listResp     []chatapp.SessionSummary
	detailResp   *chatapp.SessionDetail
	detailErr    error
	startResp    *chatapp.AcceptedResponse
	startErr     error
	lastStartReq chatapp.Request
}

func (s *stubLeadChatService) Chat(context.Context, chatapp.Request) (*chatapp.Response, error) {
	return &chatapp.Response{SessionID: "s-1", Reply: "ok"}, nil
}

func (s *stubLeadChatService) StartChat(_ context.Context, req chatapp.Request) (*chatapp.AcceptedResponse, error) {
	s.lastStartReq = req
	if s.startResp != nil || s.startErr != nil {
		return s.startResp, s.startErr
	}
	return &chatapp.AcceptedResponse{SessionID: "s-1", WSPath: "/api/ws?session_id=s-1&types=chat.output"}, nil
}

func (s *stubLeadChatService) ListSessions(context.Context) ([]chatapp.SessionSummary, error) {
	return s.listResp, nil
}

func (s *stubLeadChatService) GetSession(context.Context, string) (*chatapp.SessionDetail, error) {
	return s.detailResp, s.detailErr
}

func (s *stubLeadChatService) SetConfigOption(context.Context, string, string, string) ([]chatapp.ConfigOption, error) {
	return nil, nil
}
func (s *stubLeadChatService) SetSessionMode(context.Context, string, string) (*chatapp.SessionModeState, error) {
	return nil, nil
}
func (s *stubLeadChatService) ResolvePermission(string, string, bool) error { return nil }
func (s *stubLeadChatService) CancelChat(string) error                      { return nil }
func (s *stubLeadChatService) CloseSession(string)                          {}
func (s *stubLeadChatService) DeleteSession(string)                         {}
func (s *stubLeadChatService) IsSessionAlive(string) bool {
	return false
}
func (s *stubLeadChatService) IsSessionRunning(string) bool {
	return false
}

func TestChatRoutes_ListSessions(t *testing.T) {
	svc := &stubLeadChatService{
		listResp: []chatapp.SessionSummary{
			{
				SessionID:    "acp-session-1",
				Title:        "历史会话",
				Status:       "alive",
				CreatedAt:    time.Now().UTC(),
				UpdatedAt:    time.Now().UTC(),
				MessageCount: 2,
			},
		},
	}

	r := chi.NewRouter()
	registerChatRoutes(r, &Handler{lead: svc})

	req := httptest.NewRequest(http.MethodGet, "/chat/sessions", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var got []chatapp.SessionSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got) != 1 || got[0].SessionID != "acp-session-1" {
		t.Fatalf("unexpected sessions: %+v", got)
	}
}

func TestChatRoutes_GetSession_NotFound(t *testing.T) {
	svc := &stubLeadChatService{detailErr: core.ErrNotFound}
	r := chi.NewRouter()
	registerChatRoutes(r, &Handler{lead: svc})

	req := httptest.NewRequest(http.MethodGet, "/chat/missing", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestChatRoutes_SendMessage_Deprecated(t *testing.T) {
	svc := &stubLeadChatService{}
	r := chi.NewRouter()
	registerChatRoutes(r, &Handler{lead: svc})

	req := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader(`{"message":"hello"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusGone {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusGone)
	}
	if svc.lastStartReq.Message != "" {
		t.Fatalf("unexpected ws start call: %+v", svc.lastStartReq)
	}

	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got["code"] != "CHAT_HTTP_DEPRECATED" {
		t.Fatalf("unexpected code: %+v", got)
	}
}

func TestChatRoutes_CrystallizeThread(t *testing.T) {
	svc := &stubLeadChatService{
		detailResp: &chatapp.SessionDetail{
			SessionSummary: chatapp.SessionSummary{
				SessionID: "chat-1",
				Title:     "认证方案讨论",
			},
			Messages: []chatapp.Message{
				{Role: "user", Content: "我们先定方案"},
				{Role: "assistant", Content: "建议拆成 thread 再落 work item"},
			},
		},
	}
	_, ts := setupAPIWithLead(t, svc)

	resp, err := post(ts, "/chat/sessions/chat-1/crystallize-thread", map[string]any{
		"owner_id":             "human-1",
		"thread_summary":       "确定由 thread 汇总后创建 work item。",
		"participant_user_ids": []string{"human-2", "human-2"},
		"create_work_item":     true,
		"work_item_title":      "实现 thread 结晶接口",
	})
	if err != nil {
		t.Fatalf("crystallize thread: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	var got crystallizeChatSessionResponse
	if err := decodeJSON(resp, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Thread == nil || got.Thread.Metadata["source_chat_session_id"] != "chat-1" {
		t.Fatalf("unexpected thread: %+v", got.Thread)
	}
	if got.WorkItem == nil || got.WorkItem.Title != "实现 thread 结晶接口" {
		t.Fatalf("unexpected work item: %+v", got.WorkItem)
	}
	if len(got.Participants) != 2 {
		t.Fatalf("participants = %d, want 2", len(got.Participants))
	}
}

func TestChatRoutes_CrystallizeThreadRollsBackOnWorkItemFailure(t *testing.T) {
	svc := &stubLeadChatService{
		detailResp: &chatapp.SessionDetail{
			SessionSummary: chatapp.SessionSummary{
				SessionID: "chat-2",
				Title:     "无摘要讨论",
			},
		},
	}
	h, ts := setupAPIWithLead(t, svc)

	resp, err := post(ts, "/chat/sessions/chat-2/crystallize-thread", map[string]any{
		"owner_id":         "human-1",
		"create_work_item": true,
		"work_item_title":  "应该失败",
	})
	if err != nil {
		t.Fatalf("crystallize thread: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	threads, err := h.store.ListThreads(context.Background(), core.ThreadFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list threads: %v", err)
	}
	if len(threads) != 0 {
		t.Fatalf("expected no threads after rollback, got %d", len(threads))
	}
}

func setupAPIWithLead(t *testing.T, lead LeadChatService) (*Handler, *httptest.Server) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "chat-test.db")
	store, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	bus := membus.NewBus()
	executor := func(_ context.Context, step *core.Action, exec *core.Run) error {
		return nil
	}
	eng := flowapp.New(store, bus, executor, flowapp.WithConcurrency(2))
	h := NewHandler(store, bus, eng, WithLeadAgent(lead))
	r := chi.NewRouter()
	h.Register(r)
	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)
	return h, ts
}
