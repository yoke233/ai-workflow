package api

import (
	"net/http"

	"github.com/yoke233/ai-workflow/internal/core"
)

func (h *Handler) getBriefing(w http.ResponseWriter, r *http.Request) {
	id, ok := urlParamInt64(r, "briefingID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid briefing ID", "BAD_ID")
		return
	}

	b, err := h.store.GetBriefing(r.Context(), id)
	if err == core.ErrNotFound {
		writeError(w, http.StatusNotFound, "briefing not found", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}
	writeJSON(w, http.StatusOK, b)
}

func (h *Handler) getBriefingByStep(w http.ResponseWriter, r *http.Request) {
	stepID, ok := urlParamInt64(r, "stepID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid step ID", "BAD_ID")
		return
	}

	b, err := h.store.GetBriefingByStep(r.Context(), stepID)
	if err == core.ErrNotFound {
		writeError(w, http.StatusNotFound, "no briefing for this step", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}
	writeJSON(w, http.StatusOK, b)
}

