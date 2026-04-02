package ceoapp

import (
	"context"

	"github.com/yoke233/zhanggui/internal/application/orchestrateapp"
	"github.com/yoke233/zhanggui/internal/application/requirementapp"
	"github.com/yoke233/zhanggui/internal/core"
)

type RequirementService interface {
	Analyze(ctx context.Context, input requirementapp.AnalyzeInput) (*requirementapp.AnalyzeResult, error)
	CreateThread(ctx context.Context, input requirementapp.CreateThreadInput) (*requirementapp.CreateThreadResult, error)
}

type TaskService interface {
	CreateTask(ctx context.Context, input orchestrateapp.CreateTaskInput) (*orchestrateapp.CreateTaskResult, error)
	DecomposeTask(ctx context.Context, input orchestrateapp.DecomposeTaskInput) (*orchestrateapp.DecomposeTaskResult, error)
}

type Config struct {
	Requirements RequirementService
	Tasks        TaskService
}

type SubmitInput struct {
	Description string `json:"description"`
	Context     string `json:"context,omitempty"`
	OwnerID     string `json:"owner_id,omitempty"`
}

type SubmitMode string

const (
	ModeDirectExecution SubmitMode = "direct_execution"
	ModeDiscussion      SubmitMode = "discussion_thread"
)

type SubmitResult struct {
	Mode            SubmitMode                      `json:"mode"`
	Summary         string                          `json:"summary"`
	Status          string                          `json:"status"`
	NextStep        string                          `json:"next_step"`
	Analysis        *requirementapp.AnalysisResult  `json:"analysis,omitempty"`
	SuggestedThread *requirementapp.SuggestedThread `json:"suggested_thread,omitempty"`
	WorkItemID      int64                           `json:"work_item_id,omitempty"`
	ActionCount     int                             `json:"action_count,omitempty"`
	Thread          *core.Thread                    `json:"thread,omitempty"`
	ContextRefs     []*core.ThreadContextRef        `json:"context_refs,omitempty"`
	AgentIDs        []string                        `json:"agents,omitempty"`
}
