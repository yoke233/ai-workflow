package web

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/yoke233/ai-workflow/internal/core"
	"github.com/yoke233/ai-workflow/internal/teamleader"
)

type decomposeHandlers struct {
	planner DecomposePlanner
	creator ProposalIssueCreator
}

type decomposeRequest struct {
	Prompt string `json:"prompt"`
}

type confirmDecomposeRequest struct {
	ProposalID string              `json:"proposal_id"`
	Issues     []core.ProposalItem `json:"issues"`
	IssueIDs   map[string]string   `json:"issue_ids"`
}

type createdIssueRef struct {
	TempID  string `json:"temp_id"`
	IssueID string `json:"issue_id"`
}

type confirmDecomposeResponse struct {
	CreatedIssues []createdIssueRef `json:"created_issues"`
}

func registerDecomposeRoutes(r chi.Router, planner DecomposePlanner, creator ProposalIssueCreator) {
	h := &decomposeHandlers{planner: planner, creator: creator}
	r.With(RequireScope(ScopeIssuesWrite)).Post("/projects/{projectId}/decompose", h.decompose)
	r.With(RequireScope(ScopeIssuesWrite)).Post("/projects/{projectId}/decompose/confirm", h.confirm)
}

func (h *decomposeHandlers) decompose(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.planner == nil {
		writeAPIError(w, http.StatusNotImplemented, "decompose planner is not configured", "DECOMPOSE_NOT_CONFIGURED")
		return
	}
	projectID := strings.TrimSpace(chi.URLParam(r, "projectId"))
	if projectID == "" {
		writeAPIError(w, http.StatusBadRequest, "project_id is required", "PROJECT_ID_REQUIRED")
		return
	}
	var req decomposeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid json body", "INVALID_JSON")
		return
	}
	if strings.TrimSpace(req.Prompt) == "" {
		writeAPIError(w, http.StatusBadRequest, "prompt is required", "PROMPT_REQUIRED")
		return
	}
	proposal, err := h.planner.Plan(r.Context(), projectID, strings.TrimSpace(req.Prompt))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error(), "DECOMPOSE_FAILED")
		return
	}
	writeJSON(w, http.StatusOK, proposal)
}

func (h *decomposeHandlers) confirm(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.creator == nil {
		writeAPIError(w, http.StatusNotImplemented, "proposal issue creator is not configured", "CONFIRM_NOT_CONFIGURED")
		return
	}
	projectID := strings.TrimSpace(chi.URLParam(r, "projectId"))
	if projectID == "" {
		writeAPIError(w, http.StatusBadRequest, "project_id is required", "PROJECT_ID_REQUIRED")
		return
	}
	var req confirmDecomposeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid json body", "INVALID_JSON")
		return
	}
	if strings.TrimSpace(req.ProposalID) == "" {
		writeAPIError(w, http.StatusBadRequest, "proposal_id is required", "PROPOSAL_ID_REQUIRED")
		return
	}
	if len(req.Issues) == 0 {
		writeAPIError(w, http.StatusBadRequest, "issues are required", "ISSUES_REQUIRED")
		return
	}

	specs := make([]teamleader.CreateIssueSpec, 0, len(req.Issues))
	createdRefs := make([]createdIssueRef, 0, len(req.Issues))
	for _, item := range req.Issues {
		tempID := strings.TrimSpace(item.TempID)
		if tempID == "" {
			writeAPIError(w, http.StatusBadRequest, "temp_id is required", "TEMP_ID_REQUIRED")
			return
		}
		issueID := strings.TrimSpace(req.IssueIDs[tempID])
		if issueID == "" {
			issueID = core.NewIssueID()
		}
		resolvedDeps := make([]string, 0, len(item.DependsOn))
		for _, dep := range item.DependsOn {
			depID := strings.TrimSpace(dep)
			if depID == "" {
				continue
			}
			if realID := strings.TrimSpace(req.IssueIDs[depID]); realID != "" {
				resolvedDeps = append(resolvedDeps, realID)
			}
		}
		template := strings.TrimSpace(item.Template)
		if template == "" {
			template = "standard"
		}
		specs = append(specs, teamleader.CreateIssueSpec{
			ID:        issueID,
			Title:     strings.TrimSpace(item.Title),
			Body:      item.Body,
			Labels:    append([]string(nil), item.Labels...),
			DependsOn: resolvedDeps,
			Template:  template,
			AutoMerge: item.AutoMerge,
		})
		createdRefs = append(createdRefs, createdIssueRef{TempID: tempID, IssueID: issueID})
	}

	created, err := h.creator.CreateIssues(r.Context(), teamleader.CreateIssuesInput{
		ProjectID: projectID,
		Issues:    specs,
	})
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error(), "CONFIRM_FAILED")
		return
	}
	if len(created) == len(createdRefs) {
		for i := range createdRefs {
			if created[i] != nil && strings.TrimSpace(created[i].ID) != "" {
				createdRefs[i].IssueID = strings.TrimSpace(created[i].ID)
			}
		}
	}
	writeJSON(w, http.StatusCreated, confirmDecomposeResponse{CreatedIssues: createdRefs})
}
