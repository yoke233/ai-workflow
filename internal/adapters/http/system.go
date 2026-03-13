package api

import (
	"encoding/json"
	"net/http"

	"github.com/yoke233/ai-workflow/internal/adapters/llmconfig"
	v2sandbox "github.com/yoke233/ai-workflow/internal/adapters/sandbox"
	"github.com/yoke233/ai-workflow/internal/platform/config"
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

func (h *Handler) getLLMConfig(w http.ResponseWriter, r *http.Request) {
	controller := h.llmConfig
	if controller == nil {
		controller = llmconfig.NewReadOnlyControlService(config.Defaults().Runtime.LLM)
	}
	writeJSON(w, http.StatusOK, controller.Inspect(r.Context()))
}

func (h *Handler) updateLLMConfig(w http.ResponseWriter, r *http.Request) {
	controller := h.llmConfig
	if controller == nil {
		controller = llmconfig.NewReadOnlyControlService(config.Defaults().Runtime.LLM)
	}

	var req llmconfig.UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "BAD_REQUEST")
		return
	}
	if req.DefaultConfigID == nil && req.Configs == nil {
		writeError(w, http.StatusBadRequest, "default_config_id or configs is required", "MISSING_UPDATE_FIELDS")
		return
	}

	report, err := controller.Update(r.Context(), req)
	if err != nil {
		switch err {
		case llmconfig.ErrLLMConfigUnavailable:
			writeError(w, http.StatusServiceUnavailable, err.Error(), "LLM_CONFIG_UNAVAILABLE")
		default:
			writeError(w, http.StatusBadRequest, err.Error(), "LLM_CONFIG_UPDATE_FAILED")
		}
		return
	}
	writeJSON(w, http.StatusOK, report)
}
