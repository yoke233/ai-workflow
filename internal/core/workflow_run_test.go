package core

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestNewWorkflowRunID(t *testing.T) {
	id := NewWorkflowRunID()
	pat := regexp.MustCompile(`^run-\d{8}-[0-9a-f]{8}$`)
	if !pat.MatchString(id) {
		t.Fatalf("invalid workflow run id: %s", id)
	}
}

func TestWorkflowRunStatusValidate(t *testing.T) {
	cases := []struct {
		name    string
		status  WorkflowRunStatus
		wantErr bool
	}{
		{name: "created", status: WorkflowRunStatusCreated, wantErr: false},
		{name: "running", status: WorkflowRunStatusRunning, wantErr: false},
		{name: "waiting review", status: WorkflowRunStatusWaitingReview, wantErr: false},
		{name: "done", status: WorkflowRunStatusDone, wantErr: false},
		{name: "failed", status: WorkflowRunStatusFailed, wantErr: false},
		{name: "timeout", status: WorkflowRunStatusTimeout, wantErr: false},
		{name: "invalid", status: WorkflowRunStatus("paused"), wantErr: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := tc.status.Validate()
			if tc.wantErr && err == nil {
				t.Fatalf("expected validation error for status %q", tc.status)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected validation success for status %q, got: %v", tc.status, err)
			}
		})
	}
}

func TestValidateWorkflowRunTransition(t *testing.T) {
	cases := []struct {
		name    string
		from    WorkflowRunStatus
		to      WorkflowRunStatus
		wantErr bool
	}{
		{name: "created to running", from: WorkflowRunStatusCreated, to: WorkflowRunStatusRunning, wantErr: false},
		{name: "running to waiting review", from: WorkflowRunStatusRunning, to: WorkflowRunStatusWaitingReview, wantErr: false},
		{name: "waiting review back to running", from: WorkflowRunStatusWaitingReview, to: WorkflowRunStatusRunning, wantErr: false},
		{name: "running to done", from: WorkflowRunStatusRunning, to: WorkflowRunStatusDone, wantErr: false},
		{name: "running to failed", from: WorkflowRunStatusRunning, to: WorkflowRunStatusFailed, wantErr: false},
		{name: "waiting review to timeout", from: WorkflowRunStatusWaitingReview, to: WorkflowRunStatusTimeout, wantErr: false},
		{name: "created to done is illegal", from: WorkflowRunStatusCreated, to: WorkflowRunStatusDone, wantErr: true},
		{name: "done to running is illegal", from: WorkflowRunStatusDone, to: WorkflowRunStatusRunning, wantErr: true},
		{name: "unknown source status", from: WorkflowRunStatus("paused"), to: WorkflowRunStatusRunning, wantErr: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateWorkflowRunTransition(tc.from, tc.to)
			if tc.wantErr && err == nil {
				t.Fatalf("expected transition error for %q -> %q", tc.from, tc.to)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected transition success for %q -> %q, got: %v", tc.from, tc.to, err)
			}
		})
	}
}

func TestWorkflowRunValidate(t *testing.T) {
	now := time.Now()
	later := now.Add(2 * time.Minute)
	cases := []struct {
		name      string
		run       WorkflowRun
		wantErr   bool
		errSubstr string
	}{
		{
			name: "valid run",
			run: WorkflowRun{
				ID:        "run-1",
				IssueID:   "issue-1",
				Profile:   "normal",
				Status:    WorkflowRunStatusCreated,
				CreatedAt: now,
				UpdatedAt: now,
			},
			wantErr: false,
		},
		{
			name: "missing issue id",
			run: WorkflowRun{
				ID:        "run-1",
				Profile:   "normal",
				Status:    WorkflowRunStatusCreated,
				CreatedAt: now,
				UpdatedAt: now,
			},
			wantErr:   true,
			errSubstr: "issue_id",
		},
		{
			name: "missing profile",
			run: WorkflowRun{
				ID:        "run-1",
				IssueID:   "issue-1",
				Status:    WorkflowRunStatusCreated,
				CreatedAt: now,
				UpdatedAt: now,
			},
			wantErr:   true,
			errSubstr: "profile",
		},
		{
			name: "invalid status",
			run: WorkflowRun{
				ID:        "run-1",
				IssueID:   "issue-1",
				Profile:   "normal",
				Status:    WorkflowRunStatus("paused"),
				CreatedAt: now,
				UpdatedAt: now,
			},
			wantErr:   true,
			errSubstr: "status",
		},
		{
			name: "finished before created",
			run: WorkflowRun{
				ID:         "run-1",
				IssueID:    "issue-1",
				Profile:    "normal",
				Status:     WorkflowRunStatusDone,
				CreatedAt:  later,
				UpdatedAt:  later,
				FinishedAt: &now,
			},
			wantErr:   true,
			errSubstr: "finished_at",
		},
		{
			name: "terminal status requires finished at",
			run: WorkflowRun{
				ID:        "run-1",
				IssueID:   "issue-1",
				Profile:   "normal",
				Status:    WorkflowRunStatusFailed,
				CreatedAt: now,
				UpdatedAt: now,
			},
			wantErr:   true,
			errSubstr: "finished_at",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := tc.run.Validate()
			if tc.wantErr && err == nil {
				t.Fatal("expected validation error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected validation success, got: %v", err)
			}
			if tc.wantErr && tc.errSubstr != "" && !strings.Contains(err.Error(), tc.errSubstr) {
				t.Fatalf("expected error to contain %q, got: %v", tc.errSubstr, err)
			}
		})
	}
}

func TestIssueValidate_V2IssueSemanticStillPasses(t *testing.T) {
	issue := Issue{
		Title:    "v2 issue",
		Template: "standard",
	}

	if err := issue.Validate(); err != nil {
		t.Fatalf("expected v2 issue validation success, got: %v", err)
	}
}
