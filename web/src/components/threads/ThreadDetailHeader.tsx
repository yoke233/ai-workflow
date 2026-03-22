import { useTranslation } from "react-i18next";
import { ArrowLeft, Bot, MessageSquare, Users } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import type { Thread } from "@/types/apiV2";

type AgentRoutingMode = "mention_only" | "broadcast" | "auto";
type MeetingMode = "direct" | "concurrent" | "group_chat";

interface ThreadDetailHeaderProps {
  thread: Thread;
  participantsCount: number;
  agentSessionsCount: number;
  agentRoutingMode: AgentRoutingMode;
  meetingMode: MeetingMode;
  savingRoutingMode: boolean;
  savingMeetingMode: boolean;
  formatRelativeTime: (value: string) => string;
  onBack: () => void;
  onSetRoutingMode: (mode: AgentRoutingMode) => void;
  onSetMeetingMode: (mode: MeetingMode) => void;
}

export function ThreadDetailHeader({
  thread,
  participantsCount,
  agentSessionsCount,
  agentRoutingMode,
  meetingMode,
  savingRoutingMode,
  savingMeetingMode,
  formatRelativeTime,
  onBack,
  onSetRoutingMode,
  onSetMeetingMode,
}: ThreadDetailHeaderProps) {
  const { t } = useTranslation();

  return (
    <div className="flex h-14 shrink-0 items-center justify-between border-b px-5">
      <div className="flex items-center gap-3">
        <button
          type="button"
          className="flex h-8 w-8 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
          onClick={onBack}
        >
          <ArrowLeft className="h-4 w-4" />
        </button>
        <div className="flex items-center gap-2">
          <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-blue-50 text-blue-600">
            <MessageSquare className="h-4 w-4" />
          </div>
          <div className="min-w-0">
            <h1 className="truncate text-sm font-semibold leading-tight">
              {thread.title}
            </h1>
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <span className="flex items-center gap-1">
                <span
                  className={cn(
                    "h-1.5 w-1.5 rounded-full",
                    thread.status === "active"
                      ? "bg-emerald-500"
                      : "bg-slate-400",
                  )}
                />
                {thread.status}
              </span>
              {thread.owner_id && (
                <>
                  <span className="text-border">|</span>
                  <span>{thread.owner_id}</span>
                </>
              )}
              <span className="text-border">|</span>
              <span>{formatRelativeTime(thread.updated_at)}</span>
            </div>
          </div>
        </div>
      </div>
      <div className="flex items-center gap-2">
        <div className="flex items-center gap-1 rounded-lg border bg-muted/30 px-1 py-0.5 text-xs">
          <button
            type="button"
            className={cn(
              "rounded-md px-2.5 py-1 transition-colors",
              agentRoutingMode === "mention_only"
                ? "bg-background font-medium shadow-sm"
                : "text-muted-foreground hover:text-foreground",
            )}
            onClick={() => onSetRoutingMode("mention_only")}
            disabled={savingRoutingMode}
          >
            {t("threads.routingMentionOnly", "@ Only")}
          </button>
          <button
            type="button"
            className={cn(
              "rounded-md px-2.5 py-1 transition-colors",
              agentRoutingMode === "broadcast"
                ? "bg-background font-medium shadow-sm"
                : "text-muted-foreground hover:text-foreground",
            )}
            onClick={() => onSetRoutingMode("broadcast")}
            disabled={savingRoutingMode}
          >
            {t("threads.routingBroadcast", "Broadcast")}
          </button>
          <button
            type="button"
            className={cn(
              "rounded-md px-2.5 py-1 transition-colors",
              agentRoutingMode === "auto"
                ? "bg-background font-medium shadow-sm"
                : "text-muted-foreground hover:text-foreground",
            )}
            onClick={() => onSetRoutingMode("auto")}
            disabled={savingRoutingMode}
          >
            {t("threads.routingAuto", "Auto")}
          </button>
        </div>
        <div className="flex items-center gap-1 rounded-lg border bg-muted/30 px-1 py-0.5 text-xs">
          <button
            type="button"
            className={cn(
              "rounded-md px-2.5 py-1 transition-colors",
              meetingMode === "direct"
                ? "bg-background font-medium shadow-sm"
                : "text-muted-foreground hover:text-foreground",
            )}
            onClick={() => onSetMeetingMode("direct")}
            disabled={savingMeetingMode}
          >
            {t("threads.meetingDirect", "Direct")}
          </button>
          <button
            type="button"
            className={cn(
              "rounded-md px-2.5 py-1 transition-colors",
              meetingMode === "concurrent"
                ? "bg-background font-medium shadow-sm"
                : "text-muted-foreground hover:text-foreground",
            )}
            onClick={() => onSetMeetingMode("concurrent")}
            disabled={savingMeetingMode}
          >
            {t("threads.meetingConcurrent", "Concurrent")}
          </button>
          <button
            type="button"
            className={cn(
              "rounded-md px-2.5 py-1 transition-colors",
              meetingMode === "group_chat"
                ? "bg-background font-medium shadow-sm"
                : "text-muted-foreground hover:text-foreground",
            )}
            onClick={() => onSetMeetingMode("group_chat")}
            disabled={savingMeetingMode}
          >
            {t("threads.meetingGroupChat", "Group Chat")}
          </button>
        </div>
        <Badge variant="secondary" className="gap-1 text-xs">
          <Users className="h-3 w-3" />
          {participantsCount}
        </Badge>
        <Badge variant="secondary" className="gap-1 text-xs">
          <Bot className="h-3 w-3" />
          {agentSessionsCount}
        </Badge>
      </div>
    </div>
  );
}
