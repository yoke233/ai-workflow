package web

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/user/ai-workflow/internal/core"
)

func TestWebhook_VerifySignature_Success(t *testing.T) {
	store := newTestStore(t)
	project := core.Project{
		ID:          "proj-webhook-signature-success",
		Name:        "webhook-signature-success",
		RepoPath:    filepath.Join(t.TempDir(), "repo-signature-success"),
		GitHubOwner: "acme",
		GitHubRepo:  "ai-workflow",
	}
	if err := store.CreateProject(&project); err != nil {
		t.Fatalf("seed project: %v", err)
	}

	srv := NewServer(Config{
		Store:         store,
		AuthEnabled:   true,
		BearerToken:   "api-token",
		WebhookSecret: "webhook-secret",
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	payload := readWebhookFixture(t, "github_issues_opened.json")
	resp := doWebhookRequest(t, ts, payload, "issues", signWebhookPayload("webhook-secret", payload))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 202, got %d, body=%s", resp.StatusCode, string(body))
	}
}

func TestWebhook_VerifySignature_Invalid_Returns401(t *testing.T) {
	store := newTestStore(t)
	project := core.Project{
		ID:          "proj-webhook-invalid-signature",
		Name:        "webhook-invalid-signature",
		RepoPath:    filepath.Join(t.TempDir(), "repo-invalid-signature"),
		GitHubOwner: "acme",
		GitHubRepo:  "ai-workflow",
	}
	if err := store.CreateProject(&project); err != nil {
		t.Fatalf("seed project: %v", err)
	}

	srv := NewServer(Config{
		Store:         store,
		AuthEnabled:   true,
		BearerToken:   "api-token",
		WebhookSecret: "webhook-secret",
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	payload := readWebhookFixture(t, "github_issues_opened.json")
	resp := doWebhookRequest(t, ts, payload, "issues", "sha256=invalid")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 401, got %d, body=%s", resp.StatusCode, string(body))
	}
}

func TestWebhook_ProjectRouting_UsesOwnerRepo(t *testing.T) {
	store := newTestStore(t)
	project := core.Project{
		ID:          "proj-webhook-routing",
		Name:        "webhook-routing",
		RepoPath:    filepath.Join(t.TempDir(), "repo-routing"),
		GitHubOwner: "acme",
		GitHubRepo:  "ai-workflow",
	}
	if err := store.CreateProject(&project); err != nil {
		t.Fatalf("seed project: %v", err)
	}

	srv := NewServer(Config{
		Store:         store,
		WebhookSecret: "webhook-secret",
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	matchedPayload := readWebhookFixture(t, "github_issues_opened.json")
	matchedResp := doWebhookRequest(t, ts, matchedPayload, "issues", signWebhookPayload("webhook-secret", matchedPayload))
	defer matchedResp.Body.Close()
	if matchedResp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(matchedResp.Body)
		t.Fatalf("expected matched payload to return 202, got %d, body=%s", matchedResp.StatusCode, string(body))
	}

	unmatchedPayload := withRepositoryOwnerRepo(t, matchedPayload, "other-org", "ai-workflow")
	unmatchedResp := doWebhookRequest(t, ts, unmatchedPayload, "issues", signWebhookPayload("webhook-secret", unmatchedPayload))
	defer unmatchedResp.Body.Close()
	if unmatchedResp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(unmatchedResp.Body)
		t.Fatalf("expected unmatched owner/repo to return 404, got %d, body=%s", unmatchedResp.StatusCode, string(body))
	}
}

func TestWebhook_UnsupportedEvent_Returns202(t *testing.T) {
	store := newTestStore(t)
	project := core.Project{
		ID:          "proj-webhook-unsupported-event",
		Name:        "webhook-unsupported-event",
		RepoPath:    filepath.Join(t.TempDir(), "repo-unsupported-event"),
		GitHubOwner: "acme",
		GitHubRepo:  "ai-workflow",
	}
	if err := store.CreateProject(&project); err != nil {
		t.Fatalf("seed project: %v", err)
	}

	srv := NewServer(Config{
		Store:         store,
		WebhookSecret: "webhook-secret",
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	payload := readWebhookFixture(t, "github_issue_comment_created.json")
	resp := doWebhookRequest(t, ts, payload, "pull_request_review", signWebhookPayload("webhook-secret", payload))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 202 for unsupported event, got %d, body=%s", resp.StatusCode, string(body))
	}
}

func TestWebhook_IssueCommentSlashReject_AppliesPipelineAction(t *testing.T) {
	store := newTestStore(t)
	project := core.Project{
		ID:          "proj-webhook-slash-reject",
		Name:        "webhook-slash-reject",
		RepoPath:    filepath.Join(t.TempDir(), "repo-slash-reject"),
		GitHubOwner: "acme",
		GitHubRepo:  "ai-workflow",
	}
	if err := store.CreateProject(&project); err != nil {
		t.Fatalf("seed project: %v", err)
	}

	now := time.Now()
	pipeline := &core.Pipeline{
		ID:              "pipe-webhook-slash-reject",
		ProjectID:       project.ID,
		Name:            "slash reject",
		Description:     "slash reject",
		Template:        "standard",
		Status:          core.StatusWaitingHuman,
		CurrentStage:    core.StageImplement,
		Stages:          []core.StageConfig{{Name: core.StageImplement, Agent: "codex"}},
		Artifacts:       map[string]string{},
		Config:          map[string]any{"issue_number": 42},
		MaxTotalRetries: 5,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := store.SavePipeline(pipeline); err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}

	var gotAction core.PipelineAction
	executor := &testPipelineExecutor{
		applyActionFn: func(_ context.Context, action core.PipelineAction) error {
			gotAction = action
			return nil
		},
	}

	srv := NewServer(Config{
		Store:         store,
		PipelineExec:  executor,
		WebhookSecret: "webhook-secret",
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	payload := readWebhookFixture(t, "github_issue_comment_created.json")
	payload = withIssueComment(t, payload, "/reject implement 需要补测试", "OWNER")

	resp := doWebhookRequest(t, ts, payload, "issue_comment", signWebhookPayload("webhook-secret", payload))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 202, got %d, body=%s", resp.StatusCode, string(body))
	}

	if gotAction.PipelineID != pipeline.ID {
		t.Fatalf("expected pipeline id %q, got %q", pipeline.ID, gotAction.PipelineID)
	}
	if gotAction.Type != core.ActionReject {
		t.Fatalf("expected action reject, got %q", gotAction.Type)
	}
	if gotAction.Stage != core.StageImplement {
		t.Fatalf("expected stage implement, got %q", gotAction.Stage)
	}
	if gotAction.Message != "需要补测试" {
		t.Fatalf("expected slash reason, got %q", gotAction.Message)
	}
}

func TestWebhook_IssueCommentSlashUnauthorized_NoAction(t *testing.T) {
	store := newTestStore(t)
	project := core.Project{
		ID:          "proj-webhook-slash-unauthorized",
		Name:        "webhook-slash-unauthorized",
		RepoPath:    filepath.Join(t.TempDir(), "repo-slash-unauthorized"),
		GitHubOwner: "acme",
		GitHubRepo:  "ai-workflow",
	}
	if err := store.CreateProject(&project); err != nil {
		t.Fatalf("seed project: %v", err)
	}

	now := time.Now()
	pipeline := &core.Pipeline{
		ID:              "pipe-webhook-slash-unauthorized",
		ProjectID:       project.ID,
		Name:            "slash unauthorized",
		Description:     "slash unauthorized",
		Template:        "standard",
		Status:          core.StatusWaitingHuman,
		CurrentStage:    core.StageCodeReview,
		Stages:          []core.StageConfig{{Name: core.StageCodeReview, Agent: "claude"}},
		Artifacts:       map[string]string{},
		Config:          map[string]any{"issue_number": 42},
		MaxTotalRetries: 5,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := store.SavePipeline(pipeline); err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}

	applied := false
	executor := &testPipelineExecutor{
		applyActionFn: func(_ context.Context, action core.PipelineAction) error {
			applied = true
			return nil
		},
	}

	srv := NewServer(Config{
		Store:         store,
		PipelineExec:  executor,
		WebhookSecret: "webhook-secret",
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	payload := readWebhookFixture(t, "github_issue_comment_created.json")
	payload = withIssueComment(t, payload, "/approve", "NONE")

	resp := doWebhookRequest(t, ts, payload, "issue_comment", signWebhookPayload("webhook-secret", payload))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 202, got %d, body=%s", resp.StatusCode, string(body))
	}

	if applied {
		t.Fatal("expected unauthorized slash command to be ignored")
	}
}

func TestWebhook_IssueCommentSlashRun_CreatesPipeline(t *testing.T) {
	store := newTestStore(t)
	project := core.Project{
		ID:          "proj-webhook-slash-run",
		Name:        "webhook-slash-run",
		RepoPath:    filepath.Join(t.TempDir(), "repo-slash-run"),
		GitHubOwner: "acme",
		GitHubRepo:  "ai-workflow",
	}
	if err := store.CreateProject(&project); err != nil {
		t.Fatalf("seed project: %v", err)
	}

	srv := NewServer(Config{
		Store:         store,
		WebhookSecret: "webhook-secret",
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	payload := readWebhookFixture(t, "github_issue_comment_created.json")
	payload = withIssueComment(t, payload, "/run hotfix", "OWNER")
	payload = withIssueNumber(t, payload, 77)

	resp := doWebhookRequest(t, ts, payload, "issue_comment", signWebhookPayload("webhook-secret", payload))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 202, got %d, body=%s", resp.StatusCode, string(body))
	}

	pipelines, err := store.ListPipelines(project.ID, core.PipelineFilter{Limit: 20})
	if err != nil {
		t.Fatalf("ListPipelines() error = %v", err)
	}
	if len(pipelines) != 1 {
		t.Fatalf("expected one pipeline created by /run, got %d", len(pipelines))
	}
	created, err := store.GetPipeline(pipelines[0].ID)
	if err != nil {
		t.Fatalf("GetPipeline() error = %v", err)
	}
	if created.Template != "hotfix" {
		t.Fatalf("expected template hotfix, got %q", created.Template)
	}
	issueNumber := 0
	if created.Config != nil {
		switch raw := created.Config["issue_number"].(type) {
		case int:
			issueNumber = raw
		case float64:
			issueNumber = int(raw)
		case string:
			parsed, parseErr := strconv.Atoi(strings.TrimSpace(raw))
			if parseErr == nil {
				issueNumber = parsed
			}
		}
	}
	if issueNumber != 77 {
		t.Fatalf("expected issue_number 77, got %d", issueNumber)
	}
}

func doWebhookRequest(t *testing.T, ts *httptest.Server, payload []byte, event, signature string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/webhook", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("create webhook request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", event)
	req.Header.Set("X-Hub-Signature-256", signature)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("send webhook request: %v", err)
	}
	return resp
}

func readWebhookFixture(t *testing.T, name string) []byte {
	t.Helper()

	content, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return content
}

func signWebhookPayload(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func withRepositoryOwnerRepo(t *testing.T, payload []byte, owner, repo string) []byte {
	t.Helper()

	var body map[string]any
	if err := json.Unmarshal(payload, &body); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	repositoryRaw, ok := body["repository"]
	if !ok {
		t.Fatal("payload does not contain repository field")
	}
	repository, ok := repositoryRaw.(map[string]any)
	if !ok {
		t.Fatal("payload repository field has unexpected shape")
	}

	ownerRaw, ok := repository["owner"]
	if !ok {
		t.Fatal("payload repository does not contain owner field")
	}
	repositoryOwner, ok := ownerRaw.(map[string]any)
	if !ok {
		t.Fatal("payload repository owner field has unexpected shape")
	}

	repositoryOwner["login"] = owner
	repository["name"] = repo

	updated, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal updated payload: %v", err)
	}
	return updated
}

func withIssueComment(t *testing.T, payload []byte, commentBody string, authorAssociation string) []byte {
	t.Helper()

	var body map[string]any
	if err := json.Unmarshal(payload, &body); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	commentRaw, ok := body["comment"]
	if !ok {
		t.Fatal("payload does not contain comment field")
	}
	comment, ok := commentRaw.(map[string]any)
	if !ok {
		t.Fatal("payload comment field has unexpected shape")
	}

	comment["body"] = commentBody
	comment["author_association"] = authorAssociation

	updated, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal updated payload: %v", err)
	}
	return updated
}

func withIssueNumber(t *testing.T, payload []byte, issueNumber int) []byte {
	t.Helper()

	var body map[string]any
	if err := json.Unmarshal(payload, &body); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	issueRaw, ok := body["issue"]
	if !ok {
		t.Fatal("payload does not contain issue field")
	}
	issue, ok := issueRaw.(map[string]any)
	if !ok {
		t.Fatal("payload issue field has unexpected shape")
	}

	issue["number"] = issueNumber

	updated, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal updated payload: %v", err)
	}
	return updated
}
