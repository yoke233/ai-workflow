// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import i18n from "../i18n";
import { ThreadDetailPage } from "./ThreadDetailPage";

const { mockUseWorkbench } = vi.hoisted(() => ({
  mockUseWorkbench: vi.fn(),
}));

vi.mock("@/contexts/WorkbenchContext", () => ({
  useWorkbench: mockUseWorkbench,
}));

function buildThread(summary = "已有摘要") {
  return {
    id: 1,
    title: "讨论线程",
    status: "active",
    summary,
    created_at: "2026-03-13T00:00:00Z",
    updated_at: "2026-03-13T00:00:00Z",
  };
}

function buildProfile(id: string, role = "worker") {
  return {
    id,
    name: id,
    driver_id: "codex-cli",
    role,
    capabilities: [],
    actions_allowed: [],
  };
}

function buildAgentSession(id: number, profileID: string, status = "active") {
  return {
    id,
    thread_id: 1,
    agent_profile_id: profileID,
    acp_session_id: `acp-${id}`,
    status,
    turn_count: 0,
    total_input_tokens: 0,
    total_output_tokens: 0,
    joined_at: "2026-03-13T00:00:00Z",
    last_active_at: "2026-03-13T00:00:00Z",
  };
}

function createWsClientMock() {
  const subscriptions = new Map<string, Array<(payload: unknown) => void>>();
  const statusHandlers: Array<(status: "idle" | "connecting" | "open" | "closed") => void> = [];

  return {
    send: vi.fn(),
    getStatus: vi.fn(() => "open"),
    subscribe: vi.fn((type: string, handler: (payload: unknown) => void) => {
      const handlers = subscriptions.get(type) ?? [];
      handlers.push(handler);
      subscriptions.set(type, handlers);
      return () => {
        const current = subscriptions.get(type) ?? [];
        subscriptions.set(type, current.filter((item) => item !== handler));
      };
    }),
    onStatusChange: vi.fn((handler: (status: "idle" | "connecting" | "open" | "closed") => void) => {
      statusHandlers.push(handler);
      return () => {
        const idx = statusHandlers.indexOf(handler);
        if (idx >= 0) {
          statusHandlers.splice(idx, 1);
        }
      };
    }),
    emit(type: string, payload: unknown) {
      for (const handler of subscriptions.get(type) ?? []) {
        handler(payload);
      }
    },
    emitStatus(status: "idle" | "connecting" | "open" | "closed") {
      for (const handler of statusHandlers) {
        handler(status);
      }
    },
  };
}

function renderPage() {
  return render(
    <I18nextProvider i18n={i18n}>
      <MemoryRouter initialEntries={["/threads/1"]}>
        <Routes>
          <Route path="/threads/:threadId" element={<ThreadDetailPage />} />
        </Routes>
      </MemoryRouter>
    </I18nextProvider>,
  );
}

describe("ThreadDetailPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    void i18n.changeLanguage("zh-CN");
  });

  afterEach(() => {
    cleanup();
  });

  it("支持编辑并保存 summary", async () => {
    const wsClient = createWsClientMock();
    const apiClient = {
      getThread: vi.fn().mockResolvedValue(buildThread("旧摘要")),
      listThreadMessages: vi.fn().mockResolvedValue([]),
      listThreadParticipants: vi.fn().mockResolvedValue([]),
      listWorkItemsByThread: vi.fn().mockResolvedValue([]),
      listThreadAgents: vi.fn().mockResolvedValue([]),
      listProfiles: vi.fn().mockResolvedValue([buildProfile("worker-a")]),
      updateThread: vi.fn().mockResolvedValue(buildThread("新摘要")),
    };
    mockUseWorkbench.mockReturnValue({ apiClient, wsClient });

    renderPage();

    const textarea = await screen.findByDisplayValue("旧摘要");
    fireEvent.change(textarea, { target: { value: "新摘要" } });
    fireEvent.click(screen.getByRole("button", { name: /保存|Save/ }));

    await waitFor(() => {
      expect(apiClient.updateThread).toHaveBeenCalledWith(1, { summary: "新摘要" });
    });
    expect(await screen.findByDisplayValue("新摘要")).toBeTruthy();
  });

  it("没有 summary 时阻止创建 work item，并持续展示依赖 summary 的提示", async () => {
    const wsClient = createWsClientMock();
    const apiClient = {
      getThread: vi.fn().mockResolvedValue(buildThread("")),
      listThreadMessages: vi.fn().mockResolvedValue([]),
      listThreadParticipants: vi.fn().mockResolvedValue([]),
      listWorkItemsByThread: vi.fn().mockResolvedValue([]),
      listThreadAgents: vi.fn().mockResolvedValue([]),
      listProfiles: vi.fn().mockResolvedValue([buildProfile("worker-a")]),
      createWorkItemFromThread: vi.fn(),
    };
    mockUseWorkbench.mockReturnValue({ apiClient, wsClient });

    renderPage();

    expect(
      await screen.findByText(
        "Work item creation depends on summary. Save a summary first to turn this discussion into execution.",
      ),
    ).toBeTruthy();

    const createButtons = await screen.findAllByRole("button", { name: /创建|Create/ });
    fireEvent.click(createButtons[0]);

    expect(screen.queryByText("Create Work Item from Summary")).toBeNull();
    expect(apiClient.createWorkItemFromThread).not.toHaveBeenCalled();
  });

  it("进入页面订阅 thread，并通过 thread.send + 实时事件更新消息列表", async () => {
    const wsClient = createWsClientMock();
    const apiClient = {
      getThread: vi.fn().mockResolvedValue(buildThread("已有摘要")),
      listThreadMessages: vi.fn().mockResolvedValue([]),
      listThreadParticipants: vi.fn().mockResolvedValue([]),
      listWorkItemsByThread: vi.fn().mockResolvedValue([]),
      listThreadAgents: vi.fn().mockResolvedValue([]),
      listProfiles: vi.fn().mockResolvedValue([buildProfile("worker-a")]),
    };
    mockUseWorkbench.mockReturnValue({ apiClient, wsClient });

    renderPage();

    await waitFor(() => {
      expect(wsClient.send).toHaveBeenCalledWith({
        type: "subscribe_thread",
        data: { thread_id: 1 },
      });
    });

    const input = screen.getByPlaceholderText("Type a message...");
    fireEvent.change(input, { target: { value: "实时消息" } });
    fireEvent.keyDown(input, { key: "Enter" });

    expect(wsClient.send).toHaveBeenCalledWith(
      expect.objectContaining({
        type: "thread.send",
        data: expect.objectContaining({
          thread_id: 1,
          message: "实时消息",
        }),
      }),
    );

    const sendCall = wsClient.send.mock.calls.find((call) => call[0]?.type === "thread.send");
    const requestId = sendCall?.[0]?.data?.request_id;
    wsClient.emit("thread.ack", {
      request_id: requestId,
      thread_id: 1,
      status: "accepted",
    });
    wsClient.emit("thread.message", {
      thread_id: 1,
      message: "实时消息",
      sender_id: "human",
      role: "human",
    });
    wsClient.emit("thread.agent_output", {
      thread_id: 1,
      content: "agent reply",
      profile_id: "worker-a",
    });

    expect(await screen.findByText("实时消息")).toBeTruthy();
    expect(await screen.findByText("agent reply")).toBeTruthy();
  });

  it("支持邀请和移除 thread agent", async () => {
    const wsClient = createWsClientMock();
    const apiClient = {
      getThread: vi.fn().mockResolvedValue(buildThread("已有摘要")),
      listThreadMessages: vi.fn().mockResolvedValue([]),
      listThreadParticipants: vi.fn().mockResolvedValue([]),
      listWorkItemsByThread: vi.fn().mockResolvedValue([]),
      listThreadAgents: vi.fn()
        .mockResolvedValueOnce([])
        .mockResolvedValueOnce([buildAgentSession(11, "worker-a")])
        .mockResolvedValueOnce([]),
      listProfiles: vi.fn().mockResolvedValue([buildProfile("worker-a"), buildProfile("worker-b")]),
      inviteThreadAgent: vi.fn().mockResolvedValue(buildAgentSession(11, "worker-a", "joining")),
      removeThreadAgent: vi.fn().mockResolvedValue(undefined),
    };
    mockUseWorkbench.mockReturnValue({ apiClient, wsClient });

    renderPage();

    await screen.findByRole("button", { name: "Invite" });

    fireEvent.click(screen.getByRole("button", { name: "Invite" }));

    await waitFor(() => {
      expect(apiClient.inviteThreadAgent).toHaveBeenCalledWith(1, { agent_profile_id: "worker-a" });
    });
    expect(await screen.findByText("worker-a")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "Remove worker-a" }));

    await waitFor(() => {
      expect(apiClient.removeThreadAgent).toHaveBeenCalledWith(1, 11);
    });
    await waitFor(() => {
      expect(screen.queryByText("worker-a")).toBeNull();
    });
  });
});
