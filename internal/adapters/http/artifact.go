package api

import (
	"net/http"

	"github.com/yoke233/ai-workflow/internal/core"
)

func (h *Handler) getArtifact(w http.ResponseWriter, r *http.Request) {
	id, ok := urlParamInt64(r, "artifactID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid artifact ID", "BAD_ID")
		return
	}

	a, err := h.store.GetArtifact(r.Context(), id)
	if err == core.ErrNotFound {
		writeError(w, http.StatusNotFound, "artifact not found", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}
	writeJSON(w, http.StatusOK, a)
}

func (h *Handler) getLatestArtifact(w http.ResponseWriter, r *http.Request) {
	stepID, ok := urlParamInt64(r, "stepID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid step ID", "BAD_ID")
		return
	}

	a, err := h.store.GetLatestArtifactByStep(r.Context(), stepID)
	if err == core.ErrNotFound {
		writeError(w, http.StatusNotFound, "no artifact for this step", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}
	writeJSON(w, http.StatusOK, a)
}

func (h *Handler) listArtifactsByExec(w http.ResponseWriter, r *http.Request) {
	execID, ok := urlParamInt64(r, "execID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid execution ID", "BAD_ID")
		return
	}

	artifacts, err := h.store.ListArtifactsByExecution(r.Context(), execID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}
	if artifacts == nil {
		artifacts = []*core.Artifact{}
	}
	writeJSON(w, http.StatusOK, artifacts)
}

