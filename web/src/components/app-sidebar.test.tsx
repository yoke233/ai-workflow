import type { ComponentProps } from "react";
// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import i18n from "@/i18n";
import { AppSidebar } from "./app-sidebar";

const { mockUseWorkbench, mockSaveLanguage } = vi.hoisted(() => ({
  mockUseWorkbench: vi.fn(),
  mockSaveLanguage: vi.fn(),
}));

vi.mock("@/contexts/WorkbenchContext", () => ({
  useWorkbench: mockUseWorkbench,
}));

vi.mock("@/i18n", async () => {
  const actual = await vi.importActual<typeof import("@/i18n")>("@/i18n");
  return {
    ...actual,
    saveLanguage: mockSaveLanguage,
  };
});

function renderSidebar(
  initialEntry = "/projects",
  props: ComponentProps<typeof AppSidebar> = {},
) {
  return render(
    <I18nextProvider i18n={i18n}>
      <MemoryRouter initialEntries={[initialEntry]}>
        <AppSidebar {...props} />
      </MemoryRouter>
    </I18nextProvider>,
  );
}

describe("AppSidebar", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    void i18n.changeLanguage("zh-CN");
  });

  afterEach(() => {
    cleanup();
  });

  it("支持切换项目、切换语言并退出登录", async () => {
    const setSelectedProjectId = vi.fn();
    const logout = vi.fn();

    mockUseWorkbench.mockReturnValue({
      projects: [
        { id: 1, name: "Alpha" },
        { id: 2, name: "Beta" },
      ],
      selectedProjectId: 1,
      setSelectedProjectId,
      logout,
    });

    renderSidebar();

    fireEvent.click(screen.getByRole("button", { name: /Alpha/ }));
    fireEvent.click(screen.getByRole("button", { name: /Beta/ }));

    expect(setSelectedProjectId).toHaveBeenCalledWith(2);

    fireEvent.click(screen.getByRole("button", { name: /English|中文/ }));

    await waitFor(() => {
      expect(mockSaveLanguage).toHaveBeenCalledTimes(1);
    });

    fireEvent.click(screen.getByRole("button", { name: /退出登录|Logout/ }));
    expect(logout).toHaveBeenCalledTimes(1);

    const allButtons = screen.getAllByRole("button");
    fireEvent.click(allButtons[allButtons.length - 1]);

    expect(localStorage.getItem("sidebar-collapsed")).toBe("true");
  });

  it("移动端抽屉不会复用桌面折叠态", () => {
    mockUseWorkbench.mockReturnValue({
      projects: [
        { id: 1, name: "Alpha" },
      ],
      selectedProjectId: 1,
      setSelectedProjectId: vi.fn(),
      logout: vi.fn(),
    });
    localStorage.setItem("sidebar-collapsed", "true");

    renderSidebar("/", { drawerOpen: true, onClose: vi.fn() });

    expect(screen.getByText("AI Workflow")).toBeTruthy();
    expect(screen.getByRole("link", { name: /首页/ })).toBeTruthy();
    expect(screen.queryByTitle(/Expand sidebar|展开侧栏/)).toBeNull();
  });
});

