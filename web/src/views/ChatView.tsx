import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { ApiClient } from "../lib/apiClient";
import type { ChatMessage } from "../types/workflow";
import type { ChatRunEvent } from "../types/api";
import type { WsClient } from "../lib/wsClient";
import type {
  ACPSessionUpdate,
  ChatEventPayload,
  ChatEventType,
  WsEnvelope,
} from "../types/ws";
import FileTree from "../components/FileTree";
import GitStatusPanel from "../components/GitStatusPanel";

interface ChatViewProps {
  apiClient: ApiClient;
  wsClient: WsClient;
  projectId: string;
}

interface ChatSessionSummary {
  id: string;
  updatedAt: string;
  preview: string;
}

interface ChatSessionLike {
  id?: unknown;
  updated_at?: unknown;
  created_at?: unknown;
  messages?: unknown;
}

interface RunEventItem {
  id: string;
  sessionId: string;
  type: string;
  detail: string;
  time: string;
}

const MAX_RUN_EVENTS = 60;

const CHAT_RUN_EVENT_TYPES = new Set<ChatEventType>([
  "run_started",
  "run_update",
  "run_completed",
  "run_failed",
  "run_cancelled",
  "team_leader_thinking",
  "team_leader_files_changed",
]);

type ChatUpdateParser = (acp: ACPSessionUpdate) => string;

const CHAT_UPDATE_PARSERS: Record<string, ChatUpdateParser> = {
  agent_message_chunk: (acp) => toStringValue(acp.content?.text),
  assistant_message_chunk: (acp) => toStringValue(acp.content?.text),
  message_chunk: (acp) => toStringValue(acp.content?.text),
};

const roleLabel: Record<ChatMessage["role"], string> = {
  user: "用户",
  assistant: "助手",
};

const roleStyle: Record<ChatMessage["role"], string> = {
  user: "bg-slate-900 text-white",
  assistant: "border border-slate-200 bg-white text-slate-900",
};

const formatTime = (time: string): string => {
  const date = new Date(time);
  if (Number.isNaN(date.getTime())) {
    return time;
  }
  return date.toLocaleString("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
};

const getErrorMessage = (error: unknown): string => {
  if (error instanceof Error && error.message.trim().length > 0) {
    return error.message;
  }
  return "请求失败，请稍后重试";
};

const parseFilePathsDraft = (raw: string): string[] => {
  const unique: string[] = [];
  const seen = new Set<string>();
  raw
    .split(",")
    .map((item) => item.trim())
    .filter((item) => item.length > 0)
    .forEach((item) => {
      if (!seen.has(item)) {
        seen.add(item);
        unique.push(item);
      }
    });
  return unique;
};

const toStringValue = (value: unknown): string => {
  if (typeof value !== "string") {
    return "";
  }
  return value.trim();
};

const toRecord = (value: unknown): Record<string, unknown> | null => {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return null;
  }
  return value as Record<string, unknown>;
};

const getLatestMessagePreview = (rawMessages: unknown): string => {
  if (!Array.isArray(rawMessages)) {
    return "";
  }
  for (let index = rawMessages.length - 1; index >= 0; index -= 1) {
    const message = toRecord(rawMessages[index]);
    const content = toStringValue(message?.content);
    if (content) {
      return content.length > 80 ? `${content.slice(0, 80)}...` : content;
    }
  }
  return "";
};

const toChatSessionSummary = (raw: unknown): ChatSessionSummary | null => {
  const session = toRecord(raw) as ChatSessionLike | null;
  const id = toStringValue(session?.id);
  if (!id) {
    return null;
  }
  const updatedAt = toStringValue(session?.updated_at) || toStringValue(session?.created_at) || nowIso();
  return {
    id,
    updatedAt,
    preview: getLatestMessagePreview(session?.messages),
  };
};

const extractChatSessions = (raw: unknown): ChatSessionSummary[] => {
  const listSource = Array.isArray(raw)
    ? raw
    : Array.isArray((raw as { items?: unknown })?.items)
      ? ((raw as { items: unknown[] }).items ?? [])
      : [];
  return listSource
    .map((item) => toChatSessionSummary(item))
    .filter((item): item is ChatSessionSummary => item !== null)
    .sort((a, b) => new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime());
};

const buildRunEventDetail = (data: ChatEventPayload): string => {
  const updateType = toStringValue(data.acp?.sessionUpdate);
  if (!updateType) {
    return "收到增量更新";
  }
  const title = toStringValue(data.acp?.title);
  const status = toStringValue(data.acp?.status);
  const kind = toStringValue(data.acp?.kind);
  const toolCallID = toStringValue(data.acp?.toolCallId);
  const entries = Array.isArray(data.acp?.entries)
    ? data.acp.entries
        .map((entry) => toStringValue(toRecord(entry)?.content))
        .filter((entryText) => entryText.length > 0)
        .slice(0, 2)
    : [];
  const fragments = [updateType];
  if (title) {
    fragments.push(`title=${title}`);
  }
  if (kind) {
    fragments.push(`kind=${kind}`);
  }
  if (status) {
    fragments.push(`status=${status}`);
  }
  if (toolCallID) {
    fragments.push(`toolCallId=${toolCallID}`);
  }
  if (entries.length > 0) {
    fragments.push(`entries=${entries.join(" | ")}`);
  }
  return fragments.join(" · ");
};

const toStoredRunEventItem = (event: ChatRunEvent): RunEventItem => {
  const payload = toRecord(event.payload) as ChatEventPayload | null;
  let detail = "";
  if (payload) {
    detail = buildRunEventDetail(payload);
    if (!detail || detail === "收到增量更新") {
      detail = toStringValue(payload.text) || toStringValue(payload.error);
    }
  }
  if (!detail) {
    detail = toStringValue(event.update_type) || toStringValue(event.event_type) || "历史运行事件";
  }
  return {
    id: `stored-${event.id}`,
    sessionId: event.session_id,
    type: toStringValue(event.event_type) || "run_update",
    detail,
    time: toStringValue(event.created_at) || nowIso(),
  };
};

const nowIso = (): string => new Date().toISOString();

const toEventTimestampMs = (value: unknown): number => {
  const raw = toStringValue(value);
  if (!raw) {
    return 0;
  }
  const parsed = new Date(raw).getTime();
  if (Number.isNaN(parsed)) {
    return 0;
  }
  return parsed;
};

const getStreamingDelta = (payload: ChatEventPayload): string => {
  const acp = payload.acp;
  if (!acp || typeof acp !== "object") {
    return "";
  }
  const updateType = toStringValue(acp.sessionUpdate);
  if (!updateType) {
    return "";
  }
  const parser = CHAT_UPDATE_PARSERS[updateType];
  if (!parser) {
    return "";
  }
  return parser(acp);
};

const parseInlineMarkdown = (text: string, keyPrefix: string) => {
  const nodes: Array<string | JSX.Element> = [];
  const pattern = /`([^`]+)`|\[([^\]]+)\]\((https?:\/\/[^\s)]+)\)|\*\*([^*]+)\*\*|(\*[^*]+\*)/g;
  let lastIndex = 0;
  let matchIndex = 0;
  let match = pattern.exec(text);
  while (match) {
    const startIndex = match.index;
    if (startIndex > lastIndex) {
      nodes.push(text.slice(lastIndex, startIndex));
    }

    if (match[1]) {
      nodes.push(
        <code
          key={`${keyPrefix}-inline-code-${matchIndex}`}
          className="rounded bg-slate-100 px-1 py-0.5 font-mono text-[0.9em] text-slate-900"
        >
          {match[1]}
        </code>,
      );
    } else if (match[2] && match[3]) {
      nodes.push(
        <a
          key={`${keyPrefix}-link-${matchIndex}`}
          href={match[3]}
          target="_blank"
          rel="noreferrer"
          className="text-sky-700 underline"
        >
          {match[2]}
        </a>,
      );
    } else if (match[4]) {
      nodes.push(
        <strong key={`${keyPrefix}-strong-${matchIndex}`} className="font-semibold">
          {match[4]}
        </strong>,
      );
    } else if (match[5]) {
      nodes.push(
        <em key={`${keyPrefix}-em-${matchIndex}`} className="italic">
          {match[5].slice(1, -1)}
        </em>,
      );
    }

    lastIndex = startIndex + match[0].length;
    matchIndex += 1;
    match = pattern.exec(text);
  }

  if (lastIndex < text.length) {
    nodes.push(text.slice(lastIndex));
  }
  if (nodes.length === 0) {
    nodes.push(text);
  }
  return nodes;
};

const renderBasicMarkdown = (content: string, keyPrefix: string): JSX.Element[] => {
  const lines = content.replace(/\r\n/g, "\n").split("\n");
  const elements: JSX.Element[] = [];
  let index = 0;
  while (index < lines.length) {
    const rawLine = lines[index] ?? "";
    const line = rawLine.trim();

    if (!line) {
      index += 1;
      continue;
    }

    if (line.startsWith("```")) {
      const codeLines: string[] = [];
      index += 1;
      while (index < lines.length && !(lines[index] ?? "").trim().startsWith("```")) {
        codeLines.push(lines[index] ?? "");
        index += 1;
      }
      index += 1;
      elements.push(
        <pre
          key={`${keyPrefix}-code-block-${index}`}
          className="overflow-x-auto rounded-md bg-slate-900 p-2 text-xs text-slate-100"
        >
          <code>{codeLines.join("\n")}</code>
        </pre>,
      );
      continue;
    }

    const headingMatch = line.match(/^(#{1,6})\s+(.+)$/);
    if (headingMatch) {
      const level = headingMatch[1].length;
      const headingText = headingMatch[2];
      const HeadingTag = `h${level}` as keyof JSX.IntrinsicElements;
      elements.push(
        <HeadingTag key={`${keyPrefix}-heading-${index}`} className="font-semibold leading-snug">
          {parseInlineMarkdown(headingText, `${keyPrefix}-heading-${index}`)}
        </HeadingTag>,
      );
      index += 1;
      continue;
    }

    if (/^[-*]\s+/.test(line)) {
      const items: string[] = [];
      while (index < lines.length) {
        const candidate = (lines[index] ?? "").trim();
        const itemMatch = candidate.match(/^[-*]\s+(.+)$/);
        if (!itemMatch) {
          break;
        }
        items.push(itemMatch[1]);
        index += 1;
      }
      elements.push(
        <ul key={`${keyPrefix}-list-${index}`} className="list-disc space-y-1 pl-5">
          {items.map((item, itemIndex) => (
            <li key={`${keyPrefix}-item-${index}-${itemIndex}`}>
              {parseInlineMarkdown(item, `${keyPrefix}-item-${index}-${itemIndex}`)}
            </li>
          ))}
        </ul>,
      );
      continue;
    }

    const paragraphLines = [line];
    index += 1;
    while (index < lines.length) {
      const nextLine = (lines[index] ?? "").trim();
      if (!nextLine || /^#{1,6}\s+/.test(nextLine) || /^[-*]\s+/.test(nextLine) || nextLine.startsWith("```")) {
        break;
      }
      paragraphLines.push(nextLine);
      index += 1;
    }
    const paragraph = paragraphLines.join(" ");
    elements.push(
      <p key={`${keyPrefix}-paragraph-${index}`} className="whitespace-pre-wrap">
        {parseInlineMarkdown(paragraph, `${keyPrefix}-paragraph-${index}`)}
      </p>,
    );
  }

  if (elements.length === 0) {
    elements.push(
      <p key={`${keyPrefix}-empty`} className="whitespace-pre-wrap">
        {content}
      </p>,
    );
  }
  return elements;
};

const ChatView = ({ apiClient, wsClient, projectId }: ChatViewProps) => {
  const [draft, setDraft] = useState("");
  const [filePathsDraft, setFilePathsDraft] = useState("");
  const [selectedFiles, setSelectedFiles] = useState<string[]>([]);
  const [leftPanelTab, setLeftPanelTab] = useState<"tree" | "git">("tree");
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [streamingText, setStreamingText] = useState("");
  const [isStreaming, setIsStreaming] = useState(false);
  const [chatLoading, setChatLoading] = useState(false);
  const [chatCancelling, setChatCancelling] = useState(false);
  const [issueFromFilesLoading, setIssueFromFilesLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [issueNotice, setIssueNotice] = useState<string | null>(null);
  const [chatSessions, setChatSessions] = useState<ChatSessionSummary[]>([]);
  const [chatsLoading, setChatsLoading] = useState(false);
  const [chatsError, setChatsError] = useState<string | null>(null);
  const [runEvents, setRunEvents] = useState<RunEventItem[]>([]);
  const chatRequestIdRef = useRef(0);
  const issueFromFilesRequestIdRef = useRef(0);
  const sessionRefreshRequestIdRef = useRef(0);
  const chatListRequestIdRef = useRef(0);
  const runEventsRequestIdRef = useRef(0);
  const activeRunStartedAtRef = useRef(0);
  const messagesEndRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    chatRequestIdRef.current += 1;
    issueFromFilesRequestIdRef.current += 1;
    setDraft("");
    setFilePathsDraft("");
    setSelectedFiles([]);
    setLeftPanelTab("tree");
    setSessionId(null);
    setMessages([]);
    setStreamingText("");
    setIsStreaming(false);
    setError(null);
    setIssueNotice(null);
    setChatSessions([]);
    setChatsLoading(false);
    setChatsError(null);
    setRunEvents([]);
    setChatLoading(false);
    setChatCancelling(false);
    setIssueFromFilesLoading(false);
    sessionRefreshRequestIdRef.current += 1;
    chatListRequestIdRef.current += 1;
    runEventsRequestIdRef.current += 1;
    activeRunStartedAtRef.current = 0;
  }, [projectId]);

  const canSubmit = chatLoading ? !!sessionId && !chatCancelling : draft.trim().length > 0;
  const filePaths = useMemo(() => parseFilePathsDraft(filePathsDraft), [filePathsDraft]);
  const canCreateIssueFromFiles =
    !!sessionId && filePaths.length > 0 && !issueFromFilesLoading && !chatLoading;

  const sortedMessages = useMemo(
    () =>
      [...messages].sort((a, b) => {
        return new Date(a.time).getTime() - new Date(b.time).getTime();
      }),
    [messages],
  );
  const hasMessages = sortedMessages.length > 0 || isStreaming;

  const listChats = apiClient.listChats;

  const upsertChatSessionSummary = useCallback((session: ChatSessionSummary) => {
    setChatSessions((prev) => {
      const next = prev.filter((item) => item.id !== session.id);
      next.push(session);
      next.sort((a, b) => new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime());
      return next;
    });
  }, []);

  const pushRunEvent = useCallback((session: string, type: string, detail: string) => {
    setRunEvents((prev) => {
      const next: RunEventItem[] = [
        ...prev,
        {
          id: `${Date.now()}-${Math.random().toString(16).slice(2)}`,
          sessionId: session,
          type,
          detail,
          time: nowIso(),
        },
      ];
      if (next.length <= MAX_RUN_EVENTS) {
        return next;
      }
      return next.slice(next.length - MAX_RUN_EVENTS);
    });
  }, []);

  const refreshChatSessions = useCallback(
    async (targetProjectId: string) => {
      const requestId = chatListRequestIdRef.current + 1;
      chatListRequestIdRef.current = requestId;
      setChatsLoading(true);
      setChatsError(null);
      try {
        const response = await listChats(targetProjectId);
        if (chatListRequestIdRef.current !== requestId) {
          return;
        }
        setChatSessions(extractChatSessions(response));
      } catch (listError) {
        if (chatListRequestIdRef.current !== requestId) {
          return;
        }
        setChatsError(getErrorMessage(listError));
      } finally {
        if (chatListRequestIdRef.current === requestId) {
          setChatsLoading(false);
        }
      }
    },
    [listChats],
  );

  const refreshChatRunEvents = useCallback(
    async (targetProjectId: string, targetSessionId: string) => {
      const normalizedSessionID = targetSessionId.trim();
      if (!normalizedSessionID) {
        return;
      }
      const requestId = runEventsRequestIdRef.current + 1;
      runEventsRequestIdRef.current = requestId;
      try {
        const events = await apiClient.listChatRunEvents(targetProjectId, normalizedSessionID);
        if (runEventsRequestIdRef.current !== requestId) {
          return;
        }
        if (targetSessionIdRef.current !== normalizedSessionID) {
          return;
        }
        const mapped = events.map(toStoredRunEventItem);
        setRunEvents((prev) => {
          const otherSessionEvents = prev.filter((event) => event.sessionId !== normalizedSessionID);
          return [...otherSessionEvents, ...mapped];
        });
      } catch {
        // 历史事件加载失败不影响主流程，保留当前 UI 状态。
      }
    },
    [apiClient],
  );

  const refreshSession = useCallback(
    async (targetProjectId: string, targetSessionId: string) => {
      const requestId = sessionRefreshRequestIdRef.current + 1;
      sessionRefreshRequestIdRef.current = requestId;
      try {
        const session = await apiClient.getChat(targetProjectId, targetSessionId);
        if (sessionRefreshRequestIdRef.current !== requestId) {
          return;
        }
        if (targetSessionIdRef.current !== targetSessionId) {
          return;
        }
        setMessages(session.messages);
        upsertChatSessionSummary({
          id: session.id,
          updatedAt: session.updated_at,
          preview: getLatestMessagePreview(session.messages),
        });
      } catch {
        // completed 事件后拉库失败时保持当前界面状态，避免覆盖为错误文案。
      }
    },
    [apiClient, upsertChatSessionSummary],
  );

  const handleStartChat = async () => {
    if (chatLoading) {
      return;
    }
    const message = draft.trim();
    if (!message) {
      return;
    }

    setChatLoading(true);
    setChatCancelling(false);
    setIsStreaming(false);
    setStreamingText("");
    activeRunStartedAtRef.current = 0;
    setError(null);
    setIssueNotice(null);
    const requestId = chatRequestIdRef.current + 1;
    chatRequestIdRef.current = requestId;
    const targetProjectId = projectId;
    const currentSessionId = targetSessionIdRef.current;

    setMessages((prev) => [
      ...prev,
      {
        role: "user",
        content: message,
        time: nowIso(),
      },
    ]);
    setDraft("");

    try {
      const payload = currentSessionId
        ? { message, session_id: currentSessionId }
        : { message };
      const created = await apiClient.createChat(targetProjectId, payload);
      if (chatRequestIdRef.current !== requestId) {
        return;
      }
      targetSessionIdRef.current = created.session_id;
      setSessionId(created.session_id);
      upsertChatSessionSummary({
        id: created.session_id,
        updatedAt: nowIso(),
        preview: message,
      });
    } catch (requestError) {
      if (chatRequestIdRef.current !== requestId) {
        return;
      }
      setChatLoading(false);
      setChatCancelling(false);
      setIsStreaming(false);
      setStreamingText("");
      setError(getErrorMessage(requestError));
    }
  };

  const handleCancelChat = async () => {
    if (!sessionId || !chatLoading || chatCancelling) {
      return;
    }
    const targetProjectId = projectId;
    const targetSessionId = sessionId;
    setChatCancelling(true);
    setError(null);
    try {
      await apiClient.cancelChat(targetProjectId, targetSessionId);
    } catch (requestError) {
      if (targetSessionIdRef.current !== targetSessionId) {
        return;
      }
      setChatCancelling(false);
      setError(getErrorMessage(requestError));
    }
  };

  const handleCreateIssueFromFiles = async () => {
    if (!sessionId || filePaths.length === 0) {
      return;
    }

    setIssueFromFilesLoading(true);
    setError(null);
    setIssueNotice(null);
    const requestId = issueFromFilesRequestIdRef.current + 1;
    issueFromFilesRequestIdRef.current = requestId;
    const targetProjectId = projectId;
    const targetSessionId = sessionId;
    try {
      const createdIssue = await apiClient.createIssueFromFiles(targetProjectId, {
        session_id: targetSessionId,
        file_paths: filePaths,
      });
      if (issueFromFilesRequestIdRef.current !== requestId) {
        return;
      }
      setIssueNotice(`已从文件创建 issue：${createdIssue.id}`);
    } catch (requestError) {
      if (issueFromFilesRequestIdRef.current !== requestId) {
        return;
      }
      setError(getErrorMessage(requestError));
    } finally {
      if (issueFromFilesRequestIdRef.current === requestId) {
        setIssueFromFilesLoading(false);
      }
    }
  };

  const handleToggleFile = (filePath: string, selected: boolean) => {
    const normalizedPath = filePath.trim();
    if (!normalizedPath) {
      return;
    }

    setSelectedFiles((prev) => {
      const exists = prev.includes(normalizedPath);
      let next = prev;
      if (selected && !exists) {
        next = [...prev, normalizedPath];
      }
      if (!selected && exists) {
        next = prev.filter((item) => item !== normalizedPath);
      }
      setFilePathsDraft(next.join(", "));
      return next;
    });
  };

  const handleSwitchSession = async (nextSessionId: string) => {
    const normalizedSessionID = nextSessionId.trim();
    if (!normalizedSessionID || normalizedSessionID === targetSessionIdRef.current || chatLoading) {
      return;
    }
    targetSessionIdRef.current = normalizedSessionID;
    setSessionId(normalizedSessionID);
    setMessages([]);
    setStreamingText("");
    setIsStreaming(false);
    activeRunStartedAtRef.current = 0;
    setChatLoading(false);
    setChatCancelling(false);
    setIssueNotice(null);
    setError(null);
    setRunEvents([]);
    await refreshSession(projectId, normalizedSessionID);
    await refreshChatRunEvents(projectId, normalizedSessionID);
  };

  const targetSessionIdRef = useRef<string | null>(sessionId);
  useEffect(() => {
    targetSessionIdRef.current = sessionId;
  }, [sessionId]);

  useEffect(() => {
    void refreshChatSessions(projectId);
  }, [projectId, refreshChatSessions]);

  useEffect(() => {
    const unsubscribe = wsClient.subscribe<WsEnvelope>("*", (payload) => {
      const envelope = payload as WsEnvelope<ChatEventPayload>;
      if (!CHAT_RUN_EVENT_TYPES.has(envelope.type as ChatEventType)) {
        return;
      }
      if (
        envelope.project_id &&
        envelope.project_id.trim().length > 0 &&
        envelope.project_id !== projectId
      ) {
        return;
      }

      const data = (envelope.data ?? envelope.payload ?? {}) as ChatEventPayload;
      const wsSessionID = toStringValue(data.session_id);
      if (!wsSessionID) {
        return;
      }
      const activeSessionID = targetSessionIdRef.current;
      if (!activeSessionID || activeSessionID !== wsSessionID) {
        return;
      }

      switch (envelope.type as ChatEventType) {
        case "run_started": {
          activeRunStartedAtRef.current = toEventTimestampMs(data.timestamp) || Date.now();
          setChatLoading(true);
          setChatCancelling(false);
          setIsStreaming(true);
          setStreamingText("");
          setError(null);
          pushRunEvent(wsSessionID, "run_started", "运行已开始");
          break;
        }
        case "run_update":
        case "team_leader_thinking":
        case "team_leader_files_changed": {
          const eventTimestampMs = toEventTimestampMs(data.timestamp);
          const runStartedAt = activeRunStartedAtRef.current;
          if (runStartedAt === 0) {
            break;
          }
          if (eventTimestampMs > 0 && eventTimestampMs < runStartedAt) {
            break;
          }
          const delta = getStreamingDelta(data);
          if (delta.length > 0) {
            setStreamingText((prev) => `${prev}${delta}`);
          } else {
            pushRunEvent(wsSessionID, String(envelope.type), buildRunEventDetail(data));
          }
          break;
        }
        case "run_completed": {
          activeRunStartedAtRef.current = 0;
          setChatLoading(false);
          setChatCancelling(false);
          setIsStreaming(false);
          setStreamingText("");
          pushRunEvent(wsSessionID, "run_completed", "运行完成");
          void refreshSession(projectId, wsSessionID);
          void refreshChatRunEvents(projectId, wsSessionID);
          void refreshChatSessions(projectId);
          break;
        }
        case "run_cancelled": {
          activeRunStartedAtRef.current = 0;
          setChatLoading(false);
          setChatCancelling(false);
          setIsStreaming(false);
          setStreamingText("");
          setError("当前请求已取消");
          pushRunEvent(wsSessionID, "run_cancelled", "运行已取消");
          void refreshChatRunEvents(projectId, wsSessionID);
          void refreshChatSessions(projectId);
          break;
        }
        case "run_failed": {
          activeRunStartedAtRef.current = 0;
          setChatLoading(false);
          setChatCancelling(false);
          setIsStreaming(false);
          setStreamingText("");
          const reason = toStringValue(data.error);
          setError(reason || "chat 执行失败");
          pushRunEvent(wsSessionID, "run_failed", reason || "chat 执行失败");
          void refreshChatRunEvents(projectId, wsSessionID);
          void refreshChatSessions(projectId);
          break;
        }
        default:
          break;
      }
    });

    return () => {
      unsubscribe();
    };
  }, [projectId, pushRunEvent, refreshChatRunEvents, refreshChatSessions, refreshSession, wsClient]);

  useEffect(() => {
    const endNode = messagesEndRef.current;
    if (!endNode || typeof endNode.scrollIntoView !== "function") {
      return;
    }
    endNode.scrollIntoView({
      block: "end",
    });
  }, [sortedMessages, streamingText]);

  const submitButtonLabel = chatLoading ? "停止" : sessionId ? "发送" : "发送并创建会话";
  const visibleRunEvents = useMemo(() => {
    if (!sessionId) {
      return runEvents.slice(-20);
    }
    return runEvents.filter((event) => event.sessionId === sessionId).slice(-20);
  }, [runEvents, sessionId]);

  return (
    <section className="grid gap-4 lg:grid-cols-[280px_minmax(0,2fr)_320px]">
      <aside className="hidden rounded-xl border border-slate-200 bg-white p-4 shadow-sm lg:flex lg:min-h-[680px] lg:flex-col">
        <h3 className="text-base font-semibold text-slate-900">仓库视图</h3>
        <p className="mt-1 text-xs text-slate-600">
          在文件树中选择文件后，会自动同步到右侧“文件路径”输入框。
        </p>
        <div className="mt-3 grid grid-cols-2 rounded-md bg-slate-100 p-1 text-xs">
          <button
            type="button"
            className={`rounded px-2 py-1 font-medium ${
              leftPanelTab === "tree"
                ? "bg-white text-slate-900 shadow-sm"
                : "text-slate-600 hover:text-slate-900"
            }`}
            onClick={() => {
              setLeftPanelTab("tree");
            }}
          >
            文件树
          </button>
          <button
            type="button"
            className={`rounded px-2 py-1 font-medium ${
              leftPanelTab === "git"
                ? "bg-white text-slate-900 shadow-sm"
                : "text-slate-600 hover:text-slate-900"
            }`}
            onClick={() => {
              setLeftPanelTab("git");
            }}
          >
            Git Status
          </button>
        </div>
        <div className="mt-3 min-h-0 flex-1 overflow-y-auto">
          {leftPanelTab === "tree" ? (
            <FileTree
              apiClient={apiClient}
              projectId={projectId}
              selectedFiles={selectedFiles}
              onToggleFile={handleToggleFile}
            />
          ) : (
            <GitStatusPanel apiClient={apiClient} projectId={projectId} />
          )}
        </div>
      </aside>

      <div className="min-w-0 rounded-xl border border-slate-200 bg-white p-4 shadow-sm">
        <h2 className="text-xl font-bold">Chat</h2>
        <p className="mt-1 text-sm text-slate-600">
          发送消息先通过 ACK 建立/续用会话，再通过 WS 流式接收状态与增量内容。
        </p>

        <div className="mt-4 h-[30rem] rounded-lg border border-slate-200 bg-slate-50 p-3">
          {hasMessages ? (
            <div className="flex h-full flex-col gap-3 overflow-y-auto pr-1">
              {sortedMessages.map((message, index) => (
                <article
                  key={`${message.time}-${index}`}
                  className={`max-w-[92%] rounded-lg px-3 py-2 text-sm ${
                    roleStyle[message.role]
                  } ${message.role === "user" ? "self-end" : "self-start"}`}
                >
                  <p className="mb-1 text-xs font-semibold opacity-80">
                    {roleLabel[message.role]} · {formatTime(message.time)}
                  </p>
                  <div className="space-y-2">
                    {renderBasicMarkdown(message.content, `${message.time}-${index}`)}
                  </div>
                </article>
              ))}
              {isStreaming ? (
                <article
                  className={`max-w-[92%] self-start rounded-lg px-3 py-2 text-sm ${roleStyle.assistant}`}
                >
                  <p className="mb-1 text-xs font-semibold opacity-80">助手 · 输入中...</p>
                  <div className="space-y-2">
                    {renderBasicMarkdown(
                      streamingText.length > 0 ? streamingText : "...",
                      "streaming-temp",
                    )}
                  </div>
                </article>
              ) : null}
              <div ref={messagesEndRef} />
            </div>
          ) : (
            <p className="text-sm text-slate-500">当前会话暂无消息。</p>
          )}
        </div>

        <div className="mt-4">
          <label htmlFor="chat-message" className="mb-2 block text-sm font-medium">
            新消息
          </label>
          <textarea
            id="chat-message"
            rows={4}
            className="min-h-[7rem] w-full resize-y rounded-lg border border-slate-300 px-3 py-2 text-sm"
            placeholder="请输入要拆分为 issue 的需求..."
            value={draft}
            onChange={(event) => {
              setDraft(event.target.value);
            }}
          />
          <div className="mt-3 flex justify-end">
            <button
              type="button"
              className="w-36 rounded-md bg-slate-900 px-4 py-2 text-center text-sm font-semibold text-white disabled:cursor-not-allowed disabled:bg-slate-400"
              disabled={!canSubmit}
              onClick={() => {
                if (chatLoading) {
                  void handleCancelChat();
                  return;
                }
                void handleStartChat();
              }}
            >
              {submitButtonLabel}
            </button>
          </div>
        </div>
      </div>

      <aside className="rounded-xl border border-slate-200 bg-white p-4 shadow-sm">
        <h3 className="text-lg font-semibold">会话与 Team Leader</h3>
        <p className="mt-2 break-all text-xs text-slate-600">
          Session ID: {sessionId ?? "未创建"}
        </p>

        <div className="mt-3">
          <div className="flex items-center justify-between">
            <h4 className="text-sm font-semibold text-slate-800">会话列表</h4>
            <button
              type="button"
              className="rounded border border-slate-300 px-2 py-1 text-xs text-slate-700 hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50"
              onClick={() => {
                void refreshChatSessions(projectId);
              }}
              disabled={chatsLoading}
            >
              {chatsLoading ? "刷新中..." : "刷新"}
            </button>
          </div>
          {chatSessions.length > 0 ? (
            <div className="mt-2 max-h-44 overflow-y-auto rounded-md border border-slate-200">
              {chatSessions.map((session) => {
                const active = sessionId === session.id;
                return (
                  <button
                    key={session.id}
                    type="button"
                    aria-label={session.id}
                    className={`w-full border-b border-slate-100 px-3 py-2 text-left last:border-b-0 ${
                      active ? "bg-slate-900 text-white" : "bg-white text-slate-700 hover:bg-slate-50"
                    }`}
                    onClick={() => {
                      void handleSwitchSession(session.id);
                    }}
                    disabled={chatLoading}
                  >
                    <p className="truncate text-sm font-medium">{session.id}</p>
                    <p className={`truncate text-xs ${active ? "text-slate-100" : "text-slate-500"}`}>
                      {session.preview || "暂无消息预览"}
                    </p>
                  </button>
                );
              })}
            </div>
          ) : (
            <p className="mt-2 text-xs text-slate-500">暂无会话</p>
          )}
          {chatsError ? (
            <p className="mt-2 rounded border border-rose-200 bg-rose-50 px-2 py-1 text-xs text-rose-700">
              加载会话失败：{chatsError}
            </p>
          ) : null}
        </div>

        <div className="mt-4">
          <h4 className="text-sm font-semibold text-slate-800">运行事件</h4>
          <div className="mt-2 max-h-44 overflow-y-auto rounded-md border border-slate-200 bg-slate-50">
            {visibleRunEvents.length > 0 ? (
              <ul className="divide-y divide-slate-200">
                {visibleRunEvents.map((event) => (
                  <li key={event.id} className="px-3 py-2 text-xs text-slate-700">
                    <p className="font-medium text-slate-800">
                      [{event.type}] {formatTime(event.time)}
                    </p>
                    <p className="mt-1 whitespace-pre-wrap break-words">{event.detail}</p>
                  </li>
                ))}
              </ul>
            ) : (
              <p className="px-3 py-2 text-xs text-slate-500">暂无运行事件</p>
            )}
          </div>
        </div>

        <label className="mt-4 block text-xs text-slate-700" htmlFor="issue-file-paths">
          文件路径（逗号分隔）
          <input
            id="issue-file-paths"
            className="mt-1 w-full rounded-md border border-slate-300 px-2 py-1 text-sm"
            placeholder="例如：cmd/app/main.go, internal/core/task.go"
            value={filePathsDraft}
            onChange={(event) => {
              const nextValue = event.target.value;
              setFilePathsDraft(nextValue);
              setSelectedFiles(parseFilePathsDraft(nextValue));
            }}
          />
        </label>
        <button
          type="button"
          className="mt-2 w-full rounded-md border border-sky-700 px-3 py-2 text-sm font-semibold text-sky-700 disabled:cursor-not-allowed disabled:border-slate-300 disabled:text-slate-400"
          disabled={!canCreateIssueFromFiles}
          onClick={() => {
            void handleCreateIssueFromFiles();
          }}
        >
          {issueFromFilesLoading ? "从文件创建中..." : "从文件创建 issue"}
        </button>

        {issueNotice ? (
          <p className="mt-3 rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">
            {issueNotice}
          </p>
        ) : null}
        {error ? (
          <p className="mt-3 rounded-md border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700">
            {error}
          </p>
        ) : null}
      </aside>
    </section>
  );
};

export default ChatView;
