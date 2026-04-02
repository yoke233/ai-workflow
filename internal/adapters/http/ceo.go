package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/yoke233/zhanggui/internal/application/ceoapp"
	requirementapp "github.com/yoke233/zhanggui/internal/application/requirementapp"
	"github.com/yoke233/zhanggui/internal/core"
)

type ceoSubmitResponse struct {
	Mode         ceoapp.SubmitMode              `json:"mode"`
	Summary      string                         `json:"summary"`
	Status       string                         `json:"status"`
	NextStep     string                         `json:"next_step"`
	Analysis     *requirementapp.AnalysisResult `json:"analysis,omitempty"`
	ActionCount  int                            `json:"action_count,omitempty"`
	WorkItemID   int64                          `json:"work_item_id,omitempty"`
	ThreadID     int64                          `json:"thread_id,omitempty"`
	ContextRefs  []*core.ThreadContextRef       `json:"context_refs,omitempty"`
	Agents       []string                       `json:"agents,omitempty"`
	Message      *core.ThreadMessage            `json:"message,omitempty"`
	InviteErrors map[string]string              `json:"invite_errors,omitempty"`
}

func registerCEORoutes(r chi.Router, h *Handler) {
	r.Post("/ceo/submit", h.submitCEORequirement)
}

func (h *Handler) submitCEORequirement(w http.ResponseWriter, r *http.Request) {
	var req ceoapp.SubmitInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "BAD_REQUEST")
		return
	}
	if strings.TrimSpace(req.OwnerID) == "" {
		req.OwnerID = "human"
	}

	result, err := h.ceoService().Submit(r.Context(), req)
	if err != nil {
		if strings.Contains(err.Error(), "description is required") {
			writeError(w, http.StatusBadRequest, err.Error(), "MISSING_DESCRIPTION")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error(), "CEO_SUBMIT_FAILED")
		return
	}

	resp := ceoSubmitResponse{
		Mode:        result.Mode,
		Summary:     result.Summary,
		Status:      result.Status,
		NextStep:    result.NextStep,
		Analysis:    result.Analysis,
		ActionCount: result.ActionCount,
		WorkItemID:  result.WorkItemID,
		ContextRefs: result.ContextRefs,
		Agents:      append([]string(nil), result.AgentIDs...),
	}
	if result.Thread != nil {
		resp.ThreadID = result.Thread.ID
	}

	if result.Mode == ceoapp.ModeDiscussion && result.Thread != nil {
		kickoff, routeErr := h.routeRequirementThreadKickoff(r.Context(), result.Thread.ID, req.OwnerID, requirementapp.CreateThreadInput{
			Description: req.Description,
			Context:     req.Context,
			OwnerID:     req.OwnerID,
			Analysis:    result.Analysis,
		}, result.AgentIDs, "ceo.submit")
		if routeErr != nil {
			writeError(w, http.StatusInternalServerError, routeErr.Error(), "CEO_THREAD_MESSAGE_FAILED")
			return
		}
		resp.Message = kickoff.Message
		resp.Agents = kickoff.Agents
		resp.InviteErrors = kickoff.InviteErrors
	}

	writeJSON(w, http.StatusAccepted, resp)
}
