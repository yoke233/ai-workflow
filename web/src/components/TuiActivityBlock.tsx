import { useState, useCallback } from "react";
import { TuiMarkdown } from "./TuiMarkdown";

interface TuiActivityBlockProps {
  activityType: string;
  detail: string;
  time: string;
  groupId?: string;
  onExpandGroup?: (groupId: string) => void;
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

  return (
    <div className="ml-6 my-1 border-l-2 border-slate-200 px-3 py-1 text-sm">
      <div className="flex items-center gap-2">
        {collapsible ? (
          <button
            type="button"
            onClick={handleToggle}
            className="flex items-center gap-1 text-xs text-slate-500 hover:text-slate-800"
            aria-label={expanded ? "收起" : "展开"}
          >
            <span className="select-none">{expanded ? "▼" : "▶"}</span>
            {!expanded && (
              <>
                <span className="font-mono">{getCollapsedPreview(detail)}</span>
                {extraLines > 0 && (
                  <span className="text-slate-400">… +{extraLines} lines</span>
                )}
              </>
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
                  <div key={`${child.id}-${idx}`} className="ml-2 border-l border-slate-200 px-2 py-1">
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
