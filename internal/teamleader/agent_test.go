package teamleader

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	acpproto "github.com/coder/acp-go-sdk"
	"github.com/yoke233/ai-workflow/internal/acpclient"
	"github.com/yoke233/ai-workflow/internal/core"
)

type mockAgent struct {
	opts   []core.ExecOpts
	cmd    []string
	parser core.StreamParser
}

func (a *mockAgent) Name() string { return "mock-agent" }

func (a *mockAgent) Init(context.Context) error { return nil }

func (a *mockAgent) Close() error { return nil }

func (a *mockAgent) BuildCommand(opts core.ExecOpts) ([]string, error) {
	a.opts = append(a.opts, opts)
	if len(a.cmd) == 0 {
		return []string{"mock"}, nil
	}
	return a.cmd, nil
}

func (a *mockAgent) NewStreamParser(io.Reader) core.StreamParser {
	return a.parser
}

type fakeRuntime struct {
	lastOpts core.RuntimeOpts
	session  *core.Session
}

func (r *fakeRuntime) Name() string { return "fake-runtime" }

func (r *fakeRuntime) Init(context.Context) error { return nil }

func (r *fakeRuntime) Close() error { return nil }

func (r *fakeRuntime) Kill(string) error { return nil }

func (r *fakeRuntime) Create(_ context.Context, opts core.RuntimeOpts) (*core.Session, error) {
	r.lastOpts = opts
	return r.session, nil
}

type sliceParser struct {
	events []*core.StreamEvent
	index  int
}

func (p *sliceParser) Next() (*core.StreamEvent, error) {
	if p.index >= len(p.events) {
		return nil, io.EOF
	}
	evt := p.events[p.index]
	p.index++
	return evt, nil
}

type nopWriteCloser struct{}

func (nopWriteCloser) Write(data []byte) (int, error) { return len(data), nil }

func (nopWriteCloser) Close() error { return nil }

type stubTeamLeaderSessionClient struct {
	loadReqs []acpproto.LoadSessionRequest
	newReqs  []acpproto.NewSessionRequest
	calls    []string
	loadResp acpproto.SessionId
	loadErr  error
	newResp  acpproto.SessionId
	newErr   error
}

func (c *stubTeamLeaderSessionClient) LoadSession(_ context.Context, req acpproto.LoadSessionRequest) (acpproto.SessionId, error) {
	c.calls = append(c.calls, "load")
	c.loadReqs = append(c.loadReqs, req)
	if c.loadErr != nil {
		return "", c.loadErr
	}
	return c.loadResp, nil
}

func (c *stubTeamLeaderSessionClient) NewSession(_ context.Context, req acpproto.NewSessionRequest) (acpproto.SessionId, error) {
	c.calls = append(c.calls, "new")
	c.newReqs = append(c.newReqs, req)
	if c.newErr != nil {
		return "", c.newErr
	}
	return c.newResp, nil
}

func TestAgentDecomposeBuildsPromptAndReturnsRawOutput(t *testing.T) {
	waitCalled := false
	runtime := &fakeRuntime{
		session: &core.Session{
			ID:     "session-1",
			Stdin:  nopWriteCloser{},
			Stdout: strings.NewReader(""),
			Stderr: strings.NewReader(""),
			Wait: func() error {
				waitCalled = true
				return nil
			},
		},
	}

	output := "```json\n{\n  \"name\": \"oauth-rollout\",\n  \"tasks\": [\n    {\n      \"id\": \"task-1\",\n      \"title\": \"后端接入 OAuth\",\n      \"description\": \"完成 OAuth 登录接口并补充单测。\",\n      \"labels\": [\"backend\", \"auth\"],\n      \"depends_on\": [],\n      \"inputs\": [\"oauth_app_id\", \"oauth_secret\"],\n      \"outputs\": [\"oauth_login_api\"],\n      \"acceptance\": [\"valid callback returns 200\"],\n      \"constraints\": [\"保持现有用户表结构\"],\n      \"template\": \"standard\"\n    },\n    {\n      \"id\": \"task-2\",\n      \"title\": \"审计日志落库\",\n      \"description\": \"记录登录审计日志并提供查询接口。\",\n      \"labels\": [\"backend\", \"database\"],\n      \"depends_on\": [\"task-1\"],\n      \"inputs\": [\"oauth_user_id\"],\n      \"outputs\": [\"audit_log_query_api\"],\n      \"acceptance\": [\"audit log query works\"],\n      \"constraints\": [\"最小化写放大\"],\n      \"template\": \"full\"\n    }\n  ]\n}\n```"
	agent := &mockAgent{
		cmd: []string{"mock-TeamLeader"},
		parser: &sliceParser{
			events: []*core.StreamEvent{
				{Type: "done", Content: output},
			},
		},
	}

	templatePath := filepath.Join("..", "..", "configs", "prompts", "team_leader.tmpl")
	driver, err := NewAgentWithTemplatePath(agent, runtime, templatePath)
	if err != nil {
		t.Fatalf("new TeamLeader agent: %v", err)
	}

	req := Request{
		Conversation:                "用户希望新增 OAuth 登录并补充审计日志。",
		ProjectName:                 "ai-workflow",
		TechStack:                   "Go + SQLite",
		RepoPath:                    "D:/project/ai-workflow",
		OriginalConversationSummary: "用户希望增加 OAuth 登录与审计日志能力。",
		PreviousTaskPlanJSON:        `{"name":"oauth-v1","tasks":[{"id":"task-1","title":"旧任务"}]}`,
		AIReviewSummaryJSON:         `{"rounds":2,"last_decision":"fix","top_issues":["coverage_gap"]}`,
		HumanFeedbackJSON:           `{"category":"coverage_gap","detail":"上一版遗漏了审计日志相关任务","expected_direction":"补齐日志任务并明确依赖"}`,
		WorkDir:                     "D:/project/ai-workflow",
	}

	rawOutput, err := driver.Decompose(context.Background(), req)
	if err != nil {
		t.Fatalf("decompose failed: %v", err)
	}

	if !waitCalled {
		t.Fatal("session.Wait must be called")
	}
	if len(agent.opts) != 1 {
		t.Fatalf("BuildCommand should be called once, got %d", len(agent.opts))
	}
	if agent.opts[0].WorkDir != req.WorkDir {
		t.Fatalf("exec opts workdir mismatch, got %q", agent.opts[0].WorkDir)
	}
	if agent.opts[0].MaxTurns <= 0 {
		t.Fatalf("max turns should be set, got %d", agent.opts[0].MaxTurns)
	}
	if !reflect.DeepEqual(agent.opts[0].AllowedTools, []string{"Read(*)"}) {
		t.Fatalf("allowed tools mismatch: %#v", agent.opts[0].AllowedTools)
	}

	prompt := agent.opts[0].Prompt
	for _, s := range []string{
		"输入 1：原始对话摘要",
		"输入 2：上一版 TaskPlan（完整 JSON）",
		"输入 3：AI review 问题摘要（结构化）",
		"输入 4：人类反馈（标准化 JSON）",
		req.OriginalConversationSummary,
		req.PreviousTaskPlanJSON,
		req.AIReviewSummaryJSON,
		req.HumanFeedbackJSON,
		req.Conversation,
		req.ProjectName,
		req.TechStack,
		req.RepoPath,
	} {
		if !strings.Contains(prompt, s) {
			t.Fatalf("prompt must include %q, got:\n%s", s, prompt)
		}
	}

	if runtime.lastOpts.WorkDir != req.WorkDir {
		t.Fatalf("runtime workdir mismatch, got %q", runtime.lastOpts.WorkDir)
	}
	if !reflect.DeepEqual(runtime.lastOpts.Command, []string{"mock-TeamLeader"}) {
		t.Fatalf("runtime command mismatch: %#v", runtime.lastOpts.Command)
	}

	if rawOutput != output {
		t.Fatalf("unexpected decompose output, got:\n%s", rawOutput)
	}
}

func TestPlanParserUsesRoleBinding(t *testing.T) {
	agent := &mockAgent{}
	templatePath := filepath.Join("..", "..", "configs", "prompts", "team_leader.tmpl")
	driver, err := NewAgentWithTemplatePath(agent, nil, templatePath)
	if err != nil {
		t.Fatalf("new TeamLeader agent: %v", err)
	}

	defaultReq := Request{
		Conversation: "请拆解当前任务",
		WorkDir:      "D:/project/ai-workflow",
	}
	if _, err := driver.BuildCommand(defaultReq); err != nil {
		t.Fatalf("build command with default role: %v", err)
	}
	if len(agent.opts) != 1 {
		t.Fatalf("expected 1 exec opts, got %d", len(agent.opts))
	}
	assertRoleID(t, agent.opts[0].AppendContext, "team_leader")

	overrideReq := defaultReq
	overrideReq.Role = "custom_role"
	if _, err := driver.BuildCommand(overrideReq); err != nil {
		t.Fatalf("build command with custom role: %v", err)
	}
	if len(agent.opts) != 2 {
		t.Fatalf("expected 2 exec opts, got %d", len(agent.opts))
	}
	assertRoleID(t, agent.opts[1].AppendContext, "custom_role")
}

func TestRenderPrompt_UsesFileContentsWhenConversationMissing(t *testing.T) {
	templatePath := filepath.Join("..", "..", "configs", "prompts", "team_leader.tmpl")
	driver, err := NewAgentWithTemplatePath(&mockAgent{}, nil, templatePath)
	if err != nil {
		t.Fatalf("new TeamLeader agent: %v", err)
	}

	prompt, err := driver.RenderPrompt(Request{
		FileContents: map[string]string{
			"docs/plans/wave3.md": "## Wave3\n- parser 支持 file-based 输入\n- 回归测试",
			"README.md":           "",
		},
	})
	if err != nil {
		t.Fatalf("RenderPrompt with file contents: %v", err)
	}

	for _, needle := range []string{
		"输入 5：补充文件内容（按路径聚合）",
		"<<<FILE:README.md>>>",
		"(empty file)",
		"<<<FILE:docs/plans/wave3.md>>>",
		"parser 支持 file-based 输入",
		"补充了 2 个文件上下文：README.md, docs/plans/wave3.md",
	} {
		if !strings.Contains(prompt, needle) {
			t.Fatalf("prompt must include %q, got:\n%s", needle, prompt)
		}
	}

	first := strings.Index(prompt, "<<<FILE:README.md>>>")
	second := strings.Index(prompt, "<<<FILE:docs/plans/wave3.md>>>")
	if first < 0 || second < 0 || first >= second {
		t.Fatalf("file content blocks must be stable-sorted by path, got prompt:\n%s", prompt)
	}
}

func TestRenderPrompt_AppendsFileContentsToConversation(t *testing.T) {
	templatePath := filepath.Join("..", "..", "configs", "prompts", "team_leader.tmpl")
	driver, err := NewAgentWithTemplatePath(&mockAgent{}, nil, templatePath)
	if err != nil {
		t.Fatalf("new TeamLeader agent: %v", err)
	}

	req := Request{
		Conversation: "用户希望把计划文件解析为结构化 tasks，并保留原始语义。",
		FileContents: map[string]string{
			"plan.md": "1. 先实现 parser\n2. 再补审查",
		},
	}
	prompt, err := driver.RenderPrompt(req)
	if err != nil {
		t.Fatalf("RenderPrompt with conversation + file contents: %v", err)
	}

	for _, needle := range []string{
		req.Conversation,
		"<<<FILE:plan.md>>>",
		"1. 先实现 parser",
	} {
		if !strings.Contains(prompt, needle) {
			t.Fatalf("prompt must include %q, got:\n%s", needle, prompt)
		}
	}
}

func TestTeamLeaderUsesBoundRole(t *testing.T) {
	resolver := acpclient.NewRoleResolver(
		[]acpclient.AgentProfile{
			{
				ID: "codex",
				CapabilitiesMax: acpclient.ClientCapabilities{
					FSRead:   true,
					FSWrite:  true,
					Terminal: true,
				},
			},
		},
		[]acpclient.RoleProfile{
			{
				ID:      "TeamLeader_custom",
				AgentID: "codex",
				Capabilities: acpclient.ClientCapabilities{
					FSRead:   true,
					FSWrite:  true,
					Terminal: true,
				},
				SessionPolicy: acpclient.SessionPolicy{
					Reuse:             true,
					PreferLoadSession: true,
				},
				MCPTools: []string{"query_issues"},
			},
		},
	)
	client := &stubTeamLeaderSessionClient{
		loadErr: errors.New("session not found"),
		newResp: acpproto.SessionId("sid-new"),
	}

	session, roleID, err := startTeamLeaderSession(
		context.Background(),
		client,
		resolver,
		"",
		"TeamLeader_custom",
		"sid-old",
		"D:/project/ai-workflow",
		nil,
	)
	if err != nil {
		t.Fatalf("startTeamLeaderSession() error = %v", err)
	}
	if roleID != "TeamLeader_custom" {
		t.Fatalf("role id = %q, want %q", roleID, "TeamLeader_custom")
	}
	if string(session) != "sid-new" {
		t.Fatalf("session id = %q, want %q", string(session), "sid-new")
	}
	if !reflect.DeepEqual(client.calls, []string{"load", "new"}) {
		t.Fatalf("call order = %#v, want load->new fallback", client.calls)
	}
	if len(client.loadReqs) != 1 {
		t.Fatalf("LoadSession calls = %d, want 1", len(client.loadReqs))
	}
	if len(client.newReqs) != 1 {
		t.Fatalf("NewSession calls = %d, want 1", len(client.newReqs))
	}
	if got, _ := client.loadReqs[0].Meta["role_id"].(string); got != "TeamLeader_custom" {
		t.Fatalf("load metadata role_id = %q, want %q", got, "TeamLeader_custom")
	}
	if got, _ := client.newReqs[0].Meta["role_id"].(string); got != "TeamLeader_custom" {
		t.Fatalf("new metadata role_id = %q, want %q", got, "TeamLeader_custom")
	}
	if len(client.newReqs[0].McpServers) != 1 {
		t.Fatalf("new session mcp servers = %d, want 1 from role config", len(client.newReqs[0].McpServers))
	}
	stdio := client.newReqs[0].McpServers[0].Stdio
	if stdio == nil {
		t.Fatalf("expected stdio mcp server, got %#v", client.newReqs[0].McpServers[0])
	}
	if len(stdio.Env) != 1 || stdio.Env[0].Name != "AI_WORKFLOW_MCP_TOOL" || stdio.Env[0].Value != "query_issues" {
		t.Fatalf("new session mcp env = %#v, want AI_WORKFLOW_MCP_TOOL=query_issues", stdio.Env)
	}
}

func TestStartTeamLeaderSessionSkipsLoadWhenReuseDisabled(t *testing.T) {
	resolver := acpclient.NewRoleResolver(
		[]acpclient.AgentProfile{
			{
				ID: "codex",
				CapabilitiesMax: acpclient.ClientCapabilities{
					FSRead:   true,
					FSWrite:  true,
					Terminal: true,
				},
			},
		},
		[]acpclient.RoleProfile{
			{
				ID:      "TeamLeader_custom",
				AgentID: "codex",
				Capabilities: acpclient.ClientCapabilities{
					FSRead:   true,
					FSWrite:  true,
					Terminal: true,
				},
				SessionPolicy: acpclient.SessionPolicy{
					Reuse:             false,
					PreferLoadSession: true,
				},
			},
		},
	)
	client := &stubTeamLeaderSessionClient{
		loadResp: acpproto.SessionId("sid-loaded"),
		newResp:  acpproto.SessionId("sid-new"),
	}

	session, roleID, err := startTeamLeaderSession(
		context.Background(),
		client,
		resolver,
		"",
		"TeamLeader_custom",
		"sid-old",
		"D:/project/ai-workflow",
		nil,
	)
	if err != nil {
		t.Fatalf("startTeamLeaderSession() error = %v", err)
	}
	if roleID != "TeamLeader_custom" {
		t.Fatalf("role id = %q, want %q", roleID, "TeamLeader_custom")
	}
	if string(session) != "sid-new" {
		t.Fatalf("session id = %q, want %q", string(session), "sid-new")
	}
	if len(client.loadReqs) != 0 {
		t.Fatalf("LoadSession calls = %d, want 0", len(client.loadReqs))
	}
	if len(client.newReqs) != 1 {
		t.Fatalf("NewSession calls = %d, want 1", len(client.newReqs))
	}
}

func TestStartTeamLeaderSessionSkipsLoadWhenPreferLoadDisabled(t *testing.T) {
	resolver := acpclient.NewRoleResolver(
		[]acpclient.AgentProfile{
			{
				ID: "codex",
				CapabilitiesMax: acpclient.ClientCapabilities{
					FSRead:   true,
					FSWrite:  true,
					Terminal: true,
				},
			},
		},
		[]acpclient.RoleProfile{
			{
				ID:      "TeamLeader_custom",
				AgentID: "codex",
				Capabilities: acpclient.ClientCapabilities{
					FSRead:   true,
					FSWrite:  true,
					Terminal: true,
				},
				SessionPolicy: acpclient.SessionPolicy{
					Reuse:             true,
					PreferLoadSession: false,
				},
			},
		},
	)
	client := &stubTeamLeaderSessionClient{
		loadResp: acpproto.SessionId("sid-loaded"),
		newResp:  acpproto.SessionId("sid-new"),
	}

	session, roleID, err := startTeamLeaderSession(
		context.Background(),
		client,
		resolver,
		"",
		"TeamLeader_custom",
		"sid-old",
		"D:/project/ai-workflow",
		nil,
	)
	if err != nil {
		t.Fatalf("startTeamLeaderSession() error = %v", err)
	}
	if roleID != "TeamLeader_custom" {
		t.Fatalf("role id = %q, want %q", roleID, "TeamLeader_custom")
	}
	if string(session) != "sid-new" {
		t.Fatalf("session id = %q, want %q", string(session), "sid-new")
	}
	if len(client.loadReqs) != 0 {
		t.Fatalf("LoadSession calls = %d, want 0", len(client.loadReqs))
	}
	if len(client.newReqs) != 1 {
		t.Fatalf("NewSession calls = %d, want 1", len(client.newReqs))
	}
}

func TestCollectOutputPrefersDoneChunk(t *testing.T) {
	got, err := collectOutput(&sliceParser{
		events: []*core.StreamEvent{
			{Type: "text", Content: `{"partial":"a"}`},
			{Type: "done", Content: `{"name":"final-plan"}`},
		},
	})
	if err != nil {
		t.Fatalf("collectOutput returned error: %v", err)
	}
	if got != `{"name":"final-plan"}` {
		t.Fatalf("collectOutput should prefer done chunk, got %q", got)
	}
}

func TestCollectOutputJoinsTextChunksWhenDoneMissing(t *testing.T) {
	got, err := collectOutput(&sliceParser{
		events: []*core.StreamEvent{
			{Type: "text", Content: "line-1"},
			{Type: "text", Content: "line-2"},
		},
	})
	if err != nil {
		t.Fatalf("collectOutput returned error: %v", err)
	}
	if got != "line-1\nline-2" {
		t.Fatalf("collectOutput should join text chunks, got %q", got)
	}
}

func TestCollectOutputRejectsEmptyChunks(t *testing.T) {
	_, err := collectOutput(&sliceParser{
		events: []*core.StreamEvent{
			nil,
			{Type: "text", Content: "   "},
		},
	})
	if err == nil {
		t.Fatal("expected error for empty output")
	}
	if !strings.Contains(err.Error(), "empty output") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertRoleID(t *testing.T, appendContext, wantRole string) {
	t.Helper()

	if strings.TrimSpace(appendContext) == "" {
		t.Fatal("append context should not be empty")
	}

	payload := map[string]string{}
	if err := json.Unmarshal([]byte(appendContext), &payload); err != nil {
		t.Fatalf("append context should be json: %v", err)
	}

	if payload["role_id"] != wantRole {
		t.Fatalf("unexpected role_id, got %q want %q", payload["role_id"], wantRole)
	}
}
