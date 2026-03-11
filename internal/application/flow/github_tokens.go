package flow

import "strings"

// GitHubTokens carries optional PATs used by builtin PR automation steps.
// Tokens should be loaded from secrets.toml and never written to artifacts/events/logs.
type GitHubTokens struct {
	CommitPAT string
	MergePAT  string
}

func (t GitHubTokens) EffectiveCommitPAT() string {
	// For local end-to-end tests we prefer using MergePAT for everything when provided
	// (it is expected to be a superset of CommitPAT permissions).
	// This also simplifies retry loops that may require both push and merge.
	if v := strings.TrimSpace(t.MergePAT); v != "" {
		return v
	}
	return strings.TrimSpace(t.CommitPAT)
}

func (t GitHubTokens) EffectiveMergePAT() string {
	if v := strings.TrimSpace(t.MergePAT); v != "" {
		return v
	}
	return strings.TrimSpace(t.CommitPAT)
}
