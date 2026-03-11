package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/yoke233/ai-workflow/internal/config"
)

func TestGitHubValidate_MissingIssueWrite_FailsValidation(t *testing.T) {
	origProbe := probeGitHubAppPermissions
	t.Cleanup(func() {
		probeGitHubAppPermissions = origProbe
	})
	probeGitHubAppPermissions = func(context.Context, config.GitHubConfig) error {
		return errors.New("missing required github app permission: issues=write")
	}

	err := validateGitHubConfigWithProbe(context.Background(), config.GitHubConfig{
		Enabled:        true,
		WebhookSecret:  "secret",
		AppID:          1,
		InstallationID: 2,
		PrivateKeyPath: "key.pem",
	})
	if err == nil {
		t.Fatal("expected validate error")
	}
	if !strings.Contains(err.Error(), "issues=write") {
		t.Fatalf("expected issues permission error, got %v", err)
	}
}

func TestGitHubValidate_MissingPRWrite_FailsValidation(t *testing.T) {
	origProbe := probeGitHubAppPermissions
	t.Cleanup(func() {
		probeGitHubAppPermissions = origProbe
	})
	probeGitHubAppPermissions = func(context.Context, config.GitHubConfig) error {
		return errors.New("missing required github app permission: pull_requests=write")
	}

	err := validateGitHubConfigWithProbe(context.Background(), config.GitHubConfig{
		Enabled:        true,
		WebhookSecret:  "secret",
		AppID:          1,
		InstallationID: 2,
		PrivateKeyPath: "key.pem",
	})
	if err == nil {
		t.Fatal("expected validate error")
	}
	if !strings.Contains(err.Error(), "pull_requests=write") {
		t.Fatalf("expected pull_requests permission error, got %v", err)
	}
}

func TestGitHubValidate_GitHubAppInstallationPasses(t *testing.T) {
	origProbe := probeGitHubAppPermissions
	t.Cleanup(func() {
		probeGitHubAppPermissions = origProbe
	})
	probeGitHubAppPermissions = func(context.Context, config.GitHubConfig) error {
		return nil
	}

	if err := validateGitHubConfigWithProbe(context.Background(), config.GitHubConfig{
		Enabled:        true,
		WebhookSecret:  "secret",
		AppID:          1,
		InstallationID: 2,
		PrivateKeyPath: "key.pem",
	}); err != nil {
		t.Fatalf("validateGitHubConfigWithProbe() error = %v", err)
	}
}

func TestCommand_GitHubValidate_InvalidConfig_Fails(t *testing.T) {
	err := validateGitHubConfigWithProbe(context.Background(), config.GitHubConfig{
		Enabled: true,
		// webhook secret intentionally missing
	})
	if err == nil {
		t.Fatal("expected invalid config validation to fail")
	}
	if !strings.Contains(err.Error(), "webhook_secret") {
		t.Fatalf("expected webhook secret validation error, got %v", err)
	}
}
