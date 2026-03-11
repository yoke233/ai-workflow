package github

import (
	"context"
	"strings"
	"testing"
)

func TestPermissionsProbe_MissingIssueWrite_FailsValidation(t *testing.T) {
	probe := NewPermissionsProbe(func(context.Context) (AppInstallationPermissions, error) {
		return AppInstallationPermissions{
			Issues:       "read",
			PullRequests: "write",
			Contents:     "write",
			Metadata:     "read",
		}, nil
	})

	err := probe.Probe(context.Background())
	if err == nil {
		t.Fatal("expected permissions probe error")
	}
	if !strings.Contains(err.Error(), "issues=write") {
		t.Fatalf("expected issues permission error, got %v", err)
	}
}

func TestPermissionsProbe_MissingPRWrite_FailsValidation(t *testing.T) {
	probe := NewPermissionsProbe(func(context.Context) (AppInstallationPermissions, error) {
		return AppInstallationPermissions{
			Issues:       "write",
			PullRequests: "read",
			Contents:     "write",
			Metadata:     "read",
		}, nil
	})

	err := probe.Probe(context.Background())
	if err == nil {
		t.Fatal("expected permissions probe error")
	}
	if !strings.Contains(err.Error(), "pull_requests=write") {
		t.Fatalf("expected pull_requests permission error, got %v", err)
	}
}

func TestPermissionsProbe_GitHubAppInstallationPasses(t *testing.T) {
	probe := NewPermissionsProbe(func(context.Context) (AppInstallationPermissions, error) {
		return AppInstallationPermissions{
			Issues:       "write",
			PullRequests: "write",
			Contents:     "write",
			Metadata:     "read",
		}, nil
	})

	if err := probe.Probe(context.Background()); err != nil {
		t.Fatalf("Probe() error = %v", err)
	}
}
