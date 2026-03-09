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
	proposal := core.DecomposeProposal{
		ID:        strings.TrimSpace(req.ProposalID),
		ProjectID: projectID,
		Items:     append([]core.ProposalItem(nil), req.Issues...),
	}
	if err := proposal.Validate(); err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error(), "INVALID_PROPOSAL")
		return
	}

	specs := make([]teamleader.CreateIssueSpec, 0, len(req.Issues))
	createdRefs := make([]createdIssueRef, 0, len(req.Issues))
	tempToIssueID := make(map[string]string, len(req.Issues))
	tempToBlocks := make(map[string][]string, len(req.Issues))
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
		tempToIssueID[tempID] = issueID
	}
	for _, item := range req.Issues {
		for _, dep := range item.DependsOn {
			depID := strings.TrimSpace(dep)
			if depID == "" {
				continue
			}
			realID := strings.TrimSpace(tempToIssueID[depID])
			if realID == "" {
				writeAPIError(w, http.StatusBadRequest, "unknown depends_on temp_id: "+depID, "UNKNOWN_DEPENDENCY")
				return
			}
			tempID := strings.TrimSpace(item.TempID)
			if tempID == "" {
				writeAPIError(w, http.StatusBadRequest, "temp_id is required", "TEMP_ID_REQUIRED")
				return
			}
			tempToBlocks[depID] = append(tempToBlocks[depID], strings.TrimSpace(tempToIssueID[tempID]))
		}
	}
	for _, item := range req.Issues {
		tempID := strings.TrimSpace(item.TempID)
		issueID := tempToIssueID[tempID]
		resolvedDeps := make([]string, 0, len(item.DependsOn))
		for _, dep := range item.DependsOn {
			depID := strings.TrimSpace(dep)
			if depID == "" {
				continue
			}
			realID := strings.TrimSpace(tempToIssueID[depID])
			if realID == "" {
				writeAPIError(w, http.StatusBadRequest, "unknown depends_on temp_id: "+depID, "UNKNOWN_DEPENDENCY")
				return
			}
			resolvedDeps = append(resolvedDeps, realID)
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
			Blocks:    append([]string(nil), tempToBlocks[tempID]...),
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
	createdIDs := make([]string, 0, len(created))
	for i := range created {
		if created[i] == nil || strings.TrimSpace(created[i].ID) == "" {
			continue
		}
		createdIDs = append(createdIDs, strings.TrimSpace(created[i].ID))
	}
	confirmed, err := h.creator.ConfirmCreatedIssues(r.Context(), createdIDs, "confirmed from decompose proposal")
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error(), "CONFIRM_ACTIVATE_FAILED")
		return
	}
	if len(confirmed) == len(createdRefs) {
		for i := range createdRefs {
			if confirmed[i] != nil && strings.TrimSpace(confirmed[i].ID) != "" {
				createdRefs[i].IssueID = strings.TrimSpace(confirmed[i].ID)
			}
		}
	}
	writeJSON(w, http.StatusCreated, confirmDecomposeResponse{CreatedIssues: createdRefs})
}
