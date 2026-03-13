// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import i18n from "../i18n";
import { ChatPage } from "./ChatPage";

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

vi.mock("@/components/chat/ChatSessionSidebar", () => ({
  ChatSessionSidebar: () => <div data-testid="chat-session-sidebar" />,
}));

vi.mock("@/components/chat/DraftSessionSetup", () => ({
  DraftSessionSetup: () => <div data-testid="draft-session-setup">draft setup</div>,
}));

vi.mock("@/components/chat/MessageFeedView", () => ({
  MessageFeedView: () => <div data-testid="message-feed-view" />,
}));

vi.mock("@/components/chat/ChatInputBar", () => ({
  ChatInputBar: () => <div data-testid="chat-input-bar" />,
}));

vi.mock("@/components/chat/ChatEventsPanel", () => ({
  ChatEventsPanel: () => <div data-testid="chat-events-panel" />,
}));

vi.mock("@/components/chat/PermissionBar", () => ({
  PermissionBar: () => <div data-testid="permission-bar" />,
}));

vi.mock("@/components/chat/ChatScrollTrack", () => ({
  ChatScrollTrack: () => <div data-testid="chat-scroll-track" />,
}));

function createWsClientMock() {
  return {
    subscribe: vi.fn(() => vi.fn()),
    send: vi.fn(),
  };
}

function buildProject(id = 9, name = "Project Alpha") {
  return {
    id,
    name,
    kind: "dev",
    created_at: "2026-03-13T00:00:00Z",
    updated_at: "2026-03-13T00:00:00Z",
  };
}

function buildSession() {
  return {
    session_id: "session-1",
    title: "多 agent 协作聊天",
    project_id: 9,
    project_name: "Project Alpha",
    profile_id: "lead-1",
    profile_name: "Lead One",
    driver_id: "codex-cli",
    created_at: "2026-03-13T00:00:00Z",
    updated_at: "2026-03-13T01:00:00Z",
    status: "alive",
    message_count: 2,
  };
}

function buildSessionDetail() {
  return {
    ...buildSession(),
    messages: [
      {
        role: "user",
        content: "把这次聊天整理成 thread",
        time: "2026-03-13T00:30:00Z",
      },
      {
        role: "assistant",
        content: "我会先整理重点，再转成执行项。",
        time: "2026-03-13T00:31:00Z",
      },
    ],
    available_commands: [],
    config_options: [],
    modes: null,
  };
}

function buildLeadProfile() {
  return {
    id: "lead-1",
    name: "Lead One",
    role: "lead",
    driver_id: "codex-cli",
    capabilities: [],
    actions_allowed: [],
  };
}

function buildDriver() {
  return {
    id: "codex-cli",
    name: "Codex CLI",
  };
}

function renderPage() {
  return render(
    <I18nextProvider i18n={i18n}>
      <MemoryRouter>
        <ChatPage />
      </MemoryRouter>
    </I18nextProvider>,
  );
}

describe("ChatPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    void i18n.changeLanguage("zh-CN");
    Object.defineProperty(HTMLElement.prototype, "scrollIntoView", {
      configurable: true,
      value: vi.fn(),
    });
    vi.stubGlobal("requestAnimationFrame", (callback: FrameRequestCallback) => window.setTimeout(() => callback(0), 0));
    vi.stubGlobal("cancelAnimationFrame", (handle: number) => window.clearTimeout(handle));
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("有活动 session 时支持结晶为 thread / work item 并跳转", async () => {
    const wsClient = createWsClientMock();
    const apiClient = {
      listChatSessions: vi.fn().mockResolvedValue([buildSession()]),
      getChatSession: vi.fn().mockResolvedValue(buildSessionDetail()),
      listEvents: vi.fn().mockResolvedValue([]),
      listProfiles: vi.fn().mockResolvedValue([buildLeadProfile()]),
      listDrivers: vi.fn().mockResolvedValue([buildDriver()]),
      crystallizeChatSessionThread: vi.fn().mockResolvedValue({
        thread: {
          id: 42,
          title: "整理后的 thread",
          status: "active",
          summary: "整理后的摘要",
          created_at: "2026-03-13T02:00:00Z",
          updated_at: "2026-03-13T02:00:00Z",
        },
        participants: [],
      }),
      closeChat: vi.fn(),
    };
    mockUseWorkbench.mockReturnValue({
      apiClient,
      wsClient,
      projects: [buildProject()],
      selectedProjectId: 9,
      setSelectedProjectId: vi.fn(),
    });

    renderPage();

    const openButton = await screen.findByRole("button", { name: "结晶" });
    fireEvent.click(openButton);

    const titleInput = await screen.findByLabelText("Thread 标题");
    const summaryInput = screen.getByLabelText("Thread 摘要");
    const workItemTitleInput = screen.getByLabelText("Work item 标题");

    fireEvent.change(titleInput, { target: { value: "整理后的 thread" } });
    fireEvent.change(summaryInput, { target: { value: "整理后的摘要" } });
    fireEvent.change(workItemTitleInput, { target: { value: "执行项标题" } });
    fireEvent.click(screen.getByRole("button", { name: "确认结晶" }));

    await waitFor(() => {
      expect(apiClient.crystallizeChatSessionThread).toHaveBeenCalledWith(
        "session-1",
        expect.objectContaining({
          thread_title: "整理后的 thread",
          thread_summary: "整理后的摘要",
          work_item_title: "执行项标题",
          project_id: 9,
          create_work_item: true,
        }),
      );
    });

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith("/threads/42");
    });
  });

  it("draft session 视图不显示结晶入口", async () => {
    const wsClient = createWsClientMock();
    const apiClient = {
      listChatSessions: vi.fn().mockResolvedValue([]),
      listProfiles: vi.fn().mockResolvedValue([buildLeadProfile()]),
      listDrivers: vi.fn().mockResolvedValue([buildDriver()]),
    };
    mockUseWorkbench.mockReturnValue({
      apiClient,
      wsClient,
      projects: [buildProject()],
      selectedProjectId: 9,
      setSelectedProjectId: vi.fn(),
    });

    renderPage();

    await screen.findByTestId("draft-session-setup");
    expect(screen.queryByRole("button", { name: "结晶" })).toBeNull();
  });
});
