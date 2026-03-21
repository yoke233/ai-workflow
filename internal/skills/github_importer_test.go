package skills

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	workspaceclone "github.com/yoke233/zhanggui/internal/adapters/workspace/clone"
)

func TestGitHubImporterImportSuccess(t *testing.T) {
	repoDir := t.TempDir()
	skillName := "vercel-react-best-practices"
	writeImportedSkillFile(t, repoDir, "skills", skillName, "SKILL.md", DefaultSkillMD(skillName))
	writeImportedSkillFile(t, repoDir, "skills", skillName, "references/notes.md", "# notes\n")

	skillsRoot := filepath.Join(t.TempDir(), "skills")
	importer := NewGitHubImporter(&fakeRepoCloner{repoPath: repoDir})

	imported, err := importer.Import(context.Background(), skillsRoot, GitHubImportRequest{
		RepoURL:   "https://github.com/vercel-labs/agent-skills",
		SkillName: skillName,
	})
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if imported == nil || !imported.Valid {
		t.Fatalf("expected valid imported skill, got %+v", imported)
	}
	if imported.Metadata == nil || imported.Metadata.Name != skillName {
		t.Fatalf("unexpected metadata: %+v", imported.Metadata)
	}
	if _, err := os.Stat(filepath.Join(skillsRoot, skillName, "references", "notes.md")); err != nil {
		t.Fatalf("expected copied companion file, got %v", err)
	}
}

func TestGitHubImporterImportMissingSkillDir(t *testing.T) {
	repoDir := t.TempDir()
	writeImportedSkillFile(t, repoDir, "skills", "existing-skill", "SKILL.md", DefaultSkillMD("existing-skill"))

	importer := NewGitHubImporter(&fakeRepoCloner{repoPath: repoDir})
	_, err := importer.Import(context.Background(), filepath.Join(t.TempDir(), "skills"), GitHubImportRequest{
		RepoURL:   "https://github.com/vercel-labs/agent-skills",
		SkillName: "missing-skill",
	})
	if !errors.Is(err, ErrGitHubSkillNotFound) {
		t.Fatalf("expected ErrGitHubSkillNotFound, got %v", err)
	}
}

func TestGitHubImporterImportRejectsInvalidSkillMD(t *testing.T) {
	repoDir := t.TempDir()
	skillName := "broken-skill"
	writeImportedSkillFile(t, repoDir, "skills", skillName, "SKILL.md", "# invalid\n")

	skillsRoot := filepath.Join(t.TempDir(), "skills")
	importer := NewGitHubImporter(&fakeRepoCloner{repoPath: repoDir})
	_, err := importer.Import(context.Background(), skillsRoot, GitHubImportRequest{
		RepoURL:   "https://github.com/vercel-labs/agent-skills",
		SkillName: skillName,
	})
	if !errors.Is(err, ErrInvalidImportedSkill) {
		t.Fatalf("expected ErrInvalidImportedSkill, got %v", err)
	}
	var validationErr *RepoSkillValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected RepoSkillValidationError, got %T", err)
	}
	if len(validationErr.ValidationErrors) == 0 {
		t.Fatalf("expected validation errors, got %+v", validationErr)
	}
	if _, statErr := os.Stat(filepath.Join(skillsRoot, skillName)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected no installed skill on validation failure, got %v", statErr)
	}
}

func TestGitHubImporterImportRejectsExistingSkill(t *testing.T) {
	repoDir := t.TempDir()
	skillName := "strict-review"
	writeImportedSkillFile(t, repoDir, "skills", skillName, "SKILL.md", DefaultSkillMD(skillName))

	skillsRoot := filepath.Join(t.TempDir(), "skills")
	if err := os.MkdirAll(filepath.Join(skillsRoot, skillName), 0o755); err != nil {
		t.Fatalf("mkdir existing skill: %v", err)
	}

	importer := NewGitHubImporter(&fakeRepoCloner{repoPath: repoDir})
	_, err := importer.Import(context.Background(), skillsRoot, GitHubImportRequest{
		RepoURL:   "https://github.com/vercel-labs/agent-skills",
		SkillName: skillName,
	})
	if !errors.Is(err, ErrSkillAlreadyExists) {
		t.Fatalf("expected ErrSkillAlreadyExists, got %v", err)
	}
}

func TestGitHubImporterImportSuccessFromRepoRoot(t *testing.T) {
	repoDir := t.TempDir()
	skillName := "office-hours"
	writeImportedSkillFile(t, repoDir, "", skillName, "SKILL.md", DefaultSkillMD(skillName))
	writeImportedSkillFile(t, repoDir, "", skillName, "templates/example.md", "# example\n")

	skillsRoot := filepath.Join(t.TempDir(), "skills")
	importer := NewGitHubImporter(&fakeRepoCloner{repoPath: repoDir})

	imported, err := importer.Import(context.Background(), skillsRoot, GitHubImportRequest{
		RepoURL:   "https://github.com/garrytan/gstack",
		SkillName: skillName,
	})
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if imported == nil || !imported.Valid {
		t.Fatalf("expected valid imported skill, got %+v", imported)
	}
	if _, err := os.Stat(filepath.Join(skillsRoot, skillName, "templates", "example.md")); err != nil {
		t.Fatalf("expected copied top-level skill companion file, got %v", err)
	}
}

func TestGitHubImporterImportCopiesSymlinkTargetAsRegularFile(t *testing.T) {
	repoDir := t.TempDir()
	skillName := "linked-skill"
	writeImportedSkillFile(t, repoDir, "skills", skillName, "SKILL.md", DefaultSkillMD(skillName))
	writeImportedSkillFile(t, repoDir, "skills", skillName, "references/notes.md", "# notes\n")
	symlinkPath := filepath.Join(repoDir, "skills", skillName, "references", "notes-link.md")
	target := filepath.Join(repoDir, "skills", skillName, "references", "notes.md")
	if err := os.Symlink(target, symlinkPath); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlink unavailable on this Windows host: %v", err)
		}
		t.Fatalf("create symlink: %v", err)
	}

	skillsRoot := filepath.Join(t.TempDir(), "skills")
	importer := NewGitHubImporter(&fakeRepoCloner{repoPath: repoDir})

	_, err := importer.Import(context.Background(), skillsRoot, GitHubImportRequest{
		RepoURL:   "https://github.com/vercel-labs/agent-skills",
		SkillName: skillName,
	})
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	installedPath := filepath.Join(skillsRoot, skillName, "references", "notes-link.md")
	info, err := os.Lstat(installedPath)
	if err != nil {
		t.Fatalf("stat installed symlink copy: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("expected regular file copy, got symlink mode %v", info.Mode())
	}
	content, err := os.ReadFile(installedPath)
	if err != nil {
		t.Fatalf("read installed symlink copy: %v", err)
	}
	if string(content) != "# notes\n" {
		t.Fatalf("unexpected copied content %q", string(content))
	}
}

func TestGitHubImporterImportRejectsEscapingSymlink(t *testing.T) {
	repoDir := t.TempDir()
	skillName := "escaping-skill"
	writeImportedSkillFile(t, repoDir, "skills", skillName, "SKILL.md", DefaultSkillMD(skillName))
	outside := filepath.Join(t.TempDir(), "outside.md")
	if err := os.WriteFile(outside, []byte("# outside\n"), 0o644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	symlinkPath := filepath.Join(repoDir, "skills", skillName, "outside-link.md")
	if err := os.Symlink(outside, symlinkPath); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlink unavailable on this Windows host: %v", err)
		}
		t.Fatalf("create escaping symlink: %v", err)
	}

	importer := NewGitHubImporter(&fakeRepoCloner{repoPath: repoDir})
	_, err := importer.Import(context.Background(), filepath.Join(t.TempDir(), "skills"), GitHubImportRequest{
		RepoURL:   "https://github.com/vercel-labs/agent-skills",
		SkillName: skillName,
	})
	if err == nil {
		t.Fatal("expected escaping symlink to be rejected")
	}
}

type fakeRepoCloner struct {
	repoPath string
	err      error
}

func (f *fakeRepoCloner) Clone(_ context.Context, req workspaceclone.CloneRequest) (workspaceclone.CloneResult, error) {
	if f.err != nil {
		return workspaceclone.CloneResult{}, f.err
	}
	if err := copyDir(f.repoPath, req.TargetPath); err != nil {
		return workspaceclone.CloneResult{}, err
	}
	return workspaceclone.CloneResult{RepoPath: req.TargetPath, Host: "github.com", Owner: "vercel-labs", Repo: "agent-skills"}, nil
}

func writeImportedSkillFile(t *testing.T, repoDir, skillsSubdir, skillName, relativePath, content string) {
	t.Helper()
	parts := []string{repoDir}
	if skillsSubdir != "" {
		parts = append(parts, skillsSubdir)
	}
	parts = append(parts, skillName, filepath.FromSlash(relativePath))
	fullPath := filepath.Join(parts...)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("mkdir imported skill file: %v", err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write imported skill file: %v", err)
	}
}
