// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MessageListViewport } from "./MessageListViewport";

describe("MessageListViewport", () => {
  afterEach(() => {
    cleanup();
  });

  it("渲染内容并转发滚动事件", () => {
    const onScroll = vi.fn();

    render(
      <MessageListViewport
        messageContainerRef={{ current: null }}
        messagesEndRef={{ current: null }}
        onMessageListScroll={onScroll}
        overlay={<div>overlay marker</div>}
        viewportClassName="viewport"
      >
        <div>message content</div>
      </MessageListViewport>,
    );

    fireEvent.scroll(document.querySelector(".viewport")!);

    expect(screen.getByText("message content")).toBeTruthy();
    expect(screen.getByText("overlay marker")).toBeTruthy();
    expect(onScroll).toHaveBeenCalled();
  });
});
