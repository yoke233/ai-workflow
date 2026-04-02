package ceoapp

import (
	"context"
	"fmt"
	"strings"

	"github.com/yoke233/zhanggui/internal/application/orchestrateapp"
	"github.com/yoke233/zhanggui/internal/application/requirementapp"
	"github.com/yoke233/zhanggui/internal/core"
)

type Service struct {
	requirements RequirementService
	tasks        TaskService
}

func New(cfg Config) *Service {
	return &Service{
		requirements: cfg.Requirements,
		tasks:        cfg.Tasks,
	}
}

func (s *Service) Submit(ctx context.Context, input SubmitInput) (*SubmitResult, error) {
	description := strings.TrimSpace(input.Description)
	if description == "" {
		return nil, fmt.Errorf("description is required")
	}
	if s == nil || s.requirements == nil {
		return nil, fmt.Errorf("requirement service is not configured")
	}
	ownerID := firstNonEmpty(input.OwnerID, "human")

	analysisResult, err := s.requirements.Analyze(ctx, requirementapp.AnalyzeInput{
		Description: description,
		Context:     strings.TrimSpace(input.Context),
	})
	if err != nil {
		return nil, err
	}
	analysis := analysisResult.Analysis

	if shouldUseDiscussionThread(analysis) {
		threadResult, err := s.requirements.CreateThread(ctx, requirementapp.CreateThreadInput{
			Description:  description,
			Context:      strings.TrimSpace(input.Context),
			OwnerID:      ownerID,
			Analysis:     &analysis,
			ThreadConfig: analysisResult.SuggestedThread,
		})
		if err != nil {
			return nil, err
		}

		suggestedThread := analysisResult.SuggestedThread
		return &SubmitResult{
			Mode:            ModeDiscussion,
			Summary:         firstNonEmpty(strings.TrimSpace(analysis.Summary), "created discussion thread"),
			Status:          "discussion_started",
			NextStep:        "discussion_in_thread",
			Analysis:        &analysis,
			SuggestedThread: &suggestedThread,
			Thread:          threadResult.Thread,
			ContextRefs:     cloneThreadContextRefs(threadResult.ContextRefs),
			AgentIDs:        cloneStrings(threadResult.AgentIDs),
		}, nil
	}

	if s.tasks == nil {
		return nil, fmt.Errorf("task service is not configured")
	}
	if len(analysis.MatchedProjects) != 1 {
		return nil, fmt.Errorf("direct execution requires exactly one matched project")
	}
	projectID := analysis.MatchedProjects[0].ProjectID
	workItemTitle := deriveWorkItemTitle(analysis.Summary, description)

	created, err := s.tasks.CreateTask(ctx, orchestrateapp.CreateTaskInput{
		Title:     workItemTitle,
		Body:      buildWorkItemBody(description, strings.TrimSpace(input.Context), analysis),
		ProjectID: &projectID,
		Priority:  "medium",
	})
	if err != nil {
		return nil, err
	}
	decomposed, err := s.tasks.DecomposeTask(ctx, orchestrateapp.DecomposeTaskInput{
		WorkItemID: created.WorkItem.ID,
		Objective:  description,
	})
	if err != nil {
		return nil, err
	}

	return &SubmitResult{
		Mode:        ModeDirectExecution,
		Summary:     firstNonEmpty(strings.TrimSpace(analysis.Summary), "created executable work item"),
		Status:      "direct_ready",
		NextStep:    "run_work_item",
		Analysis:    &analysis,
		WorkItemID:  created.WorkItem.ID,
		ActionCount: decomposed.ActionCount,
	}, nil
}

func shouldUseDiscussionThread(analysis requirementapp.AnalysisResult) bool {
	if len(analysis.MatchedProjects) != 1 {
		return true
	}
	if strings.TrimSpace(analysis.Complexity) == "high" {
		return true
	}
	switch strings.TrimSpace(analysis.SuggestedMeetingMode) {
	case "", "direct":
		return false
	default:
		return true
	}
}

func deriveWorkItemTitle(summary string, description string) string {
	title := strings.TrimSpace(summary)
	if title == "" {
		title = strings.TrimSpace(description)
	}
	if idx := strings.IndexAny(title, "\r\n"); idx >= 0 {
		title = strings.TrimSpace(title[:idx])
	}
	if len(title) > 80 {
		title = strings.TrimSpace(title[:80])
	}
	if title == "" {
		return "CEO submitted task"
	}
	return title
}

func buildWorkItemBody(description string, context string, analysis requirementapp.AnalysisResult) string {
	var b strings.Builder
	b.WriteString(description)
	if context != "" {
		b.WriteString("\n\nContext:\n")
		b.WriteString(context)
	}
	if len(analysis.Risks) > 0 {
		b.WriteString("\n\nRisks:\n")
		for _, risk := range analysis.Risks {
			risk = strings.TrimSpace(risk)
			if risk == "" {
				continue
			}
			b.WriteString("- ")
			b.WriteString(risk)
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String())
}

func cloneStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func cloneThreadContextRefs(in []*core.ThreadContextRef) []*core.ThreadContextRef {
	if len(in) == 0 {
		return nil
	}
	out := make([]*core.ThreadContextRef, 0, len(in))
	for _, item := range in {
		if item == nil {
			continue
		}
		copied := *item
		out = append(out, &copied)
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
