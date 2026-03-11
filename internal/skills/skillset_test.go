package skills

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yoke233/ai-workflow/internal/core"
)

func TestValidateSkillMD(t *testing.T) {
	t.Parallel()

	valid := `---
name: demo-skill
description: demo
assign_when: for demos
version: 1
---

# Demo
`
	meta, errs := ValidateSkillMD("demo-skill", valid)
	if len(errs) != 0 {
		t.Fatalf("expected valid metadata, got errors: %v", errs)
	}
	if meta == nil || meta.Name != "demo-skill" || meta.Version != 1 {
		t.Fatalf("unexpected metadata: %+v", meta)
	}

	_, errs = ValidateSkillMD("demo-skill", "# no frontmatter")
	if len(errs) != 1 || !strings.Contains(errs[0], "missing YAML frontmatter") {
		t.Fatalf("expected missing frontmatter error, got %v", errs)
	}

	_, errs = ValidateSkillMD("demo-skill", `---
name: other
description: demo
assign_when: for demos
version: 0
---
`)
	if len(errs) < 2 {
		t.Fatalf("expected multiple validation errors, got %v", errs)
	}
}

func TestInspectSkillAndListSkills(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	validDir := filepath.Join(root, "valid-skill")
	if err := os.MkdirAll(validDir, 0o755); err != nil {
		t.Fatalf("mkdir valid skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(validDir, "SKILL.md"), []byte(DefaultSkillMD("valid-skill")), 0o644); err != nil {
		t.Fatalf("write valid skill: %v", err)
	}

	invalidDir := filepath.Join(root, "invalid-skill")
	if err := os.MkdirAll(invalidDir, 0o755); err != nil {
		t.Fatalf("mkdir invalid skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(invalidDir, "SKILL.md"), []byte("# invalid"), 0o644); err != nil {
		t.Fatalf("write invalid skill: %v", err)
	}

	missingDir := filepath.Join(root, "missing-skill")
	if err := os.MkdirAll(missingDir, 0o755); err != nil {
		t.Fatalf("mkdir missing skill: %v", err)
	}

	skill, err := InspectSkill(root, "valid-skill")
	if err != nil {
		t.Fatalf("InspectSkill(valid): %v", err)
	}
	if !skill.Valid || !skill.HasSkillMD {
		t.Fatalf("expected valid skill, got %+v", skill)
	}

	skill, err = InspectSkill(root, "invalid-skill")
	if err != nil {
		t.Fatalf("InspectSkill(invalid): %v", err)
	}
	if skill.Valid || len(skill.ValidationErrors) == 0 {
		t.Fatalf("expected invalid skill with errors, got %+v", skill)
	}

	skill, err = InspectSkill(root, "missing-skill")
	if err != nil {
		t.Fatalf("InspectSkill(missing file): %v", err)
	}
	if skill.HasSkillMD || skill.Valid {
		t.Fatalf("expected missing SKILL.md to be invalid, got %+v", skill)
	}

	list, err := ListSkills(root)
	if err != nil {
		t.Fatalf("ListSkills: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 skills, got %d", len(list))
	}
}

func TestValidateProfileSkillsFromRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	validDir := filepath.Join(root, "strict-review")
	if err := os.MkdirAll(validDir, 0o755); err != nil {
		t.Fatalf("mkdir valid skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(validDir, "SKILL.md"), []byte(DefaultSkillMD("strict-review")), 0o644); err != nil {
		t.Fatalf("write valid skill: %v", err)
	}

	invalidDir := filepath.Join(root, "broken-skill")
	if err := os.MkdirAll(invalidDir, 0o755); err != nil {
		t.Fatalf("mkdir invalid skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(invalidDir, "SKILL.md"), []byte("# invalid"), 0o644); err != nil {
		t.Fatalf("write invalid skill: %v", err)
	}

	if err := ValidateProfileSkillsFromRoot(root, []string{"strict-review"}); err != nil {
		t.Fatalf("expected valid profile skills, got %v", err)
	}

	err := ValidateProfileSkillsFromRoot(root, []string{"strict-review", "broken-skill", "missing-skill"})
	if !errors.Is(err, core.ErrInvalidSkills) {
		t.Fatalf("expected ErrInvalidSkills, got %v", err)
	}
	var invalid *InvalidSkillsError
	if !errors.As(err, &invalid) {
		t.Fatalf("expected InvalidSkillsError, got %T", err)
	}
	if len(invalid.Issues) != 2 {
		t.Fatalf("expected 2 issues, got %+v", invalid.Issues)
	}
}
