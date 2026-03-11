package github

import (
	"context"
	"fmt"
	"strings"
)

// AppInstallationPermissions is normalized GitHub App installation permission set.
type AppInstallationPermissions struct {
	Issues       string
	PullRequests string
	Contents     string
	Metadata     string
}

// PermissionsProbe fetches and validates required GitHub App installation permissions.
type PermissionsProbe struct {
	fetch func(context.Context) (AppInstallationPermissions, error)
}

// NewPermissionsProbe builds a permissions probe from a fetch function.
func NewPermissionsProbe(fetch func(context.Context) (AppInstallationPermissions, error)) *PermissionsProbe {
	return &PermissionsProbe{fetch: fetch}
}

// Probe validates installation permissions for issues/pr/contents/metadata.
func (p *PermissionsProbe) Probe(ctx context.Context) error {
	if p == nil || p.fetch == nil {
		return fmt.Errorf("permissions probe fetcher is not configured")
	}

	permissions, err := p.fetch(ctx)
	if err != nil {
		return err
	}
	if err := validateRequiredPermissions(permissions); err != nil {
		return err
	}
	return nil
}

func validateRequiredPermissions(permissions AppInstallationPermissions) error {
	if !isWritePermission(permissions.Issues) {
		return fmt.Errorf("missing required github app permission: issues=write")
	}
	if !isWritePermission(permissions.PullRequests) {
		return fmt.Errorf("missing required github app permission: pull_requests=write")
	}
	if !isWritePermission(permissions.Contents) {
		return fmt.Errorf("missing required github app permission: contents=write")
	}
	metadata := strings.ToLower(strings.TrimSpace(permissions.Metadata))
	if metadata != "read" && metadata != "write" {
		return fmt.Errorf("missing required github app permission: metadata=read")
	}
	return nil
}

func isWritePermission(raw string) bool {
	return strings.EqualFold(strings.TrimSpace(raw), "write")
}
