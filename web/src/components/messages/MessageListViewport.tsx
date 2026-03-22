import type { ReactNode, RefObject, UIEventHandler } from "react";
import { cn } from "@/lib/utils";

export interface MessageListScrollProps {
  messageContainerRef: RefObject<HTMLDivElement>;
  messagesEndRef: RefObject<HTMLDivElement>;
  onMessageListScroll?: UIEventHandler<HTMLDivElement>;
}

interface MessageListViewportProps extends MessageListScrollProps {
  children: ReactNode;
  overlay?: ReactNode;
  className?: string;
  viewportClassName?: string;
  contentClassName?: string;
}

export function MessageListViewport({
  children,
  overlay,
  className,
  viewportClassName,
  contentClassName,
  messageContainerRef,
  messagesEndRef,
  onMessageListScroll,
}: MessageListViewportProps) {
  const content = contentClassName
    ? <div className={contentClassName}>{children}</div>
    : children;

  return (
    <div className={cn("relative flex-1 min-h-0", className)}>
      <div
        ref={messageContainerRef}
        className={cn("h-full overflow-y-auto", viewportClassName)}
        onScroll={onMessageListScroll}
      >
        {content}
        <div ref={messagesEndRef} />
      </div>
      {overlay}
    </div>
  );
}
