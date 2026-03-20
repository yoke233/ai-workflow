import { useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link, useNavigate } from "react-router-dom";
import {
  ArrowRight,
  Clock,
  Loader2,
  MessageCircle,
  MessagesSquare,
  Search,
  Send,
  User,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { useWorkbench } from "@/contexts/WorkbenchContext";
import { formatRelativeTime, getErrorMessage } from "@/lib/v2Workbench";
import { cn } from "@/lib/utils";
import type { Thread } from "@/types/apiV2";

/* ── status helpers ── */

function statusColor(status: string) {
  switch (status) {
    case "active":
      return "bg-emerald-400";
    case "closed":
      return "bg-slate-300";
    case "archived":
      return "bg-amber-300";
    default:
      return "bg-slate-300";
  }
}

function statusVariant(status: string): "default" | "secondary" | "outline" {
  switch (status) {
    case "active":
      return "default";
    case "closed":
      return "secondary";
    case "archived":
      return "outline";
    default:
      return "default";
  }
}

/* ── page ── */

export function ThreadsPage() {
  const { t } = useTranslation();
  const { apiClient } = useWorkbench();
  const navigate = useNavigate();
  const inputRef = useRef<HTMLTextAreaElement>(null);

  const [threads, setThreads] = useState<Thread[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const [creating, setCreating] = useState(false);
  const [draft, setDraft] = useState("");

  /* ── load threads ── */

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      setLoading(true);
      setError(null);
      try {
        const listed = await apiClient.listThreads({ limit: 200 });
        if (!cancelled) setThreads(listed);
      } catch (e) {
        if (!cancelled) setError(getErrorMessage(e));
      } finally {
        if (!cancelled) setLoading(false);
      }
    };
    void load();
    return () => {
      cancelled = true;
    };
  }, [apiClient]);

  /* ── filter ── */

  const filtered = useMemo(() => {
    if (!search.trim()) return threads;
    const q = search.toLowerCase();
    return threads.filter(
      (th) =>
        th.title.toLowerCase().includes(q) ||
        (th.summary ?? "").toLowerCase().includes(q) ||
        String(th.id).includes(q),
    );
  }, [threads, search]);

  /* ── create thread from first message ── */

  const handleCreate = async () => {
    const message = draft.trim();
    if (!message) return;
    setCreating(true);
    setError(null);
    try {
      // Create thread with first message as title (truncated).
      const title = message.length > 80 ? message.slice(0, 77) + "..." : message;
      const created = await apiClient.createThread({ title });
      // Post the first message into the new thread.
      await apiClient.createThreadMessage(created.id, {
        content: message,
        role: "human",
        sender_id: "human",
      });
      // Navigate directly to the thread.
      navigate(`/threads/${created.id}`);
    } catch (e) {
      setError(getErrorMessage(e));
      setCreating(false);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      void handleCreate();
    }
  };

  /* ── stats ── */

  const activeCount = threads.filter((th) => th.status === "active").length;

  return (
    <div className="mx-auto max-w-4xl px-6 py-8">
      {/* ── Header ── */}
      <div className="mb-8">
        <div className="flex items-start justify-between gap-4">
          <div>
            <h1 className="text-xl font-semibold tracking-tight text-slate-900">
              {t("nav.threads")}
            </h1>
            <p className="mt-1 text-sm text-slate-500">
              {activeCount > 0
                ? `${activeCount} ${t("threads.activeThreads", "active")}`
                : t("threads.noActiveThreads", "Start a conversation")}
            </p>
          </div>
          <Link
            to="/requirements/new"
            className="inline-flex items-center rounded-lg border border-slate-200 px-3 py-2 text-sm font-medium text-slate-700 transition-colors hover:border-slate-300 hover:bg-slate-50 hover:text-slate-900"
          >
            从需求创建
          </Link>
        </div>
      </div>

      {/* ── Composer: first message creates thread ── */}
      <div className="group relative mb-8">
        <div className="rounded-xl border border-slate-200 bg-white shadow-sm transition-shadow focus-within:border-slate-300 focus-within:shadow-md">
          <textarea
            ref={inputRef}
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder={t("threads.composerPlaceholder", "Start a new discussion...")}
            rows={2}
            disabled={creating}
            className={cn(
              "w-full resize-none rounded-xl bg-transparent px-4 pt-4 pb-12 text-sm text-slate-900 placeholder:text-slate-400 focus:outline-none",
              creating && "opacity-60",
            )}
          />
          <div className="absolute bottom-3 right-3 flex items-center gap-2">
            <span className="text-[11px] text-slate-400 opacity-0 transition-opacity group-focus-within:opacity-100">
              Enter {t("threads.toSend", "to send")}
            </span>
            <button
              type="button"
              onClick={() => void handleCreate()}
              disabled={creating || !draft.trim()}
              className={cn(
                "flex h-8 w-8 items-center justify-center rounded-lg transition-all",
                draft.trim()
                  ? "bg-slate-900 text-white hover:bg-slate-800"
                  : "bg-slate-100 text-slate-400",
              )}
            >
              {creating ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                <Send className="h-3.5 w-3.5" />
              )}
            </button>
          </div>
        </div>
      </div>

      {/* ── Search ── */}
      {threads.length > 0 && (
        <div className="relative mb-6">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
          <input
            type="text"
            placeholder={t("threads.search", "Search threads...")}
            className="h-9 w-full rounded-lg border border-slate-200 bg-white pl-9 pr-3 text-sm text-slate-900 placeholder:text-slate-400 focus:border-slate-300 focus:outline-none focus:ring-1 focus:ring-slate-200"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        </div>
      )}

      {/* ── Error ── */}
      {error && (
        <div className="mb-6 rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">
          {error}
        </div>
      )}

      {/* ── Thread list ── */}
      {loading ? (
        <div className="flex items-center justify-center py-20">
          <Loader2 className="h-5 w-5 animate-spin text-slate-400" />
        </div>
      ) : filtered.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-center">
          <div className="flex h-12 w-12 items-center justify-center rounded-full bg-slate-100">
            <MessagesSquare className="h-5 w-5 text-slate-400" />
          </div>
          <p className="mt-4 text-sm font-medium text-slate-600">
            {search.trim()
              ? t("threads.noResults", "No matching threads")
              : t("threads.emptyState", "No discussions yet")}
          </p>
          <p className="mt-1 text-xs text-slate-400">
            {search.trim()
              ? t("threads.tryDifferentSearch", "Try a different search term")
              : t("threads.emptyHint", "Type a message above to start your first thread")}
          </p>
        </div>
      ) : (
        <div className="space-y-2">
          {filtered.map((th) => (
            <Link
              key={th.id}
              to={`/threads/${th.id}`}
              className="group/card flex items-start gap-4 rounded-xl border border-slate-150 bg-white px-5 py-4 transition-all hover:border-slate-300 hover:shadow-sm"
            >
              {/* Left: status dot + icon */}
              <div className="relative mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-slate-100 text-slate-500 transition-colors group-hover/card:bg-slate-200">
                <MessageCircle className="h-4 w-4" />
                <span
                  className={cn(
                    "absolute -right-0.5 -top-0.5 h-2.5 w-2.5 rounded-full border-2 border-white",
                    statusColor(th.status),
                  )}
                />
              </div>

              {/* Center: title + summary + meta */}
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <span className="truncate text-sm font-medium text-slate-900 group-hover/card:text-slate-950">
                    {th.title}
                  </span>
                  {th.status !== "active" && (
                    <Badge variant={statusVariant(th.status)} className="text-[9px]">
                      {th.status}
                    </Badge>
                  )}
                </div>
                {th.summary && (
                  <p className="mt-0.5 truncate text-xs text-slate-500">
                    {th.summary}
                  </p>
                )}
                <div className="mt-2 flex items-center gap-3 text-[11px] text-slate-400">
                  <span className="flex items-center gap-1">
                    <Clock className="h-3 w-3" />
                    {formatRelativeTime(th.updated_at)}
                  </span>
                  {th.owner_id && (
                    <span className="flex items-center gap-1">
                      <User className="h-3 w-3" />
                      {th.owner_id}
                    </span>
                  )}
                  <span className="font-mono text-slate-300">#{th.id}</span>
                </div>
              </div>

              {/* Right: arrow */}
              <ArrowRight className="mt-2 h-4 w-4 shrink-0 text-slate-300 transition-all group-hover/card:translate-x-0.5 group-hover/card:text-slate-500" />
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
