// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import i18n from "../i18n";
import { CreateProjectPage } from "./CreateProjectPage";

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
        <CreateProjectPage />
      </MemoryRouter>
    </I18nextProvider>,
  );
}

describe("CreateProjectPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    void i18n.changeLanguage("zh-CN");
  });

  afterEach(() => {
    cleanup();
    vi.useRealTimers();
  });

  it("检测本地 Git 仓库后可切换为 Git 资源并创建项目", async () => {
    vi.useFakeTimers();

    const apiClient = {
      detectGitInfo: vi.fn().mockResolvedValue({
        is_git: true,
        remote_url: "https://github.com/acme/alpha.git",
        default_branch: "main",
        current_branch: "feature/refactor",
      }),
      createProject: vi.fn().mockResolvedValue({ id: 101 }),
      createProjectResource: vi.fn().mockResolvedValue({}),
    };
    const reloadProjects = vi.fn().mockResolvedValue(undefined);

    mockUseWorkbench.mockReturnValue({
      apiClient,
      reloadProjects,
    });

    renderPage();

    fireEvent.change(screen.getByPlaceholderText("例如：ai-workflow"), {
      target: { value: "Alpha Project" },
    });
    fireEvent.change(screen.getByPlaceholderText("描述项目的目标、技术栈和范围..."), {
      target: { value: "用于验证项目与 Git 资源创建" },
    });

    fireEvent.change(screen.getByPlaceholderText("D:/project/my-repo"), {
      target: { value: "D:/project/alpha" },
    });

    await vi.advanceTimersByTimeAsync(650);
    await Promise.resolve();
    vi.useRealTimers();

    expect(apiClient.detectGitInfo).toHaveBeenCalledWith("D:/project/alpha");

    expect(await screen.findByText("检测到 Git 仓库")).toBeTruthy();
    expect(screen.getByText("feature/refactor")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "切换为 Git 仓库资源" }));

    expect(await screen.findByText("启用 PR/CR 流程")).toBeTruthy();
    expect(screen.getAllByText("github").length).toBeGreaterThan(0);
    expect(screen.getByDisplayValue("main")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "创建项目" }));

    await waitFor(() => {
      expect(apiClient.createProject).toHaveBeenCalledWith({
        name: "Alpha Project",
        kind: "dev",
        description: "用于验证项目与 Git 资源创建",
      });
    });

    await waitFor(() => {
      expect(apiClient.createProjectResource).toHaveBeenCalledWith(101, {
        kind: "git",
        root_uri: "D:/project/alpha",
        label: "工作目录",
        config: {
          provider: "github",
          enable_scm_flow: true,
          base_branch: "main",
          merge_method: "squash",
        },
      });
    });

    await waitFor(() => {
      expect(reloadProjects).toHaveBeenCalledWith(101);
      expect(mockNavigate).toHaveBeenCalledWith("/projects");
    });
  });

  it("未填写项目名称时阻止提交", async () => {
    const apiClient = {
      detectGitInfo: vi.fn(),
      createProject: vi.fn(),
      createProjectResource: vi.fn(),
    };

    mockUseWorkbench.mockReturnValue({
      apiClient,
      reloadProjects: vi.fn(),
    });

    renderPage();

    fireEvent.click(screen.getByRole("button", { name: "创建项目" }));

    expect(await screen.findByText("项目名称不能为空。")).toBeTruthy();
    expect(apiClient.createProject).not.toHaveBeenCalled();
    expect(apiClient.createProjectResource).not.toHaveBeenCalled();
  });

  it("填写需求路由元数据时会写入 project metadata", async () => {
    const apiClient = {
      detectGitInfo: vi.fn(),
      createProject: vi.fn().mockResolvedValue({ id: 202 }),
      createProjectResource: vi.fn().mockResolvedValue({}),
    };

    mockUseWorkbench.mockReturnValue({
      apiClient,
      reloadProjects: vi.fn().mockResolvedValue(undefined),
    });

    renderPage();

    fireEvent.change(screen.getByPlaceholderText("例如：ai-workflow"), {
      target: { value: "Beta Project" },
    });
    fireEvent.change(screen.getByPlaceholderText("例如：负责认证、登录、两步验证和账号安全策略"), {
      target: { value: "负责认证与登录安全" },
    });
    fireEvent.change(screen.getByPlaceholderText("例如：go, gin, postgres"), {
      target: { value: "go, postgres" },
    });
    fireEvent.change(screen.getByPlaceholderText("例如：auth, otp, login"), {
      target: { value: "auth, otp" },
    });
    fireEvent.change(screen.getByPlaceholderText("例如：team-auth"), {
      target: { value: "team-auth" },
    });
    fireEvent.change(screen.getByPlaceholderText("例如：backend-api, infra-sms"), {
      target: { value: "infra-sms" },
    });
    fireEvent.change(screen.getByPlaceholderText("例如：arch-reviewer, backend-dev"), {
      target: { value: "backend-dev" },
    });

    fireEvent.click(screen.getByRole("button", { name: "创建项目" }));

    await waitFor(() => {
      expect(apiClient.createProject).toHaveBeenCalledWith({
        name: "Beta Project",
        kind: "dev",
        description: undefined,
        metadata: {
          scope: "负责认证与登录安全",
          tech_stack: "go, postgres",
          keywords: "auth, otp",
          owner: "team-auth",
          depends_on: "infra-sms",
          agent_hints: "backend-dev",
        },
      });
    });
  });
});
