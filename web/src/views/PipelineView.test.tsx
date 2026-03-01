/** @vitest-environment jsdom */

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import PipelineView from "./PipelineView";
import type { ApiClient } from "../lib/apiClient";
import type { ApiPipeline } from "../types/api";

const buildPipeline = (id: string): ApiPipeline => ({
  id,
  project_id: "proj-1",
  task_item_id: `task-${id}`,
  name: `Pipeline ${id}`,
  description: "",
  template: "standard",
  status: "running",
  current_stage: "implement",
  artifacts: {},
  config: {},
  branch_name: "",
  worktree_path: "",
  max_total_retries: 5,
  total_retries: 0,
  started_at: "2026-03-01T10:00:00.000Z",
  finished_at: "",
  created_at: "2026-03-01T10:00:00.000Z",
  updated_at: "2026-03-01T10:10:00.000Z",
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
    listPipelines: vi.fn().mockResolvedValue({
      items: [buildPipeline("pipe-1")],
      total: 1,
      offset: 0,
    }),
    createPipeline: vi.fn(),
    createChat: vi.fn(),
    getChat: vi.fn(),
    createPlan: vi.fn(),
    submitPlanReview: vi.fn(),
    applyPlanAction: vi.fn(),
    applyTaskAction: vi.fn(),
    getPipeline: vi.fn().mockResolvedValue(buildPipeline("pipe-1")),
    getPipelineCheckpoints: vi.fn().mockResolvedValue([
      {
        pipeline_id: "pipe-1",
        stage_name: "requirements",
        status: "success",
        artifacts: { summary: "ok" },
        started_at: "2026-03-01T10:00:00.000Z",
        finished_at: "2026-03-01T10:01:00.000Z",
        agent_used: "claude",
        tokens_used: 12,
        retry_count: 0,
        error: "",
      },
    ]),
    applyPipelineAction: vi.fn().mockResolvedValue({
      status: "aborted",
      current_stage: "requirements",
    }),
    listPlans: vi.fn(),
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

describe("PipelineView", () => {
  afterEach(() => {
    cleanup();
    vi.useRealTimers();
  });

  it("调用 pipelines API 并渲染最小列表", async () => {
    const apiClient = createMockApiClient();
    render(<PipelineView apiClient={apiClient} projectId="proj-1" refreshToken={0} />);

    await waitFor(() => {
      expect(apiClient.listPipelines).toHaveBeenCalledWith("proj-1", {
        limit: 50,
        offset: 0,
      });
    });

    expect(screen.getByText("Pipeline pipe-1")).toBeTruthy();
    expect(screen.getByText("running")).toBeTruthy();
    expect(screen.getAllByTestId("pipeline-row")).toHaveLength(1);
  });

  it("会加载并显示 checkpoint 区，且人工动作按钮可调用 API", async () => {
    const apiClient = createMockApiClient();
    render(<PipelineView apiClient={apiClient} projectId="proj-1" refreshToken={0} />);

    await waitFor(() => {
      expect(apiClient.getPipelineCheckpoints).toHaveBeenCalledWith("proj-1", "pipe-1");
    });
    expect(screen.getByText("requirements")).toBeTruthy();
    expect(screen.getByText("success")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "Abort" }));
    await waitFor(() => {
      expect(apiClient.applyPipelineAction).toHaveBeenCalledWith("proj-1", "pipe-1", {
        action: "abort",
      });
    });
  });

  it("会循环拉取分页数据直到拉全量", async () => {
    const apiClient = createMockApiClient();
    vi.mocked(apiClient.listPipelines)
      .mockResolvedValueOnce({
        items: Array.from({ length: 50 }, (_, index) => buildPipeline(`pipe-${index}`)),
        total: 50,
        offset: 0,
      })
      .mockResolvedValueOnce({
        items: [buildPipeline("pipe-50")],
        total: 1,
        offset: 50,
      });

    render(<PipelineView apiClient={apiClient} projectId="proj-1" refreshToken={0} />);

    await waitFor(() => {
      expect(apiClient.listPipelines).toHaveBeenNthCalledWith(1, "proj-1", {
        limit: 50,
        offset: 0,
      });
      expect(apiClient.listPipelines).toHaveBeenNthCalledWith(2, "proj-1", {
        limit: 50,
        offset: 50,
      });
    });

    expect(screen.getAllByTestId("pipeline-row")).toHaveLength(51);
  });

  it("项目切换后会忽略旧请求返回，避免脏回写", async () => {
    const staleDeferred = createDeferred<{
      items: ApiPipeline[];
      total: number;
      offset: number;
    }>();
    const apiClient = createMockApiClient();
    vi.mocked(apiClient.listPipelines).mockImplementation((projectId) => {
      if (projectId === "proj-1") {
        return staleDeferred.promise;
      }
      return Promise.resolve({
        items: [buildPipeline("pipe-fresh")],
        total: 1,
        offset: 0,
      });
    });

    const { rerender } = render(<PipelineView apiClient={apiClient} projectId="proj-1" refreshToken={0} />);

    rerender(<PipelineView apiClient={apiClient} projectId="proj-2" refreshToken={0} />);
    staleDeferred.resolve({
      items: [buildPipeline("pipe-stale")],
      total: 1,
      offset: 0,
    });

    await waitFor(() => {
      expect(apiClient.listPipelines).toHaveBeenCalledWith("proj-2", {
        limit: 50,
        offset: 0,
      });
    });

    expect(screen.getByText("Pipeline pipe-fresh")).toBeTruthy();
    expect(screen.queryByText("Pipeline pipe-stale")).toBeNull();
  });

  it("refreshToken 变化后会立即触发一次刷新", async () => {
    const apiClient = createMockApiClient();
    const { rerender } = render(<PipelineView apiClient={apiClient} projectId="proj-1" refreshToken={0} />);

    await waitFor(() => {
      expect(apiClient.listPipelines).toHaveBeenCalledTimes(1);
    });

    rerender(<PipelineView apiClient={apiClient} projectId="proj-1" refreshToken={1} />);

    await waitFor(() => {
      expect(apiClient.listPipelines).toHaveBeenCalledTimes(2);
    });
    expect(apiClient.listPipelines).toHaveBeenNthCalledWith(2, "proj-1", {
      limit: 50,
      offset: 0,
    });
  });

  it("会通过定时拉取做刷新兜底", async () => {
    vi.useFakeTimers();
    const apiClient = createMockApiClient();
    render(<PipelineView apiClient={apiClient} projectId="proj-1" refreshToken={0} />);

    expect(apiClient.listPipelines).toHaveBeenCalledTimes(1);

    await vi.advanceTimersByTimeAsync(10_000);

    expect(apiClient.listPipelines).toHaveBeenCalledTimes(2);
  });
});
