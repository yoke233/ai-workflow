// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import i18n from "../i18n";
import { LLMConfigPage } from "./LLMConfigPage";

const { mockUseWorkbench } = vi.hoisted(() => ({
  mockUseWorkbench: vi.fn(),
}));

vi.mock("@/contexts/WorkbenchContext", () => ({
  useWorkbench: mockUseWorkbench,
}));

function renderPage() {
  return render(
    <I18nextProvider i18n={i18n}>
      <MemoryRouter>
        <LLMConfigPage />
      </MemoryRouter>
    </I18nextProvider>,
  );
}

describe("LLMConfigPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    void i18n.changeLanguage("zh-CN");
  });

  afterEach(() => {
    cleanup();
  });

  it("支持新增、移除、切换默认配置并保存", async () => {
    const apiClient = {
      getLLMConfig: vi.fn().mockResolvedValue({
        default_config_id: "openai-prod",
        configs: [
          {
            id: "openai-prod",
            type: "openai_response",
            model: "gpt-4.1-mini",
          },
        ],
      }),
      updateLLMConfig: vi.fn().mockResolvedValue({
        default_config_id: "llm-config-2",
        configs: [
          {
            id: "llm-config-2",
            type: "anthropic",
            model: "claude-3-7-sonnet-latest",
          },
        ],
      }),
    };

    mockUseWorkbench.mockReturnValue({ apiClient });

    renderPage();

    expect(await screen.findByText("LLM API 管理")).toBeTruthy();
    expect(screen.getByDisplayValue("gpt-4.1-mini")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "新增配置" }));

    const newConfigCard = (await screen.findByDisplayValue("llm-config-2")).closest(".rounded-2xl");
    expect(newConfigCard).toBeTruthy();
    fireEvent.change(within(newConfigCard as HTMLElement).getByLabelText("类型"), {
      target: { value: "anthropic" },
    });
    fireEvent.change(within(newConfigCard as HTMLElement).getByLabelText("配置 ID"), {
      target: { value: "claude-backup" },
    });
    fireEvent.change(within(newConfigCard as HTMLElement).getByLabelText("MODEL"), {
      target: { value: "claude-3-7-sonnet-latest" },
    });
    fireEvent.change(within(newConfigCard as HTMLElement).getByLabelText("Temperature"), {
      target: { value: "0.3" },
    });
    fireEvent.change(within(newConfigCard as HTMLElement).getByLabelText("最大输出 Tokens"), {
      target: { value: "4096" },
    });
    fireEvent.change(within(newConfigCard as HTMLElement).getByLabelText("思考强度"), {
      target: { value: "high" },
    });
    fireEvent.change(within(newConfigCard as HTMLElement).getByLabelText("Thinking Budget"), {
      target: { value: "2048" },
    });
    fireEvent.change(screen.getAllByRole("combobox")[0], { target: { value: "claude-backup" } });

    const originalCard = screen.getByDisplayValue("openai-prod").closest(".rounded-2xl");
    expect(originalCard).toBeTruthy();
    fireEvent.click((originalCard as HTMLElement).querySelector("button") as HTMLButtonElement);

    fireEvent.click(screen.getByRole("button", { name: "保存配置" }));

    await waitFor(() => {
      expect(apiClient.updateLLMConfig).toHaveBeenCalledWith({
        default_config_id: "claude-backup",
        configs: [
          {
            id: "claude-backup",
            type: "anthropic",
            model: "claude-3-7-sonnet-latest",
            temperature: 0.3,
            max_output_tokens: 4096,
            reasoning_effort: "high",
            thinking_budget_tokens: 2048,
          },
        ],
      });
    });
  });

  it("加载失败时展示错误信息", async () => {
    const apiClient = {
      getLLMConfig: vi.fn().mockRejectedValue(new Error("config unavailable")),
      updateLLMConfig: vi.fn(),
    };

    mockUseWorkbench.mockReturnValue({ apiClient });

    renderPage();

    expect(await screen.findByText("config unavailable")).toBeTruthy();
  });
});
