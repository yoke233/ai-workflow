import type React from "react";
import { useTranslation } from "react-i18next";
import { Bot, Paperclip, Send, Users, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import type {
  AgentProfile,
  MessageFileRef,
  ThreadFileRef,
  ThreadMember,
} from "@/types/apiV2";

type AgentRoutingMode = "mention_only" | "broadcast" | "auto";
type MeetingMode = "direct" | "concurrent" | "group_chat";

export interface ThreadComposerMentionCandidate {
  id: string;
  label: string;
  status: string;
}

interface ThreadComposerPanelProps {
  threadStatus: string;
  agentRoutingMode: AgentRoutingMode;
  meetingMode: MeetingMode;
  sending: boolean;
  newMessage: string;
  messageInputRef: React.Ref<HTMLTextAreaElement>;
  selectedDiscussionAgents: string[];
  profileByID: Map<string, AgentProfile>;
  selectedFileRefs: MessageFileRef[];
  committedMentionTargetID: string | null;
  committedMentionProfile?: AgentProfile;
  committedMentionSession?: ThreadMember;
  agentStatusColor: (status: string) => string;
  hashDraftActive: boolean;
  fileCandidates: ThreadFileRef[];
  selectedHashIndex: number;
  mentionDraftActive: boolean;
  mentionCandidates: ThreadComposerMentionCandidate[];
  selectedMentionIndex: number;
  onFocusAgentProfile: (profileID: string) => void;
  onRemoveSelectedDiscussionAgent: (profileID: string) => void;
  onRemoveFileRef: (path: string) => void;
  onChooseHashCandidate: (file: ThreadFileRef) => void;
  onChooseMentionCandidate: (profileID: string) => void;
  onInputChange: React.ChangeEventHandler<HTMLTextAreaElement>;
  onInputClick: React.MouseEventHandler<HTMLTextAreaElement>;
  onInputKeyUp: React.KeyboardEventHandler<HTMLTextAreaElement>;
  onInputBlur: React.FocusEventHandler<HTMLTextAreaElement>;
  onInputKeyDown: React.KeyboardEventHandler<HTMLTextAreaElement>;
  onInputPaste: React.ClipboardEventHandler<HTMLTextAreaElement>;
  onUploadInputChange: React.ChangeEventHandler<HTMLInputElement>;
  onSend: () => void;
}

export function ThreadComposerPanel({
  threadStatus,
  agentRoutingMode,
  meetingMode,
  sending,
  newMessage,
  messageInputRef,
  selectedDiscussionAgents,
  profileByID,
  selectedFileRefs,
  committedMentionTargetID,
  committedMentionProfile,
  committedMentionSession,
  agentStatusColor,
  hashDraftActive,
  fileCandidates,
  selectedHashIndex,
  mentionDraftActive,
  mentionCandidates,
  selectedMentionIndex,
  onFocusAgentProfile,
  onRemoveSelectedDiscussionAgent,
  onRemoveFileRef,
  onChooseHashCandidate,
  onChooseMentionCandidate,
  onInputChange,
  onInputClick,
  onInputKeyUp,
  onInputBlur,
  onInputKeyDown,
  onInputPaste,
  onUploadInputChange,
  onSend,
}: ThreadComposerPanelProps) {
  const { t } = useTranslation();

  return (
    <div className="shrink-0 border-t bg-background px-5 py-3">
      <div className="mx-auto max-w-3xl">
        {committedMentionTargetID ? (
          <div className="mb-2 flex items-center gap-2 rounded-lg bg-blue-50 px-3 py-1.5 text-xs">
            <Bot className="h-3.5 w-3.5 text-blue-500" />
            <span className="text-slate-600">
              {t("threads.mentionResolved", "Target agent")}: 
            </span>
            <button
              type="button"
              className="inline-flex items-center gap-1 rounded-full bg-white px-2 py-0.5 font-semibold text-blue-800 shadow-sm transition-colors hover:bg-blue-100"
              onClick={() => onFocusAgentProfile(committedMentionTargetID)}
            >
              @{committedMentionTargetID}
            </button>
            <span className="text-slate-500">
              {committedMentionProfile?.name ?? committedMentionTargetID}
            </span>
            <span className="inline-flex items-center gap-1 rounded-full bg-white px-2 py-0.5 text-[10px] font-medium text-slate-600">
              <span
                className={cn(
                  "h-1.5 w-1.5 rounded-full",
                  agentStatusColor(committedMentionSession?.status ?? "active"),
                )}
              />
              {committedMentionSession?.status ?? "active"}
            </span>
          </div>
        ) : null}

        <div className="relative">
          {hashDraftActive && fileCandidates.length > 0 ? (
            <div className="absolute bottom-full left-0 right-0 z-20 mb-2 overflow-hidden rounded-xl border bg-white shadow-lg dark:bg-zinc-900">
              <div className="border-b px-3 py-1.5">
                <span className="text-[10px] font-semibold uppercase tracking-widest text-muted-foreground">
                  Select file
                </span>
              </div>
              <div className="max-h-48 overflow-y-auto py-1">
                {fileCandidates.map((file, index) => (
                  <button
                    key={file.path}
                    type="button"
                    className={cn(
                      "flex w-full items-center justify-between px-3 py-2 text-left text-sm transition-colors",
                      index === selectedHashIndex
                        ? "bg-blue-100 dark:bg-blue-900/40"
                        : "hover:bg-accent/50",
                    )}
                    onMouseDown={(event) => {
                      event.preventDefault();
                      onChooseHashCandidate(file);
                    }}
                  >
                    <div className="flex min-w-0 items-center gap-2">
                      <Paperclip className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                      <span className="truncate font-medium">{file.name}</span>
                    </div>
                    <span className="ml-2 shrink-0 text-xs text-muted-foreground">
                      {file.source}
                    </span>
                  </button>
                ))}
              </div>
            </div>
          ) : null}

          {mentionDraftActive && mentionCandidates.length > 0 ? (
            <div className="absolute bottom-full left-0 right-0 z-20 mb-2 overflow-hidden rounded-xl border bg-white shadow-lg dark:bg-zinc-900">
              <div className="border-b px-3 py-1.5">
                <span className="text-[10px] font-semibold uppercase tracking-widest text-muted-foreground">
                  {t("threads.mentionCandidates", "Select agent")}
                </span>
              </div>
              <div className="py-1">
                {mentionCandidates.map((candidate, index) => (
                  <button
                    key={candidate.id}
                    type="button"
                    className={cn(
                      "flex w-full items-center justify-between px-3 py-2 text-left text-sm transition-colors",
                      index === selectedMentionIndex
                        ? "bg-blue-100 dark:bg-blue-900/40"
                        : "hover:bg-accent/50",
                    )}
                    onMouseDown={(event) => {
                      event.preventDefault();
                      onChooseMentionCandidate(candidate.id);
                    }}
                  >
                    <div className="flex items-center gap-2">
                      <div
                        className={cn(
                          "flex h-6 w-6 items-center justify-center rounded-full",
                          candidate.id === "all"
                            ? "bg-blue-100 text-blue-700"
                            : "bg-emerald-100 text-emerald-700",
                        )}
                      >
                        {candidate.id === "all" ? (
                          <Users className="h-3 w-3" />
                        ) : (
                          <Bot className="h-3 w-3" />
                        )}
                      </div>
                      <span className="font-medium">@{candidate.id}</span>
                      {candidate.id === "all" && (
                        <span className="text-xs text-muted-foreground">
                          广播给所有 agent
                        </span>
                      )}
                    </div>
                    {candidate.id !== "all" && (
                      <span className="flex items-center gap-1.5 text-xs text-muted-foreground">
                        <span
                          className={cn(
                            "h-1.5 w-1.5 rounded-full",
                            agentStatusColor(candidate.status),
                          )}
                        />
                        {candidate.status}
                      </span>
                    )}
                  </button>
                ))}
              </div>
            </div>
          ) : null}

          <div className="flex flex-wrap items-center gap-1.5 rounded-xl border bg-muted/30 px-3 py-2 transition-colors focus-within:border-blue-300 focus-within:bg-background focus-within:ring-2 focus-within:ring-blue-100">
            {selectedDiscussionAgents.map((profileID) => {
              const profile = profileByID.get(profileID);
              return (
                <span
                  key={profileID}
                  className="inline-flex shrink-0 items-center gap-1 rounded-md border border-emerald-200 bg-emerald-50 px-2 py-0.5 text-xs text-emerald-700"
                >
                  <Bot className="h-3 w-3" />
                  {profile?.name ?? profileID}
                  <button
                    type="button"
                    className="ml-0.5 rounded-sm hover:bg-emerald-200"
                    onClick={() => onRemoveSelectedDiscussionAgent(profileID)}
                  >
                    <X className="h-3 w-3" />
                  </button>
                </span>
              );
            })}
            {selectedFileRefs.map((ref) => (
              <span
                key={ref.path}
                className="inline-flex shrink-0 items-center gap-1 rounded-md border border-blue-200 bg-blue-50 px-2 py-0.5 text-xs text-blue-700 dark:border-blue-700 dark:bg-blue-900/30 dark:text-blue-300"
              >
                <Paperclip className="h-3 w-3" />
                {ref.name}
                <button
                  type="button"
                  className="ml-0.5 rounded-sm hover:bg-blue-200 dark:hover:bg-blue-800"
                  onClick={() => onRemoveFileRef(ref.path)}
                >
                  <X className="h-3 w-3" />
                </button>
              </span>
            ))}
            <textarea
              ref={messageInputRef}
              rows={2}
              placeholder={
                threadStatus !== "active"
                  ? t("threads.threadClosed", "Thread is closed")
                  : selectedDiscussionAgents.length > 0
                    ? t(
                        "threads.messagePlaceholderSelectedAgents",
                        "Type a message (will send to selected agents)...",
                      )
                    : meetingMode === "concurrent"
                      ? t(
                          "threads.messagePlaceholderConcurrent",
                          "Type a message (concurrent meeting with routed agents)...",
                        )
                      : meetingMode === "group_chat"
                        ? t(
                            "threads.messagePlaceholderGroupChat",
                            "Type a message (round-robin discussion with routed agents)...",
                          )
                        : agentRoutingMode === "auto"
                          ? t(
                              "threads.messagePlaceholderAuto",
                              "Type a message (auto-routed to the best-fit agent)...",
                            )
                          : agentRoutingMode === "broadcast"
                            ? t(
                                "threads.messagePlaceholderBroadcast",
                                "Type a message (broadcasts to all agents)...",
                              )
                            : t(
                                "threads.messagePlaceholder",
                                "Type @ to mention an agent, # to reference a file...",
                              )
              }
              className="flex-1 resize-none border-0 bg-transparent p-0 text-sm shadow-none outline-none focus:ring-0"
              value={newMessage}
              onChange={onInputChange}
              onClick={onInputClick}
              onKeyUp={onInputKeyUp}
              onBlur={onInputBlur}
              onKeyDown={onInputKeyDown}
              onPaste={onInputPaste}
              disabled={sending || threadStatus !== "active"}
            />
            <input
              type="file"
              className="hidden"
              id="chat-file-upload"
              multiple
              onChange={onUploadInputChange}
            />
            <label
              htmlFor="chat-file-upload"
              className="flex h-8 w-8 shrink-0 cursor-pointer items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
              title={t("threads.uploadFile", "Upload file")}
            >
              <Paperclip className="h-4 w-4" />
            </label>
            <Button
              size="icon"
              className="h-8 w-8 shrink-0 rounded-lg"
              onClick={onSend}
              disabled={!newMessage.trim() || sending || threadStatus !== "active"}
            >
              <Send className="h-4 w-4" />
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}
