package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/yoke233/ai-workflow/internal/core"
	skillset "github.com/yoke233/ai-workflow/internal/skills"
)

type skillInfo struct {
	Name             string             `json:"name"`
	HasSkillMD       bool               `json:"has_skill_md"`
	Valid            bool               `json:"valid"`
	Metadata         *skillset.Metadata `json:"metadata,omitempty"`
	ValidationErrors []string           `json:"validation_errors,omitempty"`
	ProfilesUsing    []string           `json:"profiles_using,omitempty"`
}

type getSkillResponse struct {
	Name             string             `json:"name"`
	SkillMD          string             `json:"skill_md"`
	HasSkillMD       bool               `json:"has_skill_md"`
	Valid            bool               `json:"valid"`
	Metadata         *skillset.Metadata `json:"metadata,omitempty"`
	ValidationErrors []string           `json:"validation_errors,omitempty"`
	ProfilesUsing    []string           `json:"profiles_using,omitempty"`
}

type createSkillRequest struct {
	Name    string `json:"name"`
	SkillMD string `json:"skill_md,omitempty"`
	GitHub  string `json:"github_url,omitempty"`
	Subdir  string `json:"subdir,omitempty"`
	DirName string `json:"dir_name,omitempty"`
}

type updateSkillRequest struct {
	SkillMD string `json:"skill_md"`
}

type importGitHubSkillRequest struct {
	RepoURL   string `json:"repo_url"`
	SkillName string `json:"skill_name"`
}

func registerSkillRoutes(r chi.Router, root string, registry core.AgentRegistry, importer skillset.GitHubImporter) {
	if importer == nil {
		importer = skillset.NewGitHubImporter(nil)
	}
	h := &skillsHandler{
		root:     strings.TrimSpace(root),
		registry: registry,
		importer: importer,
	}
	r.Route("/skills", func(r chi.Router) {
		r.Get("/", h.listSkills)
		r.Post("/", h.createSkill)
		r.Post("/import/github", h.importGitHubSkill)
		r.Get("/{skillName}", h.getSkill)
		r.Put("/{skillName}", h.updateSkill)
		r.Delete("/{skillName}", h.deleteSkill)
	})
}

type skillsHandler struct {
	root     string
	registry core.AgentRegistry
	importer skillset.GitHubImporter
}

func (h *skillsHandler) skillsRoot() (string, error) {
	if h.root != "" {
		return filepath.Clean(h.root), nil
	}
	return "", errors.New("skills root is not configured")
}

func (h *skillsHandler) listSkills(w http.ResponseWriter, r *http.Request) {
	root, err := h.skillsRoot()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "skills_root_error")
		return
	}
	profileRefs, err := h.skillProfileRefs(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "profile_refs_error")
		return
	}
	skills, err := skillset.ListSkills(root)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "read_dir_error")
		return
	}
	out := make([]skillInfo, 0, len(skills))
	for _, item := range skills {
		out = append(out, skillInfo{
			Name:             item.Name,
			HasSkillMD:       item.HasSkillMD,
			Valid:            item.Valid,
			Metadata:         item.Metadata,
			ValidationErrors: item.ValidationErrors,
			ProfilesUsing:    profileRefs[item.Name],
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *skillsHandler) getSkill(w http.ResponseWriter, r *http.Request) {
	name, ok := parseSkillName(w, r)
	if !ok {
		return
	}
	root, err := h.skillsRoot()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "skills_root_error")
		return
	}
	profileRefs, err := h.skillProfileRefs(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "profile_refs_error")
		return
	}
	skill, err := skillset.InspectSkill(root, name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusNotFound, "skill not found", "not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error(), "read_file_error")
		return
	}
	if !skill.HasSkillMD {
		writeError(w, http.StatusNotFound, "skill not found", "not_found")
		return
	}
	writeJSON(w, http.StatusOK, getSkillResponse{
		Name:             skill.Name,
		SkillMD:          skill.SkillMD,
		HasSkillMD:       skill.HasSkillMD,
		Valid:            skill.Valid,
		Metadata:         skill.Metadata,
		ValidationErrors: skill.ValidationErrors,
		ProfilesUsing:    profileRefs[skill.Name],
	})
}

func (h *skillsHandler) createSkill(w http.ResponseWriter, r *http.Request) {
	var req createSkillRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "bad_request")
		return
	}

	name := strings.TrimSpace(req.Name)
	if req.DirName != "" && name == "" {
		name = strings.TrimSpace(req.DirName)
	}
	if name == "" || !skillset.IsValidName(name) {
		writeError(w, http.StatusBadRequest, "invalid skill name", "invalid_name")
		return
	}

	if strings.TrimSpace(req.GitHub) != "" {
		writeError(w, http.StatusNotImplemented, "download from github not implemented yet", "not_implemented")
		return
	}

	root, err := h.skillsRoot()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "skills_root_error")
		return
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "mkdir_error")
		return
	}

	dir := filepath.Join(root, name)
	if _, err := os.Stat(dir); err == nil {
		writeError(w, http.StatusConflict, "skill already exists", "already_exists")
		return
	} else if !errors.Is(err, os.ErrNotExist) {
		writeError(w, http.StatusInternalServerError, err.Error(), "stat_error")
		return
	}
	skillMD := strings.TrimSpace(req.SkillMD)
	if skillMD == "" {
		skillMD = skillset.DefaultSkillMD(name)
	}
	meta, validationErrors := skillset.ValidateSkillMD(name, skillMD)
	if len(validationErrors) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":             "invalid skill_md",
			"code":              "invalid_skill_md",
			"validation_errors": validationErrors,
			"metadata":          meta,
		})
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "mkdir_error")
		return
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(skillMD+"\n"), 0o644); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "write_error")
		return
	}
	writeJSON(w, http.StatusCreated, skillInfo{
		Name:       name,
		HasSkillMD: true,
		Valid:      true,
		Metadata:   meta,
	})
}

func (h *skillsHandler) importGitHubSkill(w http.ResponseWriter, r *http.Request) {
	var req importGitHubSkillRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "bad_request")
		return
	}

	repoURL := strings.TrimSpace(req.RepoURL)
	skillName := strings.TrimSpace(req.SkillName)
	if repoURL == "" {
		writeError(w, http.StatusBadRequest, "repo_url is required", "missing_field")
		return
	}
	if skillName == "" || !skillset.IsValidName(skillName) {
		writeError(w, http.StatusBadRequest, "invalid skill name", "invalid_name")
		return
	}

	root, err := h.skillsRoot()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "skills_root_error")
		return
	}
	imported, err := h.importer.Import(r.Context(), root, skillset.GitHubImportRequest{
		RepoURL:   repoURL,
		SkillName: skillName,
	})
	if err != nil {
		var validationErr *skillset.RepoSkillValidationError
		switch {
		case errors.As(err, &validationErr):
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error":             "invalid skill_md",
				"code":              "invalid_skill_md",
				"validation_errors": validationErr.ValidationErrors,
				"metadata":          validationErr.Metadata,
			})
			return
		case errors.Is(err, skillset.ErrInvalidGitHubRepoURL), errors.Is(err, skillset.ErrUnsupportedGitHost):
			writeError(w, http.StatusBadRequest, err.Error(), "invalid_repo_url")
			return
		case errors.Is(err, skillset.ErrGitHubSkillNotFound):
			writeError(w, http.StatusNotFound, "skill not found in github repository", "repo_skill_not_found")
			return
		case errors.Is(err, skillset.ErrSkillAlreadyExists):
			writeError(w, http.StatusConflict, "skill already exists", "already_exists")
			return
		default:
			writeError(w, http.StatusInternalServerError, err.Error(), "import_error")
			return
		}
	}

	writeJSON(w, http.StatusCreated, skillInfo{
		Name:       imported.Name,
		HasSkillMD: imported.HasSkillMD,
		Valid:      imported.Valid,
		Metadata:   imported.Metadata,
	})
}

func (h *skillsHandler) updateSkill(w http.ResponseWriter, r *http.Request) {
	name, ok := parseSkillName(w, r)
	if !ok {
		return
	}
	var req updateSkillRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "bad_request")
		return
	}
	content := strings.TrimSpace(req.SkillMD)
	if content == "" {
		writeError(w, http.StatusBadRequest, "skill_md is required", "missing_field")
		return
	}
	meta, validationErrors := skillset.ValidateSkillMD(name, content)
	if len(validationErrors) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":             "invalid skill_md",
			"code":              "invalid_skill_md",
			"validation_errors": validationErrors,
			"metadata":          meta,
		})
		return
	}

	root, err := h.skillsRoot()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "skills_root_error")
		return
	}
	dir := filepath.Join(root, name)
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		if errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusNotFound, "skill not found", "not_found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error(), "stat_error")
			return
		}
		writeError(w, http.StatusBadRequest, "skill path is not a directory", "invalid_skill")
		return
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content+"\n"), 0o644); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "write_error")
		return
	}
	profileRefs, err := h.skillProfileRefs(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "profile_refs_error")
		return
	}
	writeJSON(w, http.StatusOK, skillInfo{
		Name:          name,
		HasSkillMD:    true,
		Valid:         true,
		Metadata:      meta,
		ProfilesUsing: profileRefs[name],
	})
}

func (h *skillsHandler) deleteSkill(w http.ResponseWriter, r *http.Request) {
	name, ok := parseSkillName(w, r)
	if !ok {
		return
	}
	root, err := h.skillsRoot()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "skills_root_error")
		return
	}
	profileRefs, err := h.skillProfileRefs(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "profile_refs_error")
		return
	}
	if refs := profileRefs[name]; len(refs) > 0 {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":          "skill is referenced by one or more profiles",
			"code":           "skill_in_use",
			"profiles_using": refs,
		})
		return
	}
	dir := filepath.Join(root, name)
	if _, err := os.Stat(dir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error(), "stat_error")
		return
	}
	if err := os.RemoveAll(dir); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "remove_error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func parseSkillName(w http.ResponseWriter, r *http.Request) (string, bool) {
	name := strings.TrimSpace(chi.URLParam(r, "skillName"))
	if name == "" || !skillset.IsValidName(name) {
		writeError(w, http.StatusBadRequest, "invalid skill name", "invalid_name")
		return "", false
	}
	return name, true
}

func (h *skillsHandler) skillProfileRefs(r *http.Request) (map[string][]string, error) {
	out := map[string][]string{}
	if h.registry == nil {
		return out, nil
	}

	profiles, err := h.registry.ListProfiles(r.Context())
	if err != nil {
		return nil, err
	}
	for _, profile := range profiles {
		if profile == nil {
			continue
		}
		for _, raw := range profile.Skills {
			name := strings.TrimSpace(raw)
			if name == "" {
				continue
			}
			out[name] = append(out[name], profile.ID)
		}
	}
	for name := range out {
		slices := out[name]
		if len(slices) < 2 {
			continue
		}
		sort.Strings(slices)
		out[name] = slices
	}
	return out, nil
}
