package api

import (
	"net/http"

	"github.com/yoke233/zhanggui/internal/core"
)

func (h *Handler) getRun(w http.ResponseWriter, r *http.Request) {
	id, ok := urlParamInt64(r, "runID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid run ID", "BAD_ID")
		return
	}

	run, err := h.store.GetRun(r.Context(), id)
	if err == core.ErrNotFound {
		writeError(w, http.StatusNotFound, "run not found", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (h *Handler) listRuns(w http.ResponseWriter, r *http.Request) {
	actionID, ok := urlParamInt64(r, "actionID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid action ID", "BAD_ID")
		return
	}

	runs, err := h.store.ListRunsByAction(r.Context(), actionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}
	if runs == nil {
		runs = []*core.Run{}
	}
	writeJSON(w, http.StatusOK, runs)
}
