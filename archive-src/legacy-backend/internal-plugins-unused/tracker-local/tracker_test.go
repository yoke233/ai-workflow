package trackerlocal

import (
	"context"
	"testing"

	"github.com/yoke233/ai-workflow/internal/core"
)

func TestLocalTracker_NameInitClose(t *testing.T) {
	tracker := New()

	if got := tracker.Name(); got != "local" {
		t.Fatalf("Name() = %q, want %q", got, "local")
	}
	if err := tracker.Init(context.Background()); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	if err := tracker.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestLocalTracker_CreateIssue(t *testing.T) {
	tracker := New()

	cases := []struct {
		name string
		item *core.Issue
		want string
	}{
		{
			name: "nil item",
			item: nil,
			want: "",
		},
		{
			name: "with external id",
			item: &core.Issue{
				ID:         "task-1",
				ExternalID: "ext-1",
			},
			want: "ext-1",
		},
		{
			name: "without external id",
			item: &core.Issue{
				ID: "task-2",
			},
			want: "task-2",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := tracker.CreateIssue(context.Background(), tc.item)
			if err != nil {
				t.Fatalf("CreateIssue() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("CreateIssue() externalID = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLocalTracker_UpdateStatus(t *testing.T) {
	tracker := New()

	if err := tracker.UpdateStatus(context.Background(), "", core.IssueStatusDone); err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}
}

func TestLocalTracker_SyncDependencies(t *testing.T) {
	tracker := New()

	item := &core.Issue{
		ID:        "task-2",
		DependsOn: []string{"task-1"},
	}
	allItems := []*core.Issue{
		{ID: "task-1", Status: core.IssueStatusDone},
		{ID: "task-2", Status: core.IssueStatusReady},
	}

	if err := tracker.SyncDependencies(context.Background(), item, allItems); err != nil {
		t.Fatalf("SyncDependencies() error = %v", err)
	}
}

func TestLocalTracker_OnExternalComplete(t *testing.T) {
	tracker := New()

	if err := tracker.OnExternalComplete(context.Background(), "ext-1"); err != nil {
		t.Fatalf("OnExternalComplete() error = %v", err)
	}
}
