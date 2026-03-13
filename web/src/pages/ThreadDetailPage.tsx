import { useEffect, useState } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { ArrowLeft, Bot, Link2, Loader2, Plus, Save, Send, Users } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { useWorkbench } from "@/contexts/WorkbenchContext";
import { formatRelativeTime, getErrorMessage } from "@/lib/v2Workbench";
import { Link } from "react-router-dom";
import type { Thread, ThreadMessage, ThreadParticipant, ThreadWorkItemLink, ThreadAgentSession, Issue } from "@/types/apiV2";

function hasSavedSummary(thread: Thread | null): boolean {
  return Boolean(thread?.summary?.trim());
}

function deriveWorkItemTitle(thread: Thread): string {
  const firstMeaningfulLine = (thread.summary ?? "")
    .split(/\r?\n/)
    .map((line) => line.replace(/^[-*#\d.\)\s]+/, "").trim())
    .find((line) => line.length > 0);
  const title = firstMeaningfulLine || thread.title.trim();
  return title.length > 80 ? `${title.slice(0, 77)}...` : title;
}

function readSourceType(issue: Issue | undefined): string | null {
  const value = issue?.metadata?.source_type;
  return typeof value === "string" && value.trim().length > 0 ? value.trim() : null;
}

export function ThreadDetailPage() {
  const { t } = useTranslation();
  const { threadId } = useParams<{ threadId: string }>();
  const navigate = useNavigate();
  const { apiClient } = useWorkbench();

  const [thread, setThread] = useState<Thread | null>(null);
  const [messages, setMessages] = useState<ThreadMessage[]>([]);
  const [participants, setParticipants] = useState<ThreadParticipant[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [workItemLinks, setWorkItemLinks] = useState<ThreadWorkItemLink[]>([]);
  const [linkedIssues, setLinkedIssues] = useState<Record<number, Issue>>({});
  const [newMessage, setNewMessage] = useState("");
  const [sending, setSending] = useState(false);
  const [summaryDraft, setSummaryDraft] = useState("");
  const [savingSummary, setSavingSummary] = useState(false);
  const [showCreateWI, setShowCreateWI] = useState(false);
  const [newWITitle, setNewWITitle] = useState("");
  const [newWIBody, setNewWIBody] = useState("");
  const [showLinkWI, setShowLinkWI] = useState(false);
  const [linkWIId, setLinkWIId] = useState("");
  const [agentSessions, setAgentSessions] = useState<ThreadAgentSession[]>([]);

  const id = Number(threadId);
  const orderedWorkItemLinks = [...workItemLinks].sort((a, b) => {
    if (a.is_primary === b.is_primary) {
      return a.id - b.id;
    }
    return a.is_primary ? -1 : 1;
  });

  useEffect(() => {
    if (!id || isNaN(id)) return;
    let cancelled = false;

    const load = async () => {
      setLoading(true);
      setError(null);
      try {
        const [th, msgs, parts, links, agents] = await Promise.all([
          apiClient.getThread(id),
          apiClient.listThreadMessages(id, { limit: 100 }),
          apiClient.listThreadParticipants(id),
          apiClient.listWorkItemsByThread(id),
          apiClient.listThreadAgents(id),
        ]);
        if (!cancelled) {
          setThread(th);
          setSummaryDraft(th.summary ?? "");
          setMessages(msgs);
          setParticipants(parts);
          setWorkItemLinks(links);
          setAgentSessions(agents);
          // Fetch issue details for each link.
          const issueMap: Record<number, Issue> = {};
          const issueResults = await Promise.allSettled(
            links.map((l) => apiClient.getWorkItem(l.work_item_id)),
          );
          issueResults.forEach((r, i) => {
            if (r.status === "fulfilled") issueMap[links[i].work_item_id] = r.value;
          });
          if (!cancelled) setLinkedIssues(issueMap);
        }
      } catch (e) {
        if (!cancelled) setError(getErrorMessage(e));
      } finally {
        if (!cancelled) setLoading(false);
      }
    };
    void load();
    return () => { cancelled = true; };
  }, [apiClient, id]);

  const handleSend = async () => {
    if (!newMessage.trim() || !id) return;
    setSending(true);
    setError(null);
    try {
      const msg = await apiClient.createThreadMessage(id, {
        content: newMessage.trim(),
        role: "human",
      });
      setMessages((prev) => [...prev, msg]);
      setNewMessage("");
    } catch (e) {
      setError(getErrorMessage(e));
    } finally {
      setSending(false);
    }
  };

  const handleSaveSummary = async () => {
    if (!thread || !id) return;
    setSavingSummary(true);
    setError(null);
    try {
      const updated = await apiClient.updateThread(id, { summary: summaryDraft.trim() });
      setThread(updated);
      setSummaryDraft(updated.summary ?? "");
      if (showCreateWI) {
        const nextSummary = updated.summary?.trim() ?? "";
        setNewWIBody(nextSummary);
        setNewWITitle(nextSummary ? deriveWorkItemTitle(updated) : "");
      }
    } catch (e) {
      setError(getErrorMessage(e));
    } finally {
      setSavingSummary(false);
    }
  };

  const handleOpenCreateWorkItem = () => {
    if (!thread) return;
    if (!hasSavedSummary(thread)) {
      setError("请先生成或填写 summary，再创建 WorkItem。");
      setShowCreateWI(false);
      return;
    }
    setError(null);
    setShowCreateWI((prev) => {
      const next = !prev;
      if (next) {
        setNewWITitle(deriveWorkItemTitle(thread));
        setNewWIBody(thread.summary?.trim() ?? "");
      }
      return next;
    });
  };

  const handleCreateWorkItem = async () => {
    if (!newWITitle.trim() || !id) return;
    setError(null);
    try {
      const trimmedBody = newWIBody.trim();
      const savedSummary = thread?.summary?.trim() ?? "";
      const issue = await apiClient.createWorkItemFromThread(id, {
        title: newWITitle.trim(),
        body: trimmedBody !== "" && trimmedBody !== savedSummary ? trimmedBody : undefined,
      });
      const links = await apiClient.listWorkItemsByThread(id);
      setWorkItemLinks(links);
      setLinkedIssues((prev) => ({ ...prev, [issue.id]: issue }));
      setNewWITitle("");
      setNewWIBody("");
      setShowCreateWI(false);
    } catch (e) {
      setError(getErrorMessage(e));
    }
  };

  const handleLinkWorkItem = async () => {
    const wiId = Number(linkWIId);
    if (!wiId || isNaN(wiId) || !id) return;
    setError(null);
    try {
      await apiClient.createThreadWorkItemLink(id, { work_item_id: wiId, relation_type: "related" });
      const links = await apiClient.listWorkItemsByThread(id);
      setWorkItemLinks(links);
      try {
        const issue = await apiClient.getWorkItem(wiId);
        setLinkedIssues((prev) => ({ ...prev, [wiId]: issue }));
      } catch { /* ignore if issue fetch fails */ }
      setLinkWIId("");
      setShowLinkWI(false);
    } catch (e) {
      setError(getErrorMessage(e));
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center py-24">
        <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (!thread) {
    return (
      <div className="space-y-4 p-6">
        <Button variant="ghost" size="sm" onClick={() => navigate("/threads")}>
          <ArrowLeft className="mr-1.5 h-4 w-4" />
          {t("threads.backToList", "Back to Threads")}
        </Button>
        <div className="rounded-md bg-destructive/10 px-4 py-3 text-sm text-destructive">
          {error || t("threads.notFound", "Thread not found")}
        </div>
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col space-y-4 p-6">
      {/* Header */}
      <div className="flex items-center gap-3">
        <Button variant="ghost" size="sm" onClick={() => navigate("/threads")}>
          <ArrowLeft className="h-4 w-4" />
        </Button>
        <div className="flex-1">
          <h1 className="text-xl font-bold">{thread.title}</h1>
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <Badge variant={thread.status === "active" ? "default" : "secondary"}>
              {thread.status}
            </Badge>
            {thread.owner_id && <span>{t("threads.owner", "Owner")}: {thread.owner_id}</span>}
            <span>{formatRelativeTime(thread.updated_at)}</span>
          </div>
        </div>
      </div>

      {error ? (
        <div className="rounded-md bg-destructive/10 px-4 py-3 text-sm text-destructive">
          {error}
        </div>
      ) : null}

      <div className="flex flex-1 gap-4 overflow-hidden">
        {/* Messages area */}
        <Card className="flex flex-1 flex-col">
          <CardHeader className="pb-2">
            <CardTitle className="text-sm">
              {t("threads.messages", "Messages")} ({messages.length})
            </CardTitle>
          </CardHeader>
          <CardContent className="flex flex-1 flex-col overflow-hidden">
            <div className="flex-1 space-y-3 overflow-y-auto pb-4">
              {messages.length === 0 ? (
                <p className="py-8 text-center text-sm text-muted-foreground">
                  {t("threads.noMessages", "No messages yet. Start the conversation.")}
                </p>
              ) : (
                messages.map((msg) => (
                  <div
                    key={msg.id}
                    className={`rounded-lg px-3 py-2 text-sm ${
                      msg.role === "agent"
                        ? "bg-muted"
                        : "bg-primary/5"
                    }`}
                  >
                    <div className="mb-1 flex items-center gap-2 text-xs text-muted-foreground">
                      <Badge variant="outline" className="text-[10px]">
                        {msg.role}
                      </Badge>
                      <span>{msg.sender_id || "anonymous"}</span>
                      <span>{formatRelativeTime(msg.created_at)}</span>
                    </div>
                    <p className="whitespace-pre-wrap">{msg.content}</p>
                  </div>
                ))
              )}
            </div>

            {/* Send input */}
            <div className="flex gap-2 border-t pt-3">
              <Input
                placeholder={t("threads.messagePlaceholder", "Type a message...")}
                value={newMessage}
                onChange={(e) => setNewMessage(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && !e.shiftKey && handleSend()}
                disabled={sending || thread.status !== "active"}
              />
              <Button
                size="sm"
                onClick={handleSend}
                disabled={!newMessage.trim() || sending || thread.status !== "active"}
              >
                <Send className="h-4 w-4" />
              </Button>
            </div>
          </CardContent>
        </Card>

        {/* Right sidebar */}
        <div className="flex w-60 shrink-0 flex-col gap-4">
          {/* Participants panel */}
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="flex items-center gap-2 text-sm">
                <Users className="h-4 w-4" />
                {t("threads.participants", "Participants")} ({participants.length})
              </CardTitle>
            </CardHeader>
            <CardContent>
              {participants.length === 0 ? (
                <p className="text-xs text-muted-foreground">
                  {t("threads.noParticipants", "No participants")}
                </p>
              ) : (
                <div className="space-y-2">
                  {participants.map((p) => (
                    <div key={p.id} className="flex items-center gap-2 text-sm">
                      <Badge variant="outline" className="text-[10px]">
                        {p.role}
                      </Badge>
                      <span className="truncate">{p.user_id}</span>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>

          {/* Agent Sessions panel */}
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="flex items-center gap-2 text-sm">
                <Bot className="h-4 w-4" />
                {t("threads.agents", "Agents")} ({agentSessions.length})
              </CardTitle>
            </CardHeader>
            <CardContent>
              {agentSessions.length === 0 ? (
                <p className="text-xs text-muted-foreground">
                  {t("threads.noAgents", "No agents joined")}
                </p>
              ) : (
                <div className="space-y-3">
                  {agentSessions.map((s) => (
                    <div key={s.id} className="space-y-1">
                      <div className="flex items-center gap-2 text-sm">
                        <span className="truncate font-medium">{s.agent_profile_id}</span>
                        <Badge
                          variant={
                            s.status === "active" ? "default" :
                            s.status === "booting" ? "secondary" :
                            s.status === "paused" ? "outline" : "destructive"
                          }
                          className="text-[10px]"
                        >
                          {s.status}
                        </Badge>
                      </div>
                      <div className="flex items-center gap-2 text-[10px] text-muted-foreground">
                        <span>{t("threads.turns", "Turns")}: {s.turn_count}</span>
                        <span>
                          {((s.total_input_tokens + s.total_output_tokens) / 1000).toFixed(1)}k {t("threads.tokens", "tokens")}
                        </span>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </div>
      </div>

      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="flex items-center justify-between text-sm">
            <span>{t("threads.summary", "Summary")}</span>
            <Button
              variant="outline"
              size="sm"
              onClick={handleSaveSummary}
              disabled={savingSummary || summaryDraft.trim() === (thread.summary?.trim() ?? "")}
            >
              {savingSummary ? (
                <Loader2 className="mr-1 h-3.5 w-3.5 animate-spin" />
              ) : (
                <Save className="mr-1 h-3.5 w-3.5" />
              )}
              {t("common.save", "Save")}
            </Button>
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <p className="text-xs text-muted-foreground">
            {t(
              "threads.summaryEntryHint",
              "Summary is the convergence bridge between discussion and execution. Save it here before creating a work item.",
            )}
          </p>
          <Textarea
            value={summaryDraft}
            onChange={(e) => setSummaryDraft(e.target.value)}
            placeholder={t(
              "threads.summaryPlaceholder",
              "Capture the current consensus, decisions, scope, risks, and next actions for this thread.",
            )}
            className="min-h-[132px] resize-y text-sm"
          />
          {!hasSavedSummary(thread) ? (
            <p className="text-xs text-amber-700">
              {t(
                "threads.summaryMissingHint",
                "Work item creation depends on summary. Save a summary first to turn this discussion into execution.",
              )}
            </p>
          ) : null}
        </CardContent>
      </Card>

      {/* Linked Work Items */}
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="flex items-center justify-between text-sm">
            <span className="flex items-center gap-2">
              <Link2 className="h-4 w-4" />
              {t("threads.linkedWorkItems", "Linked Work Items")} ({workItemLinks.length})
            </span>
            <span className="flex gap-1">
              <Button variant="ghost" size="sm" onClick={handleOpenCreateWorkItem}>
                <Plus className="mr-1 h-3 w-3" />
                {t("threads.createWorkItem", "Create")}
              </Button>
              <Button variant="ghost" size="sm" onClick={() => setShowLinkWI(!showLinkWI)}>
                <Link2 className="mr-1 h-3 w-3" />
                {t("threads.linkExisting", "Link")}
              </Button>
            </span>
          </CardTitle>
        </CardHeader>
        <CardContent>
          {showCreateWI && (
            <div className="mb-3 space-y-3 rounded-md border border-border/60 bg-muted/20 p-3">
              <div className="space-y-1">
                <p className="text-xs font-medium text-foreground">
                  {t("threads.summaryToWorkItem", "Create Work Item from Summary")}
                </p>
                <p className="text-xs text-muted-foreground">
                  {t(
                    "threads.summaryToWorkItemHint",
                    "The body is prefilled from the saved summary. Update the summary first if the discussion has changed.",
                  )}
                </p>
              </div>
              <Input
                placeholder={t("threads.workItemTitle", "Work item title...")}
                value={newWITitle}
                onChange={(e) => setNewWITitle(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && !e.shiftKey && handleCreateWorkItem()}
              />
              <Textarea
                placeholder={t("threads.workItemBody", "Work item body...")}
                value={newWIBody}
                onChange={(e) => setNewWIBody(e.target.value)}
                className="min-h-[120px] resize-y text-sm"
              />
              <div className="flex justify-end gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => {
                    setShowCreateWI(false);
                    setNewWITitle("");
                    setNewWIBody("");
                  }}
                >
                  {t("common.cancel", "Cancel")}
                </Button>
                <Button size="sm" onClick={handleCreateWorkItem} disabled={!newWITitle.trim() || !newWIBody.trim()}>
                  {t("common.create", "Create")}
                </Button>
              </div>
            </div>
          )}
          {showLinkWI && (
            <div className="mb-3 flex gap-2">
              <Input
                placeholder={t("threads.workItemId", "Work item ID...")}
                value={linkWIId}
                onChange={(e) => setLinkWIId(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && handleLinkWorkItem()}
              />
              <Button size="sm" onClick={handleLinkWorkItem} disabled={!linkWIId.trim()}>
                {t("threads.linkBtn", "Link")}
              </Button>
            </div>
          )}
          {workItemLinks.length === 0 ? (
            <p className="py-2 text-center text-xs text-muted-foreground">
              {t("threads.noLinkedWorkItems", "No linked work items")}
            </p>
          ) : (
            <div className="space-y-2">
              {orderedWorkItemLinks.map((link) => {
                const issue = linkedIssues[link.work_item_id];
                const sourceType = readSourceType(issue);
                return (
                  <div
                    key={link.id}
                    className={`rounded-md border px-3 py-2 text-sm ${
                      link.is_primary
                        ? "border-blue-200 bg-blue-50/50"
                        : "border-border/60"
                    }`}
                  >
                    <div className="flex items-center gap-2">
                      {link.is_primary && (
                        <Badge variant="default" className="text-[10px]">
                          {t("threads.primary", "primary")}
                        </Badge>
                      )}
                      <Badge variant="outline" className="text-[10px]">
                        {link.relation_type}
                      </Badge>
                      {sourceType ? (
                        <Badge variant="secondary" className="text-[10px]">
                          {sourceType === "thread_summary" ? "summary" : sourceType === "thread_manual" ? "manual" : sourceType}
                        </Badge>
                      ) : null}
                      <Link
                        to={`/work-items/${link.work_item_id}`}
                        className="min-w-0 flex-1 truncate font-medium text-primary hover:underline"
                      >
                        {issue ? issue.title : `#${link.work_item_id}`}
                      </Link>
                      {issue && (
                        <Badge variant="secondary" className="text-[10px]">
                          {issue.status}
                        </Badge>
                      )}
                    </div>
                    {link.is_primary ? (
                      <p className="mt-1 text-xs text-muted-foreground">
                        {t("threads.primaryWorkItemHint", "This is the primary work item converged from the current thread.")}
                      </p>
                    ) : null}
                  </div>
                );
              })}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
