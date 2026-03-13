// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
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

  it("支持编辑并保存 summary", async () => {
    const apiClient = {
      getThread: vi.fn().mockResolvedValue(buildThread("旧摘要")),
      listThreadMessages: vi.fn().mockResolvedValue([]),
      listThreadParticipants: vi.fn().mockResolvedValue([]),
      listWorkItemsByThread: vi.fn().mockResolvedValue([]),
      listThreadAgents: vi.fn().mockResolvedValue([]),
      updateThread: vi.fn().mockResolvedValue(buildThread("新摘要")),
    };
    mockUseWorkbench.mockReturnValue({ apiClient });

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
    const apiClient = {
      getThread: vi.fn().mockResolvedValue(buildThread("")),
      listThreadMessages: vi.fn().mockResolvedValue([]),
      listThreadParticipants: vi.fn().mockResolvedValue([]),
      listWorkItemsByThread: vi.fn().mockResolvedValue([]),
      listThreadAgents: vi.fn().mockResolvedValue([]),
      createWorkItemFromThread: vi.fn(),
    };
    mockUseWorkbench.mockReturnValue({ apiClient });

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
});
