// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import i18n from "../i18n";
import { TemplatesPage } from "./TemplatesPage";

const { mockUseWorkbench, mockNavigate } = vi.hoisted(() => ({
  mockUseWorkbench: vi.fn(),
  mockNavigate: vi.fn(),
}));

vi.mock("@/contexts/WorkbenchContext", () => ({
  useWorkbench: mockUseWorkbench,
}));

vi.mock("react-router-dom", async () => {
  const actual = await vi.importActual<typeof import("react-router-dom")>("react-router-dom");
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

function renderPage() {
  return render(
    <I18nextProvider i18n={i18n}>
      <MemoryRouter>
        <TemplatesPage />
      </MemoryRouter>
    </I18nextProvider>,
  );
}

describe("TemplatesPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    void i18n.changeLanguage("zh-CN");
  });

  afterEach(() => {
    cleanup();
  });

  it("加载模板后支持搜索、基于模板创建工作项并删除模板", async () => {
    const apiClient = {
      listDAGTemplates: vi.fn().mockResolvedValue([
        {
          id: 1,
          name: "标准发布",
          description: "包含构建、发布与验收",
          actions: [{ name: "构建", type: "exec" }, { name: "发布", type: "exec" }],
          tags: ["release"],
          created_at: "2026-03-13T00:00:00Z",
        },
        {
          id: 2,
          name: "清理流水线",
          description: "下线过期资源",
          actions: [{ name: "清理", type: "exec" }],
          tags: ["ops"],
          created_at: "2026-03-12T00:00:00Z",
        },
      ]),
      deleteDAGTemplate: vi.fn().mockResolvedValue({}),
      createWorkItemFromTemplate: vi.fn().mockResolvedValue({
        work_item: { id: 88 },
      }),
    };

    mockUseWorkbench.mockReturnValue({
      apiClient,
      selectedProjectId: 9,
    });

    renderPage();

    await waitFor(() => {
      expect(apiClient.listDAGTemplates).toHaveBeenCalledWith({
        project_id: 9,
        limit: 200,
      });
    });

    expect((await screen.findAllByText("标准发布")).length).toBeGreaterThan(0);
    expect(screen.getAllByText("清理流水线").length).toBeGreaterThan(0);

    fireEvent.change(screen.getByPlaceholderText("搜索模板..."), {
      target: { value: "标准" },
    });

    await waitFor(() => {
      expect(screen.getAllByText("标准发布").length).toBeGreaterThan(0);
      expect(screen.queryByText("清理流水线")).toBeNull();
    });

    const releaseRow = screen.getAllByText("标准发布")[0].closest("tr");
    expect(releaseRow).toBeTruthy();
    fireEvent.click(within(releaseRow as HTMLElement).getByRole("button", { name: "创建工作项" }));

    await waitFor(() => {
      expect(apiClient.createWorkItemFromTemplate).toHaveBeenCalledWith(1, {
        title: "标准发布",
        project_id: 9,
      });
      expect(mockNavigate).toHaveBeenCalledWith("/work-items/88");
    });

    fireEvent.change(screen.getByPlaceholderText("搜索模板..."), {
      target: { value: "" },
    });

    const cleanupRow = (await screen.findAllByText("清理流水线"))[0];
    fireEvent.click(within(cleanupRow.closest("tr") as HTMLElement).getAllByRole("button")[1]);

    await waitFor(() => {
      expect(apiClient.deleteDAGTemplate).toHaveBeenCalledWith(2);
    });

    await waitFor(() => {
      expect(screen.queryByText("清理流水线")).toBeNull();
    });
  });
});
