
/** @vitest-environment jsdom */

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import PlanView from "./PlanView";
import type { ApiClient } from "../lib/apiClient";
import type { WsClient } from "../lib/wsClient";
import type {
  AdminAuditLogItem,
  ApiTaskPlan,
  IssueTimelineEntry,
  PlanChangeRecord,
  PlanReviewRecord,
} from "../types/api";

const buildPlan = (
  id: string,
  name: string,
  overrides?: Partial<ApiTaskPlan>,
): ApiTaskPlan => ({
  id: overrides?.id ?? id,
  project_id: overrides?.project_id ?? "proj-1",
  session_id: overrides?.session_id ?? "chat-1",
  name: overrides?.name ?? name,
  status: overrides?.status ?? "draft",
  auto_merge: overrides?.auto_merge ?? true,
  pipeline_id: overrides?.pipeline_id ?? "",
  wait_reason: overrides?.wait_reason ?? "",
  tasks: overrides?.tasks ?? [],
  source_files: overrides?.source_files ?? [],
  file_contents: overrides?.file_contents ?? {},
  fail_policy: overrides?.fail_policy ?? "block",
  review_round: overrides?.review_round ?? 0,
  spec_profile: overrides?.spec_profile ?? "default",
  contract_version: overrides?.contract_version ?? "v1",
  contract_checksum: overrides?.contract_checksum ?? "checksum",
  created_at: overrides?.created_at ?? "2026-03-01T10:00:00.000Z",
  updated_at: overrides?.updated_at ?? "2026-03-01T10:00:00.000Z",
});

const createMockApiClient = (): ApiClient => {
  const reviewRecords: PlanReviewRecord[] = [];
  const changeRecords: PlanChangeRecord[] = [];
  const timelineRecords: IssueTimelineEntry[] = [];
  const auditRecords: AdminAuditLogItem[] = [];

  return {
    request: vi.fn(),
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    del: vi.fn(),
    getStats: vi.fn(),
    listProjects: vi.fn(),
    createProject: vi.fn(),
    createProjectCreateRequest: vi.fn(),
    getProjectCreateRequest: vi.fn(),
    listPipelines: vi.fn(),
    createPipeline: vi.fn(),
    listChats: vi.fn(),
    listChatRunEvents: vi.fn(),
    createChat: vi.fn(),
    cancelChat: vi.fn(),
    getChat: vi.fn(),
    createPlan: vi.fn(),
    createPlanFromFiles: vi.fn(),
    submitPlanReview: vi.fn().mockResolvedValue({ status: "reviewing" }),
    applyPlanAction: vi.fn().mockResolvedValue({ status: "reviewing" }),
    setIssueAutoMerge: vi.fn().mockResolvedValue({ status: "draft", auto_merge: false }),
    applyTaskAction: vi.fn(),
    listPlans: vi.fn().mockResolvedValue({
      items: [
        buildPlan("plan-1", "Plan One", {
          source_files: ["docs/spec.md"],
          file_contents: { "docs/spec.md": "original spec content" },
        }),
      ],
      total: 1,
      offset: 0,
    }),
    getPlanDag: vi.fn(),
    listPlanReviews: vi.fn().mockResolvedValue(reviewRecords),
    listPlanChanges: vi.fn().mockResolvedValue(changeRecords),
    listIssueTimeline: vi.fn().mockResolvedValue({
      items: timelineRecords,
      total: timelineRecords.length,
      offset: 0,
    }),
    listAdminAuditLog: vi.fn().mockResolvedValue({
      items: auditRecords,
      total: auditRecords.length,
      offset: 0,
    }),
    getPipeline: vi.fn(),
    getPipelineLogs: vi.fn(),
    getPipelineCheckpoints: vi.fn(),
    getRepoTree: vi.fn(),
    getRepoStatus: vi.fn(),
    getRepoDiff: vi.fn(),
    applyPipelineAction: vi.fn(),
  } as unknown as ApiClient;
};

const createMockWsClient = (): WsClient => {
  return {
    connect: vi.fn(),
    disconnect: vi.fn(),
    send: vi.fn(),
    subscribe: vi.fn().mockReturnValue(() => {}),
    onStatusChange: vi.fn().mockReturnValue(() => {}),
    getStatus: vi.fn().mockReturnValue("open"),
  } as unknown as WsClient;
};

describe("PlanView", () => {
  afterEach(() => {
    cleanup();
  });

  it("加载计划后展示步骤审阅与文件原文", async () => {
    const apiClient = createMockApiClient();
    const wsClient = createMockWsClient();

    render(
      <PlanView
        apiClient={apiClient}
        wsClient={wsClient}
        projectId="proj-1"
        refreshToken={0}
      />,
    );

    await waitFor(() => {
      expect(apiClient.listPlans).toHaveBeenCalledWith("proj-1", {
        limit: 50,
        offset: 0,
      });
    });

    await waitFor(() => {
      expect(wsClient.send).toHaveBeenCalledWith({
        type: "subscribe_plan",
        plan_id: "plan-1",
      });
    });

    expect(screen.getByText("步骤 0 · 原始输入文件")).toBeTruthy();
    expect(screen.getByRole("button", { name: "docs/spec.md" })).toBeTruthy();
    expect(screen.getAllByText(/original spec content/).length).toBeGreaterThan(0);
  });

  it("ws 未连接时订阅失败不应导致白屏", async () => {
    const apiClient = createMockApiClient();
    const wsClient = createMockWsClient();
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    vi.mocked(wsClient.send).mockImplementation(() => {
      throw new Error("WebSocket is not connected");
    });

    render(
      <PlanView
        apiClient={apiClient}
        wsClient={wsClient}
        projectId="proj-1"
        refreshToken={0}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("审阅步骤")).toBeTruthy();
      expect(screen.getByText("步骤 0 · 原始输入文件")).toBeTruthy();
    });

    expect(warnSpy).toHaveBeenCalledWith(
      "[PlanView] subscribe_plan skipped:",
      expect.any(Error),
    );
    warnSpy.mockRestore();
  });

  it("计划列表含异常项时仍可渲染有效计划", async () => {
    const apiClient = createMockApiClient();
    const wsClient = createMockWsClient();

    vi.mocked(apiClient.listPlans).mockResolvedValue({
      items: [
        null as unknown as ApiTaskPlan,
        {
          id: "plan-bad-shape",
          title: "Legacy Plan Title",
          status: "executing",
          tasks: null,
          source_files: null,
          file_contents: null,
        } as unknown as ApiTaskPlan,
      ],
      total: 2,
      offset: 0,
    });

    render(
      <PlanView
        apiClient={apiClient}
        wsClient={wsClient}
        projectId="proj-1"
        refreshToken={0}
      />,
    );

    await waitFor(() => {
      expect(screen.getAllByText("Legacy Plan Title").length).toBeGreaterThan(0);
      expect(screen.getByText("步骤 0 · 原始输入文件")).toBeTruthy();
    });
  });

  it("按步骤切换后可查看步骤后的文件结果", async () => {
    const apiClient = createMockApiClient();
    const wsClient = createMockWsClient();
    vi.mocked(apiClient.listPlanChanges!).mockResolvedValue([
      {
        id: "change-1",
        issue_id: "plan-1",
        field: "file_contents.docs/spec.md",
        old_value: "original spec content",
        new_value: "reviewed spec content",
        reason: "apply_review_fix",
        changed_by: "system",
        created_at: "2026-03-01T10:05:00.000Z",
      },
    ]);

    render(
      <PlanView
        apiClient={apiClient}
        wsClient={wsClient}
        projectId="proj-1"
        refreshToken={0}
      />,
    );

    const changeStepButton = await screen.findByRole("button", {
      name: /Change · file_contents\.docs\/spec\.md/,
    });
    fireEvent.click(changeStepButton);

    await waitFor(() => {
      expect(screen.getAllByText(/reviewed spec content/).length).toBeGreaterThan(0);
    });
  });

  it("支持提交审核动作", async () => {
    const apiClient = createMockApiClient();
    const wsClient = createMockWsClient();

    render(
      <PlanView
        apiClient={apiClient}
        wsClient={wsClient}
        projectId="proj-1"
        refreshToken={0}
      />,
    );

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "提交审核" })).toBeTruthy();
    });

    fireEvent.click(screen.getByRole("button", { name: "提交审核" }));

    await waitFor(() => {
      expect(apiClient.submitPlanReview).toHaveBeenCalledWith("proj-1", "plan-1");
    });
  });

  it("支持切换自动合并开关", async () => {
    const apiClient = createMockApiClient();
    const wsClient = createMockWsClient();

    render(
      <PlanView
        apiClient={apiClient}
        wsClient={wsClient}
        projectId="proj-1"
        refreshToken={0}
      />,
    );

    const autoMergeCheckbox = await screen.findByRole("checkbox", {
      name: "自动合并（评审通过后自动进入 pipeline 执行合并）",
    });
    expect((autoMergeCheckbox as HTMLInputElement).checked).toBe(true);

    fireEvent.click(autoMergeCheckbox);

    await waitFor(() => {
      expect(apiClient.setIssueAutoMerge).toHaveBeenCalledWith("proj-1", "plan-1", {
        auto_merge: false,
      });
    });
  });

  it("reviewing 状态支持通过与驳回", async () => {
    const apiClient = createMockApiClient();
    const wsClient = createMockWsClient();
    vi.mocked(apiClient.listPlans).mockResolvedValue({
      items: [
        buildPlan("plan-1", "Plan One", {
          status: "reviewing",
          source_files: ["docs/spec.md"],
          file_contents: { "docs/spec.md": "content" },
        }),
      ],
      total: 1,
      offset: 0,
    });

    render(
      <PlanView
        apiClient={apiClient}
        wsClient={wsClient}
        projectId="proj-1"
        refreshToken={0}
      />,
    );

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "通过" })).toBeTruthy();
    });

    fireEvent.click(screen.getByRole("button", { name: "通过" }));

    await waitFor(() => {
      expect(apiClient.applyPlanAction).toHaveBeenCalledWith("proj-1", "plan-1", {
        action: "approve",
      });
    });

    fireEvent.change(screen.getByLabelText("驳回类型"), {
      target: { value: "coverage_gap" },
    });
    fireEvent.change(screen.getByLabelText("驳回说明"), {
      target: { value: "缺少关键错误路径，请补充。" },
    });
    fireEvent.change(screen.getByLabelText("期望方向（可选）"), {
      target: { value: "补齐失败分支" },
    });

    fireEvent.click(screen.getByRole("button", { name: "驳回" }));

    await waitFor(() => {
      expect(apiClient.applyPlanAction).toHaveBeenCalledWith("proj-1", "plan-1", {
        action: "reject",
        feedback: {
          category: "coverage_gap",
          detail: "缺少关键错误路径，请补充。",
          expected_direction: "补齐失败分支",
        },
      });
    });
  });

  it("parse_failed 时展示重试解析并调用 approve", async () => {
    const apiClient = createMockApiClient();
    const wsClient = createMockWsClient();
    vi.mocked(apiClient.listPlans).mockResolvedValue({
      items: [
        buildPlan("plan-1", "Plan One", {
          status: "waiting_human",
          wait_reason: "parse_failed",
          source_files: ["docs/spec.md"],
          file_contents: { "docs/spec.md": "content" },
        }),
      ],
      total: 1,
      offset: 0,
    });

    render(
      <PlanView
        apiClient={apiClient}
        wsClient={wsClient}
        projectId="proj-1"
        refreshToken={0}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("解析失败（parse_failed），请修正输入后点击“重试解析”。")).toBeTruthy();
      expect(screen.getByRole("button", { name: "重试解析" })).toBeTruthy();
    });

    fireEvent.click(screen.getByRole("button", { name: "重试解析" }));

    await waitFor(() => {
      expect(apiClient.applyPlanAction).toHaveBeenCalledWith("proj-1", "plan-1", {
        action: "approve",
      });
    });
  });

  it("按时间线展示多步骤审阅（review/change/timeline/audit）", async () => {
    const apiClient = createMockApiClient();
    const wsClient = createMockWsClient();

    vi.mocked(apiClient.listPlanReviews!).mockResolvedValue([
      {
        id: 11,
        issue_id: "plan-1",
        round: 1,
        reviewer: "demand_reviewer",
        verdict: "fix",
        summary: "评审建议先补齐回滚与异常路径。",
        raw_output: "review raw output:\n- 缺少回滚计划\n- 需要补充失败路径测试",
        issues: [
          {
            severity: "high",
            issue_id: "i-1",
            description: "缺少回滚计划",
            suggestion: "补充 rollback 步骤",
          },
        ],
        fixes: [
          {
            issue_id: "i-1",
            description: "新增 rollback 章节",
            suggestion: "覆盖失败路径",
          },
        ],
        score: 70,
        created_at: "2026-03-01T10:02:00.000Z",
      },
    ]);

    vi.mocked(apiClient.listPlanChanges!).mockResolvedValue([
      {
        id: "c-1",
        issue_id: "plan-1",
        field: "status",
        old_value: "draft",
        new_value: "reviewing",
        reason: "submit_for_review",
        changed_by: "system",
        created_at: "2026-03-01T10:03:00.000Z",
      },
    ]);

    vi.mocked(apiClient.listIssueTimeline).mockResolvedValue({
      items: [
        {
          event_id: "log-1",
          kind: "log",
          created_at: "2026-03-01T10:04:00.000Z",
          actor_type: "agent",
          actor_name: "worker",
          actor_avatar_seed: "worker",
          title: "log · review phase",
          body: "review log output",
          status: "success",
          refs: { issue_id: "plan-1" },
          meta: {},
        },
      ],
      total: 1,
      offset: 0,
    });

    vi.mocked(apiClient.listAdminAuditLog!).mockResolvedValue({
      items: [
        {
          id: 99,
          project_id: "proj-1",
          issue_id: "plan-1",
          pipeline_id: "",
          stage: "review",
          action: "human_reject",
          message: "manual feedback",
          source: "web",
          user_id: "admin",
          created_at: "2026-03-01T10:05:00.000Z",
        },
      ],
      total: 1,
      offset: 0,
    });

    render(
      <PlanView
        apiClient={apiClient}
        wsClient={wsClient}
        projectId="proj-1"
        refreshToken={0}
      />,
    );

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /Review Round 1 · fix/ })).toBeTruthy();
      expect(screen.getByRole("button", { name: /Change · status/ })).toBeTruthy();
      expect(screen.getByRole("button", { name: /log · review phase/ })).toBeTruthy();
      expect(screen.getByRole("button", { name: /Audit · human_reject/ })).toBeTruthy();
    });

    fireEvent.click(screen.getByRole("button", { name: /Review Round 1 · fix/ }));

    await waitFor(() => {
      expect(screen.getByText("评审结论")).toBeTruthy();
      expect(screen.getByText("完整审阅输出")).toBeTruthy();
      expect(screen.getByText(/review raw output:/)).toBeTruthy();
    });
  });

  it("后端返回空数组/异常结构时不白屏", async () => {
    const apiClient = createMockApiClient();
    const wsClient = createMockWsClient();

    vi.mocked(apiClient.listPlanReviews!).mockResolvedValue(null as unknown as PlanReviewRecord[]);
    vi.mocked(apiClient.listPlanChanges!).mockResolvedValue(null as unknown as PlanChangeRecord[]);
    vi.mocked(apiClient.listIssueTimeline).mockResolvedValue({
      items: null as unknown as IssueTimelineEntry[],
      total: 0,
      offset: 0,
    });
    vi.mocked(apiClient.listAdminAuditLog!).mockResolvedValue({
      items: null as unknown as AdminAuditLogItem[],
      total: 0,
      offset: 0,
    });

    render(
      <PlanView
        apiClient={apiClient}
        wsClient={wsClient}
        projectId="proj-1"
        refreshToken={0}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("审阅步骤")).toBeTruthy();
      expect(screen.getByText("步骤 0 · 原始输入文件")).toBeTruthy();
    });
  });
});
