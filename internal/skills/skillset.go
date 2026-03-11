package skills

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/yoke233/ai-workflow/internal/core"
	"gopkg.in/yaml.v3"
)

var skillNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

type Metadata struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description" yaml:"description"`
	AssignWhen  string `json:"assign_when" yaml:"assign_when"`
	Version     int    `json:"version" yaml:"version"`
}

type ParsedSkill struct {
	Name             string    `json:"name"`
	SkillMD          string    `json:"skill_md,omitempty"`
	HasSkillMD       bool      `json:"has_skill_md"`
	Valid            bool      `json:"valid"`
	Metadata         *Metadata `json:"metadata,omitempty"`
	ValidationErrors []string  `json:"validation_errors,omitempty"`
}

type SkillIssue struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

type InvalidSkillsError struct {
	Issues []SkillIssue
}

func (e *InvalidSkillsError) Error() string {
	if e == nil || len(e.Issues) == 0 {
		return core.ErrInvalidSkills.Error()
	}
	parts := make([]string, 0, len(e.Issues))
	for _, issue := range e.Issues {
		parts = append(parts, fmt.Sprintf("%s: %s", issue.Name, issue.Reason))
	}
	return fmt.Sprintf("%s: %s", core.ErrInvalidSkills, strings.Join(parts, "; "))
}

func (e *InvalidSkillsError) Unwrap() error {
	return core.ErrInvalidSkills
}

func IsValidName(name string) bool {
	return skillNamePattern.MatchString(strings.TrimSpace(name))
}

func DefaultSkillMD(name string) string {
	return strings.TrimSpace(`
---
name: ` + name + `
description: TODO
assign_when: TODO
version: 1
---

# ` + name + `
`)
}

func InspectSkill(root, name string) (*ParsedSkill, error) {
	cleanName := strings.TrimSpace(name)
	if !IsValidName(cleanName) {
		return nil, fmt.Errorf("invalid skill name %q", name)
	}

	dir := filepath.Join(root, cleanName)
	fi, err := os.Stat(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("stat skill dir %q: %w", cleanName, err)
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("skill %q path is not a directory", cleanName)
	}

	out := &ParsedSkill{Name: cleanName}
	path := filepath.Join(dir, "SKILL.md")
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			out.Valid = false
			out.ValidationErrors = []string{"SKILL.md not found"}
			return out, nil
		}
		return nil, fmt.Errorf("read skill %q: %w", cleanName, err)
	}

	out.HasSkillMD = true
	out.SkillMD = string(b)
	meta, errs := ValidateSkillMD(cleanName, out.SkillMD)
	if meta != nil {
		out.Metadata = meta
	}
	if len(errs) == 0 {
		out.Valid = true
		return out, nil
	}
	out.ValidationErrors = errs
	return out, nil
}

func ListSkills(root string) ([]ParsedSkill, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir skills root: %w", err)
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read skills root: %w", err)
	}

	out := make([]ParsedSkill, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := strings.TrimSpace(entry.Name())
		if !IsValidName(name) {
			continue
		}
		skill, inspectErr := InspectSkill(root, name)
		if inspectErr != nil {
			return nil, inspectErr
		}
		out = append(out, *skill)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func ValidateSkillMD(dirName, content string) (*Metadata, []string) {
	meta, errs := parseSkillMD(dirName, content)
	if len(errs) > 0 {
		return meta, errs
	}
	return meta, nil
}

func ValidateProfileSkills(skillNames []string) error {
	root, err := ResolveSkillsRoot()
	if err != nil {
		return err
	}
	return ValidateProfileSkillsFromRoot(root, skillNames)
}

func ValidateProfileSkillsFromRoot(root string, skillNames []string) error {
	if len(skillNames) == 0 {
		return nil
	}

	issues := make([]SkillIssue, 0)
	seen := make(map[string]struct{}, len(skillNames))
	for _, raw := range skillNames {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}

		if !IsValidName(name) {
			issues = append(issues, SkillIssue{Name: name, Reason: "invalid skill name"})
			continue
		}

		skill, err := InspectSkill(root, name)
		if errors.Is(err, os.ErrNotExist) {
			issues = append(issues, SkillIssue{Name: name, Reason: "skill not found"})
			continue
		}
		if err != nil {
			issues = append(issues, SkillIssue{Name: name, Reason: err.Error()})
			continue
		}
		if !skill.Valid {
			reason := "invalid SKILL.md"
			if len(skill.ValidationErrors) > 0 {
				reason = strings.Join(skill.ValidationErrors, "; ")
			}
			issues = append(issues, SkillIssue{Name: name, Reason: reason})
		}
	}

	if len(issues) == 0 {
		return nil
	}
	return &InvalidSkillsError{Issues: issues}
}

func parseSkillMD(dirName, content string) (*Metadata, []string) {
	meta := &Metadata{}
	errorsList := make([]string, 0)

	frontmatter, ok := extractFrontmatter(content)
	if !ok {
		return meta, []string{"missing YAML frontmatter"}
	}

	if err := yaml.Unmarshal([]byte(frontmatter), meta); err != nil {
		return meta, []string{fmt.Sprintf("invalid YAML frontmatter: %v", err)}
	}

	name := strings.TrimSpace(meta.Name)
	if name == "" {
		errorsList = append(errorsList, "frontmatter.name is required")
	} else if name != dirName {
		errorsList = append(errorsList, "frontmatter.name must match directory name")
	}

	if strings.TrimSpace(meta.Description) == "" {
		errorsList = append(errorsList, "frontmatter.description is required")
	}
	if strings.TrimSpace(meta.AssignWhen) == "" {
		errorsList = append(errorsList, "frontmatter.assign_when is required")
	}
	if meta.Version < 1 {
		errorsList = append(errorsList, "frontmatter.version must be >= 1")
	}

	return meta, errorsList
}

func extractFrontmatter(content string) (string, bool) {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return "", false
	}

	rest := normalized[len("---\n"):]
	end := strings.Index(rest, "\n---\n")
	if end >= 0 {
		return rest[:end], true
	}
	if strings.HasSuffix(rest, "\n---") {
		return strings.TrimSuffix(rest, "\n---"), true
	}
	return "", false
}
