package web

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/yoke233/ai-workflow/internal/core"
)

type gateHandlers struct {
	store core.Store
}

// gateStatusResponse aggregates checks by gate name.
type gateStatusResponse struct {
	Name     string           `json:"name"`
	Type     string           `json:"type"`
	Status   string           `json:"status"`
	Attempts int              `json:"attempts"`
	Checks   []core.GateCheck `json:"checks"`
}

type gatesListResponse struct {
	Gates []gateStatusResponse `json:"gates"`
}

func (h *gateHandlers) listGates(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "store is not configured", "STORE_UNAVAILABLE")
		return
	}

	issueID := strings.TrimSpace(chi.URLParam(r, "id"))
	if issueID == "" {
		writeAPIError(w, http.StatusBadRequest, "issue id is required", "ISSUE_ID_REQUIRED")
		return
	}

	checks, err := h.store.GetGateChecks(issueID)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "failed to get gate checks", "GET_GATE_CHECKS_FAILED")
		return
	}

	// Aggregate checks by gate_name, preserving order of first appearance.
	gateOrder := make([]string, 0)
	gateMap := make(map[string]*gateStatusResponse)

	for _, check := range checks {
		gs, ok := gateMap[check.GateName]
		if !ok {
			gs = &gateStatusResponse{
				Name:   check.GateName,
				Type:   string(check.GateType),
				Status: string(core.GateStatusPending),
				Checks: make([]core.GateCheck, 0),
			}
			gateMap[check.GateName] = gs
			gateOrder = append(gateOrder, check.GateName)
		}
		gs.Checks = append(gs.Checks, check)
		gs.Attempts = len(gs.Checks)
		// The latest check determines overall gate status.
		gs.Status = string(check.Status)
	}

	gates := make([]gateStatusResponse, 0, len(gateOrder))
	for _, name := range gateOrder {
		gates = append(gates, *gateMap[name])
	}

	writeJSON(w, http.StatusOK, gatesListResponse{Gates: gates})
}

type resolveGateRequest struct {
	Action string `json:"action"` // "pass" or "fail"
	Reason string `json:"reason"`
}

func (h *gateHandlers) resolveGate(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "store is not configured", "STORE_UNAVAILABLE")
		return
	}

	issueID := strings.TrimSpace(chi.URLParam(r, "id"))
	gateName := strings.TrimSpace(chi.URLParam(r, "gateName"))
	if issueID == "" {
		writeAPIError(w, http.StatusBadRequest, "issue id is required", "ISSUE_ID_REQUIRED")
		return
	}
	if gateName == "" {
		writeAPIError(w, http.StatusBadRequest, "gate name is required", "GATE_NAME_REQUIRED")
		return
	}

	var body resolveGateRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid request body", "INVALID_BODY")
		return
	}

	action := strings.TrimSpace(strings.ToLower(body.Action))
	if action != "pass" && action != "fail" {
		writeAPIError(w, http.StatusBadRequest, "action must be 'pass' or 'fail'", "INVALID_ACTION")
		return
	}

	// Determine the attempt number from existing checks.
	existingChecks, err := h.store.GetGateChecks(issueID)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "failed to query gate checks", "GET_GATE_CHECKS_FAILED")
		return
	}
	attempt := 1
	for _, c := range existingChecks {
		if c.GateName == gateName && c.Attempt >= attempt {
			attempt = c.Attempt + 1
		}
	}

	status := core.GateStatusPassed
	stepAction := core.StepGatePassed
	if action == "fail" {
		status = core.GateStatusFailed
		stepAction = core.StepGateFailed
	}

	gc := &core.GateCheck{
		ID:        core.NewGateCheckID(),
		IssueID:   issueID,
		GateName:  gateName,
		GateType:  core.GateTypeOwnerReview,
		Attempt:   attempt,
		Status:    status,
		Reason:    strings.TrimSpace(body.Reason),
		CheckedBy: "human",
		CreatedAt: time.Now().UTC(),
	}

	if err := h.store.SaveGateCheck(gc); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "failed to save gate check", "SAVE_GATE_CHECK_FAILED")
		return
	}

	// Record a TaskStep for traceability.
	step := &core.TaskStep{
		ID:        core.NewTaskStepID(),
		IssueID:   issueID,
		Action:    stepAction,
		Note:      "[gate:" + gateName + "] " + strings.TrimSpace(body.Reason),
		RefID:     gc.ID,
		RefType:   "gate_check",
		CreatedAt: time.Now().UTC(),
	}
	_, _ = h.store.SaveTaskStep(step)

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "gate_check_id": gc.ID})
}
