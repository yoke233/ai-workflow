package teamleader

import (
	"fmt"

	"github.com/yoke233/ai-workflow/internal/core"
)

func transitionIssueStatus(issue *core.Issue, to core.IssueStatus) error {
	if issue == nil {
		return fmt.Errorf("issue is required")
	}
	if err := to.Validate(); err != nil {
		return err
	}
	from := issue.Status
	if from == "" {
		issue.Status = to
		return nil
	}
	if err := core.ValidateIssueTransition(from, to); err != nil {
		return fmt.Errorf("issue %s transition %q -> %q: %w", issue.ID, from, to, err)
	}
	issue.Status = to
	return nil
}
