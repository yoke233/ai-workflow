package httpx

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/yoke233/ai-workflow/internal/platform/config"
)

const (
	ScopeAll   = "*"
	ScopeAdmin = "admin"
)

type authContextKey string

const authInfoKey authContextKey = "auth_info"

type AuthInfo struct {
	Role      string
	Scopes    []string
	Submitter string
	Projects  []string
}

func (a AuthInfo) HasScope(required string) bool {
	return scopeMatches(a.Scopes, required)
}

func AuthFromContext(ctx context.Context) (AuthInfo, bool) {
	info, ok := ctx.Value(authInfoKey).(AuthInfo)
	return info, ok
}

type TokenRegistry struct {
	entries map[string]tokenRegistryEntry
}

type tokenRegistryEntry struct {
	role      string
	scopes    []string
	submitter string
	projects  []string
}

func NewTokenRegistry(tokens map[string]config.TokenEntry) *TokenRegistry {
	entries := make(map[string]tokenRegistryEntry, len(tokens))
	for role, entry := range tokens {
		tok := strings.TrimSpace(entry.Token)
		if tok == "" {
			continue
		}
		entries[tok] = tokenRegistryEntry{
			role:      role,
			scopes:    entry.Scopes,
			submitter: entry.Submitter,
			projects:  entry.Projects,
		}
	}
	return &TokenRegistry{entries: entries}
}

func (r *TokenRegistry) Lookup(token string) (AuthInfo, bool) {
	if r == nil || len(r.entries) == 0 {
		return AuthInfo{}, false
	}
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return AuthInfo{}, false
	}
	for registered, entry := range r.entries {
		if subtle.ConstantTimeCompare([]byte(trimmed), []byte(registered)) == 1 {
			return AuthInfo{
				Role:      entry.role,
				Scopes:    entry.scopes,
				Submitter: entry.submitter,
				Projects:  entry.projects,
			}, true
		}
	}
	return AuthInfo{}, false
}

func (r *TokenRegistry) IsEmpty() bool {
	return r == nil || len(r.entries) == 0
}

func TokenAuthMiddleware(registry *TokenRegistry) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractRequestToken(r)
			if token == "" {
				WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
			info, ok := registry.Lookup(token)
			if !ok {
				WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), authInfoKey, info)))
		})
	}
}

func RequireScope(scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			info, ok := AuthFromContext(r.Context())
			if !ok {
				next.ServeHTTP(w, r)
				return
			}
			if !info.HasScope(scope) {
				WriteJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden", "required_scope": scope})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func extractRequestToken(r *http.Request) string {
	if tok := strings.TrimSpace(r.URL.Query().Get("token")); tok != "" {
		return tok
	}
	auth := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(auth) < len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(auth[len(prefix):])
}

func scopeMatches(userScopes []string, required string) bool {
	for _, s := range userScopes {
		if s == ScopeAll || s == required {
			return true
		}
		if strings.HasSuffix(s, ":*") {
			prefix := strings.TrimSuffix(s, "*")
			if strings.HasPrefix(required, prefix) {
				return true
			}
		}
	}
	return false
}


