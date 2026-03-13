package flow

import (
	"context"
	"fmt"
	"time"

	"github.com/yoke233/ai-workflow/internal/core"
)

// manifestCheckEnabled returns true if the gate step has manifest_check: true.
func manifestCheckEnabled(step *core.Step) bool {
	if step.Config == nil {
		return false
	}
	v, ok := step.Config["manifest_check"].(bool)
	return ok && v
}

// checkManifestEntries evaluates the feature manifest for the gate step's issue/project.
// Returns (passed, reason, error).
func (e *IssueEngine) checkManifestEntries(ctx context.Context, step *core.Step) (bool, string, error) {
	issue, err := e.store.GetIssue(ctx, step.IssueID)
	if err != nil || issue == nil || issue.ProjectID == nil {
		return true, "", nil // no project → skip check
	}

	manifest, err := e.store.GetFeatureManifestByProject(ctx, *issue.ProjectID)
	if err != nil {
		return true, "", nil // no manifest → skip check
	}

	// Determine which entries to check.
	filter := core.FeatureEntryFilter{ManifestID: manifest.ID, Limit: 500}

	// If manifest_issue_id is configured, check only entries linked to that issue.
	if issueID, ok := step.Config["manifest_issue_id"].(float64); ok {
		id := int64(issueID)
		filter.IssueID = &id
	}
	// If manifest_required_tags is configured, filter entries by tags.
	if rawTags, ok := step.Config["manifest_required_tags"].([]any); ok {
		for _, t := range rawTags {
			if tag, ok := t.(string); ok {
				filter.Tags = append(filter.Tags, tag)
			}
		}
	}

	entries, err := e.store.ListFeatureEntries(ctx, filter)
	if err != nil {
		return true, "", err
	}
	if len(entries) == 0 {
		return true, "", nil
	}

	// Count by status.
	failCount := 0
	pendingCount := 0
	passCount := 0
	for _, entry := range entries {
		switch entry.Status {
		case core.FeatureFail:
			failCount++
		case core.FeaturePending:
			pendingCount++
		case core.FeaturePass:
			passCount++
		}
	}

	maxFail := 0
	if v, ok := step.Config["manifest_max_fail"].(float64); ok {
		maxFail = int(v)
	}
	maxPending := len(entries) // default: allow all pending
	if v, ok := step.Config["manifest_max_pending"].(float64); ok {
		maxPending = int(v)
	}

	// Publish gate-checked event.
	e.bus.Publish(ctx, core.Event{
		Type:      core.EventManifestGateChecked,
		IssueID:   step.IssueID,
		StepID:    step.ID,
		Timestamp: time.Now().UTC(),
		Data: map[string]any{
			"passed":        failCount <= maxFail && pendingCount <= maxPending,
			"total":         len(entries),
			"pass_count":    passCount,
			"fail_count":    failCount,
			"pending_count": pendingCount,
		},
	})

	if failCount > maxFail {
		return false, fmt.Sprintf("feature manifest: %d entries failed (max allowed: %d)", failCount, maxFail), nil
	}
	if pendingCount > maxPending {
		return false, fmt.Sprintf("feature manifest: %d entries still pending (max allowed: %d)", pendingCount, maxPending), nil
	}
	return true, "", nil
}
