package reviewgithubpr

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	ghapi "github.com/google/go-github/v68/github"
	"github.com/yoke233/ai-workflow/internal/core"
	githubsvc "github.com/yoke233/ai-workflow/internal/github"
)

const reviewerName = "github_pr"

type prClient interface {
	CreatePR(ctx context.Context, input githubsvc.CreatePRInput) (*ghapi.PullRequest, error)
	UpdatePR(ctx context.Context, number int, input githubsvc.UpdatePRInput) (*ghapi.PullRequest, error)
}

// ReviewGate creates GitHub review PRs and maps PR review results to review decisions.
type ReviewGate struct {
	store  core.Store
	client prClient

	mu     sync.RWMutex
	closed bool
}

func New(store core.Store, client prClient) *ReviewGate {
	return &ReviewGate{
		store:  store,
		client: client,
	}
}

func (g *ReviewGate) Name() string {
	return "review-github-pr"
}

func (g *ReviewGate) Init(context.Context) error {
	if g == nil {
		return errors.New("review-github-pr gate is nil")
	}
	if g.store == nil {
		return errors.New("review-github-pr store is nil")
	}
	g.mu.Lock()
	g.closed = false
	g.mu.Unlock()
	return nil
}

func (g *ReviewGate) Close() error {
	if g == nil {
		return nil
	}
	g.mu.Lock()
	g.closed = true
	g.mu.Unlock()
	return nil
}

func (g *ReviewGate) Submit(ctx context.Context, issues []*core.Issue) (string, error) {
	if err := g.ensureReady(); err != nil {
		return "", err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	issue, err := primaryIssue(issues)
	if err != nil {
		return "", err
	}
	if g.client == nil {
		return "", errors.New("review-github-pr submit: github client is required")
	}

	issueID := strings.TrimSpace(issue.ID)
	records, err := g.store.GetReviewRecords(issueID)
	if err != nil {
		return "", fmt.Errorf("review-github-pr submit list records: %w", err)
	}
	round := nextRound(records)

	pr, err := g.client.CreatePR(ctx, githubsvc.CreatePRInput{
		Title: buildPRTitle(issue),
		Body:  buildPRBody(issue),
		Head:  "ai-flow/review-" + issueID,
		Base:  "main",
		Draft: true,
	})
	if err != nil {
		return "", err
	}

	fixes := []core.ProposedFix{}
	if pr != nil {
		if number := pr.GetNumber(); number > 0 {
			fixes = append(fixes, core.ProposedFix{
				Description: "pr_number",
				Suggestion:  strconv.Itoa(number),
			})
		}
		if url := strings.TrimSpace(pr.GetHTMLURL()); url != "" {
			fixes = append(fixes, core.ProposedFix{
				Description: "pr_url",
				Suggestion:  url,
			})
		}
	}

	record := &core.ReviewRecord{
		IssueID:   issueID,
		Round:     round,
		Reviewer:  reviewerName,
		Verdict:   "pending",
		Summary:   "已创建 GitHub PR，等待评审结论",
		RawOutput: strings.TrimSpace(buildPRBody(issue)),
		Fixes:     fixes,
	}
	if err := g.store.SaveReviewRecord(record); err != nil {
		return "", fmt.Errorf("review-github-pr submit save record: %w", err)
	}
	return issueID, nil
}

func (g *ReviewGate) Check(ctx context.Context, reviewID string) (*core.ReviewResult, error) {
	if err := g.ensureReady(); err != nil {
		return nil, err
	}
	issueID := strings.TrimSpace(reviewID)
	if issueID == "" {
		return nil, errors.New("review-github-pr check: review id is required")
	}

	records, err := g.store.GetReviewRecords(issueID)
	if err != nil {
		return nil, fmt.Errorf("review-github-pr check list records: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("review-github-pr check: review %q not found", issueID)
	}

	latest := records[len(records)-1]
	status, decision := mapReviewVerdict(latest.Verdict)
	score := 0
	if latest.Score != nil {
		score = *latest.Score
	}
	return &core.ReviewResult{
		Status: status,
		Verdicts: []core.ReviewVerdict{
			{
				Reviewer:  latest.Reviewer,
				Status:    strings.TrimSpace(latest.Verdict),
				Summary:   strings.TrimSpace(latest.Summary),
				RawOutput: strings.TrimSpace(latest.RawOutput),
				Issues:    append([]core.ReviewIssue(nil), latest.Issues...),
				Score:     score,
			},
		},
		Decision: decision,
		Comments: fixesToComments(latest.Fixes),
	}, nil
}

func (g *ReviewGate) Cancel(ctx context.Context, reviewID string) error {
	if err := g.ensureReady(); err != nil {
		return err
	}
	issueID := strings.TrimSpace(reviewID)
	if issueID == "" {
		return errors.New("review-github-pr cancel: review id is required")
	}

	records, err := g.store.GetReviewRecords(issueID)
	if err != nil {
		return fmt.Errorf("review-github-pr cancel list records: %w", err)
	}
	if len(records) == 0 {
		return fmt.Errorf("review-github-pr cancel: review %q not found", issueID)
	}

	latest := records[len(records)-1]
	if normalizedReviewVerdict(latest.Verdict) == "cancelled" {
		return nil
	}

	if g.client != nil {
		if prNumber := extractPRNumber(latest.Fixes); prNumber > 0 {
			closed := "closed"
			if _, err := g.client.UpdatePR(ctx, prNumber, githubsvc.UpdatePRInput{State: &closed}); err != nil {
				return err
			}
		}
	}

	round := latest.Round
	if round <= 0 {
		round = 1
	}
	return g.store.SaveReviewRecord(&core.ReviewRecord{
		IssueID:   issueID,
		Round:     round,
		Reviewer:  reviewerName,
		Verdict:   "cancelled",
		Summary:   "GitHub PR 评审已取消",
		RawOutput: "review-github-pr gate cancelled",
	})
}

func primaryIssue(issues []*core.Issue) (*core.Issue, error) {
	if len(issues) == 0 {
		return nil, errors.New("review-github-pr submit: issues are required")
	}
	for _, issue := range issues {
		if issue == nil {
			continue
		}
		if strings.TrimSpace(issue.ID) == "" {
			return nil, errors.New("review-github-pr submit: issue id is required")
		}
		return issue, nil
	}
	return nil, errors.New("review-github-pr submit: issues are required")
}

func buildPRTitle(issue *core.Issue) string {
	title := strings.TrimSpace(issue.Title)
	if title == "" {
		title = strings.TrimSpace(issue.ID)
	}
	return "[Review] " + title
}

func buildPRBody(issue *core.Issue) string {
	issueID := strings.TrimSpace(issue.ID)
	issueTitle := strings.TrimSpace(issue.Title)
	if issueTitle == "" {
		issueTitle = issueID
	}
	body := strings.TrimSpace(issue.Body)
	if body == "" {
		body = "_No issue body provided._"
	}
	return strings.Join([]string{
		"Automated review gate for issue " + issueID,
		"",
		"Title: " + issueTitle,
		"",
		body,
	}, "\n")
}

func (g *ReviewGate) ensureReady() error {
	if g == nil {
		return errors.New("review-github-pr gate is nil")
	}
	if g.store == nil {
		return errors.New("review-github-pr store is nil")
	}

	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.closed {
		return errors.New("review-github-pr gate is closed")
	}
	return nil
}

func nextRound(records []core.ReviewRecord) int {
	maxRound := 0
	for _, record := range records {
		if record.Round > maxRound {
			maxRound = record.Round
		}
	}
	return maxRound + 1
}

func mapReviewVerdict(verdict string) (status string, decision string) {
	switch normalizedReviewVerdict(verdict) {
	case "", "pending":
		return "pending", "pending"
	case "approved", "pass":
		return "approved", "approve"
	case "changes_requested", "issues_found":
		return "changes_requested", "fix"
	case "rejected":
		return "rejected", "reject"
	case "cancelled":
		return "cancelled", "cancelled"
	default:
		unknown := strings.TrimSpace(verdict)
		if unknown == "" {
			return "pending", "pending"
		}
		return unknown, unknown
	}
}

func normalizedReviewVerdict(verdict string) string {
	value := strings.ToLower(strings.TrimSpace(verdict))
	if value == "canceled" {
		return "cancelled"
	}
	return value
}

func extractPRNumber(fixes []core.ProposedFix) int {
	for _, fix := range fixes {
		if strings.TrimSpace(fix.Description) != "pr_number" {
			continue
		}
		raw := strings.TrimSpace(fix.Suggestion)
		n, err := strconv.Atoi(raw)
		if err == nil && n > 0 {
			return n
		}
	}
	return 0
}

func fixesToComments(fixes []core.ProposedFix) []string {
	if len(fixes) == 0 {
		return nil
	}
	comments := make([]string, 0, len(fixes))
	for _, fix := range fixes {
		description := strings.TrimSpace(fix.Description)
		suggestion := strings.TrimSpace(fix.Suggestion)
		switch {
		case description == "" && suggestion == "":
			continue
		case description == "":
			comments = append(comments, suggestion)
		case suggestion == "":
			comments = append(comments, description)
		default:
			comments = append(comments, description+"="+suggestion)
		}
	}
	return comments
}

var _ core.ReviewGate = (*ReviewGate)(nil)
