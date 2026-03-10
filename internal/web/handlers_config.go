package web

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/yoke233/ai-workflow/internal/configruntime"
)

type adminConfigHandlers struct {
	runtime RuntimeConfigManager
}

type adminConfigTomlRequest struct {
	Content string `json:"content"`
}

type adminConfigTomlResponse struct {
	Content string `json:"content"`
}

type adminConfigRuntimeStatusResponse struct {
	ActiveVersion int64  `json:"active_version"`
	LastSuccessAt string `json:"last_success_at,omitempty"`
	LastError     string `json:"last_error,omitempty"`
	LastErrorAt   string `json:"last_error_at,omitempty"`
}

type adminConfigUpdateResponse struct {
	Status        string                           `json:"status"`
	RuntimeStatus adminConfigRuntimeStatusResponse `json:"runtime_status"`
}

func registerAdminConfigRoutes(r chi.Router, runtime RuntimeConfigManager) {
	h := &adminConfigHandlers{runtime: runtime}
	r.Get("/admin/config/runtime-status", h.handleRuntimeStatus)
	r.Get("/admin/config/toml", h.handleGetToml)
	r.Put("/admin/config/toml", h.handlePutToml)
	r.Get("/admin/config/v2-runtime", h.handleGetV2Runtime)
	r.Put("/admin/config/v2-runtime", h.handlePutV2Runtime)
}

func (h *adminConfigHandlers) handleRuntimeStatus(w http.ResponseWriter, _ *http.Request) {
	if h.runtime == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "runtime config manager is not configured", "CONFIG_RUNTIME_UNAVAILABLE")
		return
	}
	writeJSON(w, http.StatusOK, toAdminRuntimeStatus(h.runtime.Status()))
}

func (h *adminConfigHandlers) handleGetToml(w http.ResponseWriter, _ *http.Request) {
	if h.runtime == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "runtime config manager is not configured", "CONFIG_RUNTIME_UNAVAILABLE")
		return
	}
	content, err := h.runtime.ReadRawString()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "failed to read config toml", "READ_CONFIG_TOML_FAILED")
		return
	}
	writeJSON(w, http.StatusOK, adminConfigTomlResponse{Content: content})
}

func (h *adminConfigHandlers) handlePutToml(w http.ResponseWriter, r *http.Request) {
	if h.runtime == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "runtime config manager is not configured", "CONFIG_RUNTIME_UNAVAILABLE")
		return
	}
	var req adminConfigTomlRequest
	if err := decodeJSONBodyStrict(r, &req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid json body", "INVALID_JSON")
		return
	}
	if _, err := h.runtime.WriteRaw(r.Context(), req.Content); err != nil {
		h.writeMutationError(w, err, "INVALID_CONFIG_TOML")
		return
	}
	writeJSON(w, http.StatusOK, adminConfigUpdateResponse{
		Status:        "ok",
		RuntimeStatus: toAdminRuntimeStatus(h.runtime.Status()),
	})
}

func (h *adminConfigHandlers) handleGetV2Runtime(w http.ResponseWriter, _ *http.Request) {
	if h.runtime == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "runtime config manager is not configured", "CONFIG_RUNTIME_UNAVAILABLE")
		return
	}
	writeJSON(w, http.StatusOK, h.runtime.GetV2Runtime())
}

func (h *adminConfigHandlers) handlePutV2Runtime(w http.ResponseWriter, r *http.Request) {
	if h.runtime == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "runtime config manager is not configured", "CONFIG_RUNTIME_UNAVAILABLE")
		return
	}
	var req configruntime.V2RuntimeConfig
	if err := decodeJSONBodyStrict(r, &req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid json body", "INVALID_JSON")
		return
	}
	if _, err := h.runtime.UpdateV2Runtime(r.Context(), req); err != nil {
		h.writeMutationError(w, err, "INVALID_V2_CONFIG")
		return
	}
	writeJSON(w, http.StatusOK, adminConfigUpdateResponse{
		Status:        "ok",
		RuntimeStatus: toAdminRuntimeStatus(h.runtime.Status()),
	})
}

func toAdminRuntimeStatus(status configruntime.ReloadStatus) adminConfigRuntimeStatusResponse {
	resp := adminConfigRuntimeStatusResponse{
		ActiveVersion: status.ActiveVersion,
		LastError:     status.LastError,
	}
	if !status.LastSuccessAt.IsZero() {
		resp.LastSuccessAt = status.LastSuccessAt.Format(time.RFC3339)
	}
	if !status.LastErrorAt.IsZero() {
		resp.LastErrorAt = status.LastErrorAt.Format(time.RFC3339)
	}
	return resp
}

func (h *adminConfigHandlers) writeMutationError(w http.ResponseWriter, err error, invalidCode string) {
	var validationErr *configruntime.ValidationError
	if errors.As(err, &validationErr) {
		writeAPIError(w, http.StatusBadRequest, err.Error(), invalidCode)
		return
	}
	writeAPIError(w, http.StatusInternalServerError, "failed to update config", "UPDATE_CONFIG_FAILED")
}
