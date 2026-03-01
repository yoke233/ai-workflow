package trackergithub

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	ghapi "github.com/google/go-github/v68/github"
	"github.com/user/ai-workflow/internal/config"
	"github.com/user/ai-workflow/internal/core"
	githubsvc "github.com/user/ai-workflow/internal/github"
)

const (
	statusReady      = "status: ready"
	statusBlocked    = "status: blocked"
	statusInProgress = "status: in-progress"
	statusDone       = "status: done"
	statusFailed     = "status: failed"
)

// GitHubTracker mirrors TaskItem status into GitHub issues.
type GitHubTracker struct {
	issues         issueService
	warningWriter  io.Writer
	startupWarning error
}

type issueService interface {
	CreateIssue(ctx context.Context, title, body string, labels []string) (issueNumber int, err error)
	UpdateIssueLabels(ctx context.Context, issueNumber int, labels []string) error
	CloseIssue(ctx context.Context, issueNumber int) error
}

type githubIssueService struct {
	service *githubsvc.GitHubService
	client  *ghapi.Client
	owner   string
	repo    string
}

// New returns a tracker in degraded mode unless a service is injected.
func New() *GitHubTracker {
	return &GitHubTracker{
		warningWriter: os.Stderr,
	}
}

// NewWithGitHubConfig creates a tracker that uses configured GitHub credentials when available.
func NewWithGitHubConfig(cfg config.GitHubConfig) *GitHubTracker {
	tracker := New()
	if !cfg.Enabled {
		return tracker
	}
	if strings.TrimSpace(cfg.Owner) == "" || strings.TrimSpace(cfg.Repo) == "" {
		tracker.startupWarning = errors.New("github owner/repo are required")
		return tracker
	}

	client, err := githubsvc.NewClient(cfg)
	if err != nil {
		tracker.startupWarning = err
		return tracker
	}

	service, err := githubsvc.NewGitHubService(client, cfg.Owner, cfg.Repo)
	if err != nil {
		tracker.startupWarning = err
		return tracker
	}

	tracker.issues = &githubIssueService{
		service: service,
		client:  client.Client(),
		owner:   strings.TrimSpace(cfg.Owner),
		repo:    strings.TrimSpace(cfg.Repo),
	}
	return tracker
}

func newWithIssueService(issues issueService) *GitHubTracker {
	tracker := New()
	tracker.issues = issues
	return tracker
}

func (t *GitHubTracker) Name() string {
	return "tracker-github"
}

func (t *GitHubTracker) Init(context.Context) error {
	return nil
}

func (t *GitHubTracker) Close() error {
	return nil
}

func (t *GitHubTracker) CreateTask(ctx context.Context, item *core.TaskItem) (string, error) {
	if item == nil {
		return "", nil
	}
	if strings.TrimSpace(item.ExternalID) != "" {
		return item.ExternalID, nil
	}
	if t.issues == nil {
		return "", t.warning("create issue", t.unavailableReason())
	}

	title := strings.TrimSpace(item.Title)
	if title == "" {
		title = strings.TrimSpace(item.ID)
	}

	issueNumber, err := t.issues.CreateIssue(
		ctx,
		title,
		strings.TrimSpace(item.Description),
		buildCreateLabels(item),
	)
	if err != nil {
		return "", t.warning("create issue", err)
	}
	if issueNumber <= 0 {
		return "", t.warning("create issue", fmt.Errorf("invalid issue number %d", issueNumber))
	}
	return strconv.Itoa(issueNumber), nil
}

func (t *GitHubTracker) UpdateStatus(ctx context.Context, externalID string, status core.TaskItemStatus) error {
	issueNumber, err := parseIssueNumber(externalID)
	if err != nil {
		return t.warning("update status", err)
	}
	if t.issues == nil {
		return t.warning("update status", t.unavailableReason())
	}

	if err := t.issues.UpdateIssueLabels(ctx, issueNumber, []string{statusLabelForStatus(status)}); err != nil {
		return t.warning("update labels", err)
	}
	if status == core.ItemDone {
		if err := t.issues.CloseIssue(ctx, issueNumber); err != nil {
			return t.warning("close issue", err)
		}
	}
	return nil
}

func (t *GitHubTracker) SyncDependencies(ctx context.Context, item *core.TaskItem, allItems []core.TaskItem) error {
	if item == nil {
		return nil
	}

	issueNumber, err := parseIssueNumber(item.ExternalID)
	if err != nil {
		return t.warning("sync dependencies", err)
	}
	if t.issues == nil {
		return t.warning("sync dependencies", t.unavailableReason())
	}

	labels := makeDependencyLabels(item, allItems)
	if err := t.issues.UpdateIssueLabels(ctx, issueNumber, labels); err != nil {
		return t.warning("sync dependencies", err)
	}
	return nil
}

func (t *GitHubTracker) OnExternalComplete(ctx context.Context, externalID string) error {
	return t.UpdateStatus(ctx, externalID, core.ItemDone)
}

func (t *GitHubTracker) unavailableReason() error {
	if t.startupWarning != nil {
		return t.startupWarning
	}
	return errors.New("github tracker service is not configured")
}

func (t *GitHubTracker) warning(operation string, err error) error {
	if err == nil {
		return nil
	}
	warnErr := core.NewTrackerWarning(operation, err)
	if t.warningWriter != nil {
		fmt.Fprintf(t.warningWriter, "warning: %v\n", warnErr)
	}
	return warnErr
}

func (s *githubIssueService) CreateIssue(
	ctx context.Context,
	title string,
	body string,
	labels []string,
) (int, error) {
	if s == nil || s.service == nil {
		return 0, errors.New("github issue service is not initialized")
	}
	issue, err := s.service.CreateIssue(ctx, githubsvc.CreateIssueInput{
		Title:  title,
		Body:   body,
		Labels: labels,
	})
	if err != nil {
		return 0, err
	}
	return issue.GetNumber(), nil
}

func (s *githubIssueService) UpdateIssueLabels(ctx context.Context, issueNumber int, labels []string) error {
	if s == nil || s.service == nil {
		return errors.New("github issue service is not initialized")
	}
	return s.service.UpdateIssueLabels(ctx, issueNumber, labels)
}

func (s *githubIssueService) CloseIssue(ctx context.Context, issueNumber int) error {
	if s == nil || s.client == nil {
		return errors.New("github issue service is not initialized")
	}
	closed := "closed"
	_, _, err := s.client.Issues.Edit(ctx, s.owner, s.repo, issueNumber, &ghapi.IssueRequest{State: &closed})
	if err != nil {
		return fmt.Errorf("github close issue (%s/%s#%d): %w", s.owner, s.repo, issueNumber, err)
	}
	return nil
}

func buildCreateLabels(item *core.TaskItem) []string {
	if item == nil {
		return nil
	}
	labels := make([]string, 0, len(item.Labels)+3)
	labels = append(labels, item.Labels...)
	if planID := strings.TrimSpace(item.PlanID); planID != "" {
		labels = append(labels, fmt.Sprintf("plan: %s", planID))
	}
	if template := strings.TrimSpace(item.Template); template != "" {
		labels = append(labels, fmt.Sprintf("template: %s", template))
	}
	labels = append(labels, statusLabelForStatus(item.Status))
	return normalizeLabels(labels)
}

func makeDependencyLabels(item *core.TaskItem, allItems []core.TaskItem) []string {
	labels := make([]string, 0, len(item.DependsOn)+1)
	byID := make(map[string]core.TaskItem, len(allItems))
	for _, it := range allItems {
		byID[it.ID] = it
	}

	blocked := false
	for _, depID := range item.DependsOn {
		dep, ok := byID[depID]
		if !ok || !isDependencyFinished(dep.Status) {
			blocked = true
		}
		if depIssueNumber, err := parseIssueNumber(dep.ExternalID); err == nil {
			labels = append(labels, fmt.Sprintf("depends-on-#%d", depIssueNumber))
		}
	}

	if blocked {
		labels = append(labels, statusBlocked)
	} else {
		labels = append(labels, statusReady)
	}
	return normalizeLabels(labels)
}

func isDependencyFinished(status core.TaskItemStatus) bool {
	switch status {
	case core.ItemDone, core.ItemSkipped:
		return true
	default:
		return false
	}
}

func statusLabelForStatus(status core.TaskItemStatus) string {
	switch status {
	case core.ItemReady:
		return statusReady
	case core.ItemRunning:
		return statusInProgress
	case core.ItemDone:
		return statusDone
	case core.ItemFailed:
		return statusFailed
	default:
		return statusBlocked
	}
}

func parseIssueNumber(externalID string) (int, error) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(externalID, "#"))
	if trimmed == "" {
		return 0, errors.New("external id is empty")
	}

	if issueNumber, err := strconv.Atoi(trimmed); err == nil && issueNumber > 0 {
		return issueNumber, nil
	}

	start := -1
	end := -1
	for idx := len(trimmed) - 1; idx >= 0; idx-- {
		ch := trimmed[idx]
		if ch < '0' || ch > '9' {
			if end != -1 {
				break
			}
			continue
		}
		if end == -1 {
			end = idx
		}
		start = idx
	}
	if start == -1 || end == -1 {
		return 0, fmt.Errorf("invalid external id %q", externalID)
	}

	issueNumber, err := strconv.Atoi(trimmed[start : end+1])
	if err != nil || issueNumber <= 0 {
		return 0, fmt.Errorf("invalid external id %q", externalID)
	}
	return issueNumber, nil
}

func normalizeLabels(labels []string) []string {
	if len(labels) == 0 {
		return nil
	}

	out := make([]string, 0, len(labels))
	seen := make(map[string]struct{}, len(labels))
	for _, label := range labels {
		trimmed := strings.TrimSpace(label)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

var _ core.Tracker = (*GitHubTracker)(nil)
