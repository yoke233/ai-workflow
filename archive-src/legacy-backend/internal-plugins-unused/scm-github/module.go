package scmgithub

import (
	"fmt"

	"github.com/yoke233/ai-workflow/internal/core"
	ghsvc "github.com/yoke233/ai-workflow/internal/github"
)

func Module() core.PluginModule {
	return core.PluginModule{
		Name: "scm-github",
		Slot: core.SlotSCM,
		Factory: func(cfg map[string]any) (core.Plugin, error) {
			repoDir := "."
			if rawRepoDir, ok := cfg["repo_dir"]; ok {
				if value, ok := rawRepoDir.(string); ok && value != "" {
					repoDir = value
				}
			}

			rawService, ok := cfg["github_service"]
			if !ok {
				return nil, fmt.Errorf("scm-github requires github_service dependency")
			}
			service, ok := rawService.(*ghsvc.GitHubService)
			if !ok || service == nil {
				return nil, fmt.Errorf("scm-github requires valid github_service dependency")
			}

			opts := Options{}
			if rawDraft, ok := cfg["draft"]; ok {
				if draft, ok := rawDraft.(bool); ok {
					opts.DefaultDraft = draft
				}
			}
			if rawReviewers, ok := cfg["reviewers"]; ok {
				opts.DefaultReviewers = parseReviewers(rawReviewers)
			}

			return New(repoDir, service, opts), nil
		},
	}
}

func parseReviewers(raw any) []string {
	switch value := raw.(type) {
	case []string:
		return append([]string(nil), value...)
	case []any:
		out := make([]string, 0, len(value))
		for _, item := range value {
			text, ok := item.(string)
			if !ok {
				continue
			}
			out = append(out, text)
		}
		return out
	default:
		return nil
	}
}
