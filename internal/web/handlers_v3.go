package web

import (
	"github.com/go-chi/chi/v5"
	"github.com/yoke233/ai-workflow/internal/core"
)

// issueListResponse is the paginated list response for issues.
type issueListResponse struct {
	Items  []core.Issue `json:"items"`
	Total  int          `json:"total"`
	Offset int          `json:"offset"`
}

func normalizeIssuesForAPI(items []core.Issue) []core.Issue {
	if len(items) == 0 {
		return []core.Issue{}
	}
	out := make([]core.Issue, len(items))
	for i := range items {
		normalized := normalizeIssueForAPI(&items[i])
		if normalized == nil {
			out[i] = core.Issue{}
			continue
		}
		out[i] = *normalized
	}
	return out
}

func normalizeIssueForAPI(issue *core.Issue) *core.Issue {
	if issue == nil {
		return nil
	}
	clone := *issue
	clone.Labels = normalizeStringSlice(issue.Labels)
	clone.Attachments = normalizeStringSlice(issue.Attachments)
	clone.DependsOn = normalizeStringSlice(issue.DependsOn)
	clone.Blocks = normalizeStringSlice(issue.Blocks)
	return &clone
}

func normalizeStringSlice(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	out := make([]string, len(values))
	copy(out, values)
	return out
}

// registerV3Routes consolidates all API routes under /api/v3.
func registerV3Routes(
	r chi.Router,
	store core.Store,
	issueManager IssueManager,
	issueParserRoleID string,
	executor RunExecutor,
	stageRoleBindings map[string]string,
	hub *Hub,
	provisioner ProjectRepoProvisioner,
	chatAssistant ChatAssistant,
	eventPublisher chatEventPublisher,
	adminToken string,
	webhookReplayer WebhookDeliveryReplayer,
) {
	r.Get("/stats", handleStats)
	registerProjectRoutes(r, store, hub, provisioner)
	registerRepoRoutes(r, store)
	registerChatRoutes(r, store, chatAssistant, eventPublisher)
	registerAdminOpsRoutes(r, store, adminToken, webhookReplayer)
	r.Get("/ws", hub.HandleWS)

	// Issue and run endpoints (v2 handlers)
	issueHandlers := &v2IssueHandlers{store: store}
	runHandlers := &v2RunHandlers{store: store}

	r.Get("/issues", issueHandlers.listIssues)
	r.Get("/issues/{id}", issueHandlers.getIssue)

	r.Get("/workflow-profiles", handleListWorkflowProfiles)
	r.Get("/workflow-profiles/{type}", handleGetWorkflowProfile)

	r.Get("/runs", runHandlers.listRuns)
	r.Get("/runs/{id}", runHandlers.getRun)
	r.Get("/runs/{id}/events", runHandlers.listRunEvents)
}
