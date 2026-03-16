package api

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// themesDir returns the absolute path to the user themes directory.
func (h *Handler) themesDir() string {
	return filepath.Join(h.dataDir, "themes")
}

// themeMetadata is stored alongside the theme JSON for listing purposes.
type themeMetadata struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"` // "dark" | "light"
	CreatedAt string `json:"created_at"`
}

type themeListItem struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Folder    string `json:"folder"`
	CreatedAt string `json:"created_at"`
}

var themeIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9\-]{0,62}[a-z0-9]$`)

func validThemeID(id string) bool {
	if len(id) < 2 || len(id) > 64 {
		return false
	}
	return themeIDRe.MatchString(id)
}

// listUserThemes returns all themes stored in {dataDir}/themes/.
func (h *Handler) listUserThemes(w http.ResponseWriter, _ *http.Request) {
	dir := h.themesDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, http.StatusOK, []themeListItem{})
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error(), "READ_DIR_ERROR")
		return
	}

	var items []themeListItem
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		metaPath := filepath.Join(dir, e.Name(), "meta.json")
		raw, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		var meta themeMetadata
		if json.Unmarshal(raw, &meta) != nil {
			continue
		}
		items = append(items, themeListItem{
			ID:        meta.ID,
			Name:      meta.Name,
			Type:      meta.Type,
			Folder:    e.Name(),
			CreatedAt: meta.CreatedAt,
		})
	}
	if items == nil {
		items = []themeListItem{}
	}
	writeJSON(w, http.StatusOK, items)
}

// getUserTheme returns the theme.json content for a specific theme.
func (h *Handler) getUserTheme(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "themeID")
	if !validThemeID(id) {
		writeError(w, http.StatusBadRequest, "invalid theme ID", "BAD_ID")
		return
	}

	themeFile := filepath.Join(h.themesDir(), id, "theme.json")
	raw, err := os.ReadFile(themeFile)
	if err != nil {
		if os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, "theme not found", "NOT_FOUND")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error(), "READ_ERROR")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}

type saveThemeRequest struct {
	ID   string          `json:"id"`
	Name string          `json:"name"`
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"` // the full VSCode theme JSON
}

// saveUserTheme creates or replaces a user theme.
func (h *Handler) saveUserTheme(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 512*1024)) // 512KB max
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read body", "READ_BODY")
		return
	}

	var req saveThemeRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON", "BAD_JSON")
		return
	}

	// Sanitize ID
	id := strings.TrimSpace(req.ID)
	if !validThemeID(id) {
		writeError(w, http.StatusBadRequest, "invalid theme id: must be 2-64 chars, lowercase alphanumeric and hyphens", "BAD_ID")
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = id
	}
	themeType := req.Type
	if themeType != "dark" && themeType != "light" {
		themeType = "dark"
	}

	if len(req.Data) == 0 {
		writeError(w, http.StatusBadRequest, "missing theme data", "MISSING_DATA")
		return
	}

	// Validate that data is valid JSON
	var check json.RawMessage
	if json.Unmarshal(req.Data, &check) != nil {
		writeError(w, http.StatusBadRequest, "theme data is not valid JSON", "BAD_DATA")
		return
	}

	// Create directory
	themeDir := filepath.Join(h.themesDir(), id)
	if err := os.MkdirAll(themeDir, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "MKDIR_ERROR")
		return
	}

	// Write theme.json
	if err := os.WriteFile(filepath.Join(themeDir, "theme.json"), req.Data, 0o644); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "WRITE_ERROR")
		return
	}

	// Write meta.json
	meta := themeMetadata{
		ID:        id,
		Name:      name,
		Type:      themeType,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	metaRaw, _ := json.MarshalIndent(meta, "", "  ")
	if err := os.WriteFile(filepath.Join(themeDir, "meta.json"), metaRaw, 0o644); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "WRITE_META_ERROR")
		return
	}

	writeJSON(w, http.StatusCreated, meta)
}

// deleteUserTheme removes a user theme directory.
func (h *Handler) deleteUserTheme(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "themeID")
	if !validThemeID(id) {
		writeError(w, http.StatusBadRequest, "invalid theme ID", "BAD_ID")
		return
	}

	themeDir := filepath.Join(h.themesDir(), id)
	if _, err := os.Stat(themeDir); os.IsNotExist(err) {
		writeError(w, http.StatusNotFound, "theme not found", "NOT_FOUND")
		return
	}

	if err := os.RemoveAll(themeDir); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "DELETE_ERROR")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
