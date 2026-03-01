package github

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/user/ai-workflow/internal/core"
)

type prLifecycleSCM interface {
	CreatePR(ctx context.Context, req core.PullRequest) (prURL string, err error)
	ConvertToReady(ctx context.Context, number int) error
	MergePR(ctx context.Context, req core.PullRequestMerge) error
}

type PRLifecycle struct {
	store core.Store
	scm   prLifecycleSCM
	now   func() time.Time
}

func NewPRLifecycle(store core.Store, scm prLifecycleSCM) *PRLifecycle {
	return &PRLifecycle{
		store: store,
		scm:   scm,
		now:   time.Now,
	}
}

func (l *PRLifecycle) OnImplementComplete(ctx context.Context, pipelineID string) (string, error) {
	if l == nil || l.store == nil {
		return "", errors.New("pr lifecycle store is required")
	}
	if l.scm == nil {
		return "", errors.New("pr lifecycle scm is required")
	}

	pipeline, err := l.store.GetPipeline(strings.TrimSpace(pipelineID))
	if err != nil {
		return "", err
	}

	if existing := prNumberFromPipeline(pipeline); existing > 0 {
		if pipeline.Config == nil {
			pipeline.Config = map[string]any{}
		}
		if url, _ := pipeline.Config["pr_url"].(string); strings.TrimSpace(url) != "" {
			return strings.TrimSpace(url), nil
		}
	}

	base := "main"
	if pipeline.Config != nil {
		if v, _ := pipeline.Config["base_branch"].(string); strings.TrimSpace(v) != "" {
			base = strings.TrimSpace(v)
		}
	}
	head := strings.TrimSpace(pipeline.BranchName)
	if head == "" {
		head = "ai-flow/" + pipeline.ID
	}

	draft := true
	prURL, err := l.scm.CreatePR(ctx, core.PullRequest{
		Title: pipeline.Name,
		Body:  pipeline.Description,
		Head:  head,
		Base:  base,
		Draft: &draft,
	})
	if err != nil {
		return "", err
	}

	if pipeline.Config == nil {
		pipeline.Config = map[string]any{}
	}
	pipeline.Config["pr_url"] = strings.TrimSpace(prURL)
	if prNumber := parsePRNumber(prURL); prNumber > 0 {
		pipeline.Config["pr_number"] = prNumber
	}
	pipeline.UpdatedAt = l.now()
	if err := l.store.SavePipeline(pipeline); err != nil {
		return "", err
	}
	return strings.TrimSpace(prURL), nil
}

func (l *PRLifecycle) OnMergeApproved(ctx context.Context, pipelineID string) error {
	if l == nil || l.store == nil {
		return errors.New("pr lifecycle store is required")
	}
	if l.scm == nil {
		return errors.New("pr lifecycle scm is required")
	}

	pipeline, err := l.store.GetPipeline(strings.TrimSpace(pipelineID))
	if err != nil {
		return err
	}
	prNumber := prNumberFromPipeline(pipeline)
	if prNumber <= 0 {
		return errors.New("pr number is required")
	}

	if err := l.scm.ConvertToReady(ctx, prNumber); err != nil {
		return err
	}
	return l.scm.MergePR(ctx, core.PullRequestMerge{
		Number:      prNumber,
		CommitTitle: fmt.Sprintf("merge pipeline %s", pipeline.ID),
	})
}

func (l *PRLifecycle) OnPullRequestClosed(
	ctx context.Context,
	projectID string,
	prNumber int,
	merged bool,
) error {
	if l == nil || l.store == nil || strings.TrimSpace(projectID) == "" || prNumber <= 0 {
		return nil
	}

	pipeline, err := findPipelineByPRNumber(l.store, projectID, prNumber)
	if err != nil {
		return err
	}
	if pipeline == nil {
		return nil
	}

	if merged {
		pipeline.Status = core.StatusDone
		pipeline.ErrorMessage = ""
	} else {
		pipeline.Status = core.StatusFailed
		pipeline.ErrorMessage = "pull request closed without merge"
	}
	pipeline.FinishedAt = l.now()
	pipeline.UpdatedAt = l.now()
	return l.store.SavePipeline(pipeline)
}

func findPipelineByPRNumber(store core.Store, projectID string, prNumber int) (*core.Pipeline, error) {
	if store == nil || strings.TrimSpace(projectID) == "" || prNumber <= 0 {
		return nil, nil
	}

	pipelines, err := store.ListPipelines(projectID, core.PipelineFilter{Limit: 500})
	if err != nil {
		return nil, err
	}
	for i := range pipelines {
		pipeline, err := store.GetPipeline(pipelines[i].ID)
		if err != nil {
			return nil, err
		}
		if prNumberFromPipeline(pipeline) == prNumber {
			return pipeline, nil
		}
	}
	return nil, nil
}

func prNumberFromPipeline(p *core.Pipeline) int {
	if p == nil {
		return 0
	}
	if p.Config != nil {
		for _, key := range []string{"pr_number", "github_pr_number"} {
			if prNumber := parsePRNumberValue(p.Config[key]); prNumber > 0 {
				return prNumber
			}
		}
	}
	if p.Artifacts != nil {
		for _, key := range []string{"pr_number", "github_pr_number"} {
			if prNumber := parsePRNumberValue(p.Artifacts[key]); prNumber > 0 {
				return prNumber
			}
		}
	}
	return 0
}

func parsePRNumberValue(raw any) int {
	switch value := raw.(type) {
	case int:
		if value > 0 {
			return value
		}
	case int32:
		if value > 0 {
			return int(value)
		}
	case int64:
		if value > 0 {
			return int(value)
		}
	case float64:
		if value > 0 {
			return int(value)
		}
	case string:
		if number := parsePRNumber(strings.TrimSpace(value)); number > 0 {
			return number
		}
	}
	return 0
}

func parsePRNumber(prURL string) int {
	trimmed := strings.TrimSpace(prURL)
	if trimmed == "" {
		return 0
	}

	parts := strings.Split(trimmed, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		part := strings.TrimSpace(parts[i])
		if part == "" {
			continue
		}
		if n, err := strconv.Atoi(part); err == nil && n > 0 {
			return n
		}
	}
	return 0
}
