package core

import "testing"

func TestEventTypes_GitHubConstants_Defined(t *testing.T) {
	cases := map[string]EventType{
		"github_webhook_received":              EventGitHubWebhookReceived,
		"github_issue_opened":                  EventGitHubIssueOpened,
		"github_issue_comment_created":         EventGitHubIssueCommentCreated,
		"github_pull_request_review_submitted": EventGitHubPullRequestReviewSubmitted,
		"github_pull_request_closed":           EventGitHubPullRequestClosed,
		"github_reconnected":                   EventGitHubReconnected,
	}

	for want, got := range cases {
		if got == "" {
			t.Fatalf("expected GitHub event constant %q to be defined", want)
		}
		if string(got) != want {
			t.Fatalf("expected GitHub event constant %q, got %q", want, got)
		}
	}
}
