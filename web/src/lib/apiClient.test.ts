import { describe, expect, it, vi, afterEach } from "vitest";
import { ApiError, createApiClient } from "./apiClient";

describe("apiClient", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("会在请求头注入 Bearer token 并返回 JSON", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ ok: true }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    const client = createApiClient({
      baseUrl: "http://localhost:8080/api/v1",
      getToken: () => "secret-token",
    });

    const result = await client.request<{ ok: boolean }>({
      path: "/projects",
    });

    expect(result.ok).toBe(true);
    expect(fetchMock).toHaveBeenCalledOnce();
    const call = fetchMock.mock.calls[0];
    expect(call?.[0]).toBe("http://localhost:8080/api/v1/projects");

    const requestInit = call?.[1];
    const headers = requestInit?.headers;
    expect(headers).toBeInstanceOf(Headers);
    expect((headers as Headers).get("Authorization")).toBe("Bearer secret-token");
  });

  it("当响应非 2xx 时抛出 ApiError", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify({ message: "bad request" }), {
          status: 400,
          headers: { "Content-Type": "application/json" },
        }),
      ),
    );

    const client = createApiClient({
      baseUrl: "http://localhost:8080/api/v1",
      getToken: () => "",
    });

    await expect(client.request({ path: "/projects" })).rejects.toBeInstanceOf(
      ApiError,
    );
  });

  it("listPlans/listPipelines 会透传 limit 与 offset 查询参数", async () => {
    const fetchMock = vi.fn().mockImplementation(async () => {
      return new Response(JSON.stringify({ items: [], total: 0, offset: 0 }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    });
    vi.stubGlobal("fetch", fetchMock);

    const client = createApiClient({
      baseUrl: "http://localhost:8080/api/v1",
    });

    await client.listPlans("proj-1", { limit: 50, offset: 100 });
    await client.listPipelines("proj-1", { limit: 20, offset: 40 });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "http://localhost:8080/api/v1/projects/proj-1/plans?limit=50&offset=100",
    );
    expect(fetchMock.mock.calls[1]?.[0]).toBe(
      "http://localhost:8080/api/v1/projects/proj-1/pipelines?limit=20&offset=40",
    );
  });

  it("createProject 支持 github 字段并不包含多余字段", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ id: "p1" }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    const client = createApiClient({
      baseUrl: "http://localhost:8080/api/v1",
    });

    await client.createProject({
      name: "proj",
      repo_path: "D:/repo/proj",
      github: {
        owner: "acme",
        repo: "repo",
      },
    });

    const requestInit = fetchMock.mock.calls[0]?.[1] as RequestInit;
    const parsedBody = JSON.parse(String(requestInit.body)) as Record<string, unknown>;
    expect(parsedBody).toEqual({
      name: "proj",
      repo_path: "D:/repo/proj",
      github: {
        owner: "acme",
        repo: "repo",
      },
    });
    expect(parsedBody).not.toHaveProperty("config");
  });

  it("计划/任务/Pipeline 动作接口会命中正确路由并透传请求体", async () => {
    const fetchMock = vi.fn().mockImplementation(async () => {
      return new Response(JSON.stringify({ status: "ok" }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    });
    vi.stubGlobal("fetch", fetchMock);

    const client = createApiClient({
      baseUrl: "http://localhost:8080/api/v1",
    });

    await client.submitPlanReview("proj-1", "plan-1");
    await client.applyPlanAction("proj-1", "plan-1", {
      action: "reject",
      feedback: {
        category: "coverage_gap",
        detail: "补齐异常路径与失败回滚逻辑。",
      },
    });
    await client.setIssueAutoMerge("proj-1", "plan-1", {
      auto_merge: false,
    });
    await client.applyTaskAction("proj-1", "plan-1", "task-1", {
      action: "retry",
    });
    await client.applyPipelineAction("proj-1", "pipe-1", {
      action: "abort",
    });
    await client.getPipelineCheckpoints("proj-1", "pipe-1");

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "http://localhost:8080/api/v1/projects/proj-1/plans/plan-1/review",
    );
    expect(fetchMock.mock.calls[1]?.[0]).toBe(
      "http://localhost:8080/api/v1/projects/proj-1/plans/plan-1/action",
    );
    expect(fetchMock.mock.calls[2]?.[0]).toBe(
      "http://localhost:8080/api/v1/projects/proj-1/issues/plan-1/auto-merge",
    );
    expect(fetchMock.mock.calls[3]?.[0]).toBe(
      "http://localhost:8080/api/v1/projects/proj-1/plans/plan-1/tasks/task-1/action",
    );
    expect(fetchMock.mock.calls[4]?.[0]).toBe(
      "http://localhost:8080/api/v1/projects/proj-1/pipelines/pipe-1/action",
    );
    expect(fetchMock.mock.calls[5]?.[0]).toBe(
      "http://localhost:8080/api/v1/projects/proj-1/pipelines/pipe-1/checkpoints",
    );

    const reviewBody = JSON.parse(String((fetchMock.mock.calls[1]?.[1] as RequestInit)?.body));
    expect(reviewBody).toEqual({
      action: "reject",
      feedback: {
        category: "coverage_gap",
        detail: "补齐异常路径与失败回滚逻辑。",
      },
    });

    const autoMergeBody = JSON.parse(String((fetchMock.mock.calls[2]?.[1] as RequestInit)?.body));
    expect(autoMergeBody).toEqual({
      auto_merge: false,
    });

    const taskBody = JSON.parse(String((fetchMock.mock.calls[3]?.[1] as RequestInit)?.body));
    expect(taskBody).toEqual({
      action: "retry",
    });

    const pipelineBody = JSON.parse(String((fetchMock.mock.calls[4]?.[1] as RequestInit)?.body));
    expect(pipelineBody).toEqual({
      action: "abort",
    });
  });

  it("计划历史与审计接口会命中正确路由并透传查询参数", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(JSON.stringify([]), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify([]), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            items: [],
            total: 0,
            offset: 0,
          }),
          {
            status: 200,
            headers: { "Content-Type": "application/json" },
          },
        ),
      );
    vi.stubGlobal("fetch", fetchMock);

    const client = createApiClient({
      baseUrl: "http://localhost:8080/api/v1",
    });

    await client.listPlanReviews?.("proj-1", "plan-1");
    await client.listPlanChanges?.("proj-1", "plan-1");
    await client.listAdminAuditLog?.({
      projectId: "proj-1",
      action: "force_ready",
      user: "admin",
      since: "2026-03-01T00:00:00Z",
      until: "2026-03-03T23:59:59Z",
      limit: 50,
      offset: 10,
    });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "http://localhost:8080/api/v1/projects/proj-1/plans/plan-1/reviews",
    );
    expect(fetchMock.mock.calls[1]?.[0]).toBe(
      "http://localhost:8080/api/v1/projects/proj-1/plans/plan-1/changes",
    );
    expect(fetchMock.mock.calls[2]?.[0]).toBe(
      "http://localhost:8080/api/v1/admin/audit-log?project_id=proj-1&action=force_ready&user=admin&since=2026-03-01T00%3A00%3A00Z&until=2026-03-03T23%3A59%3A59Z&limit=50&offset=10",
    );
  });

  it("pipeline logs 接口会命中正确路由并透传 stage/limit/offset", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          items: [
            {
              id: 2,
              pipeline_id: "pipe-1",
              stage: "implement",
              type: "stdout",
              agent: "codex",
              content: "implement-log-2",
              timestamp: "2026-03-03T10:02:00Z",
            },
          ],
          total: 2,
          offset: 1,
        }),
        {
          status: 200,
          headers: { "Content-Type": "application/json" },
        },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    const client = createApiClient({
      baseUrl: "http://localhost:8080/api/v1",
    });

    const logs = await client.getPipelineLogs("proj-1", "pipe-1", {
      stage: "implement",
      limit: 1,
      offset: 1,
    });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "http://localhost:8080/api/v1/projects/proj-1/pipelines/pipe-1/logs?stage=implement&limit=1&offset=1",
    );
    const requestInit = fetchMock.mock.calls[0]?.[1] as RequestInit;
    expect(requestInit.method).toBe("GET");
    expect(logs.total).toBe(2);
    expect(logs.offset).toBe(1);
    expect(logs.items[0]?.content).toBe("implement-log-2");
  });

  it("issue timeline 接口会命中正确路由并返回分页事件列表", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          items: [
            {
              event_id: "change:chg-1",
              kind: "change",
              created_at: "2026-03-03T10:03:00Z",
              actor_type: "system",
              actor_name: "system",
              actor_avatar_seed: "system",
              title: "change · status",
              body: "draft -> reviewing · submit review",
              status: "info",
              refs: {
                issue_id: "issue-1",
                pipeline_id: "pipe-1",
              },
              meta: { field: "status" },
            },
            {
              event_id: "review:7",
              kind: "review",
              created_at: "2026-03-03T10:04:00Z",
              actor_type: "agent",
              actor_name: "reviewer",
              actor_avatar_seed: "reviewer",
              title: "review · reviewer",
              body: "verdict=changes_requested · score=70",
              status: "warning",
              refs: {
                issue_id: "issue-1",
                pipeline_id: "pipe-1",
              },
              meta: { verdict: "changes_requested", score: 70 },
            },
          ],
          total: 2,
          offset: 0,
        }),
        {
          status: 200,
          headers: { "Content-Type": "application/json" },
        },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    const client = createApiClient({
      baseUrl: "http://localhost:8080/api/v1",
    });

    const timeline = await client.listIssueTimeline("proj-1", "issue-1", {
      limit: 20,
      offset: 0,
    });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "http://localhost:8080/api/v1/projects/proj-1/issues/issue-1/timeline?limit=20&offset=0",
    );
    const requestInit = fetchMock.mock.calls[0]?.[1] as RequestInit;
    expect(requestInit.method).toBe("GET");
    expect(timeline.total).toBe(2);
    expect(timeline.items).toHaveLength(2);
    expect(timeline.items[0]?.kind).toBe("change");
    expect(timeline.items[1]?.kind).toBe("review");
    expect(timeline.items[1]?.event_id).toBe("review:7");
  });

  it("issue timeline 返回旧结构时会抛出明确错误", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          items: [
            {
              kind: "checkpoint",
              timestamp: "2026-03-03T10:03:00Z",
              checkpoint: {
                stage_name: "implement",
              },
            },
          ],
          total: 1,
          offset: 0,
        }),
        {
          status: 200,
          headers: { "Content-Type": "application/json" },
        },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    const client = createApiClient({
      baseUrl: "http://localhost:8080/api/v1",
    });

    await expect(client.listIssueTimeline("proj-1", "issue-1", { limit: 20, offset: 0 })).rejects
      .toThrow(/issue timeline 响应结构不兼容/i);
  });

  it("getPipeline/listPlans 能携带 task_item_id 与结构化任务字段", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            id: "pipe-1",
            project_id: "proj-1",
            name: "pipeline-one",
            description: "pipeline",
                    template: "standard",
                    status: "created",
                    current_stage: "implement",
                    artifacts: {},
                    config: {
                      issue_number: 201,
                      issue_url: "https://github.com/acme/ai-workflow/issues/201",
                      pr_number: 301,
                      pr_url: "https://github.com/acme/ai-workflow/pull/301",
                      github_connection_status: "connected",
                    },
            branch_name: "",
            worktree_path: "",
            max_total_retries: 5,
            total_retries: 0,
            task_item_id: "task-1",
            started_at: "",
            finished_at: "",
            created_at: "2026-03-01T00:00:00Z",
            updated_at: "2026-03-01T00:00:00Z",
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        ),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            items: [
              {
                id: "plan-1",
                project_id: "proj-1",
                session_id: "chat-1",
                name: "plan",
                status: "draft",
                wait_reason: "",
                fail_policy: "block",
                review_round: 0,
                tasks: [
                  {
                    id: "task-1",
                    plan_id: "plan-1",
                    title: "task",
                    description: "task description",
                    labels: [],
                    depends_on: [],
                    inputs: ["oauth_app_id"],
                    outputs: ["oauth_token"],
                    acceptance: ["callback returns 200"],
                    constraints: ["keep backward compatibility"],
                    template: "standard",
                    pipeline_id: "",
                    external_id: "https://github.com/acme/ai-workflow/issues/201",
                    status: "pending",
                    created_at: "2026-03-01T00:00:00Z",
                    updated_at: "2026-03-01T00:00:00Z",
                  },
                ],
                created_at: "2026-03-01T00:00:00Z",
                updated_at: "2026-03-01T00:00:00Z",
              },
            ],
            total: 1,
            offset: 0,
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        ),
      );
    vi.stubGlobal("fetch", fetchMock);

    const client = createApiClient({
      baseUrl: "http://localhost:8080/api/v1",
    });

    const pipeline = await client.getPipeline("proj-1", "pipe-1");
    const plans = await client.listPlans("proj-1");

    expect(pipeline.task_item_id).toBe("task-1");
    expect(plans.items[0]?.tasks[0]?.inputs[0]).toBe("oauth_app_id");
    expect(plans.items[0]?.tasks[0]?.outputs[0]).toBe("oauth_token");
    expect(plans.items[0]?.tasks[0]?.acceptance[0]).toBe("callback returns 200");
    expect(plans.items[0]?.tasks[0]?.constraints[0]).toBe("keep backward compatibility");
    expect(pipeline.github?.issue_number).toBe(201);
    expect(pipeline.github?.issue_url).toBe("https://github.com/acme/ai-workflow/issues/201");
    expect(pipeline.github?.pr_number).toBe(301);
    expect(pipeline.github?.pr_url).toBe("https://github.com/acme/ai-workflow/pull/301");
    expect(pipeline.github?.connection_status).toBe("connected");
    expect(plans.items[0]?.tasks[0]?.github?.issue_number).toBe(201);
    expect(plans.items[0]?.tasks[0]?.github?.issue_url).toBe(
      "https://github.com/acme/ai-workflow/issues/201",
    );
  });

  it("createProjectCreateRequest/getProjectCreateRequest 命中 create-requests 路由", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ request_id: "req-1" }), {
          status: 202,
          headers: { "Content-Type": "application/json" },
        }),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            request_id: "req-1",
            status: "succeeded",
            project_id: "proj-9",
          }),
          {
            status: 200,
            headers: { "Content-Type": "application/json" },
          },
        ),
      );
    vi.stubGlobal("fetch", fetchMock);

    const client = createApiClient({
      baseUrl: "http://localhost:8080/api/v1",
    });

    const accepted = await client.createProjectCreateRequest({
      name: "demo",
      source_type: "github_clone",
      remote_url: "https://github.com/acme/demo.git",
      ref: "main",
    });
    const status = await client.getProjectCreateRequest("req-1");

    expect(accepted.request_id).toBe("req-1");
    expect(status.status).toBe("succeeded");
    expect(status.project_id).toBe("proj-9");
    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "http://localhost:8080/api/v1/projects/create-requests",
    );
    expect(fetchMock.mock.calls[1]?.[0]).toBe(
      "http://localhost:8080/api/v1/projects/create-requests/req-1",
    );

    const requestInit = fetchMock.mock.calls[0]?.[1] as RequestInit;
    const parsedBody = JSON.parse(String(requestInit.body)) as Record<string, unknown>;
    expect(parsedBody).toEqual({
      name: "demo",
      source_type: "github_clone",
      remote_url: "https://github.com/acme/demo.git",
      ref: "main",
    });
  });

  it("createPlanFromFiles 命中 from-files 路由并返回规范化任务结构", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          id: "plan-files-1",
          project_id: "proj-1",
          session_id: "chat-1",
          name: "Plan From Files",
          status: "draft",
          wait_reason: "",
          fail_policy: "block",
          review_round: 0,
          spec_profile: "default",
          contract_version: "v1",
          contract_checksum: "checksum",
          tasks: [
            {
              id: "task-1",
              plan_id: "plan-files-1",
              title: "Parse files",
              description: "parse target files",
              labels: [],
              depends_on: [],
              inputs: null,
              outputs: null,
              acceptance: null,
              constraints: null,
              template: "standard",
              pipeline_id: "",
              external_id: "",
              status: "pending",
              created_at: "2026-03-01T00:00:00Z",
              updated_at: "2026-03-01T00:00:00Z",
            },
          ],
          created_at: "2026-03-01T00:00:00Z",
          updated_at: "2026-03-01T00:00:00Z",
        }),
        {
          status: 200,
          headers: { "Content-Type": "application/json" },
        },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    const client = createApiClient({
      baseUrl: "http://localhost:8080/api/v1",
    });

    const plan = await client.createPlanFromFiles("proj-1", {
      session_id: "chat-1",
      name: "Plan From Files",
      fail_policy: "block",
      file_paths: ["cmd/ai-flow/main.go", "internal/core/workflow.go"],
    });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "http://localhost:8080/api/v1/projects/proj-1/plans/from-files",
    );
    const requestInit = fetchMock.mock.calls[0]?.[1] as RequestInit;
    const parsedBody = JSON.parse(String(requestInit.body)) as Record<string, unknown>;
    expect(parsedBody).toEqual({
      session_id: "chat-1",
      name: "Plan From Files",
      fail_policy: "block",
      file_paths: ["cmd/ai-flow/main.go", "internal/core/workflow.go"],
    });
    expect(plan.tasks[0]?.inputs).toEqual([]);
    expect(plan.tasks[0]?.outputs).toEqual([]);
    expect(plan.tasks[0]?.acceptance).toEqual([]);
    expect(plan.tasks[0]?.constraints).toEqual([]);
  });

  it("listChats 命中 chat 列表路由并返回会话数组", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify([
          {
            id: "chat-1",
            project_id: "proj-1",
            messages: [
              {
                role: "user",
                content: "hello",
                time: "2026-03-02T00:00:00Z",
              },
            ],
            created_at: "2026-03-02T00:00:00Z",
            updated_at: "2026-03-02T00:00:00Z",
          },
        ]),
        {
          status: 200,
          headers: { "Content-Type": "application/json" },
        },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    const client = createApiClient({
      baseUrl: "http://localhost:8080/api/v1",
    });

    const sessions = await client.listChats("proj-1");

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "http://localhost:8080/api/v1/projects/proj-1/chat",
    );
    const requestInit = fetchMock.mock.calls[0]?.[1] as RequestInit;
    expect(requestInit.method).toBe("GET");
    expect(sessions).toHaveLength(1);
    expect(sessions[0]?.id).toBe("chat-1");
  });

  it("listChatRunEvents 命中会话事件路由并返回事件数组", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify([
          {
            id: 1,
            session_id: "chat-1",
            project_id: "proj-1",
            event_type: "chat_run_update",
            update_type: "tool_call",
            payload: {
              session_id: "chat-1",
              acp: {
                sessionUpdate: "tool_call",
                title: "Terminal",
              },
            },
            created_at: "2026-03-03T00:00:00Z",
          },
        ]),
        {
          status: 200,
          headers: { "Content-Type": "application/json" },
        },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    const client = createApiClient({
      baseUrl: "http://localhost:8080/api/v1",
    });

    const events = await client.listChatRunEvents("proj-1", "chat-1");

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "http://localhost:8080/api/v1/projects/proj-1/chat/chat-1/events",
    );
    const requestInit = fetchMock.mock.calls[0]?.[1] as RequestInit;
    expect(requestInit.method).toBe("GET");
    expect(events).toHaveLength(1);
    expect(events[0]?.update_type).toBe("tool_call");
  });

  it("cancelChat 命中 cancel 路由并返回 session/status", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          session_id: "chat-9",
          status: "cancelling",
        }),
        {
          status: 202,
          headers: { "Content-Type": "application/json" },
        },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    const client = createApiClient({
      baseUrl: "http://localhost:8080/api/v1",
    });

    const response = await client.cancelChat("proj-1", "chat-9");

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "http://localhost:8080/api/v1/projects/proj-1/chat/chat-9/cancel",
    );
    const requestInit = fetchMock.mock.calls[0]?.[1] as RequestInit;
    expect(requestInit.method).toBe("POST");
    expect(response).toEqual({
      session_id: "chat-9",
      status: "cancelling",
    });
  });

  it("仓库树/状态/diff 接口命中正确路由并透传查询参数", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            dir: "",
            items: [],
          }),
          {
            status: 200,
            headers: { "Content-Type": "application/json" },
          },
        ),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            items: [],
          }),
          {
            status: 200,
            headers: { "Content-Type": "application/json" },
          },
        ),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            file_path: "src/main.ts",
            diff: "diff --git a/src/main.ts b/src/main.ts",
          }),
          {
            status: 200,
            headers: { "Content-Type": "application/json" },
          },
        ),
      );
    vi.stubGlobal("fetch", fetchMock);

    const client = createApiClient({
      baseUrl: "http://localhost:8080/api/v1",
    });

    await client.getRepoTree("proj-1", "src");
    await client.getRepoStatus("proj-1");
    await client.getRepoDiff("proj-1", "src/main.ts");

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "http://localhost:8080/api/v1/projects/proj-1/repo/tree?dir=src",
    );
    expect(fetchMock.mock.calls[1]?.[0]).toBe(
      "http://localhost:8080/api/v1/projects/proj-1/repo/status",
    );
    expect(fetchMock.mock.calls[2]?.[0]).toBe(
      "http://localhost:8080/api/v1/projects/proj-1/repo/diff?file=src%2Fmain.ts",
    );
  });
});
