import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";
import { ChevronDown, ChevronUp, Link2, Loader2, Plus, Save } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { cn } from "@/lib/utils";
import { formatRelativeTime } from "@/lib/v2Workbench";
import type { Thread, ThreadWorkItemLink, WorkItem } from "@/types/apiV2";

interface ThreadDetailsPanelProps {
  thread: Thread;
  messagesCount: number;
  summaryCollapsed: boolean;
  summaryDraft: string;
  savingSummary: boolean;
  showSummaryMissingHint: boolean;
  showCreateWI: boolean;
  newWITitle: string;
  newWIBody: string;
  showLinkWI: boolean;
  linkWIId: string;
  workItemLinks: ThreadWorkItemLink[];
  orderedWorkItemLinks: ThreadWorkItemLink[];
  linkedWorkItems: Record<number, WorkItem>;
  onSummaryCollapsedChange: (collapsed: boolean) => void;
  onSummaryDraftChange: (value: string) => void;
  onSaveSummary: () => void;
  onOpenCreateWorkItem: () => void;
  onShowCreateWIChange: (open: boolean) => void;
  onNewWITitleChange: (value: string) => void;
  onNewWIBodyChange: (value: string) => void;
  onCreateWorkItem: () => void;
  onShowLinkWIChange: (open: boolean) => void;
  onLinkWIIdChange: (value: string) => void;
  onLinkWorkItem: () => void;
  onResetCreateWorkItemDraft: () => void;
}

function readWorkItemSourceType(workItem: WorkItem | undefined): string | null {
  const value = workItem?.metadata?.source_type;
  return typeof value === "string" && value.trim().length > 0 ? value.trim() : null;
}

export function ThreadDetailsPanel({
  thread,
  messagesCount,
  summaryCollapsed,
  summaryDraft,
  savingSummary,
  showSummaryMissingHint,
  showCreateWI,
  newWITitle,
  newWIBody,
  showLinkWI,
  linkWIId,
  workItemLinks,
  orderedWorkItemLinks,
  linkedWorkItems,
  onSummaryCollapsedChange,
  onSummaryDraftChange,
  onSaveSummary,
  onOpenCreateWorkItem,
  onShowCreateWIChange,
  onNewWITitleChange,
  onNewWIBodyChange,
  onCreateWorkItem,
  onShowLinkWIChange,
  onLinkWIIdChange,
  onLinkWorkItem,
  onResetCreateWorkItemDraft,
}: ThreadDetailsPanelProps) {
  const { t } = useTranslation();

  return (
    <div className="space-y-4 p-4">
      <div className="space-y-2">
        <button
          type="button"
          className="flex w-full items-center justify-between text-xs font-semibold uppercase tracking-wider text-muted-foreground"
          onClick={() => onSummaryCollapsedChange(!summaryCollapsed)}
        >
          <span>{t("threads.summary", "Summary")}</span>
          {summaryCollapsed ? <ChevronDown className="h-3.5 w-3.5" /> : <ChevronUp className="h-3.5 w-3.5" />}
        </button>
        {!summaryCollapsed && (
          <div className="space-y-2">
            <p className="text-[11px] text-muted-foreground">
              {t("threads.summaryEntryHint", "Capture decisions, scope, risks, and next actions.")}
            </p>
            <Textarea
              value={summaryDraft}
              onChange={(e) => onSummaryDraftChange(e.target.value)}
              placeholder={t(
                "threads.summaryPlaceholder",
                "Capture the current consensus, decisions, scope, risks, and next actions for this thread.",
              )}
              className="min-h-[100px] resize-y text-xs"
            />
            <div className="flex justify-end">
              <Button
                variant="outline"
                size="sm"
                onClick={onSaveSummary}
                disabled={savingSummary || summaryDraft.trim() === (thread.summary?.trim() ?? "")}
              >
                {savingSummary ? <Loader2 className="mr-1 h-3.5 w-3.5 animate-spin" /> : <Save className="mr-1 h-3.5 w-3.5" />}
                {t("common.save", "Save")}
              </Button>
            </div>
            {showSummaryMissingHint && (
              <p className="text-[11px] text-amber-600">
                {t("threads.summaryMissingHint", "Save a summary first to create work items.")}
              </p>
            )}
          </div>
        )}
      </div>

      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <h3 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
            {t("threads.linkedWorkItems", "Work Items")} ({workItemLinks.length})
          </h3>
          <div className="flex gap-1">
            <button
              type="button"
              className="flex h-6 items-center gap-1 rounded-md px-1.5 text-[11px] text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
              onClick={onOpenCreateWorkItem}
            >
              <Plus className="h-3 w-3" />
              {t("threads.createWorkItem", "Create")}
            </button>
            <button
              type="button"
              className="flex h-6 items-center gap-1 rounded-md px-1.5 text-[11px] text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
              onClick={() => onShowLinkWIChange(!showLinkWI)}
            >
              <Link2 className="h-3 w-3" />
              {t("threads.linkExisting", "Link")}
            </button>
          </div>
        </div>

        {showCreateWI && (
          <div className="space-y-2 rounded-lg border bg-muted/20 p-3">
            <p className="text-[11px] font-medium">{t("threads.summaryToWorkItem", "Create from Summary")}</p>
            <Input
              placeholder={t("threads.workItemTitle", "Title...")}
              className="text-xs"
              value={newWITitle}
              onChange={(e) => onNewWITitleChange(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && !e.shiftKey && onCreateWorkItem()}
            />
            <Textarea
              placeholder={t("threads.workItemBody", "Body...")}
              value={newWIBody}
              onChange={(e) => onNewWIBodyChange(e.target.value)}
              className="min-h-[80px] resize-y text-xs"
            />
            <div className="flex justify-end gap-2">
              <Button
                variant="ghost"
                size="sm"
                className="h-7 text-xs"
                onClick={() => {
                  onShowCreateWIChange(false);
                  onResetCreateWorkItemDraft();
                }}
              >
                {t("common.cancel", "Cancel")}
              </Button>
              <Button size="sm" className="h-7 text-xs" onClick={onCreateWorkItem} disabled={!newWITitle.trim() || !newWIBody.trim()}>
                {t("common.create", "Create")}
              </Button>
            </div>
          </div>
        )}

        {showLinkWI && (
          <div className="flex gap-2">
            <Input
              placeholder={t("threads.workItemId", "Work item ID...")}
              className="text-xs"
              value={linkWIId}
              onChange={(e) => onLinkWIIdChange(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && onLinkWorkItem()}
            />
            <Button size="sm" className="h-8 text-xs" onClick={onLinkWorkItem} disabled={!linkWIId.trim()}>
              {t("threads.linkBtn", "Link")}
            </Button>
          </div>
        )}

        {workItemLinks.length === 0 ? (
          <p className="py-4 text-center text-[11px] text-muted-foreground">
            {t("threads.noLinkedWorkItems", "No linked work items")}
          </p>
        ) : (
          <div className="space-y-1.5">
            {orderedWorkItemLinks.map((link) => {
              const workItem = linkedWorkItems[link.work_item_id];
              const sourceType = readWorkItemSourceType(workItem);
              return (
                <div
                  key={link.id}
                  className={cn(
                    "rounded-lg border px-3 py-2 text-xs",
                    link.is_primary ? "border-blue-200 bg-blue-50/50" : "border-border/60",
                  )}
                >
                  <div className="flex items-center gap-1.5">
                    {link.is_primary && <Badge variant="default" className="text-[9px]">primary</Badge>}
                    <Badge variant="outline" className="text-[9px]">{link.relation_type}</Badge>
                    {sourceType ? (
                      <Badge variant="secondary" className="text-[9px]">
                        {sourceType === "thread_summary" ? "summary" : sourceType === "thread_manual" ? "manual" : sourceType}
                      </Badge>
                    ) : null}
                    <Link
                      to={`/work-items/${link.work_item_id}`}
                      className="min-w-0 flex-1 truncate font-medium text-primary hover:underline"
                    >
                      {workItem ? workItem.title : `#${link.work_item_id}`}
                    </Link>
                    {workItem && <Badge variant="secondary" className="text-[9px]">{workItem.status}</Badge>}
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>

      <div className="space-y-2">
        <h3 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
          {t("threads.info", "Thread Info")}
        </h3>
        <div className="space-y-1 rounded-lg border bg-muted/20 p-3 text-xs">
          <div className="flex justify-between">
            <span className="text-muted-foreground">ID</span>
            <span className="font-mono">{thread.id}</span>
          </div>
          <div className="flex justify-between">
            <span className="text-muted-foreground">{t("threads.status", "Status")}</span>
            <span>{thread.status}</span>
          </div>
          <div className="flex justify-between">
            <span className="text-muted-foreground">{t("threads.owner", "Owner")}</span>
            <span>{thread.owner_id || "-"}</span>
          </div>
          <div className="flex justify-between">
            <span className="text-muted-foreground">{t("threads.updated", "Updated")}</span>
            <span>{formatRelativeTime(thread.updated_at)}</span>
          </div>
          <div className="flex justify-between">
            <span className="text-muted-foreground">{t("threads.messages", "Messages")}</span>
            <span>{messagesCount}</span>
          </div>
        </div>
      </div>
    </div>
  );
}
