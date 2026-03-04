package core

import "testing"

func TestEventTypes_TeamLeaderAndRunConstants_Defined(t *testing.T) {
	cases := map[EventType]string{
		EventTeamLeaderThinking:     "team_leader_thinking",
		EventTeamLeaderFilesChanged: "team_leader_files_changed",
		EventRunStarted:             "run_started",
		EventRunUpdate:              "run_update",
		EventRunCompleted:           "run_completed",
		EventRunFailed:              "run_failed",
		EventRunCancelled:           "run_cancelled",
	}

	for eventType, want := range cases {
		if eventType == "" {
			t.Fatalf("expected event constant %q to be defined", want)
		}
		if string(eventType) != want {
			t.Fatalf("expected event constant %q, got %q", want, eventType)
		}
	}
}

func TestIsIssueScopedEvent_TeamLeaderAndIssueLifecycle(t *testing.T) {
	scoped := []EventType{
		EventTeamLeaderThinking,
		EventIssueCreated,
		EventIssueReviewing,
		EventReviewDone,
		EventIssueApproved,
		EventIssueQueued,
		EventIssueReady,
		EventIssueExecuting,
		EventIssueDone,
		EventIssueFailed,
		EventIssueDependencyChanged,
	}

	for _, eventType := range scoped {
		if !IsIssueScopedEvent(eventType) {
			t.Fatalf("expected %q to be issue scoped", eventType)
		}
	}

	nonScoped := []EventType{
		EventRunStarted,
		EventRunUpdate,
		EventTeamLeaderFilesChanged,
		EventStageStart,
		EventType("secretary_thinking"),
	}

	for _, eventType := range nonScoped {
		if IsIssueScopedEvent(eventType) {
			t.Fatalf("expected %q to not be issue scoped", eventType)
		}
	}
}

func TestIsAlwaysBroadcastIssueEvent_IssueLifecycle(t *testing.T) {
	alwaysBroadcast := []EventType{
		EventIssueCreated,
		EventIssueDone,
		EventIssueFailed,
	}

	for _, eventType := range alwaysBroadcast {
		if !IsAlwaysBroadcastIssueEvent(eventType) {
			t.Fatalf("expected %q to be always broadcast", eventType)
		}
	}

	notAlwaysBroadcast := []EventType{
		EventIssueReady,
		EventRunCompleted,
		EventTeamLeaderThinking,
	}

	for _, eventType := range notAlwaysBroadcast {
		if IsAlwaysBroadcastIssueEvent(eventType) {
			t.Fatalf("expected %q to not be always broadcast", eventType)
		}
	}
}
