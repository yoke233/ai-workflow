package main

import (
	"context"
	"testing"
)

func TestGitHubReplayCommand_ReplaysByDeliveryID(t *testing.T) {
	origReplayFn := githubReplayByDeliveryID
	t.Cleanup(func() {
		githubReplayByDeliveryID = origReplayFn
	})

	called := 0
	gotDeliveryID := ""
	githubReplayByDeliveryID = func(_ context.Context, deliveryID string) (bool, error) {
		called++
		gotDeliveryID = deliveryID
		return true, nil
	}

	if err := cmdGitHubReplay([]string{"--delivery-id", "delivery-123"}); err != nil {
		t.Fatalf("cmdGitHubReplay() error = %v", err)
	}
	if called != 1 {
		t.Fatalf("expected replay fn called once, got %d", called)
	}
	if gotDeliveryID != "delivery-123" {
		t.Fatalf("expected delivery id %q, got %q", "delivery-123", gotDeliveryID)
	}
}

func TestGitHubReplayCommand_IdempotentNoDoubleApply(t *testing.T) {
	origReplayFn := githubReplayByDeliveryID
	t.Cleanup(func() {
		githubReplayByDeliveryID = origReplayFn
	})

	replayCalls := 0
	applied := 0
	githubReplayByDeliveryID = func(_ context.Context, _ string) (bool, error) {
		replayCalls++
		if replayCalls == 1 {
			applied++
			return true, nil
		}
		return false, nil
	}

	if err := cmdGitHubReplay([]string{"--delivery-id", "delivery-idempotent"}); err != nil {
		t.Fatalf("first cmdGitHubReplay() error = %v", err)
	}
	if err := cmdGitHubReplay([]string{"--delivery-id", "delivery-idempotent"}); err != nil {
		t.Fatalf("second cmdGitHubReplay() error = %v", err)
	}
	if replayCalls != 2 {
		t.Fatalf("expected replay function called twice, got %d", replayCalls)
	}
	if applied != 1 {
		t.Fatalf("expected only one effective apply, got %d", applied)
	}
}
