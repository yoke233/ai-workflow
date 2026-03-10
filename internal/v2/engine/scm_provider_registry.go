package engine

import (
	"context"
	"os"
	"strings"
)

func newChangeRequestProviders(token string) []ChangeRequestProvider {
	return []ChangeRequestProvider{
		NewGitHubProvider(token),
		NewCodeupProvider(CodeupProviderConfig{
			Token:          token,
			Domain:         strings.TrimSpace(os.Getenv("AI_WORKFLOW_CODEUP_DOMAIN")),
			OrganizationID: strings.TrimSpace(os.Getenv("AI_WORKFLOW_CODEUP_ORGANIZATION_ID")),
		}),
	}
}

func detectChangeRequestProvider(ctx context.Context, originURL string, providers []ChangeRequestProvider) (ChangeRequestProvider, ChangeRequestRepo, bool, error) {
	for _, provider := range providers {
		if provider == nil {
			continue
		}
		repo, ok, err := provider.Detect(ctx, originURL)
		if err != nil {
			return nil, ChangeRequestRepo{}, false, err
		}
		if ok {
			return provider, repo, true, nil
		}
	}
	return nil, ChangeRequestRepo{}, false, nil
}
