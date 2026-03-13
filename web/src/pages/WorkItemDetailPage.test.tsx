// @vitest-environment jsdom
import { render, screen, waitFor } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import i18n from "../i18n";
import { WorkItemDetailPage } from "./WorkItemDetailPage";

const { mockUseWorkbench } = vi.hoisted(() => ({
  mockUseWorkbench: vi.fn(),
}));

vi.mock("@/contexts/WorkbenchContext", () => ({
  useWorkbench: mockUseWorkbench,
}));

function renderPage() {
  return render(
    <I18nextProvider i18n={i18n}>
      <MemoryRouter initialEntries={["/work-items/42"]}>
        <Routes>
          <Route path="/work-items/:workItemId" element={<WorkItemDetailPage />} />
        </Routes>
      </MemoryRouter>
    </I18nextProvider>,
  );
}

describe("WorkItemDetailPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    void i18n.changeLanguage("zh-CN");
  });

  it("显示来源 Thread、来源类型，并能跳回来源讨论", async () => {
    const apiClient = {
      getWorkItem: vi.fn().mockResolvedValue({
        id: 42,
        title: "从讨论收敛出的 WorkItem",
        body: "需要执行的内容",
        priority: "medium",
        status: "open",
        metadata: {
          source_thread_id: 7,
          source_type: "thread_summary",
          body_from_summary: true,
        },
        created_at: "2026-03-13T00:00:00Z",
        updated_at: "2026-03-13T00:00:00Z",
      }),
      listActions: vi.fn().mockResolvedValue([]),
      listThreadsByWorkItem: vi.fn().mockResolvedValue([
        {
          id: 1,
          thread_id: 7,
          work_item_id: 42,
          relation_type: "drives",
          is_primary: true,
          created_at: "2026-03-13T00:00:00Z",
        },
      ]),
      getThread: vi.fn().mockResolvedValue({
        id: 7,
        title: "来源讨论线程",
        status: "active",
        summary: "这是来源 summary",
        created_at: "2026-03-13T00:00:00Z",
        updated_at: "2026-03-13T00:00:00Z",
      }),
    };
    mockUseWorkbench.mockReturnValue({
      apiClient,
      projects: [],
    });

    renderPage();

    expect(await screen.findByText("来源 Thread")).toBeTruthy();
    const threadTitles = await screen.findAllByText("来源讨论线程");
    expect(threadTitles.length).toBeGreaterThan(0);
    expect(await screen.findByText("Summary 收敛")).toBeTruthy();

    await waitFor(() => {
      const links = screen.getAllByRole("link");
      expect(links.some((link) => link.getAttribute("href") === "/threads/7")).toBe(true);
    });
  });
});
