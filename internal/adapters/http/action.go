package api

import (
	"encoding/json"
	"net/http"
	"time"

	flowapp "github.com/yoke233/zhanggui/internal/application/flow"
	"github.com/yoke233/zhanggui/internal/core"
)

// createActionRequest is the request body for POST /work-items/{workItemID}/actions.
type createActionRequest struct {
	Name                 string          `json:"name"`
	Description          string          `json:"description,omitempty"`
	Type                 core.ActionType `json:"type"`
	Position             *int            `json:"position,omitempty"`
	DependsOn            []int64         `json:"depends_on,omitempty"`
	AgentRole            string          `json:"agent_role,omitempty"`
	RequiredCapabilities []string        `json:"required_capabilities,omitempty"`
	AcceptanceCriteria   []string        `json:"acceptance_criteria,omitempty"`
	Timeout              string          `json:"timeout,omitempty"` // Go duration string
	MaxRetries           int             `json:"max_retries"`
	Config               map[string]any  `json:"config,omitempty"`
}

func (h *Handler) createAction(w http.ResponseWriter, r *http.Request) {
	workItemID, ok := urlParamInt64(r, "workItemID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid work item ID", "BAD_ID")
		return
	}

	var req createActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "BAD_REQUEST")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required", "MISSING_NAME")
		return
	}
	if req.Type == "" {
		writeError(w, http.StatusBadRequest, "type is required", "MISSING_TYPE")
		return
	}
	position, err := flowapp.ResolveCreateActionPosition(r.Context(), h.store, workItemID, req.Position)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "INVALID_POSITION")
		return
	}

	var timeout time.Duration
	if req.Timeout != "" {
		timeout, err = time.ParseDuration(req.Timeout)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid timeout duration", "BAD_TIMEOUT")
			return
		}
	}

	s := &core.Action{
		WorkItemID:           workItemID,
		Name:                 req.Name,
		Description:          req.Description,
		Type:                 req.Type,
		Status:               core.ActionPending,
		Position:             position,
		DependsOn:            req.DependsOn,
		AgentRole:            req.AgentRole,
		RequiredCapabilities: req.RequiredCapabilities,
		AcceptanceCriteria:   req.AcceptanceCriteria,
		Timeout:              timeout,
		MaxRetries:           req.MaxRetries,
		Config:               req.Config,
	}
	if err := flowapp.ValidateDAGConsistency(r.Context(), h.store, workItemID, 0, s); err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "INCOMPLETE_DAG")
		return
	}
	id, err := h.store.CreateAction(r.Context(), s)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}
	s.ID = id
	writeJSON(w, http.StatusCreated, s)
}

func (h *Handler) listActions(w http.ResponseWriter, r *http.Request) {
	workItemID, ok := urlParamInt64(r, "workItemID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid work item ID", "BAD_ID")
		return
	}

	actions, err := h.store.ListActionsByWorkItem(r.Context(), workItemID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}
	if actions == nil {
		actions = []*core.Action{}
	}
	writeJSON(w, http.StatusOK, actions)
}

// updateActionRequest is the request body for PUT /actions/{actionID}.
// All fields are optional — only provided fields are applied.
type updateActionRequest struct {
	Name                 *string          `json:"name,omitempty"`
	Description          *string          `json:"description,omitempty"`
	Type                 *core.ActionType `json:"type,omitempty"`
	Position             *int             `json:"position,omitempty"`
	DependsOn            *[]int64         `json:"depends_on,omitempty"`
	AgentRole            *string          `json:"agent_role,omitempty"`
	RequiredCapabilities *[]string        `json:"required_capabilities,omitempty"`
	AcceptanceCriteria   *[]string        `json:"acceptance_criteria,omitempty"`
	Timeout              *string          `json:"timeout,omitempty"`
	MaxRetries           *int             `json:"max_retries,omitempty"`
	Config               map[string]any   `json:"config,omitempty"`
}

func (h *Handler) updateAction(w http.ResponseWriter, r *http.Request) {
	id, ok := urlParamInt64(r, "actionID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid action ID", "BAD_ID")
		return
	}

	existing, err := h.store.GetAction(r.Context(), id)
	if err == core.ErrNotFound {
		writeError(w, http.StatusNotFound, "action not found", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}

	// Only allow editing pending actions.
	if existing.Status != core.ActionPending {
		writeError(w, http.StatusConflict, "only pending actions can be edited", "INVALID_STATE")
		return
	}

	var req updateActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "BAD_REQUEST")
		return
	}

	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.Description != nil {
		existing.Description = *req.Description
	}
	if req.Type != nil {
		existing.Type = *req.Type
	}
	if req.Position != nil {
		if err := flowapp.ValidateActionPosition(r.Context(), h.store, existing.WorkItemID, existing.ID, *req.Position); err != nil {
			writeError(w, http.StatusBadRequest, err.Error(), "INVALID_POSITION")
			return
		}
		existing.Position = *req.Position
	}
	if req.DependsOn != nil {
		existing.DependsOn = *req.DependsOn
	}
	if req.AgentRole != nil {
		existing.AgentRole = *req.AgentRole
	}
	if req.RequiredCapabilities != nil {
		existing.RequiredCapabilities = *req.RequiredCapabilities
	}
	if req.AcceptanceCriteria != nil {
		existing.AcceptanceCriteria = *req.AcceptanceCriteria
	}
	if req.Timeout != nil {
		t, err := time.ParseDuration(*req.Timeout)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid timeout duration", "BAD_TIMEOUT")
			return
		}
		existing.Timeout = t
	}
	if req.MaxRetries != nil {
		existing.MaxRetries = *req.MaxRetries
	}
	if req.Config != nil {
		existing.Config = req.Config
	}
	if err := flowapp.ValidateDAGConsistency(r.Context(), h.store, existing.WorkItemID, existing.ID, existing); err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "INCOMPLETE_DAG")
		return
	}

	if err := h.store.UpdateAction(r.Context(), existing); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}
	writeJSON(w, http.StatusOK, existing)
}

func (h *Handler) deleteAction(w http.ResponseWriter, r *http.Request) {
	id, ok := urlParamInt64(r, "actionID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid action ID", "BAD_ID")
		return
	}

	// Only allow deleting pending actions.
	existing, err := h.store.GetAction(r.Context(), id)
	if err == core.ErrNotFound {
		writeError(w, http.StatusNotFound, "action not found", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}
	if existing.Status != core.ActionPending {
		writeError(w, http.StatusConflict, "only pending actions can be deleted", "INVALID_STATE")
		return
	}

	if err := h.store.DeleteAction(r.Context(), id); err != nil {
		if err == core.ErrNotFound {
			writeError(w, http.StatusNotFound, "action not found", "NOT_FOUND")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) getAction(w http.ResponseWriter, r *http.Request) {
	id, ok := urlParamInt64(r, "actionID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid action ID", "BAD_ID")
		return
	}

	s, err := h.store.GetAction(r.Context(), id)
	if err == core.ErrNotFound {
		writeError(w, http.StatusNotFound, "action not found", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}
	writeJSON(w, http.StatusOK, s)
}

