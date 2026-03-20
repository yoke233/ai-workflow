// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import i18n from "../i18n";
import { ThreadsPage } from "./ThreadsPage";

const { mockUseWorkbench } = vi.hoisted(() => ({
  mockUseWorkbench: vi.fn(),
}));

const { mockNavigate } = vi.hoisted(() => ({
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
        <ThreadsPage />
      </MemoryRouter>
    </I18nextProvider>,
  );
}

describe("ThreadsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    void i18n.changeLanguage("zh-CN");
  });

  afterEach(() => {
    cleanup();
  });

  it("加载讨论列表并支持搜索", async () => {
    const apiClient = {
      listThreads: vi.fn().mockResolvedValue([
        {
          id: 7,
          title: "支付问题排查",
          status: "active",
          owner_id: "alice",
          created_at: "2026-03-15T00:00:00Z",
          updated_at: "2026-03-15T00:00:00Z",
        },
        {
          id: 8,
          title: "发布复盘",
          status: "closed",
          owner_id: "bob",
          created_at: "2026-03-15T00:00:00Z",
          updated_at: "2026-03-15T00:00:00Z",
        },
      ]),
    };

    mockUseWorkbench.mockReturnValue({ apiClient });

    renderPage();

    expect(await screen.findByText("支付问题排查")).toBeTruthy();
    expect(screen.getByText("发布复盘")).toBeTruthy();
    expect(screen.getByRole("link", { name: "从需求创建" }).getAttribute("href")).toBe("/requirements/new");

    fireEvent.change(screen.getByPlaceholderText("Search threads..."), {
      target: { value: "发布" },
    });

    await waitFor(() => {
      expect(screen.getByText("发布复盘")).toBeTruthy();
      expect(screen.queryByText("支付问题排查")).toBeNull();
    });
  });

  it("输入首条消息创建讨论并跳转", async () => {
    const apiClient = {
      listThreads: vi.fn().mockResolvedValue([]),
      createThread: vi.fn().mockResolvedValue({
        id: 9,
        title: "新的讨论",
        status: "active",
        created_at: "2026-03-15T00:00:00Z",
        updated_at: "2026-03-15T00:00:00Z",
      }),
      createThreadMessage: vi.fn().mockResolvedValue({
        id: 1,
        thread_id: 9,
        content: "新的讨论",
        role: "human",
        sender_id: "human",
        created_at: "2026-03-15T00:00:00Z",
      }),
    };

    mockUseWorkbench.mockReturnValue({ apiClient });

    renderPage();

    await screen.findByPlaceholderText("Start a new discussion...");

    fireEvent.change(screen.getByPlaceholderText("Start a new discussion..."), {
      target: { value: "新的讨论" },
    });

    // Click the send button
    const sendButton = screen.getByRole("button");
    fireEvent.click(sendButton);

    await waitFor(() => {
      expect(apiClient.createThread).toHaveBeenCalledWith({ title: "新的讨论" });
      expect(apiClient.createThreadMessage).toHaveBeenCalledWith(9, {
        content: "新的讨论",
        role: "human",
        sender_id: "human",
      });
      expect(mockNavigate).toHaveBeenCalledWith("/threads/9");
    });
  });

  it("创建讨论失败时展示错误", async () => {
    const apiClient = {
      listThreads: vi.fn().mockResolvedValue([]),
      createThread: vi.fn().mockRejectedValue(new Error("create failed")),
    };

    mockUseWorkbench.mockReturnValue({ apiClient });

    renderPage();

    await screen.findByPlaceholderText("Start a new discussion...");

    fireEvent.change(screen.getByPlaceholderText("Start a new discussion..."), {
      target: { value: "失败的讨论" },
    });

    const sendButton = screen.getByRole("button");
    fireEvent.click(sendButton);

    expect(await screen.findByText("create failed")).toBeTruthy();
  });
});
