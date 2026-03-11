package api

import (
	"encoding/json"
	"net/http"

	v2sandbox "github.com/yoke233/ai-workflow/internal/adapters/sandbox"
)

func (h *Handler) getSandboxSupport(w http.ResponseWriter, r *http.Request) {
	controller := h.sandbox
	if controller == nil {
		controller = v2sandbox.NewReadOnlyControlService(v2sandbox.NewDefaultSupportInspector(false, ""))
	}
	writeJSON(w, http.StatusOK, controller.Inspect(r.Context()))
}

func (h *Handler) updateSandboxSupport(w http.ResponseWriter, r *http.Request) {
	controller := h.sandbox
	if controller == nil {
		controller = v2sandbox.NewReadOnlyControlService(v2sandbox.NewDefaultSupportInspector(false, ""))
	}

	var req v2sandbox.UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "BAD_REQUEST")
		return
	}
	if req.Enabled == nil && req.Provider == nil {
		writeError(w, http.StatusBadRequest, "enabled or provider is required", "MISSING_UPDATE_FIELDS")
		return
	}

	report, err := controller.Update(r.Context(), req)
	if err != nil {
		switch err {
		case v2sandbox.ErrSandboxConfigUnavailable:
			writeError(w, http.StatusServiceUnavailable, err.Error(), "SANDBOX_CONFIG_UNAVAILABLE")
		default:
			writeError(w, http.StatusBadRequest, err.Error(), "SANDBOX_UPDATE_FAILED")
		}
		return
	}
	writeJSON(w, http.StatusOK, report)
}
