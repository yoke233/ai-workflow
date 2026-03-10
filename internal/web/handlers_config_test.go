package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yoke233/ai-workflow/internal/config"
	"github.com/yoke233/ai-workflow/internal/configruntime"
	"github.com/yoke233/ai-workflow/internal/teamleader"
)

func TestAdminConfig_GetTomlAndStatus(t *testing.T) {
	runtimeManager, initialRaw := newRuntimeManagerForTest(t)

	srv := NewServer(Config{Auth: testAdminAuthRegistry(), RuntimeConfig: runtimeManager})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/admin/config/toml", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer admin-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/v1/admin/config/toml: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var payload adminConfigTomlResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Content != initialRaw {
		t.Fatalf("content mismatch")
	}

	req, err = http.NewRequest(http.MethodGet, ts.URL+"/api/v1/admin/config/runtime-status", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer admin-token")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/v1/admin/config/runtime-status: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var status adminConfigRuntimeStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if status.ActiveVersion < 1 {
		t.Fatalf("expected active version >= 1, got %d", status.ActiveVersion)
	}
}

func TestAdminConfig_PutTomlRejectsInvalidAndKeepsOldConfig(t *testing.T) {
	runtimeManager, initialRaw := newRuntimeManagerForTest(t)

	srv := NewServer(Config{Auth: testAdminAuthRegistry(), RuntimeConfig: runtimeManager})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp := putJSON(t, ts.URL+"/api/v1/admin/config/toml", map[string]any{
		"content": "v2 = [",
	}, "admin-token")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	nextRaw, err := runtimeManager.ReadRawString()
	if err != nil {
		t.Fatalf("ReadRawString() error = %v", err)
	}
	if nextRaw != initialRaw {
		t.Fatalf("expected config.toml to remain unchanged")
	}
}

func TestAdminConfig_PutTomlAppliesValidChange(t *testing.T) {
	runtimeManager, initialRaw := newRuntimeManagerForTest(t)
	updatedRaw := strings.Replace(initialRaw, "max_retries = 2", "max_retries = 7", 1)

	srv := NewServer(Config{Auth: testAdminAuthRegistry(), RuntimeConfig: runtimeManager})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp := putJSON(t, ts.URL+"/api/v1/admin/config/toml", map[string]any{
		"content": updatedRaw,
	}, "admin-token")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var payload adminConfigUpdateResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Status != "ok" {
		t.Fatalf("unexpected status %q", payload.Status)
	}
	if payload.RuntimeStatus.ActiveVersion < 2 {
		t.Fatalf("expected active version >= 2, got %d", payload.RuntimeStatus.ActiveVersion)
	}

	nextRaw, err := runtimeManager.ReadRawString()
	if err != nil {
		t.Fatalf("ReadRawString() error = %v", err)
	}
	if !strings.Contains(nextRaw, "max_retries = 7") {
		t.Fatalf("expected updated config to be persisted")
	}
}

func TestAdminConfig_PutV2RuntimeUpdatesRuntime(t *testing.T) {
	runtimeManager, _ := newRuntimeManagerForTest(t)

	srv := NewServer(Config{Auth: testAdminAuthRegistry(), RuntimeConfig: runtimeManager})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	body := map[string]any{
		"agents": map[string]any{
			"drivers": []map[string]any{{
				"id":             "worker-driver",
				"launch_command": "npx",
				"launch_args":    []string{"-y", "@zed-industries/codex-acp"},
			}},
			"profiles": []map[string]any{{
				"id":      "worker-default",
				"name":    "Worker Default",
				"driver":  "worker-driver",
				"role":    "worker",
				"session": map[string]any{"reuse": true, "max_turns": 12, "idle_ttl": "5m"},
			}},
		},
		"mcp": map[string]any{
			"servers": []map[string]any{{
				"id":        "ai-workflow-query",
				"name":      "ai-workflow-query",
				"kind":      "internal",
				"transport": "sse",
				"enabled":   true,
			}},
			"profile_bindings": []map[string]any{{
				"profile":   "worker-default",
				"server":    "ai-workflow-query",
				"enabled":   true,
				"tool_mode": "all",
			}},
		},
	}

	resp := putJSON(t, ts.URL+"/api/v1/admin/config/v2-runtime", body, "admin-token")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	current := runtimeManager.GetV2Runtime()
	if len(current.Agents.Drivers) != 1 || current.Agents.Drivers[0].ID != "worker-driver" {
		t.Fatalf("unexpected drivers: %+v", current.Agents.Drivers)
	}
	if len(current.MCP.ProfileBindings) != 1 || current.MCP.ProfileBindings[0].Profile != "worker-default" {
		t.Fatalf("unexpected bindings: %+v", current.MCP.ProfileBindings)
	}
}

func TestAdminConfig_AdminScopeRequired(t *testing.T) {
	runtimeManager, _ := newRuntimeManagerForTest(t)
	auth := NewTokenRegistry(map[string]config.TokenEntry{
		"viewer": {Token: "viewer-token", Scopes: []string{ScopeProjectsRead}},
	})
	srv := NewServer(Config{Auth: auth, RuntimeConfig: runtimeManager})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/admin/config/runtime-status", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer viewer-token")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func newRuntimeManagerForTest(t *testing.T) (*configruntime.Manager, string) {
	t.Helper()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	secretsPath := filepath.Join(dir, "secrets.toml")
	initialRaw := string(config.DefaultsTOML())
	if err := os.WriteFile(cfgPath, []byte(initialRaw), 0o644); err != nil {
		t.Fatalf("write config.toml: %v", err)
	}
	if err := config.SaveSecrets(secretsPath, &config.Secrets{
		Tokens: map[string]config.TokenEntry{
			"admin": {Token: "runtime-admin-token", Scopes: []string{"*"}},
		},
	}); err != nil {
		t.Fatalf("write secrets.toml: %v", err)
	}

	runtimeManager, err := configruntime.NewManager(cfgPath, secretsPath, teamleader.MCPEnvConfig{}, nil, nil)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	return runtimeManager, initialRaw
}

func testAdminAuthRegistry() *TokenRegistry {
	return NewTokenRegistry(map[string]config.TokenEntry{
		"admin": {Token: "admin-token", Scopes: []string{ScopeAdmin}},
	})
}

func putJSON(t *testing.T, url string, body map[string]any, bearerToken string) *http.Response {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal json body: %v", err)
	}
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	return resp
}
