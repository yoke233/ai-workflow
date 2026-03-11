package skills

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	workspaceclone "github.com/yoke233/ai-workflow/internal/adapters/workspace/clone"
)

var (
	ErrInvalidGitHubRepoURL = errors.New("invalid github repo url")
	ErrUnsupportedGitHost   = errors.New("unsupported git host")
	ErrGitHubSkillNotFound  = errors.New("skill not found in github repository")
	ErrSkillAlreadyExists   = errors.New("skill already exists")
	ErrInvalidImportedSkill = errors.New("imported skill is invalid")
)

type GitHubImportRequest struct {
	RepoURL   string
	SkillName string
	Ref       string
}

type GitHubImporter interface {
	Import(ctx context.Context, skillsRoot string, req GitHubImportRequest) (*ParsedSkill, error)
}

type repoCloner interface {
	Clone(ctx context.Context, req workspaceclone.CloneRequest) (workspaceclone.CloneResult, error)
}

type RepoSkillValidationError struct {
	Name             string
	Metadata         *Metadata
	ValidationErrors []string
}

func (e *RepoSkillValidationError) Error() string {
	if e == nil {
		return ErrInvalidImportedSkill.Error()
	}
	if len(e.ValidationErrors) == 0 {
		return fmt.Sprintf("%s: %s", ErrInvalidImportedSkill, e.Name)
	}
	return fmt.Sprintf("%s: %s (%s)", ErrInvalidImportedSkill, e.Name, strings.Join(e.ValidationErrors, "; "))
}

func (e *RepoSkillValidationError) Unwrap() error {
	return ErrInvalidImportedSkill
}

type githubImporter struct {
	cloner repoCloner
}

func NewGitHubImporter(cloner repoCloner) GitHubImporter {
	if cloner == nil {
		cloner = workspaceclone.New()
	}
	return &githubImporter{cloner: cloner}
}

func (i *githubImporter) Import(ctx context.Context, skillsRoot string, req GitHubImportRequest) (*ParsedSkill, error) {
	skillsRoot = strings.TrimSpace(skillsRoot)
	if skillsRoot == "" {
		return nil, fmt.Errorf("skills root is empty")
	}
	skillsRoot = filepath.Clean(skillsRoot)

	skillName := strings.TrimSpace(req.SkillName)
	if !IsValidName(skillName) {
		return nil, fmt.Errorf("invalid skill name %q", req.SkillName)
	}

	repoURL := strings.TrimSpace(req.RepoURL)
	meta, err := workspaceclone.ParseRemoteURL(repoURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidGitHubRepoURL, err)
	}
	if meta.Host != "github.com" {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedGitHost, meta.Host)
	}

	dstDir := filepath.Join(skillsRoot, skillName)
	if _, err := os.Stat(dstDir); err == nil {
		return nil, fmt.Errorf("%w: %s", ErrSkillAlreadyExists, skillName)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("stat destination skill: %w", err)
	}

	cloneRoot, err := os.MkdirTemp("", "ai-workflow-skill-import-*")
	if err != nil {
		return nil, fmt.Errorf("create import temp dir: %w", err)
	}
	defer os.RemoveAll(cloneRoot)

	repoPath := filepath.Join(cloneRoot, "repo")
	if _, err := i.cloner.Clone(ctx, workspaceclone.CloneRequest{
		RemoteURL:  repoURL,
		TargetPath: repoPath,
		Ref:        strings.TrimSpace(req.Ref),
	}); err != nil {
		return nil, fmt.Errorf("clone repository: %w", err)
	}

	repoSkillsRoot := filepath.Join(repoPath, "skills")
	imported, err := InspectSkill(repoSkillsRoot, skillName)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s", ErrGitHubSkillNotFound, skillName)
	}
	if err != nil {
		return nil, fmt.Errorf("inspect imported skill: %w", err)
	}
	if !imported.HasSkillMD || !imported.Valid {
		return nil, &RepoSkillValidationError{
			Name:             skillName,
			Metadata:         imported.Metadata,
			ValidationErrors: append([]string(nil), imported.ValidationErrors...),
		}
	}

	if err := os.MkdirAll(skillsRoot, 0o755); err != nil {
		return nil, fmt.Errorf("create skills root: %w", err)
	}

	stagingRoot, err := os.MkdirTemp(skillsRoot, "."+skillName+".import-*")
	if err != nil {
		return nil, fmt.Errorf("create staging dir: %w", err)
	}
	defer os.RemoveAll(stagingRoot)

	stagedSkillDir := filepath.Join(stagingRoot, skillName)
	if err := copyDir(filepath.Join(repoSkillsRoot, skillName), stagedSkillDir); err != nil {
		return nil, fmt.Errorf("copy imported skill: %w", err)
	}
	if err := os.Rename(stagedSkillDir, dstDir); err != nil {
		if _, statErr := os.Stat(dstDir); statErr == nil {
			return nil, fmt.Errorf("%w: %s", ErrSkillAlreadyExists, skillName)
		}
		return nil, fmt.Errorf("install imported skill: %w", err)
	}

	installed, err := InspectSkill(skillsRoot, skillName)
	if err != nil {
		return nil, fmt.Errorf("inspect installed skill: %w", err)
	}
	return installed, nil
}

func copyDir(srcDir, dstDir string) error {
	return filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		targetPath := dstDir
		if rel != "." {
			targetPath = filepath.Join(dstDir, rel)
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		mode := info.Mode()

		if d.IsDir() {
			return os.MkdirAll(targetPath, mode.Perm())
		}
		if mode&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(linkTarget, targetPath)
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		return copyFile(path, targetPath, mode)
	})
}

func copyFile(srcPath, dstPath string, mode fs.FileMode) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode.Perm())
	if err != nil {
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return err
	}
	return nil
}
