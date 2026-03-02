package secretary

import (
	"fmt"
	"strings"

	"github.com/user/ai-workflow/internal/acpclient"
)

var requiredReviewerRoles = []string{
	"completeness",
	"dependency",
	"feasibility",
}

type ReviewRoleBindingInput struct {
	Reviewers  map[string]string
	Aggregator string
}

type ReviewRoleRuntime struct {
	ReviewerRoles           map[string]string
	ReviewerSessionPolicies map[string]acpclient.SessionPolicy
	AggregatorRole          string
	AggregatorSessionPolicy acpclient.SessionPolicy
}

func ResolveReviewOrchestratorRoles(bindings ReviewRoleBindingInput, resolver *acpclient.RoleResolver) (*ReviewRoleRuntime, error) {
	out := &ReviewRoleRuntime{
		ReviewerRoles:           make(map[string]string, len(requiredReviewerRoles)),
		ReviewerSessionPolicies: make(map[string]acpclient.SessionPolicy, len(requiredReviewerRoles)),
	}

	for _, reviewer := range requiredReviewerRoles {
		roleID := strings.TrimSpace(bindings.Reviewers[reviewer])
		if roleID == "" {
			return nil, fmt.Errorf("review role binding is required for reviewer %q", reviewer)
		}

		policy := acpclient.SessionPolicy{
			Reuse:       true,
			ResetPrompt: true,
		}
		if resolver != nil {
			_, role, err := resolver.Resolve(roleID)
			if err != nil {
				return nil, fmt.Errorf("resolve review_orchestrator reviewer %q role %q: %w", reviewer, roleID, err)
			}
			policy = role.SessionPolicy
		}

		out.ReviewerRoles[reviewer] = roleID
		out.ReviewerSessionPolicies[reviewer] = policy
	}

	aggregatorRole := strings.TrimSpace(bindings.Aggregator)
	if aggregatorRole == "" {
		return nil, fmt.Errorf("review role binding is required for aggregator")
	}

	aggregatorPolicy := acpclient.SessionPolicy{
		Reuse:       true,
		ResetPrompt: true,
	}
	if resolver != nil {
		_, role, err := resolver.Resolve(aggregatorRole)
		if err != nil {
			return nil, fmt.Errorf("resolve review_orchestrator aggregator role %q: %w", aggregatorRole, err)
		}
		aggregatorPolicy = role.SessionPolicy
	}
	out.AggregatorRole = aggregatorRole
	out.AggregatorSessionPolicy = aggregatorPolicy
	return out, nil
}
