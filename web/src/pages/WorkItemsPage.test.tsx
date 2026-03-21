// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import i18n from "../i18n";
import { WorkItemsPage } from "./WorkItemsPage";

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
        <WorkItemsPage />
      </MemoryRouter>
    </I18nextProvider>,
  );
}

describe("WorkItemsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    void i18n.changeLanguage("zh-CN");
  });

  afterEach(() => {
    cleanup();
  });

  it("按当前项目加载工作项，并支持搜索、优先级/标签过滤和列表切换", async () => {
    const apiClient = {
      listWorkItems: vi.fn().mockResolvedValue([
        {
          id: 1001,
          project_id: 9,
          title: "API 巡检",
          body: "检查接口可用性",
          priority: "high",
          status: "open",
          labels: ["backend", "api"],
          created_at: "2026-03-14T00:00:00Z",
          updated_at: "2026-03-14T00:00:00Z",
        },
        {
          id: 1002,
          project_id: 9,
          title: "修复导出缺陷",
          body: "处理 CSV 导出失败",
          priority: "urgent",
          status: "running",
          labels: ["bug", "frontend"],
          created_at: "2026-03-14T00:00:00Z",
          updated_at: "2026-03-14T00:30:00Z",
        },
        {
          id: 1003,
          project_id: 10,
          title: "补充发布脚本",
          body: "CI/CD 自动化",
          priority: "low",
          status: "done",
          labels: ["ops"],
          created_at: "2026-03-14T00:00:00Z",
          updated_at: "2026-03-14T00:30:00Z",
        },
      ]),
    };

    mockUseWorkbench.mockReturnValue({
      apiClient,
      selectedProjectId: 9,
      selectedProject: { id: 9, name: "Alpha" },
      projects: [
        { id: 9, name: "Alpha" },
        { id: 10, name: "Beta" },
      ],
    });

    renderPage();

    expect(await screen.findByText("API 巡检")).toBeTruthy();
    expect(apiClient.listWorkItems).toHaveBeenCalledWith({
      project_id: 9,
      archived: false,
      limit: 200,
      offset: 0,
    });

    fireEvent.change(screen.getByPlaceholderText("搜索工作项..."), {
      target: { value: "1002" },
    });

    await waitFor(() => {
      expect(screen.queryByText("API 巡检")).toBeNull();
      expect(screen.getByText("修复导出缺陷")).toBeTruthy();
    });

    fireEvent.change(screen.getByPlaceholderText("搜索工作项..."), {
      target: { value: "" },
    });

    fireEvent.click(screen.getByRole("button", { name: "全部优先级" }));
    fireEvent.click(screen.getByRole("button", { name: "高" }));

    await waitFor(() => {
      expect(screen.getByText("API 巡检")).toBeTruthy();
      expect(screen.queryByText("修复导出缺陷")).toBeNull();
    });

    fireEvent.click(screen.getByRole("button", { name: "高" }));
    fireEvent.click(screen.getByRole("button", { name: "全部优先级" }));
    fireEvent.click(screen.getByRole("button", { name: "全部标签" }));
    fireEvent.click(screen.getByRole("button", { name: "frontend" }));

    await waitFor(() => {
      expect(screen.getByText("修复导出缺陷")).toBeTruthy();
      expect(screen.queryByText("API 巡检")).toBeNull();
    });

    fireEvent.click(screen.getByRole("button", { name: "列表" }));

    expect(await screen.findByText("标题")).toBeTruthy();
    expect(screen.getByText("修复导出缺陷")).toBeTruthy();
  });

  it("加载失败时展示错误信息", async () => {
    const apiClient = {
      listWorkItems: vi.fn().mockRejectedValue(new Error("工作项读取失败")),
    };

    mockUseWorkbench.mockReturnValue({
      apiClient,
      selectedProjectId: null,
      selectedProject: null,
      projects: [],
    });

    renderPage();

    expect(await screen.findByText("工作项读取失败")).toBeTruthy();
  });
});
