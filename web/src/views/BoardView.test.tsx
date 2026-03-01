/** @vitest-environment jsdom */

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import BoardView, { groupBoardTasks, toBoardStatus, type BoardTask } from "./BoardView";
import type { ApiClient } from "../lib/apiClient";
import type { TaskPlan } from "../types/workflow";

const buildPlan = (tasks: TaskPlan["tasks"]): TaskPlan => ({
  id: "plan-1",
  project_id: "proj-1",
  session_id: "chat-1",
  name: "Plan One",
  status: "draft",
  wait_reason: "",
  tasks,
  fail_policy: "block",
  review_round: 0,
  created_at: "2026-03-01T10:00:00.000Z",
  updated_at: "2026-03-01T10:00:00.000Z",
});

const createMockApiClient = (): ApiClient => {
  return {
    request: vi.fn(),
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    del: vi.fn(),
    getStats: vi.fn(),
    listProjects: vi.fn(),
    createProject: vi.fn(),
    listPipelines: vi.fn(),
    createPipeline: vi.fn(),
    createChat: vi.fn(),
    getChat: vi.fn(),
    createPlan: vi.fn(),
    submitPlanReview: vi.fn(),
    applyPlanAction: vi.fn(),
    applyTaskAction: vi.fn().mockImplementation(async () => ({
      status: "ready",
    })),
    getPipeline: vi.fn(),
    getPipelineCheckpoints: vi.fn(),
    applyPipelineAction: vi.fn(),
    listPlans: vi.fn().mockResolvedValue({
      items: [
        buildPlan([
          {
            id: "task-1",
            plan_id: "plan-1",
            title: "Task pending",
            description: "d1",
            labels: [],
            depends_on: [],
            template: "",
            pipeline_id: "",
            external_id: "",
            status: "pending",
            created_at: "2026-03-01T10:00:00.000Z",
            updated_at: "2026-03-01T10:00:00.000Z",
          },
          {
            id: "task-2",
            plan_id: "plan-1",
            title: "Task failed",
            description: "d2",
            labels: [],
            depends_on: [],
            template: "",
            pipeline_id: "",
            external_id: "",
            status: "blocked_by_failure",
            created_at: "2026-03-01T10:00:00.000Z",
            updated_at: "2026-03-01T10:00:00.000Z",
          },
        ]),
      ],
      total: 1,
      offset: 0,
    }),
    getPlanDag: vi.fn(),
  } as unknown as ApiClient;
};

const createDeferred = <T,>() => {
  let resolve: (value: T | PromiseLike<T>) => void = () => {};
  const promise = new Promise<T>((r) => {
    resolve = r;
  });
  return { promise, resolve };
};

describe("BoardView helpers", () => {
  it("toBoardStatus maps blocked/skipped states into board columns", () => {
    expect(toBoardStatus("blocked_by_failure")).toBe("failed");
    expect(toBoardStatus("skipped")).toBe("done");
  });

  it("groupBoardTasks returns all five columns", () => {
    const tasks: BoardTask[] = [
      {
        id: "t1",
        title: "task",
        plan_id: "p1",
        plan_name: "Plan",
        status: "running",
        pipeline_id: "",
      },
    ];

    const grouped = groupBoardTasks(tasks);
    expect(Object.keys(grouped)).toEqual([
      "pending",
      "ready",
      "running",
      "done",
      "failed",
    ]);
    expect(grouped.running).toHaveLength(1);
  });
});

describe("BoardView", () => {
  afterEach(() => {
    cleanup();
    vi.useRealTimers();
  });

  it("从真实 plans/tasks 数据渲染看板列", async () => {
    const apiClient = createMockApiClient();
    render(<BoardView apiClient={apiClient} projectId="proj-1" refreshToken={0} />);

    await waitFor(() => {
      expect(apiClient.listPlans).toHaveBeenCalledWith("proj-1", {
        limit: 50,
        offset: 0,
      });
    });

    expect(screen.getByText("Pending")).toBeTruthy();
    expect(screen.getByText("Failed")).toBeTruthy();
    expect(screen.getByText("Task pending")).toBeTruthy();
    expect(screen.getByText("Task failed")).toBeTruthy();
  });

  it("看板会循环拉取所有分页计划数据", async () => {
    const apiClient = createMockApiClient();
    vi.mocked(apiClient.listPlans)
      .mockResolvedValueOnce({
        items: Array.from({ length: 50 }, (_, index) =>
          buildPlan([
            {
              id: `task-${index}`,
              plan_id: `plan-${index}`,
              title: `Task ${index}`,
              description: "d",
              labels: [],
              depends_on: [],
              template: "",
              pipeline_id: "",
              external_id: "",
              status: "pending",
              created_at: "2026-03-01T10:00:00.000Z",
              updated_at: "2026-03-01T10:00:00.000Z",
            },
          ]),
        ),
        total: 51,
        offset: 0,
      })
      .mockResolvedValueOnce({
        items: [
          {
            ...buildPlan([
              {
                id: "task-last",
                plan_id: "plan-last",
                title: "Task Last",
                description: "d",
                labels: [],
                depends_on: [],
                template: "",
                pipeline_id: "",
                external_id: "",
                status: "pending",
                created_at: "2026-03-01T10:00:00.000Z",
                updated_at: "2026-03-01T10:00:00.000Z",
              },
            ]),
            id: "plan-last",
          },
        ],
        total: 51,
        offset: 50,
      });

    render(<BoardView apiClient={apiClient} projectId="proj-1" refreshToken={0} />);

    await waitFor(() => {
      expect(apiClient.listPlans).toHaveBeenNthCalledWith(1, "proj-1", {
        limit: 50,
        offset: 0,
      });
      expect(apiClient.listPlans).toHaveBeenNthCalledWith(2, "proj-1", {
        limit: 50,
        offset: 50,
      });
    });

    expect(screen.getByText("Task Last")).toBeTruthy();
  });

  it("项目切换后会忽略旧请求返回，避免脏回写", async () => {
    const staleDeferred = createDeferred<{
      items: TaskPlan[];
      total: number;
      offset: number;
    }>();
    const apiClient = createMockApiClient();
    vi.mocked(apiClient.listPlans).mockImplementation((projectId) => {
      if (projectId === "proj-1") {
        return staleDeferred.promise;
      }
      return Promise.resolve({
        items: [
          buildPlan([
            {
              id: "task-fresh",
              plan_id: "plan-1",
              title: "Task fresh",
              description: "d",
              labels: [],
              depends_on: [],
              template: "",
              pipeline_id: "",
              external_id: "",
              status: "ready",
              created_at: "2026-03-01T10:00:00.000Z",
              updated_at: "2026-03-01T10:00:00.000Z",
            },
          ]),
        ],
        total: 1,
        offset: 0,
      });
    });

    const { rerender } = render(<BoardView apiClient={apiClient} projectId="proj-1" refreshToken={0} />);

    rerender(<BoardView apiClient={apiClient} projectId="proj-2" refreshToken={0} />);

    staleDeferred.resolve({
      items: [
        buildPlan([
          {
            id: "task-stale",
            plan_id: "plan-1",
            title: "Task stale",
            description: "d",
            labels: [],
            depends_on: [],
            template: "",
            pipeline_id: "",
            external_id: "",
            status: "pending",
            created_at: "2026-03-01T10:00:00.000Z",
            updated_at: "2026-03-01T10:00:00.000Z",
          },
        ]),
      ],
      total: 1,
      offset: 0,
    });

    await waitFor(() => {
      expect(apiClient.listPlans).toHaveBeenCalledWith("proj-2", {
        limit: 50,
        offset: 0,
      });
    });

    expect(screen.getByText("Task fresh")).toBeTruthy();
    expect(screen.queryByText("Task stale")).toBeNull();
  });

  it("refreshToken 变化后会立即触发一次刷新", async () => {
    const apiClient = createMockApiClient();
    const { rerender } = render(<BoardView apiClient={apiClient} projectId="proj-1" refreshToken={0} />);

    await waitFor(() => {
      expect(apiClient.listPlans).toHaveBeenCalledTimes(1);
    });

    rerender(<BoardView apiClient={apiClient} projectId="proj-1" refreshToken={1} />);

    await waitFor(() => {
      expect(apiClient.listPlans).toHaveBeenCalledTimes(2);
    });
    expect(apiClient.listPlans).toHaveBeenNthCalledWith(2, "proj-1", {
      limit: 50,
      offset: 0,
    });
  });

  it("看板视图会通过定时拉取做刷新兜底", async () => {
    vi.useFakeTimers();
    const apiClient = createMockApiClient();
    render(<BoardView apiClient={apiClient} projectId="proj-1" refreshToken={0} />);

    expect(apiClient.listPlans).toHaveBeenCalledTimes(1);

    await vi.advanceTimersByTimeAsync(10_000);

    expect(apiClient.listPlans).toHaveBeenCalledTimes(2);
  });

  it("任务右键菜单包含 retry/skip/abort，并调用 task action API", async () => {
    const apiClient = createMockApiClient();
    render(<BoardView apiClient={apiClient} projectId="proj-1" refreshToken={0} />);

    await waitFor(() => {
      expect(screen.getByText("Task pending")).toBeTruthy();
    });

    const taskCard = screen.getByText("Task pending").closest("button");
    if (!taskCard) {
      throw new Error("task card not found");
    }

    fireEvent.contextMenu(taskCard);
    expect(screen.getByRole("button", { name: "retry" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "skip" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "abort" })).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "skip" }));

    await waitFor(() => {
      expect(apiClient.applyTaskAction).toHaveBeenCalledWith(
        "proj-1",
        "plan-1",
        "task-1",
        { action: "skip" },
      );
    });
  });

  it("拖拽到目标列会触发对应 task action API", async () => {
    const apiClient = createMockApiClient();
    render(<BoardView apiClient={apiClient} projectId="proj-1" refreshToken={0} />);

    await waitFor(() => {
      expect(screen.getByText("Task pending")).toBeTruthy();
    });

    const taskCard = screen.getByText("Task pending").closest("button");
    if (!taskCard) {
      throw new Error("task card not found");
    }

    fireEvent.dragStart(taskCard);
    fireEvent.dragOver(screen.getByTestId("board-column-ready"));
    fireEvent.drop(screen.getByTestId("board-column-ready"));

    await waitFor(() => {
      expect(apiClient.applyTaskAction).toHaveBeenCalledWith(
        "proj-1",
        "plan-1",
        "task-1",
        { action: "retry" },
      );
    });
  });
});
