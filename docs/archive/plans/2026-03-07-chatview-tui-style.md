# ChatView TUI 终端风格改造 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 将 ChatView 聊天界面从卡片式改为 TUI 终端风格，支持语法高亮、Markdown 渲染和快速导航。

**Architecture:** 从 ChatView 中提取消息渲染逻辑到独立组件（TuiCodeBlock、TuiMarkdown、TuiMessage、TuiActivityBlock、ScrollNavBar），ChatView 本身只负责数据流和布局编排。渲染层使用 react-syntax-highlighter 做代码高亮。

**Tech Stack:** React 18, TypeScript, Tailwind CSS, react-syntax-highlighter (Prism), Vitest

---

### Task 1: 安装 react-syntax-highlighter 依赖

**Files:**
- Modify: `web/package.json`

**Step 1: 安装依赖**

Run:
```bash
cd web && npm install react-syntax-highlighter && npm install -D @types/react-syntax-highlighter
```

**Step 2: 验证安装**

Run: `cd web && npm ls react-syntax-highlighter`
Expected: react-syntax-highlighter 版本号输出

**Step 3: 类型检查**

Run: `cd web && npx tsc --noEmit`
Expected: 无新增类型错误

**Step 4: Commit**

```bash
git add web/package.json web/package-lock.json
git commit -m "chore(web): add react-syntax-highlighter for code highlighting"
```

---

### Task 2: 创建 TuiCodeBlock 组件

**Files:**
- Create: `web/src/components/TuiCodeBlock.tsx`
- Create: `web/src/components/TuiCodeBlock.test.tsx`

**Step 1: 写测试**

```tsx
// web/src/components/TuiCodeBlock.test.tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { TuiCodeBlock } from "./TuiCodeBlock";

describe("TuiCodeBlock", () => {
  it("renders code content", () => {
    render(<TuiCodeBlock code={'const x = 1;'} language="javascript" />);
    expect(screen.getByText(/const/)).toBeTruthy();
  });

  it("shows language label when provided", () => {
    render(<TuiCodeBlock code="fmt.Println()" language="go" />);
    expect(screen.getByText("go")).toBeTruthy();
  });

  it("renders copy button", () => {
    render(<TuiCodeBlock code="hello" />);
    expect(screen.getByRole("button", { name: /复制/i })).toBeTruthy();
  });

  it("copies code to clipboard on click", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.assign(navigator, { clipboard: { writeText } });
    render(<TuiCodeBlock code="hello world" />);
    screen.getByRole("button", { name: /复制/i }).click();
    expect(writeText).toHaveBeenCalledWith("hello world");
  });
});
```

**Step 2: 运行测试确认失败**

Run: `cd web && npx vitest run src/components/TuiCodeBlock.test.tsx`
Expected: FAIL — module not found

**Step 3: 实现组件**

```tsx
// web/src/components/TuiCodeBlock.tsx
import { useState, useCallback } from "react";
import { Prism as SyntaxHighlighter } from "react-syntax-highlighter";
import { oneDark } from "react-syntax-highlighter/dist/esm/styles/prism";

interface TuiCodeBlockProps {
  code: string;
  language?: string;
}

export function TuiCodeBlock({ code, language }: TuiCodeBlockProps) {
  const [copied, setCopied] = useState(false);

  const handleCopy = useCallback(() => {
    void navigator.clipboard.writeText(code).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  }, [code]);

  return (
    <div className="group relative my-2 rounded-md bg-slate-900">
      <div className="flex items-center justify-between px-3 py-1 text-xs text-slate-400">
        {language ? <span>{language}</span> : <span />}
        <button
          type="button"
          aria-label="复制代码"
          className="rounded px-2 py-0.5 text-slate-400 hover:bg-slate-700 hover:text-slate-200"
          onClick={handleCopy}
        >
          {copied ? "已复制" : "复制"}
        </button>
      </div>
      <SyntaxHighlighter
        language={language || "text"}
        style={oneDark}
        customStyle={{ margin: 0, padding: "0.5rem 0.75rem", background: "transparent", fontSize: "0.75rem" }}
        wrapLongLines
      >
        {code}
      </SyntaxHighlighter>
    </div>
  );
}
```

**Step 4: 运行测试确认通过**

Run: `cd web && npx vitest run src/components/TuiCodeBlock.test.tsx`
Expected: PASS

**Step 5: Commit**

```bash
git add web/src/components/TuiCodeBlock.tsx web/src/components/TuiCodeBlock.test.tsx
git commit -m "feat(web): add TuiCodeBlock with syntax highlighting and copy button"
```

---

### Task 3: 创建 TuiMarkdown 组件

替换 ChatView 中的 `renderBasicMarkdown` / `parseInlineMarkdown`，在代码块部分使用 TuiCodeBlock。

**Files:**
- Create: `web/src/components/TuiMarkdown.tsx`
- Create: `web/src/components/TuiMarkdown.test.tsx`

**Step 1: 写测试**

```tsx
// web/src/components/TuiMarkdown.test.tsx
import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { TuiMarkdown } from "./TuiMarkdown";

describe("TuiMarkdown", () => {
  it("renders plain text as paragraph", () => {
    render(<TuiMarkdown content="hello world" />);
    expect(screen.getByText("hello world")).toBeTruthy();
  });

  it("renders inline code", () => {
    render(<TuiMarkdown content="use `npm install` to install" />);
    expect(screen.getByText("npm install")).toBeTruthy();
    expect(screen.getByText("npm install").tagName).toBe("CODE");
  });

  it("renders bold text", () => {
    render(<TuiMarkdown content="this is **bold** text" />);
    expect(screen.getByText("bold").tagName).toBe("STRONG");
  });

  it("renders code blocks with syntax highlighter", () => {
    const content = "```javascript\nconst x = 1;\n```";
    render(<TuiMarkdown content={content} />);
    expect(screen.getByText("javascript")).toBeTruthy();
  });

  it("renders headings", () => {
    render(<TuiMarkdown content="## Hello" />);
    const heading = screen.getByText("Hello");
    expect(heading.tagName).toBe("H2");
  });

  it("renders unordered lists", () => {
    render(<TuiMarkdown content="- item 1\n- item 2" />);
    expect(screen.getByText("item 1")).toBeTruthy();
    expect(screen.getByText("item 2")).toBeTruthy();
  });

  it("renders links", () => {
    render(<TuiMarkdown content="[click](https://example.com)" />);
    const link = screen.getByText("click");
    expect(link.tagName).toBe("A");
    expect(link.getAttribute("href")).toBe("https://example.com");
  });
});
```

**Step 2: 运行测试确认失败**

Run: `cd web && npx vitest run src/components/TuiMarkdown.test.tsx`
Expected: FAIL

**Step 3: 实现组件**

将 ChatView.tsx 中 `parseInlineMarkdown`（L753-L816）和 `renderBasicMarkdown`（L818-L936）的逻辑迁移到此组件，但代码块部分改用 `<TuiCodeBlock>`。

```tsx
// web/src/components/TuiMarkdown.tsx
import type { JSX } from "react";
import { TuiCodeBlock } from "./TuiCodeBlock";

interface TuiMarkdownProps {
  content: string;
  keyPrefix?: string;
}

const parseInlineMarkdown = (text: string, keyPrefix: string): Array<string | JSX.Element> => {
  const nodes: Array<string | JSX.Element> = [];
  const pattern =
    /`([^`]+)`|\[([^\]]+)\]\((https?:\/\/[^\s)]+)\)|\*\*([^*]+)\*\*|(\*[^*]+\*)/g;
  let lastIndex = 0;
  let matchIndex = 0;
  let match = pattern.exec(text);
  while (match) {
    if (match.index > lastIndex) {
      nodes.push(text.slice(lastIndex, match.index));
    }
    if (match[1]) {
      nodes.push(
        <code key={`${keyPrefix}-ic-${matchIndex}`} className="rounded bg-slate-100 px-1 py-0.5 font-mono text-[0.9em] text-slate-900">
          {match[1]}
        </code>,
      );
    } else if (match[2] && match[3]) {
      nodes.push(
        <a key={`${keyPrefix}-a-${matchIndex}`} href={match[3]} target="_blank" rel="noreferrer" className="text-emerald-700 underline decoration-emerald-400/50">
          {match[2]}
        </a>,
      );
    } else if (match[4]) {
      nodes.push(<strong key={`${keyPrefix}-b-${matchIndex}`} className="font-semibold">{match[4]}</strong>);
    } else if (match[5]) {
      nodes.push(<em key={`${keyPrefix}-em-${matchIndex}`} className="italic">{match[5].slice(1, -1)}</em>);
    }
    lastIndex = match.index + match[0].length;
    matchIndex += 1;
    match = pattern.exec(text);
  }
  if (lastIndex < text.length) nodes.push(text.slice(lastIndex));
  if (nodes.length === 0) nodes.push(text);
  return nodes;
};

export function TuiMarkdown({ content, keyPrefix = "md" }: TuiMarkdownProps) {
  const lines = content.replace(/\r\n/g, "\n").split("\n");
  const elements: JSX.Element[] = [];
  let i = 0;

  while (i < lines.length) {
    const line = (lines[i] ?? "").trim();

    if (!line) { i++; continue; }

    // Code block
    if (line.startsWith("```")) {
      const lang = line.slice(3).trim() || undefined;
      const codeLines: string[] = [];
      i++;
      while (i < lines.length && !(lines[i] ?? "").trim().startsWith("```")) {
        codeLines.push(lines[i] ?? "");
        i++;
      }
      i++; // skip closing ```
      elements.push(<TuiCodeBlock key={`${keyPrefix}-cb-${i}`} code={codeLines.join("\n")} language={lang} />);
      continue;
    }

    // Heading
    const headingMatch = line.match(/^(#{1,6})\s+(.+)$/);
    if (headingMatch) {
      const HeadingTag = `h${headingMatch[1].length}` as keyof JSX.IntrinsicElements;
      elements.push(
        <HeadingTag key={`${keyPrefix}-h-${i}`} className="font-semibold leading-snug">
          {parseInlineMarkdown(headingMatch[2], `${keyPrefix}-h-${i}`)}
        </HeadingTag>,
      );
      i++;
      continue;
    }

    // Unordered list
    if (/^[-*]\s+/.test(line)) {
      const items: string[] = [];
      while (i < lines.length) {
        const itemMatch = (lines[i] ?? "").trim().match(/^[-*]\s+(.+)$/);
        if (!itemMatch) break;
        items.push(itemMatch[1]);
        i++;
      }
      elements.push(
        <ul key={`${keyPrefix}-ul-${i}`} className="list-disc space-y-1 pl-5">
          {items.map((item, idx) => (
            <li key={`${keyPrefix}-li-${i}-${idx}`}>{parseInlineMarkdown(item, `${keyPrefix}-li-${i}-${idx}`)}</li>
          ))}
        </ul>,
      );
      continue;
    }

    // Paragraph (collect consecutive non-special lines)
    const paraLines = [line];
    i++;
    while (i < lines.length) {
      const next = (lines[i] ?? "").trim();
      if (!next || /^#{1,6}\s+/.test(next) || /^[-*]\s+/.test(next) || next.startsWith("```")) break;
      paraLines.push(next);
      i++;
    }
    elements.push(
      <p key={`${keyPrefix}-p-${i}`} className="whitespace-pre-wrap">
        {parseInlineMarkdown(paraLines.join(" "), `${keyPrefix}-p-${i}`)}
      </p>,
    );
  }

  if (elements.length === 0) {
    elements.push(<p key={`${keyPrefix}-empty`} className="whitespace-pre-wrap">{content}</p>);
  }

  return <>{elements}</>;
}
```

**Step 4: 运行测试确认通过**

Run: `cd web && npx vitest run src/components/TuiMarkdown.test.tsx`
Expected: PASS

**Step 5: Commit**

```bash
git add web/src/components/TuiMarkdown.tsx web/src/components/TuiMarkdown.test.tsx
git commit -m "feat(web): add TuiMarkdown component with code block highlighting"
```

---

### Task 4: 创建 TuiMessage 组件

**Files:**
- Create: `web/src/components/TuiMessage.tsx`
- Create: `web/src/components/TuiMessage.test.tsx`

**Step 1: 写测试**

```tsx
// web/src/components/TuiMessage.test.tsx
import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { TuiMessage } from "./TuiMessage";

describe("TuiMessage", () => {
  it("renders user message with icon prefix and background", () => {
    const { container } = render(
      <TuiMessage role="user" content="hello" time="2026-01-01T00:00:00Z" />,
    );
    expect(screen.getByText("hello")).toBeTruthy();
    // user messages have bg-slate-50
    expect(container.querySelector(".bg-slate-50")).toBeTruthy();
  });

  it("renders assistant message with bullet prefix", () => {
    render(
      <TuiMessage role="assistant" content="world" time="2026-01-01T00:00:00Z" />,
    );
    expect(screen.getByText("world")).toBeTruthy();
    expect(screen.getByText("•")).toBeTruthy();
  });

  it("renders markdown content in assistant message", () => {
    render(
      <TuiMessage role="assistant" content="use `npm`" time="2026-01-01T00:00:00Z" />,
    );
    expect(screen.getByText("npm").tagName).toBe("CODE");
  });

  it("shows formatted time", () => {
    render(
      <TuiMessage role="user" content="hi" time="2026-03-07T15:30:00Z" />,
    );
    // 时间应该渲染出来
    expect(screen.getByText(/15/)).toBeTruthy();
  });
});
```

**Step 2: 运行测试确认失败**

Run: `cd web && npx vitest run src/components/TuiMessage.test.tsx`
Expected: FAIL

**Step 3: 实现组件**

```tsx
// web/src/components/TuiMessage.tsx
import { forwardRef } from "react";
import { TuiMarkdown } from "./TuiMarkdown";

interface TuiMessageProps {
  role: "user" | "assistant";
  content: string;
  time: string;
  id?: string;
}

const formatTime = (time: string): string => {
  const date = new Date(time);
  if (Number.isNaN(date.getTime())) return time;
  return date.toLocaleString("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
};

export const TuiMessage = forwardRef<HTMLDivElement, TuiMessageProps>(
  function TuiMessage({ role, content, time, id }, ref) {
    if (role === "user") {
      return (
        <div ref={ref} id={id} className="border-b border-slate-200 bg-slate-50 px-4 py-3">
          <div className="flex items-start gap-2">
            <span className="mt-0.5 select-none text-base" aria-hidden>👤</span>
            <div className="min-w-0 flex-1">
              <span className="text-xs text-slate-400">{formatTime(time)}</span>
              <p className="mt-1 text-sm font-medium whitespace-pre-wrap">{content}</p>
            </div>
          </div>
        </div>
      );
    }

    return (
      <div ref={ref} id={id} className="border-b border-slate-200 px-4 py-3">
        <div className="flex items-start gap-2">
          <span className="mt-0.5 select-none text-sm font-bold text-slate-400" aria-hidden>•</span>
          <div className="min-w-0 flex-1 text-sm">
            <span className="text-xs text-slate-400">{formatTime(time)}</span>
            <div className="mt-1 space-y-2">
              <TuiMarkdown content={content} />
            </div>
          </div>
        </div>
      </div>
    );
  },
);
```

**Step 4: 运行测试确认通过**

Run: `cd web && npx vitest run src/components/TuiMessage.test.tsx`
Expected: PASS

**Step 5: Commit**

```bash
git add web/src/components/TuiMessage.tsx web/src/components/TuiMessage.test.tsx
git commit -m "feat(web): add TuiMessage component for terminal-style messages"
```

---

### Task 5: 创建 TuiActivityBlock 组件

**Files:**
- Create: `web/src/components/TuiActivityBlock.tsx`
- Create: `web/src/components/TuiActivityBlock.test.tsx`

**Step 1: 写测试**

```tsx
// web/src/components/TuiActivityBlock.test.tsx
import { describe, it, expect } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { TuiActivityBlock } from "./TuiActivityBlock";

describe("TuiActivityBlock", () => {
  it("renders collapsed by default for tool_call", () => {
    render(
      <TuiActivityBlock
        activityType="tool_call"
        detail={"Ran rg -n 'hello'\nline 1\nline 2\nline 3\nline 4"}
        time="2026-01-01T00:00:00Z"
      />,
    );
    expect(screen.getByText(/Ran rg/)).toBeTruthy();
    expect(screen.getByText(/\+\d+ lines/)).toBeTruthy();
  });

  it("expands on click", () => {
    render(
      <TuiActivityBlock
        activityType="tool_call"
        detail={"Ran rg -n 'hello'\nline 1\nline 2\nline 3"}
        time="2026-01-01T00:00:00Z"
      />,
    );
    fireEvent.click(screen.getByRole("button"));
    expect(screen.getByText("line 3")).toBeTruthy();
  });

  it("renders agent_thought with thinking style", () => {
    render(
      <TuiActivityBlock
        activityType="agent_thought"
        detail="I need to check the files"
        time="2026-01-01T00:00:00Z"
      />,
    );
    expect(screen.getByText(/I need to check/)).toBeTruthy();
  });

  it("renders plan entries", () => {
    render(
      <TuiActivityBlock
        activityType="plan"
        detail="- Step 1\n- Step 2"
        time="2026-01-01T00:00:00Z"
      />,
    );
    expect(screen.getByText(/Step 1/)).toBeTruthy();
  });
});
```

**Step 2: 运行测试确认失败**

Run: `cd web && npx vitest run src/components/TuiActivityBlock.test.tsx`
Expected: FAIL

**Step 3: 实现组件**

```tsx
// web/src/components/TuiActivityBlock.tsx
import { useState, useCallback } from "react";
import { TuiMarkdown } from "./TuiMarkdown";

interface TuiActivityBlockProps {
  activityType: string;
  detail: string;
  time: string;
  groupId?: string;
  /** 当 tool_call_group 展开时的加载回调 */
  onExpandGroup?: (groupId: string) => void;
  /** tool_call_group 的子项 */
  groupChildren?: Array<{ id: string; type: string; detail: string; time: string }>;
  groupLoading?: boolean;
  groupError?: string;
}

const formatTime = (time: string): string => {
  const date = new Date(time);
  if (Number.isNaN(date.getTime())) return time;
  return date.toLocaleString("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
};

const getCollapsedPreview = (content: string): string => {
  const firstLine = content.split("\n").map((l) => l.trim()).find((l) => l.length > 0) ?? "";
  return firstLine.length > 160 ? `${firstLine.slice(0, 160)}...` : firstLine;
};

const countExtraLines = (content: string): number => {
  const lines = content.split("\n").filter((l) => l.trim().length > 0);
  return Math.max(0, lines.length - 1);
};

const isCollapsibleType = (type: string): boolean =>
  type === "tool_call" || type === "tool_call_group";

export function TuiActivityBlock({
  activityType,
  detail,
  time,
  groupId,
  onExpandGroup,
  groupChildren,
  groupLoading,
  groupError,
}: TuiActivityBlockProps) {
  const collapsible = isCollapsibleType(activityType);
  const [expanded, setExpanded] = useState(!collapsible);
  const extraLines = countExtraLines(detail);

  const handleToggle = useCallback(() => {
    const willExpand = !expanded;
    setExpanded(willExpand);
    if (willExpand && activityType === "tool_call_group" && groupId && onExpandGroup) {
      onExpandGroup(groupId);
    }
  }, [expanded, activityType, groupId, onExpandGroup]);

  const isThought = activityType === "agent_thought";
  const borderColor = isThought ? "border-violet-300" : "border-slate-300";
  const bgColor = isThought ? "bg-violet-50" : "bg-slate-100";

  return (
    <div className={`ml-6 my-1 rounded border-l-2 ${borderColor} ${bgColor} px-3 py-2 text-sm`}>
      <div className="flex items-center gap-2">
        {collapsible ? (
          <button
            type="button"
            onClick={handleToggle}
            className="flex items-center gap-1 text-xs text-slate-500 hover:text-slate-800"
            aria-label={expanded ? "收起" : "展开"}
          >
            <span className="select-none">{expanded ? "▼" : "▶"}</span>
            <span className="font-mono">{getCollapsedPreview(detail)}</span>
            {!expanded && extraLines > 0 && (
              <span className="text-slate-400">… +{extraLines} lines</span>
            )}
          </button>
        ) : (
          <span className="text-xs text-slate-500">{isThought ? "Thinking" : activityType}</span>
        )}
        <span className="ml-auto text-[10px] text-slate-400">{formatTime(time)}</span>
      </div>

      {(expanded || !collapsible) && (
        <div className="mt-2">
          {activityType === "tool_call_group" && groupChildren ? (
            <div className="space-y-2">
              <div className="text-xs text-slate-600">{detail}</div>
              {groupLoading ? (
                <p className="text-xs text-slate-500">加载中...</p>
              ) : groupError ? (
                <p className="text-xs text-rose-600">加载失败：{groupError}</p>
              ) : groupChildren.length > 0 ? (
                groupChildren.map((child, idx) => (
                  <div key={`${child.id}-${idx}`} className="ml-2 rounded border-l border-slate-200 bg-white px-2 py-1">
                    <p className="text-[10px] text-slate-500">{child.type} · {formatTime(child.time)}</p>
                    <div className="mt-1"><TuiMarkdown content={child.detail} /></div>
                  </div>
                ))
              ) : (
                <p className="text-xs text-slate-500">暂无详情</p>
              )}
            </div>
          ) : (
            <TuiMarkdown content={detail} />
          )}
        </div>
      )}
    </div>
  );
}
```

**Step 4: 运行测试确认通过**

Run: `cd web && npx vitest run src/components/TuiActivityBlock.test.tsx`
Expected: PASS

**Step 5: Commit**

```bash
git add web/src/components/TuiActivityBlock.tsx web/src/components/TuiActivityBlock.test.tsx
git commit -m "feat(web): add TuiActivityBlock with collapsible terminal-style events"
```

---

### Task 6: 创建 ScrollNavBar 组件

**Files:**
- Create: `web/src/components/ScrollNavBar.tsx`
- Create: `web/src/components/ScrollNavBar.test.tsx`

**Step 1: 写测试**

```tsx
// web/src/components/ScrollNavBar.test.tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { ScrollNavBar } from "./ScrollNavBar";

describe("ScrollNavBar", () => {
  const markers = [
    { id: "msg-1", label: "你好世界", position: 0.1 },
    { id: "msg-2", label: "第二条消息内容比较长一些截断", position: 0.5 },
    { id: "msg-3", label: "最后一条", position: 0.9 },
  ];

  it("renders correct number of markers", () => {
    render(<ScrollNavBar markers={markers} onMarkerClick={() => {}} />);
    const buttons = screen.getAllByRole("button");
    expect(buttons.length).toBe(3);
  });

  it("calls onMarkerClick with correct id", () => {
    const onClick = vi.fn();
    render(<ScrollNavBar markers={markers} onMarkerClick={onClick} />);
    fireEvent.click(screen.getAllByRole("button")[1]);
    expect(onClick).toHaveBeenCalledWith("msg-2");
  });

  it("shows tooltip on hover", async () => {
    render(<ScrollNavBar markers={markers} onMarkerClick={() => {}} />);
    fireEvent.mouseEnter(screen.getAllByRole("button")[0]);
    expect(screen.getByText("你好世界")).toBeTruthy();
  });
});
```

**Step 2: 运行测试确认失败**

Run: `cd web && npx vitest run src/components/ScrollNavBar.test.tsx`
Expected: FAIL

**Step 3: 实现组件**

```tsx
// web/src/components/ScrollNavBar.tsx
import { useState, useCallback } from "react";

export interface ScrollMarker {
  id: string;
  label: string;
  /** 0-1，表示在总滚动高度中的相对位置 */
  position: number;
}

interface ScrollNavBarProps {
  markers: ScrollMarker[];
  onMarkerClick: (id: string) => void;
}

export function ScrollNavBar({ markers, onMarkerClick }: ScrollNavBarProps) {
  const [hoveredId, setHoveredId] = useState<string | null>(null);

  const handleClick = useCallback(
    (id: string) => {
      onMarkerClick(id);
    },
    [onMarkerClick],
  );

  return (
    <div className="relative h-full w-3 shrink-0 bg-slate-100">
      {markers.map((marker) => (
        <div
          key={marker.id}
          className="absolute left-1/2 -translate-x-1/2"
          style={{ top: `${marker.position * 100}%` }}
        >
          <button
            type="button"
            className="h-2 w-2 rounded-full bg-blue-400 transition-transform hover:scale-150 hover:bg-blue-500"
            onClick={() => handleClick(marker.id)}
            onMouseEnter={() => setHoveredId(marker.id)}
            onMouseLeave={() => setHoveredId(null)}
            aria-label={marker.label}
          />
          {hoveredId === marker.id && (
            <div className="absolute right-4 top-1/2 z-20 -translate-y-1/2 whitespace-nowrap rounded bg-slate-800 px-2 py-1 text-xs text-white shadow-lg">
              {marker.label}
            </div>
          )}
        </div>
      ))}
    </div>
  );
}
```

**Step 4: 运行测试确认通过**

Run: `cd web && npx vitest run src/components/ScrollNavBar.test.tsx`
Expected: PASS

**Step 5: Commit**

```bash
git add web/src/components/ScrollNavBar.tsx web/src/components/ScrollNavBar.test.tsx
git commit -m "feat(web): add ScrollNavBar for quick-jump user message navigation"
```

---

### Task 7: 重构 ChatView — 替换消息渲染区域

这是最核心的改动。将 ChatView.tsx 中的消息时间线渲染逻辑替换为新的 TUI 组件。

**Files:**
- Modify: `web/src/views/ChatView.tsx`

**Step 1: 在 ChatView.tsx 顶部添加新组件 import**

在文件顶部（约 L27-L31 之间）添加：

```tsx
import { TuiMessage } from "../components/TuiMessage";
import { TuiActivityBlock } from "../components/TuiActivityBlock";
import { TuiMarkdown } from "../components/TuiMarkdown";
import { ScrollNavBar } from "../components/ScrollNavBar";
import type { ScrollMarker } from "../components/ScrollNavBar";
```

**Step 2: 删除旧的渲染函数**

删除以下函数（不再需要，已迁移到 TuiMarkdown）：
- `parseInlineMarkdown`（L753-L816）
- `renderBasicMarkdown`（L818-L936）
- `getCollapsedPreview`（L938-L949）
- `roleLabel`（L115-L118）
- `roleStyle`（L120-L123）

注意：`formatTime` 保留，因为右侧 sidebar 仍用到。

**Step 3: 在组件函数体中添加 ScrollNavBar 所需的 markers 计算**

在 `timelineItems` useMemo 附近（约 L1170 后面）添加：

```tsx
const userMessageMarkers = useMemo<ScrollMarker[]>(() => {
  const userItems = timelineItems.filter(
    (item) => item.kind === "message" && item.role === "user",
  );
  const total = timelineItems.length || 1;
  return userItems.map((item) => {
    const idx = timelineItems.indexOf(item);
    return {
      id: item.id,
      label: item.kind === "message" ? (item.content.length > 30 ? `${item.content.slice(0, 30)}...` : item.content) : "",
      position: idx / total,
    };
  });
}, [timelineItems]);

const handleNavMarkerClick = useCallback((markerId: string) => {
  const el = document.getElementById(markerId);
  if (el) {
    el.scrollIntoView({ behavior: "smooth", block: "start" });
  }
}, []);
```

**Step 4: 替换消息时间线容器区域**

替换 L2252 周围的 `<div className="mt-4 h-[30rem] ...">`，将原本的 `<article>` 渲染替换为新组件。

原代码（L2252-L2448 大致范围）替换为：

```tsx
<div className="mt-4 flex h-[30rem] rounded-lg border border-slate-200 bg-white">
  {/* TUI 消息流 */}
  <div
    ref={timelineScrollRef}
    className="flex-1 overflow-y-auto font-mono text-sm"
    onScroll={handleTimelineScroll}
  >
    {historyLoadingMore ? (
      <p className="bg-slate-50 px-4 py-2 text-center text-xs text-slate-500">
        加载更早记录中...
      </p>
    ) : historyCursor ? (
      <p className="px-4 py-1 text-center text-xs text-slate-400">
        向上滚动可加载更早记录
      </p>
    ) : null}

    {timelineItems.map((item) => {
      if (item.kind === "message") {
        return (
          <TuiMessage
            key={item.id}
            id={item.id}
            role={item.role}
            content={item.content}
            time={item.time}
          />
        );
      }

      const groupState = item.groupId
        ? toolCallGroupStates[item.groupId]
        : undefined;

      return (
        <TuiActivityBlock
          key={item.id}
          activityType={item.activityType}
          detail={item.detail}
          time={item.time}
          groupId={item.groupId}
          onExpandGroup={(gid) => {
            if (sessionId) {
              void loadToolCallGroup(projectId, sessionId, gid);
            }
          }}
          groupChildren={groupState?.items.map((child) => ({
            id: child.id,
            type: child.type,
            detail: child.detail,
            time: child.time,
          }))}
          groupLoading={groupState?.loading}
          groupError={groupState?.error}
        />
      );
    })}

    {isStreaming ? (
      <div className="border-b border-slate-200 px-4 py-3">
        <div className="flex items-start gap-2">
          <span className="mt-0.5 select-none text-sm font-bold text-slate-400" aria-hidden>•</span>
          <div className="min-w-0 flex-1 text-sm">
            <span className="text-xs text-slate-400">输入中...</span>
            <div className="mt-1 space-y-2">
              <TuiMarkdown content={streamingText.length > 0 ? streamingText : "..."} />
            </div>
          </div>
        </div>
      </div>
    ) : null}

    <div ref={messagesEndRef} />
  </div>

  {/* 右侧导航标记条 */}
  {hasMessages && (
    <ScrollNavBar markers={userMessageMarkers} onMarkerClick={handleNavMarkerClick} />
  )}
</div>
```

**Step 5: 更新 "暂无消息" 的 fallback**

确保 `{!hasMessages && ...}` 仍保持：
```tsx
{!hasMessages && (
  <div className="mt-4 flex h-[30rem] items-center justify-center rounded-lg border border-slate-200 bg-white">
    <p className="text-sm text-slate-500">当前会话暂无消息。</p>
  </div>
)}
```

**Step 6: 清理 ChatView 中不再使用的变量**

删除 `expandedActivityCards` state 和 `setExpandedActivityCards`（折叠逻辑已移入 TuiActivityBlock）。
删除 `ExpandCollapseIcon` 组件（如果存在且不再被其他地方使用）。

**Step 7: 类型检查**

Run: `cd web && npx tsc --noEmit`
Expected: 无错误

**Step 8: 运行全部前端测试**

Run: `cd web && npx vitest run`
Expected: 所有测试通过

**Step 9: Commit**

```bash
git add web/src/views/ChatView.tsx
git commit -m "feat(web): refactor ChatView to TUI terminal-style layout"
```

---

### Task 8: 样式微调与输入框适配

**Files:**
- Modify: `web/src/views/ChatView.tsx`

**Step 1: 调整输入框样式匹配终端风格**

找到 textarea（约 L2457-L2468），修改 className：

```tsx
<textarea
  id="chat-message"
  ref={messageInputRef}
  rows={3}
  className="min-h-[5rem] w-full resize-y rounded-md border border-slate-300 bg-slate-50 px-3 py-2 font-mono text-sm focus:border-slate-500 focus:outline-none"
  placeholder="请输入要拆分为 issue 的需求..."
  value={draft}
  onKeyDown={handleDraftKeyDown}
  onChange={(event) => {
    setDraft(event.target.value);
  }}
/>
```

**Step 2: 调整发送按钮样式**

将发送按钮的 `bg-slate-900` 改为更紧凑的风格：

```tsx
className="w-36 rounded-md border border-slate-300 bg-white px-4 py-2 text-center text-sm font-medium text-slate-700 hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50"
```

**Step 3: 视觉验证**

Run: `cd web && npm run dev -- --strictPort`
手动在浏览器中检查 ChatView 页面，确认：
- 消息使用 TUI 平铺风格
- 用户消息有 👤 图标和灰色背景
- 助手消息有 • bullet
- 代码块有语法高亮
- 活动事件默认折叠
- 右侧导航条有标记点
- 输入框风格协调

**Step 4: Commit**

```bash
git add web/src/views/ChatView.tsx
git commit -m "style(web): adjust input area to match TUI terminal style"
```

---

### Task 9: 清理旧代码和最终验证

**Files:**
- Modify: `web/src/views/ChatView.tsx`

**Step 1: 确认删除所有旧渲染代码残留**

搜索 ChatView.tsx 确保没有残留的 `renderBasicMarkdown`、`parseInlineMarkdown` 引用：

Run: `cd web && grep -n "renderBasicMarkdown\|parseInlineMarkdown\|roleStyle\|roleLabel" src/views/ChatView.tsx`
Expected: 无输出（A2AChatView 不受影响）

**Step 2: 运行类型检查**

Run: `cd web && npx tsc --noEmit`
Expected: 无错误

**Step 3: 运行所有测试**

Run: `cd web && npx vitest run`
Expected: 全部通过

**Step 4: 构建验证**

Run: `cd web && npm run build`
Expected: 构建成功

**Step 5: Commit**

```bash
git add -A web/src/
git commit -m "refactor(web): clean up legacy card-style rendering code from ChatView"
```
