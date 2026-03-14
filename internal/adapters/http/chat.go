package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	threadapp "github.com/yoke233/ai-workflow/internal/application/threadapp"
	"github.com/yoke233/ai-workflow/internal/core"
)

// chatHandlers holds the lead agent chat endpoint handlers.
type chatHandlers struct {
	handler *Handler
	lead    LeadChatService
}

func registerChatRoutes(r chi.Router, h *Handler) {
	if h == nil || h.lead == nil {
		return
	}
	handlers := &chatHandlers{handler: h, lead: h.lead}
	r.Get("/chat/sessions", handlers.listSessions)
	r.Post("/chat/sessions/{sessionID}/crystallize-thread", handlers.crystallizeThread)
	r.Post("/chat", handlers.sendMessage)
	r.Get("/chat/{sessionID}", handlers.getSession)
	r.Post("/chat/{sessionID}/cancel", handlers.cancelChat)
	r.Post("/chat/{sessionID}/close", handlers.closeSession)
	r.Delete("/chat/{sessionID}", handlers.deleteSession)
	r.Get("/chat/{sessionID}/status", handlers.getStatus)
}

// GET /chat/sessions — list persisted lead chat sessions.
func (h *chatHandlers) listSessions(w http.ResponseWriter, r *http.Request) {
	resp, err := h.lead.ListSessions(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "LIST_CHAT_SESSIONS_FAILED")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// POST /chat — deprecated, use WebSocket chat.send instead.
func (h *chatHandlers) sendMessage(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusGone, "POST /api/chat is deprecated; use websocket message type chat.send", "CHAT_HTTP_DEPRECATED")
}

// GET /chat/{sessionID} — load one persisted session including message history.
func (h *chatHandlers) getSession(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(chi.URLParam(r, "sessionID"))
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session_id is required", "BAD_REQUEST")
		return
	}

	resp, err := h.lead.GetSession(r.Context(), sessionID)
	if err != nil {
		if errors.Is(err, core.ErrNotFound) {
			writeError(w, http.StatusNotFound, err.Error(), "NOT_FOUND")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error(), "GET_CHAT_SESSION_FAILED")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// POST /chat/{sessionID}/cancel — cancel the current prompt.
func (h *chatHandlers) cancelChat(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(chi.URLParam(r, "sessionID"))
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session_id is required", "BAD_REQUEST")
		return
	}
	if err := h.lead.CancelChat(sessionID); err != nil {
		writeError(w, http.StatusConflict, err.Error(), "CANCEL_FAILED")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"session_id": sessionID,
		"status":     "cancelled",
	})
}

// POST /chat/{sessionID}/close — close the session (recycle agent, keep workspace).
func (h *chatHandlers) closeSession(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(chi.URLParam(r, "sessionID"))
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session_id is required", "BAD_REQUEST")
		return
	}
	h.lead.CloseSession(sessionID)
	writeJSON(w, http.StatusOK, map[string]string{
		"session_id": sessionID,
		"status":     "closed",
	})
}

// DELETE /chat/{sessionID} — permanently delete session and clean up workspace.
func (h *chatHandlers) deleteSession(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(chi.URLParam(r, "sessionID"))
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session_id is required", "BAD_REQUEST")
		return
	}
	h.lead.DeleteSession(sessionID)
	writeJSON(w, http.StatusOK, map[string]string{
		"session_id": sessionID,
		"status":     "deleted",
	})
}

// GET /chat/{sessionID}/status — check session status.
func (h *chatHandlers) getStatus(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(chi.URLParam(r, "sessionID"))
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session_id is required", "BAD_REQUEST")
		return
	}
	status := "not_found"
	if h.lead.IsSessionAlive(sessionID) {
		status = "alive"
		if h.lead.IsSessionRunning(sessionID) {
			status = "running"
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"session_id": sessionID,
		"status":     status,
	})
}

type crystallizeChatSessionRequest struct {
	ThreadTitle        string   `json:"thread_title"`
	ThreadSummary      string   `json:"thread_summary"`
	WorkItemTitle      string   `json:"work_item_title,omitempty"`
	WorkItemBody       string   `json:"work_item_body,omitempty"`
	ProjectID          *int64   `json:"project_id,omitempty"`
	ParticipantUserIDs []string `json:"participant_user_ids,omitempty"`
	CreateWorkItem     bool     `json:"create_work_item,omitempty"`
	OwnerID            string   `json:"owner_id,omitempty"`
}

type crystallizeChatSessionResponse struct {
	Thread       *core.Thread         `json:"thread"`
	WorkItem     *core.WorkItem       `json:"work_item,omitempty"`
	Participants []*core.ThreadMember `json:"participants"`
}

func (h *chatHandlers) crystallizeThread(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(chi.URLParam(r, "sessionID"))
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session_id is required", "BAD_REQUEST")
		return
	}

	detail, err := h.lead.GetSession(r.Context(), sessionID)
	if err != nil {
		if errors.Is(err, core.ErrNotFound) {
			writeError(w, http.StatusNotFound, err.Error(), "NOT_FOUND")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error(), "GET_CHAT_SESSION_FAILED")
		return
	}

	var req crystallizeChatSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "BAD_REQUEST")
		return
	}

	threadTitle := strings.TrimSpace(req.ThreadTitle)
	if threadTitle == "" {
		threadTitle = strings.TrimSpace(detail.Title)
	}
	if threadTitle == "" {
		threadTitle = fmt.Sprintf("Chat Session %s", sessionID)
	}

	threadSummary := strings.TrimSpace(req.ThreadSummary)
	ownerID := strings.TrimSpace(req.OwnerID)
	result, err := h.handler.threadService().CrystallizeChatSession(r.Context(), threadapp.CrystallizeChatSessionInput{
		SessionID:          sessionID,
		ThreadTitle:        threadTitle,
		ThreadSummary:      threadSummary,
		OwnerID:            ownerID,
		ParticipantUserIDs: req.ParticipantUserIDs,
		CreateWorkItem:     req.CreateWorkItem,
		WorkItemTitle:      req.WorkItemTitle,
		WorkItemBody:       req.WorkItemBody,
		ProjectID:          req.ProjectID,
	})
	if err != nil {
		if writeThreadAppError(w, err) {
			return
		}
		code := "CREATE_THREAD_FAILED"
		if req.CreateWorkItem {
			code = "CREATE_ISSUE_FAILED"
			if strings.Contains(err.Error(), "rollback failed") {
				code = "CREATE_LINK_FAILED"
			}
		}
		writeError(w, http.StatusInternalServerError, err.Error(), code)
		return
	}

	writeJSON(w, http.StatusCreated, crystallizeChatSessionResponse{
		Thread:       result.Thread,
		WorkItem:     result.WorkItem,
		Participants: result.Participants,
	})
}
