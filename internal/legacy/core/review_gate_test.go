package core

import (
	"encoding/json"
	"testing"
)

func TestReviewVerdictUnmarshalSnakeCaseIssueFields(t *testing.T) {
	payload := []byte(`{
		"reviewer":"dependency",
		"status":"issues_found",
		"score":75,
		"issues":[
			{
				"severity":"critical",
				"issue_id":"issue-20260302-a3f1b2c0",
				"description":"cycle detected",
				"suggestion":"remove circular edge"
			}
		]
	}`)

	var verdict ReviewVerdict
	if err := json.Unmarshal(payload, &verdict); err != nil {
		t.Fatalf("unmarshal review verdict: %v", err)
	}

	if len(verdict.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(verdict.Issues))
	}
	if verdict.Issues[0].IssueID != "issue-20260302-a3f1b2c0" {
		t.Fatalf("unexpected issue_id mapping: %q", verdict.Issues[0].IssueID)
	}
}
