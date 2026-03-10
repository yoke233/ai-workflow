package engine

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/yoke233/ai-workflow/internal/git"
	"github.com/yoke233/ai-workflow/internal/v2/core"
)

// LocalGitProvider creates git worktree-based workspaces for dev projects.
// Each flow gets an isolated worktree with a dedicated branch.
type LocalGitProvider struct{}

func (p *LocalGitProvider) Prepare(_ context.Context, _ *core.Project, bindings []*core.ResourceBinding, flowID int64) (*core.Workspace, error) {
	for _, b := range bindings {
		if b.Kind != "git" {
			continue
		}
		repoPath := b.URI
		branchName := fmt.Sprintf("ai-flow/v2-%d", flowID)
		worktreePath := filepath.Join(repoPath, ".worktrees", fmt.Sprintf("flow-%d", flowID))

		runner := git.NewRunner(repoPath)
		if err := runner.WorktreeAdd(worktreePath, branchName); err != nil {
			return nil, fmt.Errorf("create worktree for flow %d: %w", flowID, err)
		}

		defaultBranch := defaultBranchFromBinding(b)
		if defaultBranch == "" {
			defaultBranch = git.DetectDefaultBranch(repoPath)
		}

		metadata := map[string]any{
			"binding_id":     b.ID,
			"kind":           "git",
			"branch":         branchName,
			"default_branch": defaultBranch,
			"repo_path":      repoPath,
		}
		mergeSCMBindingMetadata(metadata, b.Config)

		return &core.Workspace{
			Path:     worktreePath,
			Metadata: metadata,
		}, nil
	}
	return nil, fmt.Errorf("no git resource binding found")
}

func (p *LocalGitProvider) Release(_ context.Context, ws *core.Workspace) error {
	if ws == nil || ws.Metadata == nil {
		return nil
	}
	repoPath, _ := ws.Metadata["repo_path"].(string)
	if repoPath == "" {
		return nil
	}
	runner := git.NewRunner(repoPath)
	return runner.WorktreeRemove(ws.Path)
}

func defaultBranchFromBinding(b *core.ResourceBinding) string {
	if b == nil || b.Config == nil {
		return "main"
	}
	for _, key := range []string{"base_branch", "default_branch"} {
		if v, ok := b.Config[key].(string); ok && v != "" {
			return v
		}
	}
	return "main"
}

func mergeSCMBindingMetadata(dst map[string]any, cfg map[string]any) {
	if dst == nil || cfg == nil {
		return
	}
	for _, key := range []string{
		"provider",
		"default_branch",
		"base_branch",
		"organization_id",
		"repository_id",
		"project_id",
		"source_project_id",
		"target_project_id",
		"reviewer_user_ids",
		"trigger_ai_review_run",
		"work_item_ids",
		"remove_source_branch",
		"merge_method",
	} {
		if value, ok := cfg[key]; ok {
			dst[key] = value
		}
	}
}
