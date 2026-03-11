package github

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/yoke233/ai-workflow/internal/core"
)

const defaultReconcileInterval = 10 * time.Minute

type reconcileStatusSyncer interface {
	RepairTask(ctx context.Context, issue *core.Issue, allIssues []*core.Issue) error
}

// ReconcileJob repairs issue status drift and recovers missed webhook side effects periodically.
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

// RunOnce scans active issues and repairs final-state drift.
func (j *ReconcileJob) RunOnce(ctx context.Context) error {
	if j == nil {
		return nil
	}
	if j.Store == nil {
		return errors.New("reconcile job store is required")
	}

	activeIssues, err := j.Store.GetActiveIssues("")
	if err != nil {
		return err
	}

	byID := make(map[string]*core.Issue, len(activeIssues))
	allIssues := make([]*core.Issue, 0, len(activeIssues))
	for i := range activeIssues {
		issue := &activeIssues[i]
		byID[issue.ID] = issue
		allIssues = append(allIssues, issue)
	}

	for _, issue := range allIssues {
		if issue.Status != core.IssueStatusQueued {
			continue
		}
		satisfied, depErr := dependenciesSatisfied(issue, byID, j.Store)
		if depErr != nil {
			return depErr
		}
		if !satisfied {
			continue
		}
		issue.Status = core.IssueStatusReady
		issue.UpdatedAt = j.now()
		if saveErr := j.Store.SaveIssue(issue); saveErr != nil {
			return saveErr
		}
	}

	if j.Syncer != nil {
		for _, issue := range allIssues {
			if syncErr := j.Syncer.RepairTask(ctx, issue, allIssues); syncErr != nil {
				return syncErr
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

func dependenciesSatisfied(issue *core.Issue, byID map[string]*core.Issue, store core.Store) (bool, error) {
	if issue == nil || len(issue.DependsOn) == 0 {
		return true, nil
	}
	for _, depID := range issue.DependsOn {
		dep, ok := byID[depID]
		if !ok {
			loaded, err := store.GetIssue(depID)
			if err != nil {
				if strings.Contains(strings.ToLower(err.Error()), "not found") {
					return false, nil
				}
				return false, err
			}
			dep = loaded
			byID[depID] = dep
		}
		if dep == nil {
			return false, nil
		}
		if dep.Status != core.IssueStatusDone &&
			dep.Status != core.IssueStatusSuperseded &&
			dep.Status != core.IssueStatusAbandoned {
			return false, nil
		}
	}
	return true, nil
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
