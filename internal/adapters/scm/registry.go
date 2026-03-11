package scm

import (
	"context"
	"os"
	"strings"

	flowapp "github.com/yoke233/ai-workflow/internal/application/flow"
)

func NewChangeRequestProviders(token string) []flowapp.ChangeRequestProvider {
	return []flowapp.ChangeRequestProvider{
		NewGitHubProvider(token),
		NewCodeupProvider(CodeupProviderConfig{
			Token:          token,
			Domain:         strings.TrimSpace(os.Getenv("AI_WORKFLOW_CODEUP_DOMAIN")),
			OrganizationID: strings.TrimSpace(os.Getenv("AI_WORKFLOW_CODEUP_ORGANIZATION_ID")),
		}),
	}
}

func DetectChangeRequestProvider(ctx context.Context, originURL string, providers []flowapp.ChangeRequestProvider) (flowapp.ChangeRequestProvider, flowapp.ChangeRequestRepo, bool, error) {
	for _, provider := range providers {
		if provider == nil {
			continue
		}
		repo, ok, err := provider.Detect(ctx, originURL)
		if err != nil {
			return nil, flowapp.ChangeRequestRepo{}, false, err
		}
		if ok {
			return provider, repo, true, nil
		}
	}
	return nil, flowapp.ChangeRequestRepo{}, false, nil
}
