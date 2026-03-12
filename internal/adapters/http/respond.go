package api

import (
	"encoding/json"
	"net/http"

	"github.com/yoke233/ai-workflow/internal/platform/i18n"
)

// writeJSON encodes v as JSON and writes it with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// apiError is the standard error response body.
type apiError struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// writeError writes a JSON error response.
// The error message is returned in English by default. If the request
// contains an Accept-Language header indicating Chinese, the message
// is automatically translated when a translation is available.
func writeError(w http.ResponseWriter, status int, msg, code string) {
	writeJSON(w, status, apiError{Error: msg, Code: code})
}

// writeErrorLocalized writes a JSON error response with automatic
// localization based on the Accept-Language header.
func writeErrorLocalized(w http.ResponseWriter, r *http.Request, status int, msg, code string) {
	lang := i18n.DetectLang(r)
	translated := i18n.Translate(lang, code, msg)
	writeJSON(w, status, apiError{Error: translated, Code: code})
}
