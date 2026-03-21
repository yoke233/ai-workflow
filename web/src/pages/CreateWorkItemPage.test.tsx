// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import i18n from "../i18n";
import { CreateWorkItemPage } from "./CreateWorkItemPage";

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

function renderPage(initialEntry = "/work-items/new") {
  return render(
    <I18nextProvider i18n={i18n}>
      <MemoryRouter initialEntries={[initialEntry]}>
        <CreateWorkItemPage />
      </MemoryRouter>
    </I18nextProvider>,
  );
}

describe("CreateWorkItemPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    void i18n.changeLanguage("zh-CN");
  });

  afterEach(() => {
    cleanup();
  });

  it("支持手工创建工作项并在提交时补生成动作后直接运行", async () => {
    const apiClient = {
      listDAGTemplates: vi.fn().mockResolvedValue([]),
      listProjectResources: vi.fn().mockResolvedValue([]),
      createWorkItem: vi.fn().mockResolvedValue({ id: 55 }),
      generateActions: vi.fn().mockResolvedValue([
        { id: 1, name: "编码实现", type: "exec" },
      ]),
      runWorkItem: vi.fn().mockResolvedValue({}),
      generateTitle: vi.fn(),
      uploadWorkItemAttachment: vi.fn(),
      deleteWorkItemAttachment: vi.fn(),
    };

    mockUseWorkbench.mockReturnValue({
      apiClient,
      selectedProjectId: 9,
      projects: [{ id: 9, name: "Alpha" }],
    });

    renderPage();

    await screen.findByRole("heading", { name: "新建工作项" });

    fireEvent.change(screen.getByPlaceholderText("例如：后端 API 重构"), {
      target: { value: "接入告警链路" },
    });
    fireEvent.change(screen.getByPlaceholderText("描述这个工作项要完成的目标和范围..."), {
      target: { value: "补齐异常通知与回调配置" },
    });

    fireEvent.click(screen.getByRole("button", { name: "创建并运行" }));

    await waitFor(() => {
      expect(apiClient.createWorkItem).toHaveBeenCalledWith({
        title: "接入告警链路",
        project_id: 9,
        metadata: { description: "补齐异常通知与回调配置" },
      });
    });

    await waitFor(() => {
      expect(apiClient.generateActions).toHaveBeenCalledWith(55, {
        description: "补齐异常通知与回调配置",
      });
      expect(apiClient.runWorkItem).toHaveBeenCalledWith(55);
    });

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith("/work-items/55");
    });
  });

  it("选中模板后通过模板创建并运行工作项", async () => {
    const apiClient = {
      listDAGTemplates: vi.fn().mockResolvedValue([
        {
          id: 7,
          name: "标准发布",
          description: "包含构建、发布与验收动作",
          actions: [
            { name: "构建", type: "exec" },
            { name: "发布", type: "exec" },
          ],
          tags: ["release"],
        },
      ]),
      listProjectResources: vi.fn().mockResolvedValue([]),
      createWorkItemFromTemplate: vi.fn().mockResolvedValue({
        work_item: { id: 88 },
      }),
      runWorkItem: vi.fn().mockResolvedValue({}),
      createWorkItem: vi.fn(),
      generateActions: vi.fn(),
      generateTitle: vi.fn(),
      uploadWorkItemAttachment: vi.fn(),
      deleteWorkItemAttachment: vi.fn(),
    };

    mockUseWorkbench.mockReturnValue({
      apiClient,
      selectedProjectId: 9,
      projects: [{ id: 9, name: "Alpha" }],
    });

    renderPage();

    const templateButton = await screen.findByRole("button", { name: /标准发布/ });
    fireEvent.click(templateButton);

    expect(await screen.findByDisplayValue("标准发布")).toBeTruthy();
    expect(screen.getByDisplayValue("包含构建、发布与验收动作")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "创建并运行" }));

    await waitFor(() => {
      expect(apiClient.createWorkItemFromTemplate).toHaveBeenCalledWith(7, {
        title: "标准发布",
        project_id: 9,
      });
      expect(apiClient.runWorkItem).toHaveBeenCalledWith(88);
    });

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith("/work-items/88");
    });
  });
});
