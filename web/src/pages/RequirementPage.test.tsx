// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import i18n from "@/i18n";
import { RequirementPage } from "./RequirementPage";

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
        <RequirementPage />
      </MemoryRouter>
    </I18nextProvider>,
  );
}

describe("RequirementPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    void i18n.changeLanguage("zh-CN");
  });

  afterEach(() => {
    cleanup();
  });

  it("分析需求并创建 thread", async () => {
    const apiClient = {
      listProjects: vi.fn().mockResolvedValue([{ id: 1, name: "backend-api" }]),
      listProfiles: vi.fn().mockResolvedValue([{ id: "backend-dev", name: "Backend Dev" }]),
      analyzeRequirement: vi.fn().mockResolvedValue({
        analysis: {
          summary: "登录 OTP 改造",
          type: "single_project",
          complexity: "medium",
          suggested_meeting_mode: "concurrent",
          matched_projects: [{ project_id: 1, project_name: "backend-api", reason: "涉及后端鉴权" }],
          suggested_agents: [{ profile_id: "backend-dev", reason: "处理后端接口" }],
          risks: ["注意兼容旧版登录流程"],
        },
        suggested_thread: {
          title: "讨论：登录 OTP 改造",
          context_refs: [{ project_id: 1, access: "read" }],
          agents: ["backend-dev"],
          meeting_mode: "concurrent",
          meeting_max_rounds: 4,
        },
      }),
      createThreadFromRequirement: vi.fn().mockResolvedValue({
        thread: { id: 42 },
      }),
    };

    mockUseWorkbench.mockReturnValue({ apiClient });

    renderPage();

    fireEvent.change(
      await screen.findByPlaceholderText("例如：给用户登录系统增加两步验证，后端要支持 OTP 校验，前端要补输入流程。"),
      { target: { value: "给登录系统增加 OTP 校验" } },
    );

    fireEvent.click(screen.getByRole("button", { name: /分析需求/ }));

    expect(await screen.findByText("登录 OTP 改造")).toBeTruthy();
    expect(screen.getByText("涉及后端鉴权")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: /创建讨论 Thread/ }));

    await waitFor(() => {
      expect(apiClient.createThreadFromRequirement).toHaveBeenCalledWith({
        description: "给登录系统增加 OTP 校验",
        context: undefined,
        owner_id: "human",
        analysis: {
          summary: "登录 OTP 改造",
          type: "single_project",
          complexity: "medium",
          suggested_meeting_mode: "concurrent",
          matched_projects: [{ project_id: 1, project_name: "backend-api", reason: "涉及后端鉴权" }],
          suggested_agents: [{ profile_id: "backend-dev", reason: "处理后端接口" }],
          risks: ["注意兼容旧版登录流程"],
        },
        thread_config: {
          title: "讨论：登录 OTP 改造",
          context_refs: [{ project_id: 1, access: "read" }],
          agents: ["backend-dev"],
          meeting_mode: "concurrent",
          meeting_max_rounds: 4,
        },
      });
      expect(mockNavigate).toHaveBeenCalledWith("/threads/42");
    });
  });
});
