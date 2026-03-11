package reviewaipanel

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/yoke233/ai-workflow/internal/core"
	"github.com/yoke233/ai-workflow/internal/teamleader"
)

const gateReviewer = "review_gate"

type reviewPanel interface {
	Run(ctx context.Context, issues []*core.Issue) (*teamleader.ReviewSessionResult, error)
}

type runState struct {
	round       int
	issueIDs    []string
	cancel      context.CancelFunc
	running     bool
	cancelled   bool
	terminalErr error
}

// AIReviewGate runs teamleader.ReviewOrchestrator asynchronously and exposes polling/cancel APIs.
type AIReviewGate struct {
	store core.Store
	panel reviewPanel

	mu     sync.Mutex
	closed bool
	runs   map[string]*runState
}

func New(store core.Store, panel reviewPanel) *AIReviewGate {
	return &AIReviewGate{
		store: store,
		panel: panel,
		runs:  make(map[string]*runState),
	}
}

func (g *AIReviewGate) Name() string {
	return "ai-panel"
}

func (g *AIReviewGate) Init(context.Context) error {
	if g == nil {
		return errors.New("review-ai-panel gate is nil")
	}
	if g.store == nil {
		return errors.New("review-ai-panel store is nil")
	}
	g.mu.Lock()
	if g.runs == nil {
		g.runs = make(map[string]*runState)
	}
	g.closed = false
	g.mu.Unlock()
	return nil
}

func (g *AIReviewGate) Close() error {
	if g == nil {
		return nil
	}

	g.mu.Lock()
	if g.closed {
		g.mu.Unlock()
		return nil
	}
	g.closed = true

	states := make([]*runState, 0, len(g.runs))
	for _, state := range g.runs {
		states = append(states, state)
	}
	g.mu.Unlock()

	for _, state := range states {
		if state != nil && state.running && state.cancel != nil {
			state.cancel()
		}
	}
	return nil
}

func (g *AIReviewGate) Submit(ctx context.Context, issues []*core.Issue) (string, error) {
	if err := g.ensureReady(); err != nil {
		return "", err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if g.panel == nil {
		return "", errors.New("review-ai-panel submit: review orchestrator is nil")
	}

	normalizedIssues, err := normalizeSubmitIssues(issues)
	if err != nil {
		return "", fmt.Errorf("review-ai-panel submit: %w", err)
	}
	reviewID := normalizedIssues[0].ID
	issueIDs := issueIDsFromIssues(normalizedIssues)

	round, err := g.nextRound(issueIDs)
	if err != nil {
		return "", fmt.Errorf("review-ai-panel submit compute round: %w", err)
	}

	runCtx, cancel := context.WithCancel(context.Background())
	if err := g.markRunning(reviewID, &runState{
		round:    round,
		issueIDs: issueIDs,
		cancel:   cancel,
		running:  true,
	}); err != nil {
		cancel()
		return "", err
	}

	if err := g.saveGateVerdict(issueIDs, round, "pending", nil); err != nil {
		g.unmarkRunning(reviewID)
		cancel()
		return "", fmt.Errorf("review-ai-panel submit save pending record: %w", err)
	}

	go g.runAsync(reviewID, round, normalizedIssues, runCtx)
	return reviewID, nil
}

func (g *AIReviewGate) Check(ctx context.Context, reviewID string) (*core.ReviewResult, error) {
	if err := g.ensureReady(); err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	normalizedReviewID := strings.TrimSpace(reviewID)
	if normalizedReviewID == "" {
		return nil, errors.New("review-ai-panel check: review id is required")
	}

	state, _ := g.getRunState(normalizedReviewID)
	running := g.isRunning(normalizedReviewID)
	issueIDs := g.reviewIssueIDs(normalizedReviewID, state)

	records, err := g.loadMergedRecords(issueIDs)
	if err != nil {
		return nil, fmt.Errorf("review-ai-panel check list records: %w", err)
	}

	if len(records) == 0 {
		if running {
			return &core.ReviewResult{
				Status:   core.ReviewStatusPending,
				Decision: core.ReviewDecisionPending,
			}, nil
		}
		return nil, fmt.Errorf("review-ai-panel check: review %q not found", normalizedReviewID)
	}

	if running {
		if state != nil && state.cancelled {
			return &core.ReviewResult{
				Status:   core.ReviewStatusCancelled,
				Decision: core.ReviewDecisionCancelled,
				Verdicts: recordsToVerdicts(records, latestVerdictRound(records)),
			}, nil
		}
		latestRound := latestVerdictRound(records)
		return &core.ReviewResult{
			Status:   core.ReviewStatusPending,
			Decision: core.ReviewDecisionPending,
			Verdicts: recordsToVerdicts(records, latestRound),
		}, nil
	}
	if state != nil && state.terminalErr != nil {
		return nil, fmt.Errorf("review-ai-panel check terminal state persistence: %w", state.terminalErr)
	}

	latest := records[len(records)-1]
	latestRound := latestVerdictRound(records)
	if latestRound <= 0 {
		latestRound = latest.Round
	}
	status, decision := mapStatusDecision(latest.Verdict)
	return &core.ReviewResult{
		Status:   status,
		Decision: decision,
		Verdicts: recordsToVerdicts(records, latestRound),
	}, nil
}

func (g *AIReviewGate) Cancel(ctx context.Context, reviewID string) error {
	if err := g.ensureReady(); err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	normalizedReviewID := strings.TrimSpace(reviewID)
	if normalizedReviewID == "" {
		return errors.New("review-ai-panel cancel: review id is required")
	}

	state, running := g.getRunState(normalizedReviewID)
	if running {
		g.markCancelled(normalizedReviewID)
		if state != nil && state.cancel != nil {
			state.cancel()
		}
		if err := g.persistCancelled(normalizedReviewID, stateRound(state)); err != nil {
			g.setTerminalError(normalizedReviewID, err)
			return fmt.Errorf("review-ai-panel cancel persist state: %w", err)
		}
		return nil
	}

	issueIDs := g.reviewIssueIDs(normalizedReviewID, state)
	records, err := g.loadMergedRecords(issueIDs)
	if err != nil {
		return fmt.Errorf("review-ai-panel cancel list records: %w", err)
	}
	if len(records) == 0 {
		return fmt.Errorf("review-ai-panel cancel: review %q not found", normalizedReviewID)
	}
	if normalizeVerdict(records[len(records)-1].Verdict) == "cancelled" {
		return nil
	}
	return fmt.Errorf("review-ai-panel cancel: review %q is not running", normalizedReviewID)
}

func (g *AIReviewGate) runAsync(reviewID string, round int, issues []*core.Issue, runCtx context.Context) {
	defer g.unmarkRunning(reviewID)

	session, err := g.panel.Run(runCtx, cloneIssues(issues))
	if err == nil {
		if persistErr := g.persistCompleted(reviewID, round, session); persistErr != nil {
			g.setTerminalError(reviewID, persistErr)
		}
		return
	}

	cancelled := g.wasCancelled(reviewID) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
	if cancelled {
		if persistErr := g.persistCancelled(reviewID, round); persistErr != nil {
			g.setTerminalError(reviewID, persistErr)
		}
		return
	}
	if persistErr := g.persistRejected(reviewID, round, err); persistErr != nil {
		g.setTerminalError(reviewID, persistErr)
	}
}

func (g *AIReviewGate) persistCompleted(reviewID string, fallbackRound int, session *teamleader.ReviewSessionResult) error {
	if session == nil {
		return nil
	}

	issueIDs := g.reviewIssueIDs(reviewID, nil)
	round, err := g.resolveRoundForIssues(issueIDs, fallbackRound)
	if err != nil {
		return err
	}

	verdict := decisionVerdict(session.Decision, session.Status)
	return g.saveGateVerdict(issueIDs, round, verdict, session.Verdicts)
}

func (g *AIReviewGate) persistCancelled(reviewID string, fallbackRound int) error {
	issueIDs := g.reviewIssueIDs(reviewID, nil)
	round, err := g.resolveRoundForIssues(issueIDs, fallbackRound)
	if err != nil {
		return err
	}

	for _, issueID := range issueIDs {
		records, err := g.store.GetReviewRecords(issueID)
		if err != nil {
			return err
		}
		if len(records) > 0 && normalizeVerdict(records[len(records)-1].Verdict) == "cancelled" {
			continue
		}
		if err := g.store.SaveReviewRecord(&core.ReviewRecord{
			IssueID:   issueID,
			Round:     round,
			Reviewer:  gateReviewer,
			Verdict:   "cancelled",
			Summary:   "review 已取消",
			RawOutput: "review gate cancelled by user or runtime",
		}); err != nil {
			return err
		}
	}
	return nil
}

func (g *AIReviewGate) persistRejected(reviewID string, fallbackRound int, runErr error) error {
	issueIDs := g.reviewIssueIDs(reviewID, nil)
	round, err := g.resolveRoundForIssues(issueIDs, fallbackRound)
	if err != nil {
		return err
	}

	for _, issueID := range issueIDs {
		records, err := g.store.GetReviewRecords(issueID)
		if err != nil {
			return err
		}
		if len(records) > 0 && normalizeVerdict(records[len(records)-1].Verdict) == "cancelled" {
			continue
		}

		record := &core.ReviewRecord{
			IssueID:   issueID,
			Round:     round,
			Reviewer:  gateReviewer,
			Verdict:   "rejected",
			Summary:   "review 执行失败",
			RawOutput: strings.TrimSpace(fmt.Sprintf("review gate rejected: %v", runErr)),
		}
		if runErr != nil {
			record.Issues = []core.ReviewIssue{
				{
					Severity:    "error",
					IssueID:     issueID,
					Description: strings.TrimSpace(runErr.Error()),
				},
			}
		}
		if err := g.store.SaveReviewRecord(record); err != nil {
			return err
		}
	}
	return nil
}

func (g *AIReviewGate) ensureReady() error {
	if g == nil {
		return errors.New("review-ai-panel gate is nil")
	}
	if g.store == nil {
		return errors.New("review-ai-panel store is nil")
	}
	g.mu.Lock()
	closed := g.closed
	g.mu.Unlock()
	if closed {
		return errors.New("review-ai-panel gate is closed")
	}
	return nil
}

func (g *AIReviewGate) markRunning(reviewID string, state *runState) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return errors.New("review-ai-panel gate is closed")
	}
	if existing, ok := g.runs[reviewID]; ok && existing != nil && existing.running {
		return fmt.Errorf("review-ai-panel submit: review %q is already running", reviewID)
	}
	state.running = true
	state.terminalErr = nil
	state.cancelled = false
	state.issueIDs = append([]string(nil), state.issueIDs...)
	g.runs[reviewID] = state
	return nil
}

func (g *AIReviewGate) unmarkRunning(reviewID string) {
	g.mu.Lock()
	if state, ok := g.runs[reviewID]; ok && state != nil {
		state.running = false
		state.cancel = nil
	}
	g.mu.Unlock()
}

func (g *AIReviewGate) isRunning(reviewID string) bool {
	g.mu.Lock()
	state, ok := g.runs[reviewID]
	g.mu.Unlock()
	return ok && state != nil && state.running
}

func (g *AIReviewGate) getRunState(reviewID string) (*runState, bool) {
	g.mu.Lock()
	state, ok := g.runs[reviewID]
	g.mu.Unlock()
	return state, ok && state != nil && state.running
}

func (g *AIReviewGate) markCancelled(reviewID string) {
	g.mu.Lock()
	if state, ok := g.runs[reviewID]; ok && state != nil {
		state.cancelled = true
	}
	g.mu.Unlock()
}

func (g *AIReviewGate) wasCancelled(reviewID string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	state, ok := g.runs[reviewID]
	return ok && state != nil && state.cancelled
}

func (g *AIReviewGate) setTerminalError(reviewID string, err error) {
	if err == nil {
		return
	}
	g.mu.Lock()
	if state, ok := g.runs[reviewID]; ok && state != nil {
		state.terminalErr = err
	}
	g.mu.Unlock()
}

func (g *AIReviewGate) reviewIssueIDs(reviewID string, state *runState) []string {
	if state != nil && len(state.issueIDs) > 0 {
		return append([]string(nil), state.issueIDs...)
	}

	g.mu.Lock()
	stored := g.runs[reviewID]
	g.mu.Unlock()
	if stored != nil && len(stored.issueIDs) > 0 {
		return append([]string(nil), stored.issueIDs...)
	}
	return []string{reviewID}
}

func (g *AIReviewGate) nextRound(issueIDs []string) (int, error) {
	maxRound := 0
	for _, issueID := range issueIDs {
		records, err := g.store.GetReviewRecords(issueID)
		if err != nil {
			return 0, err
		}
		for _, record := range records {
			if record.Round > maxRound {
				maxRound = record.Round
			}
		}
	}
	return maxRound + 1, nil
}

func (g *AIReviewGate) resolveRoundForIssues(issueIDs []string, fallbackRound int) (int, error) {
	round := fallbackRound
	if round <= 0 {
		round = 1
	}
	for _, issueID := range issueIDs {
		records, err := g.store.GetReviewRecords(issueID)
		if err != nil {
			return 0, err
		}
		for _, record := range records {
			if record.Round > round {
				round = record.Round
			}
		}
	}
	return round, nil
}

func (g *AIReviewGate) saveGateVerdict(issueIDs []string, round int, verdict string, verdicts map[string]core.ReviewVerdict) error {
	for _, issueID := range issueIDs {
		record := &core.ReviewRecord{
			IssueID:  issueID,
			Round:    round,
			Reviewer: gateReviewer,
			Verdict:  verdict,
		}
		if verdicts != nil {
			if summary, ok := verdicts[issueID]; ok {
				record.Summary = strings.TrimSpace(summary.Summary)
				record.RawOutput = strings.TrimSpace(summary.RawOutput)
				record.Issues = append([]core.ReviewIssue(nil), summary.Issues...)
				if summary.Score > 0 {
					score := summary.Score
					record.Score = &score
				}
			}
		}
		if record.Summary == "" {
			record.Summary = "review gate status=" + strings.TrimSpace(verdict)
		}
		if record.RawOutput == "" {
			record.RawOutput = "review gate verdict=" + strings.TrimSpace(verdict)
		}
		if err := g.store.SaveReviewRecord(record); err != nil {
			return err
		}
	}
	return nil
}

func (g *AIReviewGate) loadMergedRecords(issueIDs []string) ([]core.ReviewRecord, error) {
	normalizedIDs := normalizeIssueIDs(issueIDs)
	out := make([]core.ReviewRecord, 0)
	for _, issueID := range normalizedIDs {
		records, err := g.store.GetReviewRecords(issueID)
		if err != nil {
			return nil, err
		}
		for i := range records {
			record := records[i]
			if strings.TrimSpace(record.IssueID) == "" {
				record.IssueID = issueID
			}
			out = append(out, record)
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Round != out[j].Round {
			return out[i].Round < out[j].Round
		}
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		}
		if out[i].IssueID != out[j].IssueID {
			return out[i].IssueID < out[j].IssueID
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func stateRound(state *runState) int {
	if state == nil || state.round <= 0 {
		return 1
	}
	return state.round
}

func normalizeIssueIDs(issueIDs []string) []string {
	out := make([]string, 0, len(issueIDs))
	seen := make(map[string]struct{}, len(issueIDs))
	for _, raw := range issueIDs {
		issueID := strings.TrimSpace(raw)
		if issueID == "" {
			continue
		}
		if _, exists := seen[issueID]; exists {
			continue
		}
		seen[issueID] = struct{}{}
		out = append(out, issueID)
	}
	return out
}

func normalizeSubmitIssues(issues []*core.Issue) ([]*core.Issue, error) {
	if len(issues) == 0 {
		return nil, errors.New("issues are required")
	}
	out := make([]*core.Issue, 0, len(issues))
	seen := make(map[string]struct{}, len(issues))
	for idx, issue := range issues {
		if issue == nil {
			return nil, fmt.Errorf("issue[%d] is nil", idx)
		}
		issueID := strings.TrimSpace(issue.ID)
		if issueID == "" {
			return nil, fmt.Errorf("issue[%d] id is required", idx)
		}
		if _, exists := seen[issueID]; exists {
			return nil, fmt.Errorf("duplicate issue id %q", issueID)
		}
		seen[issueID] = struct{}{}

		cloned := cloneIssue(issue)
		cloned.ID = issueID
		normalizeIssueProfile(cloned)
		out = append(out, cloned)
	}
	return out, nil
}

func issueIDsFromIssues(issues []*core.Issue) []string {
	out := make([]string, 0, len(issues))
	for _, issue := range issues {
		if issue == nil {
			continue
		}
		issueID := strings.TrimSpace(issue.ID)
		if issueID == "" {
			continue
		}
		out = append(out, issueID)
	}
	return normalizeIssueIDs(out)
}

func cloneIssues(issues []*core.Issue) []*core.Issue {
	out := make([]*core.Issue, 0, len(issues))
	for _, issue := range issues {
		out = append(out, cloneIssue(issue))
	}
	return out
}

func cloneIssue(issue *core.Issue) *core.Issue {
	if issue == nil {
		return nil
	}
	out := *issue
	out.Labels = append([]string(nil), issue.Labels...)
	out.Attachments = append([]string(nil), issue.Attachments...)
	out.DependsOn = append([]string(nil), issue.DependsOn...)
	out.Blocks = append([]string(nil), issue.Blocks...)
	return &out
}

func normalizeIssueProfile(issue *core.Issue) {
	if issue == nil {
		return
	}
	profile := resolveIssueProfile(issue)
	labels := make([]string, 0, len(issue.Labels)+1)
	for _, label := range issue.Labels {
		trimmed := strings.TrimSpace(label)
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "profile:") {
			continue
		}
		if trimmed != "" {
			labels = append(labels, trimmed)
		}
	}
	labels = append(labels, "profile:"+string(profile))
	issue.Labels = labels
}

func resolveIssueProfile(issue *core.Issue) core.WorkflowProfileType {
	if issue == nil {
		return core.WorkflowProfileNormal
	}
	for _, label := range issue.Labels {
		lower := strings.ToLower(strings.TrimSpace(label))
		if !strings.HasPrefix(lower, "profile:") {
			continue
		}
		candidate := core.WorkflowProfileType(strings.TrimSpace(strings.TrimPrefix(lower, "profile:")))
		if candidate.Validate() == nil {
			return candidate
		}
	}
	if candidate := core.WorkflowProfileType(strings.ToLower(strings.TrimSpace(issue.Template))); candidate.Validate() == nil {
		return candidate
	}
	return core.WorkflowProfileNormal
}

func recordsToVerdicts(records []core.ReviewRecord, round int) []core.ReviewVerdict {
	if len(records) == 0 {
		return nil
	}
	out := make([]core.ReviewVerdict, 0, len(records))
	for _, record := range records {
		if round > 0 && record.Round != round {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(record.Reviewer), gateReviewer) {
			continue
		}
		score := 0
		if record.Score != nil {
			score = *record.Score
		}
		out = append(out, core.ReviewVerdict{
			Reviewer:  strings.TrimSpace(record.Reviewer),
			Status:    strings.TrimSpace(record.Verdict),
			Summary:   strings.TrimSpace(record.Summary),
			RawOutput: strings.TrimSpace(record.RawOutput),
			Issues:    append([]core.ReviewIssue(nil), record.Issues...),
			Score:     score,
		})
	}
	if len(out) > 0 {
		return out
	}

	last := records[len(records)-1]
	score := 0
	if last.Score != nil {
		score = *last.Score
	}
	return []core.ReviewVerdict{
		{
			Reviewer:  strings.TrimSpace(last.Reviewer),
			Status:    strings.TrimSpace(last.Verdict),
			Summary:   strings.TrimSpace(last.Summary),
			RawOutput: strings.TrimSpace(last.RawOutput),
			Issues:    append([]core.ReviewIssue(nil), last.Issues...),
			Score:     score,
		},
	}
}

func latestVerdictRound(records []core.ReviewRecord) int {
	latest := 0
	for _, record := range records {
		if strings.EqualFold(strings.TrimSpace(record.Reviewer), gateReviewer) {
			continue
		}
		if record.Round > latest {
			latest = record.Round
		}
	}
	if latest > 0 {
		return latest
	}
	if len(records) > 0 {
		return records[len(records)-1].Round
	}
	return 0
}

func mapStatusDecision(verdict string) (status string, decision string) {
	switch normalizeVerdict(verdict) {
	case "", "pending":
		return core.ReviewStatusPending, core.ReviewDecisionPending
	case "approved", "approve", "pass":
		return core.ReviewStatusApproved, core.ReviewDecisionApprove
	case "escalate":
		return core.ReviewStatusRejected, "escalate"
	case "rejected", "reject":
		return core.ReviewStatusRejected, core.ReviewDecisionReject
	case "changes_requested", "fix", "issues_found":
		return core.ReviewStatusChangesRequested, core.ReviewDecisionFix
	case "cancelled":
		return core.ReviewStatusCancelled, core.ReviewDecisionCancelled
	default:
		unknown := strings.TrimSpace(verdict)
		if unknown == "" {
			return core.ReviewStatusPending, core.ReviewDecisionPending
		}
		return unknown, unknown
	}
}

func decisionVerdict(decision string, status string) string {
	switch normalizeVerdict(decision) {
	case "approve":
		return "approved"
	case "fix":
		return "fix"
	case "escalate":
		return "escalate"
	case "reject":
		return "rejected"
	case "cancelled":
		return "cancelled"
	case "pending":
		return "pending"
	}
	switch normalizeVerdict(status) {
	case "approved", "rejected", "changes_requested", "cancelled", "pending":
		return normalizeVerdict(status)
	default:
		return "approved"
	}
}

func normalizeVerdict(verdict string) string {
	value := strings.ToLower(strings.TrimSpace(verdict))
	if value == "canceled" {
		return "cancelled"
	}
	return value
}

var _ core.ReviewGate = (*AIReviewGate)(nil)
