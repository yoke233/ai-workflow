package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/yoke233/zhanggui/internal/application/proposalapp"
	"github.com/yoke233/zhanggui/internal/core"
)

type createProposalRequest struct {
	Title           string                       `json:"title"`
	Summary         string                       `json:"summary"`
	Content         string                       `json:"content"`
	ProposedBy      string                       `json:"proposed_by"`
	WorkItemDrafts  []core.ProposalWorkItemDraft `json:"work_item_drafts"`
	SourceMessageID *int64                       `json:"source_message_id,omitempty"`
	Metadata        map[string]any               `json:"metadata,omitempty"`
}

type updateProposalRequest struct {
	Title           *string         `json:"title,omitempty"`
	Summary         *string         `json:"summary,omitempty"`
	Content         *string         `json:"content,omitempty"`
	SourceMessageID *int64          `json:"source_message_id,omitempty"`
	Metadata        *map[string]any `json:"metadata,omitempty"`
}

type replaceProposalDraftsRequest struct {
	WorkItemDrafts []core.ProposalWorkItemDraft `json:"work_item_drafts"`
}

type reviewProposalRequest struct {
	ReviewedBy string `json:"reviewed_by"`
	ReviewNote string `json:"review_note"`
}

func registerProposalRoutes(r chi.Router, h *Handler) {
	r.Post("/threads/{threadID}/proposals", h.createThreadProposal)
	r.Get("/threads/{threadID}/proposals", h.listThreadProposals)
	r.Get("/proposals/{proposalID}", h.getThreadProposal)
	r.Put("/proposals/{proposalID}", h.updateThreadProposal)
	r.Delete("/proposals/{proposalID}", h.deleteThreadProposal)
	r.Put("/proposals/{proposalID}/drafts", h.replaceThreadProposalDrafts)
	r.Post("/proposals/{proposalID}/submit", h.submitThreadProposal)
	r.Post("/proposals/{proposalID}/approve", h.approveThreadProposal)
	r.Post("/proposals/{proposalID}/reject", h.rejectThreadProposal)
	r.Post("/proposals/{proposalID}/revise", h.reviseThreadProposal)
}

func (h *Handler) createThreadProposal(w http.ResponseWriter, r *http.Request) {
	threadID, ok := urlParamInt64(r, "threadID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid thread ID", "BAD_ID")
		return
	}
	var req createProposalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "BAD_REQUEST")
		return
	}
	proposal, err := h.proposalService().CreateProposal(r.Context(), proposalapp.CreateProposalInput{
		ThreadID:        threadID,
		Title:           req.Title,
		Summary:         req.Summary,
		Content:         req.Content,
		ProposedBy:      req.ProposedBy,
		WorkItemDrafts:  req.WorkItemDrafts,
		SourceMessageID: req.SourceMessageID,
		Metadata:        req.Metadata,
	})
	if err != nil {
		writeProposalAppFailure(w, err, "CREATE_PROPOSAL_FAILED")
		return
	}
	writeJSON(w, http.StatusCreated, proposal)
}

func (h *Handler) listThreadProposals(w http.ResponseWriter, r *http.Request) {
	threadID, ok := urlParamInt64(r, "threadID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid thread ID", "BAD_ID")
		return
	}
	var status *core.ProposalStatus
	if raw := r.URL.Query().Get("status"); raw != "" {
		parsed, valid := core.ParseProposalStatus(raw)
		if !valid {
			writeError(w, http.StatusBadRequest, "invalid proposal status", "BAD_STATUS")
			return
		}
		status = &parsed
	}
	items, err := h.proposalService().ListThreadProposals(r.Context(), threadID, status)
	if err != nil {
		writeProposalAppFailure(w, err, "LIST_PROPOSALS_FAILED")
		return
	}
	if items == nil {
		items = []*core.ThreadProposal{}
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) getThreadProposal(w http.ResponseWriter, r *http.Request) {
	proposalID, ok := urlParamInt64(r, "proposalID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid proposal ID", "BAD_ID")
		return
	}
	proposal, err := h.proposalService().GetProposal(r.Context(), proposalID)
	if err != nil {
		writeProposalAppFailure(w, err, "GET_PROPOSAL_FAILED")
		return
	}
	writeJSON(w, http.StatusOK, proposal)
}

func (h *Handler) updateThreadProposal(w http.ResponseWriter, r *http.Request) {
	proposalID, ok := urlParamInt64(r, "proposalID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid proposal ID", "BAD_ID")
		return
	}
	var req updateProposalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "BAD_REQUEST")
		return
	}
	proposal, err := h.proposalService().UpdateProposal(r.Context(), proposalapp.UpdateProposalInput{
		ID:              proposalID,
		Title:           req.Title,
		Summary:         req.Summary,
		Content:         req.Content,
		SourceMessageID: req.SourceMessageID,
		Metadata:        req.Metadata,
	})
	if err != nil {
		writeProposalAppFailure(w, err, "UPDATE_PROPOSAL_FAILED")
		return
	}
	writeJSON(w, http.StatusOK, proposal)
}

func (h *Handler) deleteThreadProposal(w http.ResponseWriter, r *http.Request) {
	proposalID, ok := urlParamInt64(r, "proposalID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid proposal ID", "BAD_ID")
		return
	}
	if err := h.proposalService().DeleteProposal(r.Context(), proposalID); err != nil {
		writeProposalAppFailure(w, err, "DELETE_PROPOSAL_FAILED")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handler) replaceThreadProposalDrafts(w http.ResponseWriter, r *http.Request) {
	proposalID, ok := urlParamInt64(r, "proposalID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid proposal ID", "BAD_ID")
		return
	}
	var req replaceProposalDraftsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "BAD_REQUEST")
		return
	}
	proposal, err := h.proposalService().ReplaceDrafts(r.Context(), proposalID, req.WorkItemDrafts)
	if err != nil {
		writeProposalAppFailure(w, err, "REPLACE_PROPOSAL_DRAFTS_FAILED")
		return
	}
	writeJSON(w, http.StatusOK, proposal)
}

func (h *Handler) submitThreadProposal(w http.ResponseWriter, r *http.Request) {
	proposalID, ok := urlParamInt64(r, "proposalID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid proposal ID", "BAD_ID")
		return
	}
	proposal, err := h.proposalService().Submit(r.Context(), proposalID)
	if err != nil {
		writeProposalAppFailure(w, err, "SUBMIT_PROPOSAL_FAILED")
		return
	}
	writeJSON(w, http.StatusOK, proposal)
}

func (h *Handler) approveThreadProposal(w http.ResponseWriter, r *http.Request) {
	proposalID, ok := urlParamInt64(r, "proposalID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid proposal ID", "BAD_ID")
		return
	}
	var req reviewProposalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "BAD_REQUEST")
		return
	}
	proposal, err := h.proposalService().Approve(r.Context(), proposalID, proposalapp.ReviewInput{
		ReviewedBy: req.ReviewedBy,
		ReviewNote: req.ReviewNote,
	})
	if err != nil {
		writeProposalAppFailure(w, err, "APPROVE_PROPOSAL_FAILED")
		return
	}
	writeJSON(w, http.StatusOK, proposal)
}

func (h *Handler) rejectThreadProposal(w http.ResponseWriter, r *http.Request) {
	proposalID, ok := urlParamInt64(r, "proposalID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid proposal ID", "BAD_ID")
		return
	}
	var req reviewProposalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "BAD_REQUEST")
		return
	}
	proposal, err := h.proposalService().Reject(r.Context(), proposalID, proposalapp.ReviewInput{
		ReviewedBy: req.ReviewedBy,
		ReviewNote: req.ReviewNote,
	})
	if err != nil {
		writeProposalAppFailure(w, err, "REJECT_PROPOSAL_FAILED")
		return
	}
	writeJSON(w, http.StatusOK, proposal)
}

func (h *Handler) reviseThreadProposal(w http.ResponseWriter, r *http.Request) {
	proposalID, ok := urlParamInt64(r, "proposalID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid proposal ID", "BAD_ID")
		return
	}
	var req reviewProposalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "BAD_REQUEST")
		return
	}
	proposal, err := h.proposalService().Revise(r.Context(), proposalID, proposalapp.ReviseInput{
		ReviewedBy: req.ReviewedBy,
		ReviewNote: req.ReviewNote,
	})
	if err != nil {
		writeProposalAppFailure(w, err, "REVISE_PROPOSAL_FAILED")
		return
	}
	writeJSON(w, http.StatusOK, proposal)
}
