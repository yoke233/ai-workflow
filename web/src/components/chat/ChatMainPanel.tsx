import type React from "react";
import { useTranslation } from "react-i18next";
import { Loader2 } from "lucide-react";
import type { AgentDriver, AgentProfile, Event as ApiEvent } from "@/types/apiV2";
import { ChatEventsPanel } from "./ChatEventsPanel";
import { ChatScrollTrack } from "./ChatScrollTrack";
import { DraftSessionSetup } from "./DraftSessionSetup";
import { MessageFeedView } from "./MessageFeedView";
import type { ChatFeedEntry, LeadDriverOption } from "./chatTypes";

interface ChatMainPanelProps {
  detailView: "chat" | "events";
  currentEvents: ApiEvent[];
  isDraftSessionView: boolean;
  projects: Array<{ id: number; name: string }>;
  draftProjectId: number | null;
  draftProfileId: string;
  draftDriverId: string;
  leadDriverOptions: LeadDriverOption[];
  leadProfiles: AgentProfile[];
  drivers: AgentDriver[];
  messageInput: string;
  pendingFiles: File[];
  draftSessionReady: boolean;
  submitting: boolean;
  currentDriverLabel: string;
  currentProjectLabel: string;
  fileInputRef: React.RefObject<HTMLInputElement>;
  onProjectChange: (id: number | null) => void;
  onProfileChange: (id: string) => void;
  onDriverChange: (id: string) => void;
  onMessageChange: (value: string) => void;
  onSend: () => void;
  onPaste: (e: React.ClipboardEvent) => void;
  onRemovePendingFile: (index: number) => void;
  chatFeedEntries: ChatFeedEntry[];
  hasMoreFeedEntries: boolean;
  loadingMore: boolean;
  visibleFeedEntries: ChatFeedEntry[];
  copiedMessageId: string | null;
  collapsedActivityGroups: Record<string, boolean>;
  activeSession: string | null;
  chatContainerRef: React.RefObject<HTMLDivElement>;
  messagesEndRef: React.RefObject<HTMLDivElement>;
  onScroll: React.UIEventHandler<HTMLDivElement>;
  onCopyMessage: (id: string, content: string) => void;
  onCreateWorkItem: (id: string, content: string) => void;
  onActivityGroupToggle: (id: string) => void;
}

export function ChatMainPanel({
  detailView,
  currentEvents,
  isDraftSessionView,
  projects,
  draftProjectId,
  draftProfileId,
  draftDriverId,
  leadDriverOptions,
  leadProfiles,
  drivers,
  messageInput,
  pendingFiles,
  draftSessionReady,
  submitting,
  currentDriverLabel,
  currentProjectLabel,
  fileInputRef,
  onProjectChange,
  onProfileChange,
  onDriverChange,
  onMessageChange,
  onSend,
  onPaste,
  onRemovePendingFile,
  chatFeedEntries,
  hasMoreFeedEntries,
  loadingMore,
  visibleFeedEntries,
  copiedMessageId,
  collapsedActivityGroups,
  activeSession,
  chatContainerRef,
  messagesEndRef,
  onScroll,
  onCopyMessage,
  onCreateWorkItem,
  onActivityGroupToggle,
}: ChatMainPanelProps) {
  const { t } = useTranslation();

  return (
    <div className="relative flex-1">
      <div
        ref={chatContainerRef}
        className="absolute inset-0 overflow-y-auto px-5 py-4 pr-6 [scrollbar-gutter:stable]"
        onScroll={onScroll}
      >
        {detailView === "events" ? (
          <ChatEventsPanel events={currentEvents} />
        ) : isDraftSessionView ? (
          <div className="flex min-h-full items-center justify-center">
            <DraftSessionSetup
              projects={projects}
              draftProjectId={draftProjectId}
              draftProfileId={draftProfileId}
              draftDriverId={draftDriverId}
              leadDriverOptions={leadDriverOptions}
              leadProfiles={leadProfiles}
              drivers={drivers}
              messageInput={messageInput}
              pendingFiles={pendingFiles}
              draftSessionReady={draftSessionReady}
              submitting={submitting}
              currentDriverLabel={currentDriverLabel}
              currentProjectLabel={currentProjectLabel}
              fileInputRef={fileInputRef}
              onProjectChange={onProjectChange}
              onProfileChange={onProfileChange}
              onDriverChange={onDriverChange}
              onMessageChange={onMessageChange}
              onSend={onSend}
              onPaste={onPaste}
              onRemovePendingFile={onRemovePendingFile}
            />
          </div>
        ) : chatFeedEntries.length === 0 ? (
          <div className="mx-auto w-full max-w-[1200px] rounded-2xl border border-dashed bg-muted/20 px-5 py-6 text-sm text-muted-foreground">
            {t("chat.noMessagesInSession")}
          </div>
        ) : (
          <div className="mx-auto w-full max-w-[1200px] space-y-1">
            {hasMoreFeedEntries && (
              <div className="flex items-center justify-center py-2 text-xs text-muted-foreground">
                {loadingMore
                  ? t("chat.loadingMore", { defaultValue: "加载中..." })
                  : t("chat.scrollUpForMore", { defaultValue: "向上滚动加载更早消息" })}
              </div>
            )}
            <MessageFeedView
              entries={visibleFeedEntries}
              submitting={submitting}
              copiedMessageId={copiedMessageId}
              collapsedActivityGroups={collapsedActivityGroups}
              onCopyMessage={onCopyMessage}
              onCreateWorkItem={onCreateWorkItem}
              onActivityGroupToggle={onActivityGroupToggle}
            />
            {submitting && !activeSession && (
              <div className="flex items-center gap-2.5 rounded-xl border border-blue-100 bg-blue-50/60 px-4 py-3 text-sm text-blue-600">
                <Loader2 className="h-4 w-4 animate-spin" />
                <span>{t("chat.creatingSession", { defaultValue: "正在创建会话..." })}</span>
              </div>
            )}
          </div>
        )}
        <div ref={messagesEndRef} />
      </div>
      <ChatScrollTrack containerRef={chatContainerRef} />
    </div>
  );
}
