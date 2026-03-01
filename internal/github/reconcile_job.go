package github

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/user/ai-workflow/internal/core"
)

const defaultReconcileInterval = 10 * time.Minute

type reconcileStatusSyncer interface {
	RepairTask(ctx context.Context, item *core.TaskItem, allItems []core.TaskItem) error
}

// ReconcileJob repairs task status drift and recovers missed webhook side effects periodically.
type ReconcileJob struct {
	Store                core.Store
	Syncer               reconcileStatusSyncer
	RecoverMissedWebhook func(context.Context) error
	Interval             time.Duration
	Now                  func() time.Time

	mu      sync.Mutex
	lastRun time.Time
}

func NewReconcileJob(store core.Store, syncer *StatusSyncer) *ReconcileJob {
	return &ReconcileJob{
		Store:    store,
		Syncer:   syncer,
		Interval: defaultReconcileInterval,
		Now:      time.Now,
	}
}

// RunIfDue executes reconcile work once interval is reached.
func (j *ReconcileJob) RunIfDue(ctx context.Context) (bool, error) {
	if j == nil {
		return false, nil
	}
	now := j.now()

	j.mu.Lock()
	lastRun := j.lastRun
	interval := j.effectiveInterval()
	if !lastRun.IsZero() && now.Sub(lastRun) < interval {
		j.mu.Unlock()
		return false, nil
	}
	j.lastRun = now
	j.mu.Unlock()

	if err := j.RunOnce(ctx); err != nil {
		return false, err
	}
	return true, nil
}

// RunOnce scans active task plans and repairs final-state drift.
func (j *ReconcileJob) RunOnce(ctx context.Context) error {
	if j == nil {
		return nil
	}
	if j.Store == nil {
		return errors.New("reconcile job store is required")
	}

	plans, err := j.Store.GetActiveTaskPlans()
	if err != nil {
		return err
	}

	for _, plan := range plans {
		tasks, taskErr := j.Store.GetTaskItemsByPlan(plan.ID)
		if taskErr != nil {
			return taskErr
		}
		byID := make(map[string]core.TaskItem, len(tasks))
		for _, task := range tasks {
			byID[task.ID] = task
		}

		for i := range tasks {
			task := tasks[i]
			if task.Status == core.ItemBlockedByFailure && dependenciesSatisfied(task, byID) {
				task.Status = core.ItemReady
				task.UpdatedAt = j.now()
				if saveErr := j.Store.SaveTaskItem(&task); saveErr != nil {
					return saveErr
				}
				tasks[i] = task
				byID[task.ID] = task
			}
		}

		if j.Syncer != nil {
			for i := range tasks {
				task := tasks[i]
				if syncErr := j.Syncer.RepairTask(ctx, &task, tasks); syncErr != nil {
					return syncErr
				}
			}
		}
	}

	if j.RecoverMissedWebhook != nil {
		if err := j.RecoverMissedWebhook(ctx); err != nil {
			return err
		}
	}
	return nil
}

func dependenciesSatisfied(task core.TaskItem, byID map[string]core.TaskItem) bool {
	if len(task.DependsOn) == 0 {
		return true
	}
	for _, depID := range task.DependsOn {
		dep, ok := byID[depID]
		if !ok {
			return false
		}
		if dep.Status != core.ItemDone && dep.Status != core.ItemSkipped {
			return false
		}
	}
	return true
}

func (j *ReconcileJob) now() time.Time {
	if j.Now != nil {
		return j.Now()
	}
	return time.Now()
}

func (j *ReconcileJob) effectiveInterval() time.Duration {
	if j.Interval > 0 {
		return j.Interval
	}
	return defaultReconcileInterval
}
