import type { ReactNode } from "react";
import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import {
  Bot,
  Brain,
  ChevronDown,
  ChevronRight,
  Loader2,
  MessageSquareText,
  User,
  Wrench,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { compactText } from "@/components/chat/chatUtils";
import type { ChatActivityView } from "@/components/chat/chatTypes";
import type { AgentProfile, ThreadMessage } from "@/types/apiV2";

type ThreadAgentLiveOutput = {
  thought?: string;
  message?: string;
  updatedAt: string;
};

type ThreadAgentActivityEntry =
  | { type: "thought"; item: ChatActivityView }
  | { type: "message"; item: ChatActivityView }
  | { type: "tool_group"; id: string; items: ChatActivityView[] };

interface ThreadMessageListProps {
  messages: ThreadMessage[];
  profileByID: Map<string, AgentProfile>;
  thinkingAgentIDs: Set<string>;
  visibleAgentActivityIDs: string[];
  agentActivitiesByID: Record<string, ChatActivityView[]>;
  liveAgentOutputsByID: Record<string, ThreadAgentLiveOutput>;
  collapsedAgentActivityPanels: Record<string, boolean>;
  sending: boolean;
  renderMessageContent: (msg: ThreadMessage) => ReactNode;
  onToggleAgentActivityPanel: (profileID: string) => void;
  focusAgentProfile: (profileID: string) => void;
  readTargetAgentID: (metadata: Record<string, unknown> | undefined) => string | null;
  readTargetAgentIDs: (metadata: Record<string, unknown> | undefined) => string[];
  readAutoRoutedTo: (metadata: Record<string, unknown> | undefined) => string[];
  readMetadataType: (metadata: Record<string, unknown> | undefined) => string | null;
  formatRelativeTime: (value: string) => string;
}

function buildActivityEntries(activities: ChatActivityView[]): ThreadAgentActivityEntry[] {
  const entries: ThreadAgentActivityEntry[] = [];
  let toolBuffer: ChatActivityView[] = [];
  let groupCounter = 0;

  const flushTools = () => {
    if (toolBuffer.length === 0) {
      return;
    }
    entries.push({
      type: "tool_group",
      id: `tool-group-${groupCounter++}`,
      items: [...toolBuffer],
    });
    toolBuffer = [];
  };

  for (const activity of activities) {
    if (activity.type === "usage_update") {
      continue;
    }
    if (activity.type === "tool_call") {
      toolBuffer.push(activity);
      continue;
    }
    flushTools();
    if (activity.type === "agent_thought") {
      entries.push({ type: "thought", item: activity });
      continue;
    }
    if (activity.type === "agent_message") {
      entries.push({ type: "message", item: activity });
    }
  }

  flushTools();
  return entries;
}

function statusBadgeClass(status: ChatActivityView["status"]) {
  switch (status) {
    case "failed":
      return "border-rose-200 bg-rose-50 text-rose-700";
    case "completed":
      return "border-emerald-200 bg-emerald-50 text-emerald-700";
    case "running":
      return "border-amber-200 bg-amber-50 text-amber-700";
    default:
      return "border-border bg-muted/60 text-muted-foreground";
  }
}

function latestActivitySummary(
  activities: ChatActivityView[],
  liveOutput?: ThreadAgentLiveOutput,
): string {
  const liveMessage = liveOutput?.message?.trim();
  if (liveMessage) {
    return compactText(liveMessage, 120);
  }
  const liveThought = liveOutput?.thought?.trim();
  if (liveThought) {
    return compactText(liveThought, 120);
  }
  const last = activities.at(-1);
  if (!last) {
    return "";
  }
  return compactText((last.detail || last.title || "").trim(), 120);
}

function ThreadAgentActivityPanel({
  profileID,
  profile,
  isThinking,
  activities,
  liveOutput,
  collapsed,
  onToggle,
}: {
  profileID: string;
  profile?: AgentProfile;
  isThinking: boolean;
  activities: ChatActivityView[];
  liveOutput?: ThreadAgentLiveOutput;
  collapsed: boolean;
  onToggle: () => void;
}) {
  const { t } = useTranslation();
  const entries = useMemo(() => buildActivityEntries(activities), [activities]);
  const summary = latestActivitySummary(activities, liveOutput);
  const usage = [...activities]
    .reverse()
    .find((activity) => activity.type === "usage_update");

  return (
    <div className="flex gap-3">
      <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-emerald-100 text-emerald-700">
        <Bot className="h-4 w-4" />
      </div>
      <div className="max-w-[75%] min-w-0 flex-1">
        <div className="mb-1 flex items-center gap-1.5 text-[11px] text-muted-foreground">
          <span className="font-medium text-foreground/70">
            {profile?.name ?? profileID}
          </span>
          <span
            className={cn(
              "inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-[10px] font-medium",
              isThinking
                ? "border-amber-200 bg-amber-50 text-amber-700"
                : "border-border/60 bg-muted/50 text-muted-foreground",
            )}
          >
            {isThinking ? (
              <>
                <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-amber-500" />
                {t("chat.thinking", "Thinking")}
              </>
            ) : (
              t("chat.completed", "Completed")
            )}
          </span>
        </div>
        <div className="rounded-2xl rounded-tl-md border border-emerald-200/70 bg-gradient-to-br from-emerald-50/80 to-white px-4 py-3 shadow-sm">
          <button
            type="button"
            className="flex w-full items-center gap-2 text-left"
            onClick={onToggle}
          >
            {collapsed ? (
              <ChevronRight className="h-4 w-4 shrink-0 text-muted-foreground" />
            ) : (
              <ChevronDown className="h-4 w-4 shrink-0 text-muted-foreground" />
            )}
            <span className="text-sm font-medium text-foreground">
              {t("threads.agentWorkspace", "Agent workspace")}
            </span>
            {summary ? (
              <span className="min-w-0 flex-1 truncate text-xs text-muted-foreground">
                {summary}
              </span>
            ) : (
              <span className="flex items-center gap-0.5">
                <span
                  className="h-1.5 w-1.5 animate-bounce rounded-full bg-muted-foreground/50"
                  style={{ animationDelay: "0ms" }}
                />
                <span
                  className="h-1.5 w-1.5 animate-bounce rounded-full bg-muted-foreground/50"
                  style={{ animationDelay: "150ms" }}
                />
                <span
                  className="h-1.5 w-1.5 animate-bounce rounded-full bg-muted-foreground/50"
                  style={{ animationDelay: "300ms" }}
                />
              </span>
            )}
          </button>

          {!collapsed && (
            <div className="mt-3 space-y-3">
              {liveOutput?.thought?.trim() && (
                <div className="rounded-xl border border-amber-200/80 bg-amber-50/70 px-3 py-2">
                  <div className="mb-1 flex items-center gap-1.5 text-[11px] font-medium text-amber-700">
                    <Brain className="h-3.5 w-3.5" />
                    {t("threads.liveThought", "Live thought")}
                  </div>
                  <p className="whitespace-pre-wrap break-words text-xs italic leading-6 text-amber-900/80">
                    {liveOutput.thought}
                  </p>
                </div>
              )}

              {entries.map((entry) => {
                if (entry.type === "thought") {
                  return (
                    <div
                      key={entry.item.id}
                      className="flex items-start gap-2 rounded-xl border border-violet-200/70 bg-violet-50/60 px-3 py-2 text-xs text-violet-700"
                    >
                      <Brain className="mt-0.5 h-3.5 w-3.5 shrink-0" />
                      <span className="min-w-0 whitespace-pre-wrap break-words italic">
                        {entry.item.detail || entry.item.title}
                      </span>
                    </div>
                  );
                }

                if (entry.type === "message") {
                  return (
                    <div
                      key={entry.item.id}
                      className="rounded-xl border border-emerald-200/70 bg-white px-3 py-2"
                    >
                      <div className="mb-2 flex items-center gap-1.5 text-[11px] font-medium text-emerald-700">
                        <MessageSquareText className="h-3.5 w-3.5" />
                        {t("threads.streamReply", "Streaming reply")}
                      </div>
                      <div className="prose prose-sm prose-slate max-w-none text-foreground/90 prose-p:my-2 prose-ul:my-2 prose-ol:my-2 prose-li:my-0.5 prose-pre:my-2 prose-pre:overflow-x-auto prose-pre:rounded-md prose-pre:bg-slate-900 prose-pre:text-slate-50 prose-code:rounded prose-code:bg-muted prose-code:px-1 prose-code:py-0.5 prose-code:text-[13px] prose-code:before:content-none prose-code:after:content-none">
                        <ReactMarkdown remarkPlugins={[remarkGfm]}>
                          {entry.item.detail || entry.item.title}
                        </ReactMarkdown>
                      </div>
                    </div>
                  );
                }

                const activeItems = entry.items.filter((item) => item.status !== "completed");
                const displayItems = activeItems.length > 0 ? activeItems : entry.items;
                const hiddenCount = Math.max(displayItems.length - 2, 0);
                const summaryItems =
                  collapsed && displayItems.length > 2
                    ? [displayItems[0], displayItems[displayItems.length - 1]]
                    : displayItems;

                return (
                  <div
                    key={entry.id}
                    className="rounded-xl border border-amber-200/70 bg-amber-50/50 px-3 py-2"
                  >
                    <div className="mb-2 flex items-center gap-1.5 text-[11px] font-medium text-amber-700">
                      <Wrench className="h-3.5 w-3.5" />
                      {t("chat.toolCalls", "Tool calls")}
                    </div>
                    <div className="space-y-1.5">
                      {summaryItems.map((item, index) => (
                        <div key={item.id}>
                          <div className="flex items-start gap-2 text-xs">
                            <span className="min-w-0 flex-1 font-medium text-foreground">
                              {item.title}
                            </span>
                            {item.status ? (
                              <span
                                className={cn(
                                  "rounded-full border px-1.5 py-0.5 text-[10px] font-medium leading-none",
                                  statusBadgeClass(item.status),
                                )}
                              >
                                {item.status}
                              </span>
                            ) : null}
                          </div>
                          {(item.detail || item.title) && (
                            <p className="mt-1 whitespace-pre-wrap break-words text-[11px] text-muted-foreground">
                              {compactText(item.detail || item.title, 180)}
                            </p>
                          )}
                          {collapsed && index === 0 && hiddenCount > 0 && (
                            <p className="mt-1 text-[11px] text-muted-foreground/70">
                              ... {hiddenCount} more
                            </p>
                          )}
                        </div>
                      ))}
                    </div>
                  </div>
                );
              })}

              {liveOutput?.message?.trim() && (
                <div className="rounded-xl border border-emerald-200/80 bg-white px-3 py-2">
                  <div className="mb-2 flex items-center gap-1.5 text-[11px] font-medium text-emerald-700">
                    <MessageSquareText className="h-3.5 w-3.5" />
                    {t("threads.streamReply", "Streaming reply")}
                  </div>
                  <div className="prose prose-sm prose-slate max-w-none text-foreground/90 prose-p:my-2 prose-ul:my-2 prose-ol:my-2 prose-li:my-0.5 prose-pre:my-2 prose-pre:overflow-x-auto prose-pre:rounded-md prose-pre:bg-slate-900 prose-pre:text-slate-50 prose-code:rounded prose-code:bg-muted prose-code:px-1 prose-code:py-0.5 prose-code:text-[13px] prose-code:before:content-none prose-code:after:content-none">
                    <ReactMarkdown remarkPlugins={[remarkGfm]}>
                      {liveOutput.message}
                    </ReactMarkdown>
                  </div>
                </div>
              )}

              {usage && (
                <div className="text-[11px] text-muted-foreground">
                  {usage.detail || usage.title}
                </div>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

export function ThreadMessageList({
  messages,
  profileByID,
  thinkingAgentIDs,
  visibleAgentActivityIDs,
  agentActivitiesByID,
  liveAgentOutputsByID,
  collapsedAgentActivityPanels,
  sending,
  renderMessageContent,
  onToggleAgentActivityPanel,
  focusAgentProfile,
  readTargetAgentID,
  readTargetAgentIDs,
  readAutoRoutedTo,
  readMetadataType,
  formatRelativeTime,
}: ThreadMessageListProps) {
  const { t } = useTranslation();
  const hasActivityCards = visibleAgentActivityIDs.length > 0;

  if (messages.length === 0 && !hasActivityCards) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3 text-muted-foreground">
        <Bot className="h-10 w-10 text-muted-foreground/30" />
        <p className="text-sm">
          {t("threads.noMessages", "No messages yet. Start the conversation.")}
        </p>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-3xl space-y-4">
      {messages.map((msg) => {
        const isAgent = msg.role === "agent";
        const isSystem = msg.role === "system";
        const targetAgent = readTargetAgentID(msg.metadata);
        const targetAgentIDs = readTargetAgentIDs(msg.metadata);
        const autoRoutedTo = readAutoRoutedTo(msg.metadata);
        const metaType = readMetadataType(msg.metadata);
        const profile = isAgent ? profileByID.get(msg.sender_id) : undefined;


        if (
          isAgent &&
          (metaType === "task_output" ||
            metaType === "task_review_approved" ||
            metaType === "task_review_rejected")
        ) {
          const outputFile = (msg.metadata?.output_file as string) ?? "";
          const isReject = metaType === "task_review_rejected";
          const isApproved = metaType === "task_review_approved";
          return (
            <div key={msg.id} className="flex gap-3">
              <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-emerald-100 text-emerald-700">
                <Bot className="h-4 w-4" />
              </div>
              <div className="max-w-[75%] min-w-0">
                <div className="mb-1 flex items-center gap-1.5 text-[11px] text-muted-foreground">
                  <span className="font-medium text-foreground/70">
                    {profile?.name ?? msg.sender_id}
                  </span>

                  <span>{formatRelativeTime(msg.created_at)}</span>
                </div>
                <div
                  className={cn(
                    "rounded-2xl rounded-tl-md px-4 py-2.5 text-sm leading-relaxed",
                    isReject
                      ? "border border-rose-200 bg-rose-50/50 text-foreground"
                      : isApproved
                        ? "border border-emerald-200 bg-emerald-50/50 text-foreground"
                        : "bg-muted/80 text-foreground",
                  )}
                >
                  <p className="whitespace-pre-wrap break-words">{msg.content}</p>
                  {outputFile && (
                    <p className="mt-1.5 text-xs text-muted-foreground">
                      File: {outputFile}
                    </p>
                  )}
                </div>
              </div>
            </div>
          );
        }

        if (isSystem) {
          return (
            <div key={msg.id} className="flex justify-center">
              <div className="flex items-center gap-2 rounded-full border border-border/40 bg-muted/40 px-4 py-1.5 text-xs text-muted-foreground">
                <Bot className="h-3 w-3" />
                <span>{msg.content}</span>
              </div>
            </div>
          );
        }

        return (
          <div
            key={msg.id}
            className={cn("flex gap-3", !isAgent && "flex-row-reverse")}
          >
            <div
              className={cn(
                "flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-xs font-bold",
                isAgent
                  ? "bg-emerald-100 text-emerald-700"
                  : "bg-blue-100 text-blue-700",
              )}
            >
              {isAgent ? <Bot className="h-4 w-4" /> : <User className="h-4 w-4" />}
            </div>
            <div className="group/msg max-w-[75%] min-w-0">
              <div
                className={cn(
                  "mb-1 flex items-center gap-1.5 text-[11px] text-muted-foreground",
                  !isAgent && "flex-row-reverse",
                )}
              >
                <span className="font-medium text-foreground/70">
                  {isAgent
                    ? (profile?.name ?? msg.sender_id)
                    : (msg.sender_id || "You")}
                </span>
                {targetAgentIDs.length > 0
                  ? targetAgentIDs.map((agentID) => (
                      <span
                        key={agentID}
                        className="rounded bg-blue-50 px-1 py-px text-[10px] text-blue-600"
                      >
                        @{agentID}
                      </span>
                    ))
                  : targetAgent ? (
                      <span className="rounded bg-blue-50 px-1 py-px text-[10px] text-blue-600">
                        @{targetAgent}
                      </span>
                    ) : null}
                <span>{formatRelativeTime(msg.created_at)}</span>
              </div>
              <div
                className={cn(
                  "rounded-2xl px-4 py-2.5 text-sm leading-relaxed",
                  isAgent
                    ? "rounded-tl-md bg-muted/80 text-foreground"
                    : "rounded-tr-md bg-blue-600 text-white",
                )}
              >
                <p className="whitespace-pre-wrap break-words">
                  {renderMessageContent(msg)}
                </p>
              </div>
              {!isAgent && autoRoutedTo.length > 0 && (
                <div className="mt-1 flex flex-wrap items-center justify-end gap-1 text-[10px]">
                  <span className="text-muted-foreground/60">Auto</span>
                  <span className="text-muted-foreground/40">-&gt;</span>
                  {autoRoutedTo.map((agentID) => {
                    const agentProfile = profileByID.get(agentID);
                    return (
                      <button
                        key={agentID}
                        type="button"
                        className="inline-flex items-center gap-1 rounded-full border border-emerald-200 bg-emerald-50 px-1.5 py-0.5 font-medium text-emerald-700 transition-colors hover:bg-emerald-100"
                        onClick={() => focusAgentProfile(agentID)}
                      >
                        <Bot className="h-2.5 w-2.5" />
                        {agentProfile?.name ?? agentID}
                      </button>
                    );
                  })}
                </div>
              )}
            </div>
          </div>
        );
      })}

      {visibleAgentActivityIDs.map((agentID) => (
        <ThreadAgentActivityPanel
          key={agentID}
          profileID={agentID}
          profile={profileByID.get(agentID)}
          isThinking={thinkingAgentIDs.has(agentID)}
          activities={agentActivitiesByID[agentID] ?? []}
          liveOutput={liveAgentOutputsByID[agentID]}
          collapsed={collapsedAgentActivityPanels[agentID] ?? false}
          onToggle={() => onToggleAgentActivityPanel(agentID)}
        />
      ))}

      {sending && !hasActivityCards && thinkingAgentIDs.size === 0 && (
        <div className="flex items-center gap-2 px-11 text-xs text-muted-foreground">
          <Loader2 className="h-3.5 w-3.5 animate-spin" />
          <span>{t("threads.sending", "Sending")}...</span>
        </div>
      )}
    </div>
  );
}



