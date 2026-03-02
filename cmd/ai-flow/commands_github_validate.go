package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/yoke233/ai-workflow/internal/config"
	ghsvc "github.com/yoke233/ai-workflow/internal/github"
)

var probeGitHubAppPermissions = func(ctx context.Context, _ config.GitHubConfig) error {
	probe := ghsvc.NewPermissionsProbe(func(context.Context) (ghsvc.AppInstallationPermissions, error) {
		// Runtime probe wiring is extended in later waves. Keep strict validation semantics now.
		return ghsvc.AppInstallationPermissions{
			Issues:       "write",
			PullRequests: "write",
			Contents:     "write",
			Metadata:     "read",
		}, nil
	})
	return probe.Probe(ctx)
}

func cmdGitHubValidate(_ []string) error {
	cfg, err := loadBootstrapConfig()
	if err != nil {
		return err
	}
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	return validateGitHubConfigWithProbe(context.Background(), cfg.GitHub)
}

func validateGitHubConfigWithProbe(ctx context.Context, githubCfg config.GitHubConfig) error {
	if !githubCfg.Enabled {
		return nil
	}

	if strings.TrimSpace(githubCfg.WebhookSecret) == "" {
		return fmt.Errorf("github webhook_secret is required when github is enabled")
	}

	hasAppCredentials := githubCfg.AppID > 0 &&
		githubCfg.InstallationID > 0 &&
		strings.TrimSpace(githubCfg.PrivateKeyPath) != ""

	if hasAppCredentials {
		if err := probeGitHubAppPermissions(ctx, githubCfg); err != nil {
			return fmt.Errorf("github app permission probe failed: %w", err)
		}
		return nil
	}

	hasPAT := strings.TrimSpace(githubCfg.Token) != ""
	if hasPAT {
		if !githubCfg.AllowPATFallback {
			return fmt.Errorf("github pat mode requires github.allow_pat_fallback=true")
		}
		return nil
	}

	return fmt.Errorf("github credentials are required (app credentials preferred; pat requires allow_pat_fallback)")
}
