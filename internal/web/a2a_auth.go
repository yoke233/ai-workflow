package web

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const a2aIdentityKey contextKey = "a2a_identity"

// A2AAuthConfig defines token-based auth for the A2A endpoint.
type A2AAuthConfig struct {
	Tokens   map[string]A2ATokenEntry `yaml:"tokens"`
	Policies map[string]A2APolicy     `yaml:"policies"`
}

// A2ATokenEntry maps a bearer token to a submitter identity.
type A2ATokenEntry struct {
	Submitter string   `yaml:"submitter"`
	Role      string   `yaml:"role"`
	Projects  []string `yaml:"projects"` // empty = all projects
}

// A2APolicy defines what operations a role can perform.
type A2APolicy struct {
	Operations []string `yaml:"operations"` // "send", "get", "cancel", "list"
	DataScope  string   `yaml:"data_scope"` // "full" | "summary"
}

// A2AIdentity is the resolved identity for an authenticated A2A request.
type A2AIdentity struct {
	Submitter  string
	Role       string
	Projects   []string
	Operations []string
	DataScope  string
}

// CanA2AOperation returns true if the identity is allowed to perform the operation.
func (id A2AIdentity) CanA2AOperation(op string) bool {
	if len(id.Operations) == 0 {
		return true
	}
	for _, allowed := range id.Operations {
		if allowed == op {
			return true
		}
	}
	return false
}

// HasProjectAccess returns true if the identity can access the given project.
func (id A2AIdentity) HasProjectAccess(projectID string) bool {
	if len(id.Projects) == 0 {
		return true // empty = all projects
	}
	for _, p := range id.Projects {
		if p == projectID {
			return true
		}
	}
	return false
}

// A2AIdentityFromContext extracts the A2A identity from context.
func A2AIdentityFromContext(ctx context.Context) (A2AIdentity, bool) {
	id, ok := ctx.Value(a2aIdentityKey).(A2AIdentity)
	return id, ok
}

// A2AAuthMiddleware returns middleware that authenticates A2A requests.
// If auth config is nil, it falls back to simple bearer token validation.
func A2AAuthMiddleware(legacyToken string, auth *A2AAuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractBearerToken(r)

			// Token-based auth with identity resolution
			if auth != nil && len(auth.Tokens) > 0 {
				if token == "" {
					http.Error(w, "unauthorized", http.StatusUnauthorized)
					return
				}
				identity, ok := resolveA2AIdentity(token, auth)
				if !ok {
					http.Error(w, "unauthorized", http.StatusUnauthorized)
					return
				}
				ctx := context.WithValue(r.Context(), a2aIdentityKey, identity)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Legacy simple token check
			if legacyToken != "" && token != legacyToken {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// extractBearerToken extracts the bearer token from the Authorization header.
func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	const prefix = "Bearer "
	if len(auth) < len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(auth[len(prefix):])
}

// resolveA2AIdentity looks up a token in the auth config and returns the identity.
func resolveA2AIdentity(token string, auth *A2AAuthConfig) (A2AIdentity, bool) {
	if auth == nil {
		return A2AIdentity{}, false
	}
	entry, ok := auth.Tokens[token]
	if !ok {
		return A2AIdentity{}, false
	}

	identity := A2AIdentity{
		Submitter: entry.Submitter,
		Role:      entry.Role,
		Projects:  entry.Projects,
	}

	if policy, ok := auth.Policies[entry.Role]; ok {
		identity.Operations = policy.Operations
		identity.DataScope = policy.DataScope
	}

	return identity, true
}
