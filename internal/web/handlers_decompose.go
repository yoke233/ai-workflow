package web

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/yoke233/ai-workflow/internal/core"
	"github.com/yoke233/ai-workflow/internal/teamleader"
)

type decomposeHandlers struct {
	store   core.Store
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

func registerDecomposeRoutes(r chi.Router, store core.Store, planner DecomposePlanner, creator ProposalIssueCreator) {
	h := &decomposeHandlers{store: store, planner: planner, creator: creator}
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
	if !h.ensureProjectExists(w, projectID) {
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
		status, code := classifyDecomposePlanError(err)
		writeAPIError(w, status, err.Error(), code)
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
	if !h.ensureProjectExists(w, projectID) {
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
	resolvedItems, err := applyProposalChildrenModes(proposal)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error(), "INVALID_PROPOSAL")
		return
	}
	sessionID := decomposeSessionID(proposal.ID)

	specs := make([]teamleader.CreateIssueSpec, 0, len(resolvedItems))
	missingSpecs := make([]teamleader.CreateIssueSpec, 0, len(resolvedItems))
	createdRefs := make([]createdIssueRef, 0, len(resolvedItems))
	tempToIssueID := make(map[string]string, len(resolvedItems))
	tempToBlocks := make(map[string][]string, len(resolvedItems))
	existingIssues := make(map[string]struct{}, len(resolvedItems))
	for _, item := range resolvedItems {
		tempID := strings.TrimSpace(item.TempID)
		if tempID == "" {
			writeAPIError(w, http.StatusBadRequest, "temp_id is required", "TEMP_ID_REQUIRED")
			return
		}
		issueID := strings.TrimSpace(req.IssueIDs[tempID])
		if issueID == "" {
			issueID = decomposeIssueID(proposal.ID, tempID)
		}
		tempToIssueID[tempID] = issueID
		_, ok, err := h.loadExistingDecomposeIssue(projectID, sessionID, issueID)
		if err != nil {
			var conflict *conflictError
			if errors.As(err, &conflict) {
				writeAPIError(w, http.StatusConflict, err.Error(), "ISSUE_ID_CONFLICT")
				return
			}
			writeAPIError(w, http.StatusInternalServerError, "failed to load existing issue", "GET_ISSUE_FAILED")
			return
		}
		if ok {
			existingIssues[issueID] = struct{}{}
		}
	}
	for _, item := range resolvedItems {
		for _, dep := range normalizeIssueRefs(item.DependsOn) {
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
	for _, item := range resolvedItems {
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
		spec := teamleader.CreateIssueSpec{
			ID:           issueID,
			Title:        strings.TrimSpace(item.Title),
			Body:         item.Body,
			Labels:       append([]string(nil), item.Labels...),
			DependsOn:    normalizeIssueRefs(resolvedDeps),
			Blocks:       normalizeIssueRefs(tempToBlocks[tempID]),
			Template:     template,
			AutoMerge:    item.AutoMerge,
			ChildrenMode: item.ChildrenMode,
		}
		specs = append(specs, spec)
		if _, ok := existingIssues[issueID]; !ok {
			missingSpecs = append(missingSpecs, spec)
		}
		createdRefs = append(createdRefs, createdIssueRef{TempID: tempID, IssueID: issueID})
	}

	if len(missingSpecs) > 0 {
		if err := h.ensureDecomposeSession(projectID, sessionID); err != nil {
			var conflict *conflictError
			if errors.As(err, &conflict) {
				writeAPIError(w, http.StatusConflict, err.Error(), "SESSION_ID_CONFLICT")
				return
			}
			writeAPIError(w, http.StatusInternalServerError, "failed to prepare decompose session", "ENSURE_SESSION_FAILED")
			return
		}
		if _, err := h.creator.CreateIssues(r.Context(), teamleader.CreateIssuesInput{
			ProjectID: projectID,
			SessionID: sessionID,
			Issues:    missingSpecs,
		}); err != nil {
			writeAPIError(w, http.StatusInternalServerError, err.Error(), "CONFIRM_FAILED")
			return
		}
	}
	for _, spec := range specs {
		if _, ok := existingIssues[spec.ID]; !ok {
			continue
		}
		if err := h.reconcileExistingDecomposeIssue(projectID, sessionID, spec); err != nil {
			var conflict *conflictError
			if errors.As(err, &conflict) {
				writeAPIError(w, http.StatusConflict, err.Error(), "ISSUE_STATE_CONFLICT")
				return
			}
			writeAPIError(w, http.StatusInternalServerError, err.Error(), "SAVE_ISSUE_FAILED")
			return
		}
	}

	confirmIDs := make([]string, 0, len(specs))
	for _, spec := range specs {
		issue, err := h.store.GetIssue(spec.ID)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "failed to load created issue", "GET_ISSUE_FAILED")
			return
		}
		if shouldConfirmDecomposeIssue(issue) {
			confirmIDs = append(confirmIDs, spec.ID)
		}
	}

	if len(confirmIDs) > 0 {
		if _, err := h.creator.ConfirmCreatedIssues(r.Context(), confirmIDs, "confirmed from decompose proposal"); err != nil {
			writeAPIError(w, http.StatusInternalServerError, err.Error(), "CONFIRM_ACTIVATE_FAILED")
			return
		}
	}
	writeJSON(w, http.StatusCreated, confirmDecomposeResponse{CreatedIssues: createdRefs})
}

func (h *decomposeHandlers) ensureProjectExists(w http.ResponseWriter, projectID string) bool {
	if h == nil || h.store == nil {
		writeAPIError(w, http.StatusInternalServerError, "store is not configured", "STORE_NOT_CONFIGURED")
		return false
	}
	_, err := h.store.GetProject(strings.TrimSpace(projectID))
	if err == nil {
		return true
	}
	if isNotFoundError(err) {
		writeAPIError(w, http.StatusNotFound, "project "+strings.TrimSpace(projectID)+" not found", "PROJECT_NOT_FOUND")
		return false
	}
	writeAPIError(w, http.StatusInternalServerError, "failed to load project", "GET_PROJECT_FAILED")
	return false
}

func (h *decomposeHandlers) ensureDecomposeSession(projectID, sessionID string) error {
	if h == nil || h.store == nil {
		return nil
	}
	session, err := h.store.GetChatSession(strings.TrimSpace(sessionID))
	if err == nil {
		if session != nil && strings.TrimSpace(session.ProjectID) != strings.TrimSpace(projectID) {
			return &conflictError{message: "session " + strings.TrimSpace(sessionID) + " belongs to another project"}
		}
		return nil
	}
	if !isNotFoundError(err) {
		return err
	}
	return h.store.CreateChatSession(&core.ChatSession{
		ID:        strings.TrimSpace(sessionID),
		ProjectID: strings.TrimSpace(projectID),
		AgentName: "dag_decompose",
		Messages:  []core.ChatMessage{},
	})
}

func (h *decomposeHandlers) loadExistingDecomposeIssue(projectID, sessionID, issueID string) (*core.Issue, bool, error) {
	if h == nil || h.store == nil {
		return nil, false, nil
	}
	issue, err := h.store.GetIssue(strings.TrimSpace(issueID))
	if err != nil {
		if isNotFoundError(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if issue == nil {
		return nil, false, nil
	}
	if strings.TrimSpace(issue.ProjectID) != strings.TrimSpace(projectID) {
		return nil, false, &conflictError{message: "issue " + strings.TrimSpace(issueID) + " belongs to another project"}
	}
	if strings.TrimSpace(issue.SessionID) != strings.TrimSpace(sessionID) {
		return nil, false, &conflictError{message: "issue " + strings.TrimSpace(issueID) + " belongs to another decompose session"}
	}
	return issue, true, nil
}

func applyProposalChildrenModes(proposal core.DecomposeProposal) ([]core.ProposalItem, error) {
	items := append([]core.ProposalItem(nil), proposal.Items...)
	previousTempID := ""
	for idx := range items {
		currentTempID := strings.TrimSpace(items[idx].TempID)
		if items[idx].ChildrenMode == core.ChildrenModeSequential && previousTempID != "" {
			if !containsTrimmed(items[idx].DependsOn, previousTempID) {
				items[idx].DependsOn = append(append([]string(nil), items[idx].DependsOn...), previousTempID)
			}
		}
		previousTempID = currentTempID
	}
	updated := core.DecomposeProposal{
		ID:        proposal.ID,
		ProjectID: proposal.ProjectID,
		Prompt:    proposal.Prompt,
		Summary:   proposal.Summary,
		Items:     items,
		CreatedAt: proposal.CreatedAt,
	}
	if err := updated.Validate(); err != nil {
		return nil, err
	}
	return items, nil
}

func containsTrimmed(values []string, target string) bool {
	trimmedTarget := strings.TrimSpace(target)
	if trimmedTarget == "" {
		return false
	}
	for _, value := range values {
		if strings.TrimSpace(value) == trimmedTarget {
			return true
		}
	}
	return false
}

func normalizeCreateIssueSpec(spec teamleader.CreateIssueSpec, existing *core.Issue) teamleader.CreateIssueSpec {
	normalized := spec
	normalized.ID = strings.TrimSpace(normalized.ID)
	normalized.Title = strings.TrimSpace(normalized.Title)
	normalized.Template = strings.TrimSpace(normalized.Template)
	if normalized.Template == "" {
		normalized.Template = "standard"
	}
	if normalized.ChildrenMode == "" {
		normalized.ChildrenMode = core.ChildrenModeParallel
	}
	normalized.Labels = append([]string(nil), normalized.Labels...)
	normalized.DependsOn = normalizeIssueRefs(normalized.DependsOn)
	normalized.Blocks = normalizeIssueRefs(normalized.Blocks)
	if normalized.AutoMerge == nil && existing != nil {
		autoMerge := existing.AutoMerge
		normalized.AutoMerge = &autoMerge
	}
	return normalized
}

func normalizeIssueRefs(refs []string) []string {
	if len(refs) == 0 {
		return nil
	}
	out := make([]string, 0, len(refs))
	seen := make(map[string]struct{}, len(refs))
	for _, ref := range refs {
		id := strings.TrimSpace(ref)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func decomposeIssueSpecChanged(issue *core.Issue, spec teamleader.CreateIssueSpec) bool {
	if issue == nil {
		return true
	}
	if issue.Title != spec.Title ||
		issue.Body != spec.Body ||
		issue.Template != spec.Template ||
		issue.ChildrenMode != spec.ChildrenMode {
		return true
	}
	if !reflect.DeepEqual(issue.Labels, spec.Labels) ||
		!reflect.DeepEqual(issue.DependsOn, spec.DependsOn) ||
		!reflect.DeepEqual(issue.Blocks, spec.Blocks) {
		return true
	}
	if spec.AutoMerge != nil && issue.AutoMerge != *spec.AutoMerge {
		return true
	}
	return false
}

func (h *decomposeHandlers) reconcileExistingDecomposeIssue(projectID, sessionID string, spec teamleader.CreateIssueSpec) error {
	if h == nil || h.store == nil {
		return nil
	}
	latest, ok, err := h.loadExistingDecomposeIssue(projectID, sessionID, spec.ID)
	if err != nil {
		return err
	}
	if !ok || latest == nil {
		return &conflictError{message: "issue " + strings.TrimSpace(spec.ID) + " changed during confirm retry"}
	}
	normalized := normalizeCreateIssueSpec(spec, latest)
	if !decomposeIssueSpecChanged(latest, normalized) {
		return nil
	}
	if latest.Status != core.IssueStatusDraft {
		return &conflictError{message: "issue " + strings.TrimSpace(spec.ID) + " can no longer be modified from status " + string(latest.Status)}
	}
	updated := *latest
	updated.Title = normalized.Title
	updated.Body = normalized.Body
	updated.Labels = append([]string(nil), normalized.Labels...)
	updated.DependsOn = append([]string(nil), normalized.DependsOn...)
	updated.Blocks = append([]string(nil), normalized.Blocks...)
	updated.Template = normalized.Template
	updated.ChildrenMode = normalized.ChildrenMode
	if normalized.AutoMerge != nil {
		updated.AutoMerge = *normalized.AutoMerge
	}
	if err := updated.Validate(); err != nil {
		return err
	}
	return h.store.SaveIssue(&updated)
}

func decomposeSessionID(proposalID string) string {
	return "decompose:" + strings.TrimSpace(proposalID)
}

func decomposeIssueID(proposalID, tempID string) string {
	sum := sha1.Sum([]byte(strings.TrimSpace(proposalID) + ":" + strings.TrimSpace(tempID)))
	return "issue-decompose-" + hex.EncodeToString(sum[:8])
}

func shouldConfirmDecomposeIssue(issue *core.Issue) bool {
	if issue == nil {
		return false
	}
	switch issue.Status {
	case core.IssueStatusDraft, core.IssueStatusReviewing:
		return true
	default:
		return false
	}
}

type conflictError struct {
	message string
}

type decomposeBadRequestError interface {
	BadRequest() bool
}

func (e *conflictError) Error() string {
	if e == nil {
		return ""
	}
	return strings.TrimSpace(e.message)
}

func (e *conflictError) Conflict() bool {
	return true
}

func classifyDecomposePlanError(err error) (int, string) {
	if err == nil {
		return http.StatusBadGateway, "DECOMPOSE_UPSTREAM_FAILED"
	}

	var badRequest decomposeBadRequestError
	if errors.As(err, &badRequest) && badRequest.BadRequest() {
		return http.StatusBadRequest, "DECOMPOSE_FAILED"
	}
	if errors.Is(err, context.DeadlineExceeded) || isDecomposeTimeoutError(err) {
		return http.StatusGatewayTimeout, "DECOMPOSE_UPSTREAM_TIMEOUT"
	}
	if isDecomposeUnavailableError(err) {
		return http.StatusServiceUnavailable, "DECOMPOSE_UPSTREAM_UNAVAILABLE"
	}
	return http.StatusBadGateway, "DECOMPOSE_UPSTREAM_FAILED"
}

func isDecomposeTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	lowered := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(lowered, "timeout") ||
		strings.Contains(lowered, "timed out") ||
		strings.Contains(lowered, "deadline exceeded")
}

func isDecomposeUnavailableError(err error) bool {
	if err == nil {
		return false
	}
	lowered := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(lowered, "credential") ||
		strings.Contains(lowered, "not configured") ||
		strings.Contains(lowered, "provider") ||
		strings.Contains(lowered, "connection refused") ||
		strings.Contains(lowered, "network is unreachable") ||
		strings.Contains(lowered, "no such host")
}
