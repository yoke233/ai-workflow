package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/user/ai-workflow/internal/core"
	"github.com/user/ai-workflow/internal/secretary"
)

type chatHandlers struct {
	store       core.Store
	planManager PlanManager
}

type chatSessionDeleter interface {
	DeleteChatSession(id string) error
}

type createChatSessionRequest struct {
	Message string `json:"message"`
}

type createChatSessionResponse struct {
	SessionID string `json:"session_id"`
	Reply     string `json:"reply"`
	PlanID    string `json:"plan_id,omitempty"`
}

func registerChatRoutes(r chi.Router, store core.Store, planManager PlanManager) {
	h := &chatHandlers{
		store:       store,
		planManager: planManager,
	}
	r.Post("/projects/{projectID}/chat", h.createSession)
	r.Get("/projects/{projectID}/chat/{sessionID}", h.getSession)
	r.Delete("/projects/{projectID}/chat/{sessionID}", h.deleteSession)
}

func (h *chatHandlers) createSession(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "store is not configured", "STORE_UNAVAILABLE")
		return
	}

	projectID := strings.TrimSpace(chi.URLParam(r, "projectID"))
	if projectID == "" {
		writeAPIError(w, http.StatusBadRequest, "project id is required", "PROJECT_ID_REQUIRED")
		return
	}
	project, err := h.store.GetProject(projectID)
	if err != nil {
		if isNotFoundError(err) {
			writeAPIError(w, http.StatusNotFound, fmt.Sprintf("project %s not found", projectID), "PROJECT_NOT_FOUND")
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "failed to load project", "GET_PROJECT_FAILED")
		return
	}

	var req createChatSessionRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid json body", "INVALID_JSON")
		return
	}

	req.Message = strings.TrimSpace(req.Message)
	if req.Message == "" {
		writeAPIError(w, http.StatusBadRequest, "message is required", "MESSAGE_REQUIRED")
		return
	}

	now := time.Now().UTC()
	reply := "已收到你的需求，我会先整理成任务计划草案。"
	session := &core.ChatSession{
		ID:        core.NewChatSessionID(),
		ProjectID: projectID,
		Messages: []core.ChatMessage{
			{
				Role:    "user",
				Content: req.Message,
				Time:    now,
			},
			{
				Role:    "assistant",
				Content: reply,
				Time:    now,
			},
		},
	}
	if err := h.store.CreateChatSession(session); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "failed to create chat session", "CREATE_CHAT_SESSION_FAILED")
		return
	}

	createdPlanID := ""
	if h.planManager != nil {
		createReq := secretary.Request{
			Conversation: summarizeChatMessages(session.Messages),
			ProjectName:  strings.TrimSpace(project.Name),
			RepoPath:     strings.TrimSpace(project.RepoPath),
			WorkDir:      strings.TrimSpace(project.RepoPath),
		}
		if createReq.WorkDir == "" {
			createReq.WorkDir = "."
		}
		createdPlan, planErr := h.planManager.CreateDraft(r.Context(), secretary.CreateDraftInput{
			ProjectID:  projectID,
			SessionID:  session.ID,
			FailPolicy: core.FailBlock,
			Request:    createReq,
		})
		if planErr == nil && createdPlan != nil {
			createdPlanID = strings.TrimSpace(createdPlan.ID)
		}
	}

	writeJSON(w, http.StatusOK, createChatSessionResponse{
		SessionID: session.ID,
		Reply:     reply,
		PlanID:    createdPlanID,
	})
}

func (h *chatHandlers) getSession(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "store is not configured", "STORE_UNAVAILABLE")
		return
	}

	projectID := strings.TrimSpace(chi.URLParam(r, "projectID"))
	sessionID := strings.TrimSpace(chi.URLParam(r, "sessionID"))
	if projectID == "" || sessionID == "" {
		writeAPIError(w, http.StatusBadRequest, "project id and session id are required", "INVALID_PATH_PARAM")
		return
	}

	session, err := h.store.GetChatSession(sessionID)
	if err != nil {
		if isNotFoundError(err) {
			writeAPIError(w, http.StatusNotFound, fmt.Sprintf("chat session %s not found", sessionID), "CHAT_SESSION_NOT_FOUND")
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "failed to load chat session", "GET_CHAT_SESSION_FAILED")
		return
	}
	if session.ProjectID != projectID {
		writeAPIError(w, http.StatusNotFound, fmt.Sprintf("chat session %s not found in project %s", sessionID, projectID), "CHAT_SESSION_NOT_FOUND")
		return
	}

	writeJSON(w, http.StatusOK, session)
}

func (h *chatHandlers) deleteSession(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "store is not configured", "STORE_UNAVAILABLE")
		return
	}

	projectID := strings.TrimSpace(chi.URLParam(r, "projectID"))
	sessionID := strings.TrimSpace(chi.URLParam(r, "sessionID"))
	if projectID == "" || sessionID == "" {
		writeAPIError(w, http.StatusBadRequest, "project id and session id are required", "INVALID_PATH_PARAM")
		return
	}

	session, err := h.store.GetChatSession(sessionID)
	if err != nil {
		if isNotFoundError(err) {
			writeAPIError(w, http.StatusNotFound, fmt.Sprintf("chat session %s not found", sessionID), "CHAT_SESSION_NOT_FOUND")
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "failed to load chat session", "GET_CHAT_SESSION_FAILED")
		return
	}
	if session.ProjectID != projectID {
		writeAPIError(w, http.StatusNotFound, fmt.Sprintf("chat session %s not found in project %s", sessionID, projectID), "CHAT_SESSION_NOT_FOUND")
		return
	}

	deleter, ok := h.store.(chatSessionDeleter)
	if !ok {
		writeAPIError(w, http.StatusNotImplemented, "chat session delete is not supported by current store", "DELETE_CHAT_SESSION_UNSUPPORTED")
		return
	}
	if err := deleter.DeleteChatSession(sessionID); err != nil {
		if isNotFoundError(err) {
			writeAPIError(w, http.StatusNotFound, fmt.Sprintf("chat session %s not found", sessionID), "CHAT_SESSION_NOT_FOUND")
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "failed to delete chat session", "DELETE_CHAT_SESSION_FAILED")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
