package orchestrateapp

import (
	"context"

	"github.com/yoke233/zhanggui/internal/application/planning"
	"github.com/yoke233/zhanggui/internal/application/workitemapp"
	"github.com/yoke233/zhanggui/internal/core"
)

type WorkItemCreator interface {
	CreateWorkItem(ctx context.Context, input workitemapp.CreateWorkItemInput) (*core.WorkItem, error)
}

type Store interface {
	core.WorkItemStore
	core.ActionStore
	core.RunStore
}

type Planner interface {
	Generate(ctx context.Context, input planning.GenerateInput) (*planning.GeneratedDAG, error)
}

type Config struct {
	Store           Store
	WorkItemCreator WorkItemCreator
	Planner         Planner
}
