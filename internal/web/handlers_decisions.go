package web

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/yoke233/ai-workflow/internal/core"
)

type decisionHandlers struct {
	store core.Store
}

func (h *decisionHandlers) listIssueDecisions(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "store is not configured", "STORE_UNAVAILABLE")
		return
	}

	issueID := strings.TrimSpace(chi.URLParam(r, "id"))
	if issueID == "" {
		writeAPIError(w, http.StatusBadRequest, "issue id is required", "ISSUE_ID_REQUIRED")
		return
	}

	decisions, err := h.store.ListDecisions(issueID)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "failed to list decisions", "LIST_DECISIONS_FAILED")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": decisions,
		"total": len(decisions),
	})
}

func (h *decisionHandlers) getDecision(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "store is not configured", "STORE_UNAVAILABLE")
		return
	}

	decisionID := strings.TrimSpace(chi.URLParam(r, "id"))
	if decisionID == "" {
		writeAPIError(w, http.StatusBadRequest, "decision id is required", "DECISION_ID_REQUIRED")
		return
	}

	decision, err := h.store.GetDecision(decisionID)
	if err != nil {
		if isNotFoundError(err) {
			writeAPIError(w, http.StatusNotFound, "decision not found", "DECISION_NOT_FOUND")
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "failed to get decision", "GET_DECISION_FAILED")
		return
	}

	writeJSON(w, http.StatusOK, decision)
}
