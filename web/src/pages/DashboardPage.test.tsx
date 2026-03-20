// @vitest-environment jsdom
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import i18n from "@/i18n";
import { DashboardPage } from "./DashboardPage";

const { mockUseWorkbench } = vi.hoisted(() => ({
  mockUseWorkbench: vi.fn(),
}));

vi.mock("@/contexts/WorkbenchContext", () => ({
  useWorkbench: mockUseWorkbench,
}));

function renderPage() {
  return render(
    <I18nextProvider i18n={i18n}>
      <MemoryRouter>
        <DashboardPage />
      </MemoryRouter>
    </I18nextProvider>,
  );
}

describe("DashboardPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    void i18n.changeLanguage("zh-CN");
  });

  afterEach(() => {
    cleanup();
  });

  it("没有项目时展示创建首个项目引导", async () => {
    mockUseWorkbench.mockReturnValue({
      apiClient: {},
      selectedProject: null,
      selectedProjectId: null,
      projects: [],
    });

    renderPage();

    expect(screen.getByText("尚未建立工作区")).toBeTruthy();
    expect(screen.getByRole("link", { name: /创建第一个项目/ }).getAttribute("href")).toBe("/projects/new");
    expect(screen.getByText("创建完成后，仪表盘会自动展示真实工作项数据。")).toBeTruthy();
  });

  it("加载统计、执行中任务和调度队列", async () => {
    const apiClient = {
      getStats: vi.fn().mockResolvedValue({
        success_rate: 0.92,
        total_work_items: 8,
        avg_duration: "12m",
      }),
      listWorkItems: vi.fn().mockResolvedValue([
        {
          id: 11,
          title: "支付链路巡检",
          status: "running",
          created_at: "2026-03-15T00:00:00Z",
          updated_at: "2026-03-15T00:10:00Z",
        },
        {
          id: 12,
          title: "发布前校验",
          status: "queued",
          created_at: "2026-03-15T00:05:00Z",
          updated_at: "2026-03-15T00:06:00Z",
        },
        {
          id: 13,
          title: "历史归档",
          status: "done",
          created_at: "2026-03-14T00:00:00Z",
          updated_at: "2026-03-14T00:30:00Z",
        },
        {
          id: 14,
          title: "人工审核",
          status: "blocked",
          created_at: "2026-03-15T00:08:00Z",
          updated_at: "2026-03-15T00:09:00Z",
        },
      ]),
      getSchedulerStats: vi.fn().mockResolvedValue({
        enabled: true,
        message: "",
      }),
    };

    mockUseWorkbench.mockReturnValue({
      apiClient,
      selectedProject: { id: 7, name: "Alpha" },
      selectedProjectId: 7,
      projects: [{ id: 7, name: "Alpha" }],
    });

    renderPage();

    expect(await screen.findByText("Alpha 范围")).toBeTruthy();
    expect(screen.getByText("92%")).toBeTruthy();
    expect(screen.getAllByText("支付链路巡检").length).toBeGreaterThan(0);
    expect(screen.getAllByText("发布前校验").length).toBeGreaterThan(0);
    expect(screen.getByText("调度器已启用")).toBeTruthy();
    expect(screen.getByRole("link", { name: /新建工作项/ }).getAttribute("href")).toBe("/work-items/new");
    expect(screen.getByRole("link", { name: /提交需求/ }).getAttribute("href")).toBe("/requirements/new");
    expect(screen.getByRole("link", { name: /查看全部/ }).getAttribute("href")).toBe("/work-items");

    await waitFor(() => {
      expect(apiClient.listWorkItems).toHaveBeenCalledWith({
        project_id: 7,
        archived: false,
        limit: 50,
        offset: 0,
      });
    });
  });

  it("加载失败时展示错误", async () => {
    const apiClient = {
      getStats: vi.fn().mockRejectedValue(new Error("dashboard unavailable")),
      listWorkItems: vi.fn(),
      getSchedulerStats: vi.fn(),
    };

    mockUseWorkbench.mockReturnValue({
      apiClient,
      selectedProject: { id: 7, name: "Alpha" },
      selectedProjectId: 7,
      projects: [{ id: 7, name: "Alpha" }],
    });

    renderPage();

    expect(await screen.findByText("dashboard unavailable")).toBeTruthy();
  });
});
