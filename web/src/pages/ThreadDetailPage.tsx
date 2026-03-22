import { startTransition, useCallback, useEffect, useRef, useState } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import {
  ArrowLeft,
  Loader2,
  X,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { ThreadComposerPanel, type ThreadComposerMentionCandidate } from "@/components/threads/ThreadComposerPanel";
import { ThreadDetailHeader } from "@/components/threads/ThreadDetailHeader";
import { ThreadDetailShell } from "@/components/threads/ThreadDetailShell";
import { ThreadSidebar } from "@/components/threads/ThreadSidebar";
import { ThreadMessageList } from "@/components/threads/ThreadMessageList";
import { InvitePickerDialog } from "@/components/threads/InvitePickerDialog";
import type { ChatActivityView } from "@/components/chat/chatTypes";
import { applyActivityPayload } from "@/components/chat/chatUtils";
import { cn } from "@/lib/utils";
import { useWorkbench } from "@/contexts/WorkbenchContext";
import { formatRelativeTime, getErrorMessage } from "@/lib/v2Workbench";
import type {
  AgentProfile,
  Thread,
  ThreadMessage,
  ThreadMember,
  ThreadProposal,
  ThreadWorkItemLink,
  ThreadAttachment,
  ThreadFileRef,
  MessageFileRef,
  ProposalWorkItemDraft,
  WorkItemPriority,
  WorkItem,
} from "@/types/apiV2";
import type { ThreadAckPayload, ThreadEventPayload } from "@/types/ws";

/* ── helper functions (unchanged) ── */

function deriveWorkItemTitle(thread: Thread): string {
  const title = thread.title.trim();
  return title.length > 80 ? `${title.slice(0, 77)}...` : title;
}

function readTargetAgentID(
  metadata: Record<string, unknown> | undefined,
): string | null {
  const value = metadata?.target_agent_id;
  return typeof value === "string" && value.trim().length > 0
    ? value.trim()
    : null;
}

function readTargetAgentIDs(
  metadata: Record<string, unknown> | undefined,
): string[] {
  const value = metadata?.target_agent_ids;
  if (!Array.isArray(value)) {
    const single = readTargetAgentID(metadata);
    return single ? [single] : [];
  }
  return value
    .filter((item): item is string => typeof item === "string")
    .map((item) => item.trim())
    .filter((item) => item.length > 0);
}

function readAutoRoutedTo(
  metadata: Record<string, unknown> | undefined,
): string[] {
  const value = metadata?.auto_routed_to;
  if (!Array.isArray(value)) return [];
  return value
    .filter((v): v is string => typeof v === "string" && v.trim().length > 0)
    .map((v) => v.trim());
}

function readMetadataType(
  metadata: Record<string, unknown> | undefined,
): string | null {
  const value = metadata?.type;
  return typeof value === "string" && value.trim().length > 0
    ? value.trim()
    : null;
}

function parseMentionTarget(
  message: string,
  activeAgentProfileIDs: string[],
): { targetAgentID: string | null; broadcast: boolean; error: string | null } {
  const trimmed = message.trim();
  const match = trimmed.match(/^@([A-Za-z0-9._:-]+)\s+(.+)$/s);
  if (!match) {
    return { targetAgentID: null, broadcast: false, error: null };
  }

  const targetAgentID = match[1].trim();
  if (targetAgentID === "all") {
    return { targetAgentID: null, broadcast: true, error: null };
  }
  if (!activeAgentProfileIDs.includes(targetAgentID)) {
    return {
      targetAgentID: null,
      broadcast: false,
      error: `未找到活跃 agent：${targetAgentID}`,
    };
  }

  return { targetAgentID, broadcast: false, error: null };
}

function readAgentRoutingMode(
  thread: Thread | null,
): "mention_only" | "broadcast" | "auto" {
  const value = thread?.metadata?.agent_routing_mode;
  if (value === "broadcast") return "broadcast";
  if (value === "auto") return "auto";
  return "mention_only";
}

function readMeetingMode(
  thread: Thread | null,
): "direct" | "concurrent" | "group_chat" {
  const value = thread?.metadata?.meeting_mode;
  if (value === "concurrent") return "concurrent";
  if (value === "group_chat") return "group_chat";
  return "direct";
}

function detectMentionDraft(
  message: string,
  caretPosition: number | null,
): { start: number; end: number; query: string } | null {
  if (caretPosition == null || caretPosition < 0) {
    return null;
  }

  const left = message.slice(0, caretPosition);
  const leftMatch = left.match(/(^|\s)@([A-Za-z0-9._:-]*)$/);
  if (!leftMatch) {
    return null;
  }

  const prefixLength = leftMatch[1]?.length ?? 0;
  const fullMatchLength = leftMatch[0]?.length ?? 0;
  const start = left.length - fullMatchLength + prefixLength;
  const right = message.slice(caretPosition);
  const rightMatch = right.match(/^[A-Za-z0-9._:-]*/);
  const end = caretPosition + (rightMatch?.[0]?.length ?? 0);

  return {
    start,
    end,
    query: message.slice(start + 1, end),
  };
}

function replaceMentionDraft(
  message: string,
  draft: { start: number; end: number },
  profileID: string,
): { nextMessage: string; caretPosition: number } {
  const replacement = `@${profileID} `;
  const nextMessage = `${message.slice(0, draft.start)}${replacement}${message.slice(draft.end)}`;
  return {
    nextMessage,
    caretPosition: draft.start + replacement.length,
  };
}

function splitMessageMentions(
  content: string,
): Array<{ type: "text" | "mention"; value: string; profileID?: string }> {
  const parts: Array<{
    type: "text" | "mention";
    value: string;
    profileID?: string;
  }> = [];
  const mentionPattern = /@([A-Za-z0-9._:-]+)/g;
  let lastIndex = 0;
  let match: RegExpExecArray | null = mentionPattern.exec(content);
  while (match) {
    if (match.index > lastIndex) {
      parts.push({
        type: "text",
        value: content.slice(lastIndex, match.index),
      });
    }
    parts.push({ type: "mention", value: match[0], profileID: match[1] });
    lastIndex = match.index + match[0].length;
    match = mentionPattern.exec(content);
  }
  if (lastIndex < content.length) {
    parts.push({ type: "text", value: content.slice(lastIndex) });
  }
  return parts.length > 0 ? parts : [{ type: "text", value: content }];
}

function detectHashDraft(
  message: string,
  caretPosition: number | null,
): { start: number; end: number; query: string } | null {
  if (caretPosition == null || caretPosition < 0) return null;
  const left = message.slice(0, caretPosition);
  const leftMatch = left.match(/(^|\s)#([^\s#]*)$/);
  if (!leftMatch) return null;
  const prefixLength = leftMatch[1]?.length ?? 0;
  const fullMatchLength = leftMatch[0]?.length ?? 0;
  const start = left.length - fullMatchLength + prefixLength;
  const right = message.slice(caretPosition);
  const rightMatch = right.match(/^[^\s#]*/);
  const end = caretPosition + (rightMatch?.[0]?.length ?? 0);
  return { start, end, query: message.slice(start + 1, end) };
}

function readCommittedMentionTarget(
  message: string,
  activeAgentProfileIDs: string[],
): string | null {
  const trimmed = message.trimStart();
  const match = trimmed.match(/^@([A-Za-z0-9._:-]+)(?:\s|$)/);
  if (!match) {
    return null;
  }
  const profileID = match[1].trim();
  return activeAgentProfileIDs.includes(profileID) ? profileID : null;
}

function agentStatusColor(status: string): string {
  switch (status) {
    case "active":
      return "bg-emerald-500";
    case "booting":
      return "bg-amber-500";
    case "paused":
      return "bg-slate-400";
    case "joining":
      return "bg-blue-400";
    default:
      return "bg-rose-500";
  }
}

function canStartDiscussionWithAgent(status: string): boolean {
  return status === "active";
}

// Invite intent detection: match phrases like "把 XX 拉进来", "invite XX", "加个 XX" etc.
const INVITE_PATTERNS = [
  // Chinese patterns
  /(?:把|让|请|叫|邀请)\s*(.+?)\s*(?:拉进来|加进来|拉入|加入|进来|进群|加到|拉到)/,
  /(?:拉|加|邀请)\s*(?:个|一个|一位)?\s*(.+?)\s*(?:进来|进群|到群里|到线程|吧|$)/,
  /(?:需要|想要|想)\s*(.+?)\s*(?:加入|参与|进来|帮忙)/,
  // English patterns
  /(?:invite|add|bring|pull)\s+(?:in\s+)?(.+?)(?:\s+(?:in|to\s+(?:the\s+)?(?:thread|chat|group))|\s*$)/i,
  /(?:let's?\s+)?(?:get|bring)\s+(.+?)\s+(?:in|here|on\s+board)/i,
];

interface InviteIntentMatch {
  query: string;
  matchedProfiles: AgentProfile[];
}

function detectInviteIntent(
  message: string,
  inviteableProfiles: AgentProfile[],
): InviteIntentMatch | null {
  const trimmed = message.trim();
  if (!trimmed) return null;

  for (const pattern of INVITE_PATTERNS) {
    const match = trimmed.match(pattern);
    if (!match || !match[1]) continue;

    const query = match[1].trim().toLowerCase();
    if (!query) continue;

    // Match query against profile name, id, role, capabilities
    const matched = inviteableProfiles.filter((profile) => {
      const name = (profile.name ?? "").toLowerCase();
      const id = profile.id.toLowerCase();
      const role = (
        typeof profile.role === "string" ? profile.role : ""
      ).toLowerCase();
      const caps = (profile.capabilities ?? []).map((c) => c.toLowerCase());

      const includesNonEmpty = (candidate: string) =>
        candidate.length > 0 &&
        (candidate.includes(query) || query.includes(candidate));

      // Check if query contains or is contained by any field
      return (
        includesNonEmpty(name) ||
        includesNonEmpty(id) ||
        includesNonEmpty(role) ||
        caps.some((c) => includesNonEmpty(c))
      );
    });

    if (matched.length > 0) {
      return { query, matchedProfiles: matched };
    }
  }
  return null;
}

type ThreadMemberWithProfileID = ThreadMember & {
  agent_profile_id: string;
};

type ThreadAgentLiveOutput = {
  thought?: string;
  message?: string;
  updatedAt: string;
};

type ThreadAgentChunkBuffer = {
  thought?: string;
  message?: string;
};

type ProposalDraftForm = {
  temp_id: string;
  project_id: string;
  title: string;
  body: string;
  priority: WorkItemPriority;
  depends_on: string;
  labels: string;
};

type ProposalEditorState = {
  proposalId: number | null;
  title: string;
  summary: string;
  content: string;
  proposedBy: string;
  sourceMessageId: string;
  drafts: ProposalDraftForm[];
};

type ProposalReviewState = {
  reviewedBy: string;
  reviewNote: string;
};

function splitDelimitedValues(raw: string): string[] {
  return raw
    .split(",")
    .map((item) => item.trim())
    .filter((item) => item.length > 0);
}

function normalizeDraftTempID(raw: string, fallback: string): string {
  const normalized = raw
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9._:-]+/g, "-")
    .replace(/^-+|-+$/g, "");
  return normalized.length > 0 ? normalized : fallback;
}

function createEmptyProposalDraft(index = 1): ProposalDraftForm {
  return {
    temp_id: `draft-${index}`,
    project_id: "",
    title: "",
    body: "",
    priority: "medium",
    depends_on: "",
    labels: "",
  };
}

function toProposalDraftForm(
  draft: ProposalWorkItemDraft,
  index: number,
): ProposalDraftForm {
  return {
    temp_id: draft.temp_id || `draft-${index + 1}`,
    project_id:
      typeof draft.project_id === "number" ? String(draft.project_id) : "",
    title: draft.title ?? "",
    body: draft.body ?? "",
    priority: draft.priority ?? "medium",
    depends_on: (draft.depends_on ?? []).join(", "),
    labels: (draft.labels ?? []).join(", "),
  };
}

function createProposalEditorState(ownerId?: string): ProposalEditorState {
  return {
    proposalId: null,
    title: "",
    summary: "",
    content: "",
    proposedBy: ownerId?.trim() || "human",
    sourceMessageId: "",
    drafts: [createEmptyProposalDraft()],
  };
}

function createProposalEditorStateFromProposal(
  proposal: ThreadProposal,
  ownerId?: string,
): ProposalEditorState {
  return {
    proposalId: proposal.id,
    title: proposal.title,
    summary: proposal.summary ?? "",
    content: proposal.content ?? "",
    proposedBy: proposal.proposed_by || ownerId?.trim() || "human",
    sourceMessageId:
      typeof proposal.source_message_id === "number"
        ? String(proposal.source_message_id)
        : "",
    drafts:
      proposal.work_item_drafts && proposal.work_item_drafts.length > 0
        ? proposal.work_item_drafts.map(toProposalDraftForm)
        : [createEmptyProposalDraft()],
  };
}

function buildProposalDraftPayload(
  draft: ProposalDraftForm,
  index: number,
): ProposalWorkItemDraft | null {
  const title = draft.title.trim();
  const body = draft.body.trim();
  const projectRaw = draft.project_id.trim();
  const tempID = normalizeDraftTempID(
    draft.temp_id || draft.title,
    `draft-${index + 1}`,
  );
  if (!title && !body && !projectRaw && !draft.depends_on.trim() && !draft.labels.trim()) {
    return null;
  }

  const projectID =
    projectRaw.length > 0 && Number.isFinite(Number(projectRaw))
      ? Number(projectRaw)
      : undefined;

  return {
    temp_id: tempID,
    project_id: typeof projectID === "number" ? projectID : undefined,
    title,
    body,
    priority: draft.priority ?? "medium",
    depends_on: splitDelimitedValues(draft.depends_on),
    labels: splitDelimitedValues(draft.labels),
  };
}

function createProposalReviewState(
  proposal: ThreadProposal,
  ownerId?: string,
): ProposalReviewState {
  return {
    reviewedBy: proposal.reviewed_by?.trim() || ownerId?.trim() || "human",
    reviewNote: proposal.review_note ?? "",
  };
}

export function ThreadDetailPage() {
  const { t } = useTranslation();
  const { threadId } = useParams<{ threadId: string }>();
  const navigate = useNavigate();
  const { apiClient, wsClient } = useWorkbench();

  const [thread, setThread] = useState<Thread | null>(null);
  const [messages, setMessages] = useState<ThreadMessage[]>([]);
  const [participants, setParticipants] = useState<ThreadMember[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [proposals, setProposals] = useState<ThreadProposal[]>([]);
  const [proposalsLoading, setProposalsLoading] = useState(false);
  const [workItemLinks, setWorkItemLinks] = useState<ThreadWorkItemLink[]>([]);
  const [linkedWorkItems, setLinkedWorkItems] = useState<Record<number, WorkItem>>({});
  const [newMessage, setNewMessage] = useState("");
  const [sending, setSending] = useState(false);
  const [showProposalEditor, setShowProposalEditor] = useState(false);
  const [proposalEditor, setProposalEditor] = useState<ProposalEditorState>(
    () => createProposalEditorState(),
  );
  const [savingProposal, setSavingProposal] = useState(false);
  const [proposalActionLoadingID, setProposalActionLoadingID] = useState<
    number | null
  >(null);
  const [proposalReviewInputs, setProposalReviewInputs] = useState<
    Record<number, ProposalReviewState>
  >({});
  const [showCreateWI, setShowCreateWI] = useState(false);
  const [newWITitle, setNewWITitle] = useState("");
  const [newWIBody, setNewWIBody] = useState("");
  const [showLinkWI, setShowLinkWI] = useState(false);
  const [linkWIId, setLinkWIId] = useState("");
  const [agentSessions, setAgentSessions] = useState<ThreadMember[]>([]);
  const [attachments, setAttachments] = useState<ThreadAttachment[]>([]);
  const [attachmentsLoading, setAttachmentsLoading] = useState(false);
  const [availableProfiles, setAvailableProfiles] = useState<AgentProfile[]>(
    [],
  );
  const [selectedInviteIDs, setSelectedInviteIDs] = useState<Set<string>>(
    new Set(),
  );
  const [selectedDiscussionAgentIDs, setSelectedDiscussionAgentIDs] = useState<
    Set<string>
  >(new Set());
  const [invitingAgent, setInvitingAgent] = useState(false);
  const [removingAgentID, setRemovingAgentID] = useState<number | null>(null);
  const [savingRoutingMode, setSavingRoutingMode] = useState(false);
  const [savingMeetingMode, setSavingMeetingMode] = useState(false);
  const [mentionDraft, setMentionDraft] = useState<{
    start: number;
    end: number;
    query: string;
  } | null>(null);
  const [selectedMentionIndex, setSelectedMentionIndex] = useState(0);
  const [hashDraft, setHashDraft] = useState<{
    start: number;
    end: number;
    query: string;
  } | null>(null);
  const [selectedHashIndex, setSelectedHashIndex] = useState(0);
  const [fileCandidates, setFileCandidates] = useState<ThreadFileRef[]>([]);
  const [selectedFileRefs, setSelectedFileRefs] = useState<MessageFileRef[]>(
    [],
  );
  const [highlightedAgentProfileID, setHighlightedAgentProfileID] = useState<
    string | null
  >(null);
  const [hoveredMentionProfileID, setHoveredMentionProfileID] = useState<
    string | null
  >(null);
  const [thinkingAgentIDs, setThinkingAgentIDs] = useState<Set<string>>(
    new Set(),
  );
  const [invitePickerCandidates, setInvitePickerCandidates] = useState<
    AgentProfile[]
  >([]);
  const [invitePickerSelected, setInvitePickerSelected] = useState<Set<string>>(
    new Set(),
  );
  const [invitePickerBusy, setInvitePickerBusy] = useState(false);
  const [agentActivitiesByID, setAgentActivitiesByID] = useState<
    Record<string, ChatActivityView[]>
  >({});
  const [liveAgentOutputsByID, setLiveAgentOutputsByID] = useState<
    Record<string, ThreadAgentLiveOutput>
  >({});
  const [collapsedAgentActivityPanels, setCollapsedAgentActivityPanels] =
    useState<Record<string, boolean>>({});
  const pendingThreadRequestIdRef = useRef<string | null>(null);
  const syntheticMessageIDRef = useRef(-1);
  const messageInputRef = useRef<HTMLTextAreaElement | null>(null);
  const agentCardRefs = useRef<Record<string, HTMLDivElement | null>>({});
  const messageContainerRef = useRef<HTMLDivElement>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const isNearMessageListBottomRef = useRef(true);
  const pendingAgentChunkBuffersRef = useRef<
    Record<string, ThreadAgentChunkBuffer>
  >({});
  const agentChunkFlushFrameRef = useRef<number | null>(null);

  const id = Number(threadId);
  const agentSessionsWithProfileID = agentSessions.filter(
    (session): session is ThreadMemberWithProfileID =>
      typeof session.agent_profile_id === "string" &&
      session.agent_profile_id.trim().length > 0,
  );
  const joinedAgentProfileIDs = new Set(
    agentSessionsWithProfileID.map((session) => session.agent_profile_id),
  );
  const inviteableProfiles = availableProfiles.filter(
    (profile) => !joinedAgentProfileIDs.has(profile.id),
  );
  const activeAgentProfileIDs = agentSessionsWithProfileID
    .filter(
      (session) => session.status === "active" || session.status === "booting",
    )
    .map((session) => session.agent_profile_id);
  const agentRoutingMode = readAgentRoutingMode(thread);
  const meetingMode = readMeetingMode(thread);
  const profileByID = new Map(
    availableProfiles.map((profile) => [profile.id, profile]),
  );
  const agentSessionByProfileID = new Map(
    agentSessionsWithProfileID.map((session) => [
      session.agent_profile_id,
      session,
    ]),
  );
  const selectedDiscussionAgents = activeAgentProfileIDs.filter((profileID) =>
    selectedDiscussionAgentIDs.has(profileID),
  );
  const committedMentionTargetID = readCommittedMentionTarget(
    newMessage,
    activeAgentProfileIDs,
  );
  const committedMentionProfile = committedMentionTargetID
    ? profileByID.get(committedMentionTargetID)
    : undefined;
  const committedMentionSession = committedMentionTargetID
    ? agentSessionByProfileID.get(committedMentionTargetID)
    : undefined;
  const mentionCandidates: ThreadComposerMentionCandidate[] = (() => {
    if (!mentionDraft) return [];
    const query = mentionDraft.query.trim().toLowerCase();
    const agents = activeAgentProfileIDs
      .map((profileID) => {
        const profile = profileByID.get(profileID);
        const session = agentSessionByProfileID.get(profileID);
        return {
          id: profileID,
          label: profile?.name ? `${profile.name} (${profileID})` : profileID,
          status: session?.status ?? ("active" as string),
        };
      })
      .filter(
        (candidate) =>
          query === "" ||
          candidate.id.toLowerCase().includes(query) ||
          candidate.label.toLowerCase().includes(query),
      );
    // Prepend @all option when there are multiple active agents.
    const allEntry = {
      id: "all",
      label: "All agents (broadcast)",
      status: "active" as string,
    };
    const showAll =
      activeAgentProfileIDs.length > 1 &&
      (query === "" || "all".includes(query));
    return (showAll ? [allEntry, ...agents] : agents).slice(0, 8);
  })();
  const selectedMentionCandidate = mentionCandidates[selectedMentionIndex];
  const orderedWorkItemLinks = [...workItemLinks].sort((a, b) => {
    if (a.is_primary === b.is_primary) {
      return a.id - b.id;
    }
    return a.is_primary ? -1 : 1;
  });
  const orderedProposals = [...proposals].sort((a, b) => b.id - a.id);
  const visibleAgentActivityIDs = [
    ...new Set([
      ...Object.keys(liveAgentOutputsByID),
      ...Object.keys(agentActivitiesByID),
      ...thinkingAgentIDs,
    ]),
  ]
    .filter((profileID) => {
      const live = liveAgentOutputsByID[profileID];
      const hasLive = Boolean(live?.thought?.trim() || live?.message?.trim());
      const hasActivities = (agentActivitiesByID[profileID] ?? []).length > 0;
      return hasLive || hasActivities || thinkingAgentIDs.has(profileID);
    })
    .sort((left, right) => {
      const leftTime =
        liveAgentOutputsByID[left]?.updatedAt ??
        agentActivitiesByID[left]?.at(-1)?.at ??
        "";
      const rightTime =
        liveAgentOutputsByID[right]?.updatedAt ??
        agentActivitiesByID[right]?.at(-1)?.at ??
        "";
      return new Date(rightTime).getTime() - new Date(leftTime).getTime();
    });

  const handleMessageListScroll = useCallback(
    (event: React.UIEvent<HTMLDivElement>) => {
      const element = event.currentTarget;
      isNearMessageListBottomRef.current =
        element.scrollHeight - element.scrollTop - element.clientHeight < 80;
    },
    [],
  );

  /* ── auto-scroll to bottom on new messages ── */
  useEffect(() => {
    if (!isNearMessageListBottomRef.current) {
      return;
    }
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages.length]);

  useEffect(() => {
    setThread(null);
    setMessages([]);
    setParticipants([]);
    setProposals([]);
    setProposalEditor(createProposalEditorState());
    setProposalReviewInputs({});
    setWorkItemLinks([]);
    setLinkedWorkItems({});
    setAgentSessions([]);
    setAttachments([]);
    setAvailableProfiles([]);
    setSelectedInviteIDs(new Set());
    setSelectedDiscussionAgentIDs(new Set());
    setShowProposalEditor(false);
    setShowCreateWI(false);
    setShowLinkWI(false);
    setNewWITitle("");
    setNewWIBody("");
    setLinkWIId("");
    setNewMessage("");
    setSelectedFileRefs([]);
    setError(null);
    setLoading(true);
    isNearMessageListBottomRef.current = true;
  }, [id]);

  useEffect(() => {
    setThinkingAgentIDs(new Set());
    setAgentActivitiesByID({});
    setLiveAgentOutputsByID({});
    setCollapsedAgentActivityPanels({});
    pendingAgentChunkBuffersRef.current = {};
    if (agentChunkFlushFrameRef.current != null) {
      cancelAnimationFrame(agentChunkFlushFrameRef.current);
      agentChunkFlushFrameRef.current = null;
    }
  }, [id]);

  useEffect(() => {
    if (!id || isNaN(id)) return;
    let cancelled = false;

    const load = async () => {
      setLoading(true);
      setProposalsLoading(true);
      setError(null);
      try {
        const [th, msgs, parts, proposalItems, links, agents, profiles, atts] =
          await Promise.all([
            apiClient.getThread(id),
            apiClient.listThreadMessages(id, { limit: 100 }),
            apiClient.listThreadParticipants(id),
            typeof apiClient.listThreadProposals === "function"
              ? apiClient.listThreadProposals(id)
              : Promise.resolve([]),
            apiClient.listWorkItemsByThread(id),
            apiClient.listThreadAgents(id),
            apiClient.listProfiles(),
            apiClient.listThreadAttachments(id),
          ]);
        if (!cancelled) {
          setThread(th);
          setMessages(msgs);
          setParticipants(parts);
          setProposals(proposalItems);
          setProposalEditor((current) =>
            current.proposalId == null
              ? createProposalEditorState(th.owner_id)
              : current,
          );
          setProposalReviewInputs((prev) => {
            const next: Record<number, ProposalReviewState> = {};
            proposalItems.forEach((proposal) => {
              next[proposal.id] =
                prev[proposal.id] ??
                createProposalReviewState(proposal, th.owner_id);
            });
            return next;
          });
          setWorkItemLinks(links);
          setAgentSessions(agents);
          setAvailableProfiles(profiles);
          setAttachments(atts);
          const workItemMap: Record<number, WorkItem> = {};
          const workItemResults = await Promise.allSettled(
            links.map((l) => apiClient.getWorkItem(l.work_item_id)),
          );
          workItemResults.forEach((r, i) => {
            if (r.status === "fulfilled")
              workItemMap[links[i].work_item_id] = r.value;
          });
          if (!cancelled) setLinkedWorkItems(workItemMap);
        }
      } catch (e) {
        if (!cancelled) setError(getErrorMessage(e));
      } finally {
        if (!cancelled) setProposalsLoading(false);
        if (!cancelled) setLoading(false);
      }
    };
    void load();
    return () => {
      cancelled = true;
    };
  }, [apiClient, id]);

  const refreshProposals = useCallback(async () => {
    if (!id || Number.isNaN(id)) return;
    if (typeof apiClient.listThreadProposals !== "function") {
      setProposals([]);
      return;
    }
    setProposalsLoading(true);
    try {
      const items = await apiClient.listThreadProposals(id);
      setProposals(items);
      setProposalReviewInputs((prev) => {
        const next: Record<number, ProposalReviewState> = {};
        items.forEach((proposal) => {
          next[proposal.id] =
            prev[proposal.id] ??
            createProposalReviewState(proposal, thread?.owner_id);
        });
        return next;
      });
    } catch (e) {
      setError(getErrorMessage(e));
    } finally {
      setProposalsLoading(false);
    }
  }, [apiClient, id, thread?.owner_id]);

  useEffect(() => {
    // Remove selections that are no longer inviteable (e.g. agent already joined)
    setSelectedInviteIDs((prev) => {
      const inviteableSet = new Set(inviteableProfiles.map((p) => p.id));
      const next = new Set([...prev].filter((id) => inviteableSet.has(id)));
      return next.size === prev.size ? prev : next;
    });
  }, [inviteableProfiles]);

  useEffect(() => {
    setSelectedDiscussionAgentIDs((prev) => {
      const selectable = new Set(
        agentSessionsWithProfileID
          .filter((session) =>
            canStartDiscussionWithAgent(session.status ?? ""),
          )
          .map((session) => session.agent_profile_id),
      );
      const next = new Set(
        [...prev].filter((profileID) => selectable.has(profileID)),
      );
      return next.size === prev.size ? prev : next;
    });
  }, [agentSessionsWithProfileID]);

  useEffect(() => {
    if (mentionCandidates.length === 0) {
      setSelectedMentionIndex(0);
      return;
    }
    if (selectedMentionIndex >= mentionCandidates.length) {
      setSelectedMentionIndex(0);
    }
  }, [mentionCandidates.length, selectedMentionIndex]);

  useEffect(() => {
    if (!id || isNaN(id)) {
      return;
    }

    const clearAgentActivityState = (profileID: string) => {
      setAgentActivitiesByID((prev) => {
        if (!(profileID in prev)) {
          return prev;
        }
        const next = { ...prev };
        delete next[profileID];
        return next;
      });
      setLiveAgentOutputsByID((prev) => {
        if (!(profileID in prev)) {
          return prev;
        }
        const next = { ...prev };
        delete next[profileID];
        return next;
      });
    };

    const clearLiveAgentOutputField = (
      profileID: string,
      field: keyof ThreadAgentChunkBuffer,
    ) => {
      setLiveAgentOutputsByID((prev) => {
        const current = prev[profileID];
        if (!current || !current[field]) {
          return prev;
        }
        const nextEntry = { ...current };
        delete nextEntry[field];
        if (!nextEntry.thought && !nextEntry.message) {
          const next = { ...prev };
          delete next[profileID];
          return next;
        }
        return {
          ...prev,
          [profileID]: nextEntry,
        };
      });
    };

    const clearLiveAgentOutput = (profileID: string) => {
      setLiveAgentOutputsByID((prev) => {
        if (!(profileID in prev)) {
          return prev;
        }
        const next = { ...prev };
        delete next[profileID];
        return next;
      });
    };

    const flushAgentChunkBuffers = () => {
      if (agentChunkFlushFrameRef.current != null) {
        cancelAnimationFrame(agentChunkFlushFrameRef.current);
        agentChunkFlushFrameRef.current = null;
      }
      const pending = pendingAgentChunkBuffersRef.current;
      const profileIDs = Object.keys(pending);
      if (profileIDs.length === 0) {
        return;
      }
      pendingAgentChunkBuffersRef.current = {};
      const nowISO = new Date().toISOString();
      startTransition(() => {
        setLiveAgentOutputsByID((prev) => {
          const next = { ...prev };
          for (const profileID of profileIDs) {
            const chunk = pending[profileID];
            if (!chunk) {
              continue;
            }
            const current = next[profileID];
            next[profileID] = {
              thought:
                `${current?.thought ?? ""}${chunk.thought ?? ""}` || undefined,
              message:
                `${current?.message ?? ""}${chunk.message ?? ""}` || undefined,
              updatedAt: nowISO,
            };
          }
          return next;
        });
      });
    };

    const scheduleAgentChunkFlush = () => {
      if (agentChunkFlushFrameRef.current != null) {
        return;
      }
      agentChunkFlushFrameRef.current = requestAnimationFrame(() => {
        agentChunkFlushFrameRef.current = null;
        flushAgentChunkBuffers();
      });
    };

    const appendRealtimeMessage = (
      payload: ThreadEventPayload,
      roleFallback: "human" | "agent",
    ) => {
      const content =
        typeof payload.content === "string" && payload.content.trim().length > 0
          ? payload.content
          : typeof payload.message === "string"
            ? payload.message
            : "";
      if (!content.trim()) {
        return;
      }

      const senderID =
        typeof payload.sender_id === "string" &&
        payload.sender_id.trim().length > 0
          ? payload.sender_id.trim()
          : typeof payload.profile_id === "string" &&
              payload.profile_id.trim().length > 0
            ? payload.profile_id.trim()
            : roleFallback;
      const role =
        typeof payload.role === "string" && payload.role.trim().length > 0
          ? payload.role.trim()
          : roleFallback;

      const msgMetadata: Record<string, unknown> = {};
      if (payload.target_agent_id) {
        msgMetadata.target_agent_id = payload.target_agent_id;
      }
      if (
        Array.isArray(payload.target_agent_ids) &&
        payload.target_agent_ids.length > 0
      ) {
        msgMetadata.target_agent_ids = payload.target_agent_ids;
      }
      if (
        Array.isArray(payload.auto_routed_to) &&
        payload.auto_routed_to.length > 0
      ) {
        msgMetadata.auto_routed_to = payload.auto_routed_to;
      }
      if (payload.metadata && typeof payload.metadata === "object") {
        Object.assign(msgMetadata, payload.metadata);
      }

      setMessages((prev) => [
        ...prev,
        {
          id: syntheticMessageIDRef.current--,
          thread_id: id,
          sender_id: senderID,
          role,
          content,
          metadata:
            Object.keys(msgMetadata).length > 0 ? msgMetadata : undefined,
          created_at: new Date().toISOString(),
        },
      ]);
    };

    const refreshAgentSessions = async () => {
      try {
        const sessions = await apiClient.listThreadAgents(id);
        setAgentSessions(sessions);
      } catch {
        // Ignore background refresh failures
      }
    };

    const sendThreadSubscription = (
      type: "subscribe_thread" | "unsubscribe_thread",
    ) => {
      try {
        wsClient.send({
          type,
          data: { thread_id: id },
        });
      } catch {
        // Ignore send errors here
      }
    };

    const unsubscribeThreadMessage = wsClient.subscribe<ThreadEventPayload>(
      "thread.message",
      (payload) => {
        if (payload.thread_id !== id) return;
        appendRealtimeMessage(payload, "human");
        const proposalID = payload.metadata?.proposal_id;
        const metadataType =
          typeof payload.metadata?.type === "string"
            ? payload.metadata.type
            : "";
        if (
          typeof proposalID === "number" ||
          metadataType.startsWith("proposal_")
        ) {
          void refreshProposals();
        }
      },
    );
    const unsubscribeThreadOutput = wsClient.subscribe<ThreadEventPayload>(
      "thread.agent_output",
      (payload) => {
        if (payload.thread_id !== id) return;
        const agentID = payload.profile_id?.trim() || payload.sender_id?.trim();
        const updateType =
          typeof payload.type === "string" ? payload.type.trim() : "";
        const content =
          typeof payload.content === "string" ? payload.content : "";

        if (agentID && updateType) {
          if (
            updateType === "agent_message_chunk" ||
            updateType === "agent_thought_chunk"
          ) {
            const field =
              updateType === "agent_message_chunk" ? "message" : "thought";
            const existing = pendingAgentChunkBuffersRef.current[agentID] ?? {};
            pendingAgentChunkBuffersRef.current[agentID] = {
              ...existing,
              [field]: `${existing[field] ?? ""}${content}`,
            };
            scheduleAgentChunkFlush();
            return;
          }

          flushAgentChunkBuffers();
          if (updateType === "agent_message") {
            clearLiveAgentOutputField(agentID, "message");
          }
          if (updateType === "agent_thought") {
            clearLiveAgentOutputField(agentID, "thought");
          }
          startTransition(() => {
            setAgentActivitiesByID((prev) => ({
              ...prev,
              [agentID]: applyActivityPayload(
                prev[agentID] ?? [],
                `thread-${id}-${agentID}`,
                {
                  ...payload,
                  session_id: `thread-${id}-${agentID}`,
                },
                new Date().toISOString(),
                t,
              ),
            }));
          });
          return;
        }

        if (agentID) {
          flushAgentChunkBuffers();
          setThinkingAgentIDs((prev) => {
            if (!prev.has(agentID)) return prev;
            const next = new Set(prev);
            next.delete(agentID);
            return next;
          });
          clearLiveAgentOutput(agentID);
          setCollapsedAgentActivityPanels((prev) => ({
            ...prev,
            [agentID]: true,
          }));
        }
        appendRealtimeMessage(payload, "agent");
      },
    );
    const unsubscribeThreadAck = wsClient.subscribe<ThreadAckPayload>(
      "thread.ack",
      (payload) => {
        if (payload.thread_id !== id) return;
        if (
          pendingThreadRequestIdRef.current &&
          payload.request_id &&
          payload.request_id !== pendingThreadRequestIdRef.current
        )
          return;
        pendingThreadRequestIdRef.current = null;
        setSending(false);
        clearMentionComposerState();
      },
    );
    const unsubscribeThreadError = wsClient.subscribe<{
      request_id?: string;
      error?: string;
    }>("thread.error", (payload) => {
      if (
        pendingThreadRequestIdRef.current &&
        payload.request_id &&
        payload.request_id !== pendingThreadRequestIdRef.current
      )
        return;
      pendingThreadRequestIdRef.current = null;
      setSending(false);
      clearMentionComposerState();
      setError(
        payload.error?.trim() ||
          t("threads.sendFailed", "Thread message failed to send"),
      );
    });
    const unsubscribeThreadAgentEvent = wsClient.subscribe<ThreadEventPayload>(
      "thread.agent_joined",
      (payload) => {
        if (payload.thread_id === id) void refreshAgentSessions();
      },
    );
    const unsubscribeThreadAgentLeft = wsClient.subscribe<ThreadEventPayload>(
      "thread.agent_left",
      (payload) => {
        if (payload.thread_id === id) void refreshAgentSessions();
      },
    );
    const unsubscribeThreadAgentBooted = wsClient.subscribe<ThreadEventPayload>(
      "thread.agent_booted",
      (payload) => {
        if (payload.thread_id === id) void refreshAgentSessions();
      },
    );
    const unsubscribeThreadAgentFailed = wsClient.subscribe<ThreadEventPayload>(
      "thread.agent_failed",
      (payload) => {
        if (payload.thread_id !== id) return;
        const failedID = payload.profile_id?.trim();
        if (failedID) {
          flushAgentChunkBuffers();
          setThinkingAgentIDs((prev) => {
            if (!prev.has(failedID)) return prev;
            const next = new Set(prev);
            next.delete(failedID);
            return next;
          });
          clearLiveAgentOutput(failedID);
          setCollapsedAgentActivityPanels((prev) => ({
            ...prev,
            [failedID]: true,
          }));
        }
        setError(
          payload.error?.trim() ||
            t("threads.agentFailed", "An agent in this thread failed."),
        );
        void refreshAgentSessions();
      },
    );
    const unsubscribeThreadAgentThinking =
      wsClient.subscribe<ThreadEventPayload>(
        "thread.agent_thinking",
        (payload) => {
          if (payload.thread_id !== id) return;
          const thinkingID = payload.profile_id?.trim();
          if (thinkingID) {
            pendingAgentChunkBuffersRef.current[thinkingID] = {};
            clearAgentActivityState(thinkingID);
            setCollapsedAgentActivityPanels((prev) => ({
              ...prev,
              [thinkingID]: false,
            }));
            setThinkingAgentIDs((prev) => {
              if (prev.has(thinkingID)) return prev;
              const next = new Set(prev);
              next.add(thinkingID);
              return next;
            });
          }
        },
      );
    const unsubscribeStatus = wsClient.onStatusChange((status) => {
      if (status === "open") sendThreadSubscription("subscribe_thread");
    });

    if (wsClient.getStatus() === "open") {
      sendThreadSubscription("subscribe_thread");
    }

    return () => {
      unsubscribeThreadMessage();
      unsubscribeThreadOutput();
      unsubscribeThreadAck();
      unsubscribeThreadError();
      unsubscribeThreadAgentEvent();
      unsubscribeThreadAgentLeft();
      unsubscribeThreadAgentBooted();
      unsubscribeThreadAgentFailed();
      unsubscribeThreadAgentThinking();
      unsubscribeStatus();
      pendingThreadRequestIdRef.current = null;
      flushAgentChunkBuffers();
      pendingAgentChunkBuffersRef.current = {};
      if (agentChunkFlushFrameRef.current != null) {
        cancelAnimationFrame(agentChunkFlushFrameRef.current);
        agentChunkFlushFrameRef.current = null;
      }
      setThinkingAgentIDs(new Set());
      if (wsClient.getStatus() === "open") {
        sendThreadSubscription("unsubscribe_thread");
      }
    };
  }, [apiClient, id, refreshProposals, t, wsClient]);

  const toggleAgentActivityPanel = (profileID: string) => {
    setCollapsedAgentActivityPanels((prev) => ({
      ...prev,
      [profileID]: !prev[profileID],
    }));
  };

  /* ── handlers (unchanged) ── */

  const updateMentionDraft = (value: string, caretPosition: number | null) => {
    const nextMention = detectMentionDraft(value, caretPosition);
    setMentionDraft(nextMention);
    setSelectedMentionIndex(0);

    const nextHash = nextMention ? null : detectHashDraft(value, caretPosition);
    setHashDraft(nextHash);
    setSelectedHashIndex(0);
    if (nextHash && id) {
      apiClient
        .searchThreadFiles(id, nextHash.query || undefined, "all", 8)
        .then(setFileCandidates)
        .catch(() => setFileCandidates([]));
    } else if (!nextHash) {
      setFileCandidates([]);
    }
  };

  const handleMessageInputChange = (
    value: string,
    caretPosition: number | null,
  ) => {
    setNewMessage(value);
    updateMentionDraft(value, caretPosition);
  };

  const applyMentionCandidate = (profileID: string) => {
    if (!mentionDraft) return;
    const { nextMessage, caretPosition } = replaceMentionDraft(
      newMessage,
      mentionDraft,
      profileID,
    );
    setNewMessage(nextMessage);
    setMentionDraft(null);
    setSelectedMentionIndex(0);
    requestAnimationFrame(() => {
      messageInputRef.current?.focus();
      messageInputRef.current?.setSelectionRange(caretPosition, caretPosition);
    });
  };

  const focusAgentProfile = (profileID: string) => {
    setHighlightedAgentProfileID(profileID);
    const node = agentCardRefs.current[profileID];
    if (node) {
      node.scrollIntoView({ behavior: "smooth", block: "nearest" });
    }
  };

  const applyHashCandidate = (file: ThreadFileRef) => {
    if (!hashDraft) return;
    // Remove the #query text from input (don't insert #filename — show chip instead).
    const nextMessage =
      newMessage.slice(0, hashDraft.start) + newMessage.slice(hashDraft.end);
    const caretPosition = hashDraft.start;
    setNewMessage(nextMessage);
    setHashDraft(null);
    setSelectedHashIndex(0);
    setFileCandidates([]);
    setSelectedFileRefs((prev) => {
      if (prev.some((r) => r.path === file.path)) return prev;
      return [
        ...prev,
        { source: file.source, name: file.name, path: file.path },
      ];
    });
    requestAnimationFrame(() => {
      messageInputRef.current?.focus();
      messageInputRef.current?.setSelectionRange(caretPosition, caretPosition);
    });
  };

  const removeFileRef = (path: string) => {
    setSelectedFileRefs((prev) => prev.filter((r) => r.path !== path));
  };

  const clearMentionComposerState = () => {
    setNewMessage("");
    setMentionDraft(null);
    setSelectedMentionIndex(0);
    setHashDraft(null);
    setSelectedHashIndex(0);
    setFileCandidates([]);
    setSelectedFileRefs([]);
  };

  const toggleDiscussionAgentSelection = (profileID: string) => {
    setSelectedDiscussionAgentIDs((prev) => {
      const next = new Set(prev);
      if (next.has(profileID)) {
        next.delete(profileID);
      } else {
        next.add(profileID);
      }
      return next;
    });
  };

  const startDiscussionWithSelectedAgents = () => {
    if (selectedDiscussionAgentIDs.size === 0) return;
    requestAnimationFrame(() => {
      messageInputRef.current?.focus();
    });
  };

  const handleSend = async () => {
    if (!newMessage.trim() || !id) return;

    // Detect invite intent before sending as a regular message.
    const inviteIntent = detectInviteIntent(newMessage, inviteableProfiles);
    if (inviteIntent) {
      if (inviteIntent.matchedProfiles.length === 1) {
        // Single match → auto-invite directly.
        const profile = inviteIntent.matchedProfiles[0];
        setNewMessage("");
        setInvitingAgent(true);
        setError(null);
        try {
          await apiClient.inviteThreadAgent(id, {
            agent_profile_id: profile.id,
          });
          // Agent is now booting — WS events (agent_booted/agent_joined/agent_failed)
          // will drive the UI updates via refreshAgentSessions().
          setMessages((prev) => [
            ...prev,
            {
              id: syntheticMessageIDRef.current--,
              thread_id: id,
              sender_id: "system",
              role: "system",
              content: `已邀请 ${profile.name ?? profile.id} 加入对话，正在初始化...`,
              created_at: new Date().toISOString(),
            },
          ]);
        } catch (e) {
          setError(getErrorMessage(e));
        } finally {
          setInvitingAgent(false);
        }
        return;
      }
      // Multiple matches → show picker dialog.
      setInvitePickerCandidates(inviteIntent.matchedProfiles);
      setInvitePickerSelected(new Set());
      return;
    }

    const mention = parseMentionTarget(newMessage, activeAgentProfileIDs);
    if (mention.error) {
      setError(mention.error);
      return;
    }
    const discussionTargets =
      mention.targetAgentID || mention.broadcast
        ? []
        : activeAgentProfileIDs.filter((profileID) =>
            selectedDiscussionAgentIDs.has(profileID),
          );
    setSending(true);
    setError(null);
    try {
      const requestId = `thread-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
      pendingThreadRequestIdRef.current = requestId;
      const sendMetadata: Record<string, unknown> = {};
      if (selectedFileRefs.length > 0) {
        sendMetadata.file_refs = selectedFileRefs;
      }
      if (mention.broadcast) {
        sendMetadata.broadcast = true;
      }
      wsClient.send({
        type: "thread.send",
        data: {
          request_id: requestId,
          thread_id: id,
          message: mention.broadcast
            ? newMessage.trim().replace(/^@all\s+/i, "")
            : newMessage.trim(),
          sender_id: thread?.owner_id || "human",
          target_agent_ids:
            mention.targetAgentID == null &&
            !mention.broadcast &&
            discussionTargets.length > 1
              ? discussionTargets
              : undefined,
          target_agent_id:
            mention.targetAgentID ??
            (discussionTargets.length === 1 ? discussionTargets[0] : undefined),
          metadata:
            Object.keys(sendMetadata).length > 0 ? sendMetadata : undefined,
        },
      });
      if (discussionTargets.length > 0) {
        setSelectedDiscussionAgentIDs(new Set());
      }
    } catch (e) {
      pendingThreadRequestIdRef.current = null;
      setSending(false);
      setError(getErrorMessage(e));
    } finally {
      if (!pendingThreadRequestIdRef.current) {
        setSending(false);
      }
    }
  };

  const handleInvitePickerConfirm = async () => {
    if (!id || invitePickerSelected.size === 0) return;
    setInvitePickerBusy(true);
    setError(null);
    const ids = [...invitePickerSelected];
    try {
      for (const profileID of ids) {
        await apiClient.inviteThreadAgent(id, { agent_profile_id: profileID });
      }
      // Agents are now booting — WS events will drive UI updates.
      const names = ids.map((pid) => {
        const p = invitePickerCandidates.find((c) => c.id === pid);
        return p?.name ?? pid;
      });
      setMessages((prev) => [
        ...prev,
        {
          id: syntheticMessageIDRef.current--,
          thread_id: id,
          sender_id: "system",
          role: "system",
          content: `已邀请 ${names.join(", ")} 加入对话，正在初始化...`,
          created_at: new Date().toISOString(),
        },
      ]);
      setNewMessage("");
    } catch (e) {
      setError(getErrorMessage(e));
    } finally {
      setInvitePickerBusy(false);
      setInvitePickerCandidates([]);
      setInvitePickerSelected(new Set());
    }
  };

  const handleOpenCreateProposal = () => {
    setError(null);
    setProposalEditor(createProposalEditorState(thread?.owner_id));
    setShowProposalEditor(true);
  };

  const handleOpenEditProposal = (proposal: ThreadProposal) => {
    setError(null);
    setProposalEditor(
      createProposalEditorStateFromProposal(proposal, thread?.owner_id),
    );
    setShowProposalEditor(true);
  };

  const handleProposalEditorFieldChange = (
    field: Exclude<keyof ProposalEditorState, "drafts">,
    value: string | number | null,
  ) => {
    setProposalEditor((prev) => ({
      ...prev,
      [field]: value == null ? "" : String(value),
    }));
  };

  const handleProposalDraftChange = (
    index: number,
    field: keyof ProposalDraftForm,
    value: string,
  ) => {
    setProposalEditor((prev) => ({
      ...prev,
      drafts: prev.drafts.map((draft, draftIndex) =>
        draftIndex === index ? { ...draft, [field]: value } : draft,
      ),
    }));
  };

  const handleAddProposalDraft = () => {
    setProposalEditor((prev) => ({
      ...prev,
      drafts: [...prev.drafts, createEmptyProposalDraft(prev.drafts.length + 1)],
    }));
  };

  const handleRemoveProposalDraft = (index: number) => {
    setProposalEditor((prev) => {
      if (prev.drafts.length === 1) {
        return { ...prev, drafts: [createEmptyProposalDraft()] };
      }
      return {
        ...prev,
        drafts: prev.drafts.filter((_, draftIndex) => draftIndex !== index),
      };
    });
  };

  const handleSaveProposal = async () => {
    if (!id || !proposalEditor.title.trim()) return;
    const sourceMessageID = proposalEditor.sourceMessageId.trim();
    if (sourceMessageID.length > 0 && !Number.isInteger(Number(sourceMessageID))) {
      setError("Source message ID 必须是数字。");
      return;
    }
    const invalidProjectDraft = proposalEditor.drafts.find(
      (draft) =>
        draft.project_id.trim().length > 0 &&
        !Number.isInteger(Number(draft.project_id.trim())),
    );
    if (invalidProjectDraft) {
      setError("Draft project_id 必须是数字。");
      return;
    }
    setSavingProposal(true);
    setError(null);
    const drafts = proposalEditor.drafts
      .map((draft, index) => buildProposalDraftPayload(draft, index))
      .filter((draft): draft is ProposalWorkItemDraft => draft !== null);
    try {
      if (proposalEditor.proposalId == null) {
        await apiClient.createThreadProposal(id, {
          title: proposalEditor.title.trim(),
          summary: proposalEditor.summary.trim(),
          content: proposalEditor.content.trim(),
          proposed_by: proposalEditor.proposedBy.trim() || thread?.owner_id || "human",
          source_message_id:
            sourceMessageID.length > 0 ? Number(sourceMessageID) : undefined,
          work_item_drafts: drafts,
        });
      } else {
        await apiClient.updateProposal(proposalEditor.proposalId, {
          title: proposalEditor.title.trim(),
          summary: proposalEditor.summary.trim(),
          content: proposalEditor.content.trim(),
          proposed_by:
            proposalEditor.proposedBy.trim() || thread?.owner_id || "human",
          work_item_drafts: drafts,
          source_message_id:
            sourceMessageID.length > 0 ? Number(sourceMessageID) : undefined,
        });
      }
      await refreshProposals();
      setProposalEditor(createProposalEditorState(thread?.owner_id));
      setShowProposalEditor(false);
    } catch (e) {
      setError(getErrorMessage(e));
    } finally {
      setSavingProposal(false);
    }
  };

  const handleProposalReviewInputChange = (
    proposalId: number,
    field: keyof ProposalReviewState,
    value: string,
  ) => {
    setProposalReviewInputs((prev) => ({
      ...prev,
      [proposalId]: {
        ...(prev[proposalId] ?? {
          reviewedBy: thread?.owner_id || "human",
          reviewNote: "",
        }),
        [field]: value,
      },
    }));
  };

  const runProposalAction = async (
    proposalId: number,
    action: "submit" | "approve" | "reject" | "revise",
  ) => {
    setProposalActionLoadingID(proposalId);
    setError(null);
    try {
      const reviewInput = proposalReviewInputs[proposalId] ?? {
        reviewedBy: thread?.owner_id || "human",
        reviewNote: "",
      };
      if (action === "submit") {
        await apiClient.submitProposal(proposalId);
      } else if (action === "approve") {
        await apiClient.approveProposal(proposalId, {
          reviewed_by: reviewInput.reviewedBy.trim() || thread?.owner_id || "human",
          review_note: reviewInput.reviewNote.trim(),
        });
      } else if (action === "reject") {
        await apiClient.rejectProposal(proposalId, {
          reviewed_by: reviewInput.reviewedBy.trim() || thread?.owner_id || "human",
          review_note: reviewInput.reviewNote.trim(),
        });
      } else {
        await apiClient.reviseProposal(proposalId, {
          reviewed_by: reviewInput.reviewedBy.trim() || thread?.owner_id || "human",
          review_note: reviewInput.reviewNote.trim(),
        });
      }
      await refreshProposals();
    } catch (e) {
      setError(getErrorMessage(e));
    } finally {
      setProposalActionLoadingID(null);
    }
  };

  const handleOpenCreateWorkItem = () => {
    if (!thread) return;
    setError(null);
    setShowCreateWI((prev) => {
      const next = !prev;
      if (next) {
        setNewWITitle(deriveWorkItemTitle(thread));
        setNewWIBody("");
      }
      return next;
    });
  };

  const handleCreateWorkItem = async () => {
    if (!newWITitle.trim() || !id) return;
    setError(null);
    try {
      const trimmedBody = newWIBody.trim();
      const workItem = await apiClient.createWorkItemFromThread(id, {
        title: newWITitle.trim(),
        body: trimmedBody !== "" ? trimmedBody : undefined,
      });
      const links = await apiClient.listWorkItemsByThread(id);
      setWorkItemLinks(links);
      setLinkedWorkItems((prev) => ({ ...prev, [workItem.id]: workItem }));
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
      await apiClient.createThreadWorkItemLink(id, {
        work_item_id: wiId,
        relation_type: "related",
      });
      const links = await apiClient.listWorkItemsByThread(id);
      setWorkItemLinks(links);
      try {
        const workItem = await apiClient.getWorkItem(wiId);
        setLinkedWorkItems((prev) => ({ ...prev, [wiId]: workItem }));
      } catch {
        /* ignore */
      }
      setLinkWIId("");
      setShowLinkWI(false);
    } catch (e) {
      setError(getErrorMessage(e));
    }
  };

  const toggleInviteSelection = (profileID: string) => {
    setSelectedInviteIDs((prev) => {
      const next = new Set(prev);
      if (next.has(profileID)) {
        next.delete(profileID);
      } else {
        next.add(profileID);
      }
      return next;
    });
  };

  const handleInviteAgent = async () => {
    if (!id || selectedInviteIDs.size === 0) return;
    setInvitingAgent(true);
    setError(null);
    const ids = [...selectedInviteIDs];
    try {
      for (const profileID of ids) {
        await apiClient.inviteThreadAgent(id, { agent_profile_id: profileID });
      }
      const sessions = await apiClient.listThreadAgents(id);
      setAgentSessions(sessions);
      setSelectedInviteIDs(new Set());
    } catch (e) {
      setError(getErrorMessage(e));
      // Refresh sessions in case some succeeded
      try {
        const sessions = await apiClient.listThreadAgents(id);
        setAgentSessions(sessions);
      } catch {
        /* ignore */
      }
    } finally {
      setInvitingAgent(false);
    }
  };

  const handleRemoveAgent = async (agentSessionID: number) => {
    if (!id) return;
    setRemovingAgentID(agentSessionID);
    setError(null);
    try {
      await apiClient.removeThreadAgent(id, agentSessionID);
      const sessions = await apiClient.listThreadAgents(id);
      setAgentSessions(sessions);
    } catch (e) {
      setError(getErrorMessage(e));
    } finally {
      setRemovingAgentID(null);
    }
  };

  const handleUploadAttachment = async (file: File) => {
    if (!id) return;
    setAttachmentsLoading(true);
    try {
      const att = await apiClient.uploadThreadAttachment(id, file);
      setAttachments((prev) => [att, ...prev]);
    } catch (e) {
      setError(getErrorMessage(e));
    } finally {
      setAttachmentsLoading(false);
    }
  };

  const handleDeleteAttachment = async (attachmentId: number) => {
    if (!id) return;
    setAttachmentsLoading(true);
    try {
      await apiClient.deleteThreadAttachment(id, attachmentId);
      setAttachments((prev) => prev.filter((a) => a.id !== attachmentId));
    } catch (e) {
      setError(getErrorMessage(e));
    } finally {
      setAttachmentsLoading(false);
    }
  };

  const handleSetRoutingMode = async (
    nextMode: "mention_only" | "broadcast" | "auto",
  ) => {
    if (!thread || !id || nextMode === agentRoutingMode) return;
    setSavingRoutingMode(true);
    setError(null);
    try {
      const updated = await apiClient.updateThread(id, {
        metadata: {
          ...(thread.metadata ?? {}),
          agent_routing_mode: nextMode,
        },
      });
      setThread(updated);
    } catch (e) {
      setError(getErrorMessage(e));
    } finally {
      setSavingRoutingMode(false);
    }
  };

  const handleSetMeetingMode = async (
    nextMode: "direct" | "concurrent" | "group_chat",
  ) => {
    if (!thread || !id || nextMode === meetingMode) return;
    setSavingMeetingMode(true);
    setError(null);
    try {
      const updated = await apiClient.updateThread(id, {
        metadata: {
          ...(thread.metadata ?? {}),
          meeting_mode: nextMode,
        },
      });
      setThread(updated);
    } catch (e) {
      setError(getErrorMessage(e));
    } finally {
      setSavingMeetingMode(false);
    }
  };

  /* ── render helpers ── */

  const renderMessageContent = (msg: ThreadMessage) => {
    return splitMessageMentions(msg.content).map((part, index) => {
      if (part.type === "text") {
        return <span key={`${msg.id}-text-${index}`}>{part.value}</span>;
      }
      const profileID = part.profileID ?? "";
      const session = agentSessionByProfileID.get(profileID);
      const profile = profileByID.get(profileID);
      return (
        <span
          key={`${msg.id}-mention-${index}`}
          className="relative mx-0.5 inline-flex align-baseline"
        >
          <button
            type="button"
            className="inline-flex items-center rounded-md bg-blue-100 px-1.5 py-0.5 text-xs font-semibold text-blue-800 transition-colors hover:bg-blue-200"
            onClick={() => focusAgentProfile(profileID)}
            onMouseEnter={() => setHoveredMentionProfileID(profileID)}
            onMouseLeave={() =>
              setHoveredMentionProfileID((c) => (c === profileID ? null : c))
            }
          >
            {part.value}
          </button>
          {hoveredMentionProfileID === profileID ? (
            <span
              data-testid={`mention-hover-card-${profileID}`}
              className="pointer-events-none absolute bottom-full left-0 z-30 mb-2 w-56 rounded-lg border border-slate-200 bg-white p-3 text-left shadow-xl"
            >
              <span className="block text-sm font-semibold text-slate-900">
                {profile?.name ?? profileID}
              </span>
              <span className="mt-0.5 block text-xs text-slate-500">
                @{profileID}
              </span>
              <span className="mt-2 inline-flex items-center gap-1.5 rounded-full bg-slate-100 px-2 py-0.5 text-[10px] font-medium text-slate-700">
                <span
                  className={cn(
                    "h-1.5 w-1.5 rounded-full",
                    agentStatusColor(session?.status ?? "unknown"),
                  )}
                />
                {session?.status ?? "not_joined"}
              </span>
              <span className="mt-2 block text-xs text-slate-500">
                {t("threads.turns", "Turns")}: {session?.turn_count ?? 0} |{" "}
                {(
                  ((session?.total_input_tokens ?? 0) +
                    (session?.total_output_tokens ?? 0)) /
                  1000
                ).toFixed(1)}
                k {t("threads.tokens", "tokens")}
              </span>
            </span>
          ) : null}
        </span>
      );
    });
  };

  /* ── loading / not-found states ── */

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="flex flex-col items-center gap-3">
          <Loader2 className="h-8 w-8 animate-spin text-blue-500" />
          <span className="text-sm text-muted-foreground">
            {t("common.loading", "Loading...")}
          </span>
        </div>
      </div>
    );
  }

  if (!thread) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-4">
        <div className="rounded-xl border border-destructive/20 bg-destructive/5 px-6 py-4 text-center">
          <p className="text-sm text-destructive">
            {error || t("threads.notFound", "Thread not found")}
          </p>
        </div>
        <Button variant="ghost" size="sm" onClick={() => navigate("/threads")}>
          <ArrowLeft className="mr-1.5 h-4 w-4" />
          {t("threads.backToList", "Back to Threads")}
        </Button>
      </div>
    );
  }

  /* ── main layout ── */

  return (
    <ThreadDetailShell
      messageContainerRef={messageContainerRef}
      messagesEndRef={messagesEndRef}
      onMessageListScroll={handleMessageListScroll}
      header={
        <ThreadDetailHeader
          thread={thread}
          participantsCount={participants.length}
          agentSessionsCount={agentSessions.length}
          agentRoutingMode={agentRoutingMode}
          meetingMode={meetingMode}
          savingRoutingMode={savingRoutingMode}
          savingMeetingMode={savingMeetingMode}
          formatRelativeTime={formatRelativeTime}
          onBack={() => navigate("/threads")}
          onSetRoutingMode={(mode) => {
            void handleSetRoutingMode(mode);
          }}
          onSetMeetingMode={(mode) => {
            void handleSetMeetingMode(mode);
          }}
        />
      }
      errorBanner={
        error ? (
          <div className="flex items-center justify-between border-b border-destructive/20 bg-destructive/5 px-5 py-2">
            <span className="text-xs text-destructive">{error}</span>
            <button
              type="button"
              className="text-destructive/60 hover:text-destructive"
              onClick={() => setError(null)}
            >
              <X className="h-3.5 w-3.5" />
            </button>
          </div>
        ) : null
      }
      invitePickerDialog={
        <InvitePickerDialog
          candidates={invitePickerCandidates}
          selectedIDs={invitePickerSelected}
          busy={invitePickerBusy}
          onToggle={(profileID) => {
            setInvitePickerSelected((prev) => {
              const next = new Set(prev);
              if (next.has(profileID)) next.delete(profileID);
              else next.add(profileID);
              return next;
            });
          }}
          onClose={() => {
            setInvitePickerCandidates([]);
            setInvitePickerSelected(new Set());
          }}
          onConfirm={handleInvitePickerConfirm}
        />
      }
      messageList={
        <ThreadMessageList
          messages={messages}
          profileByID={profileByID}
          thinkingAgentIDs={thinkingAgentIDs}
          visibleAgentActivityIDs={visibleAgentActivityIDs}
          agentActivitiesByID={agentActivitiesByID}
          liveAgentOutputsByID={liveAgentOutputsByID}
          collapsedAgentActivityPanels={collapsedAgentActivityPanels}
          sending={sending}
          renderMessageContent={renderMessageContent}
          onToggleAgentActivityPanel={toggleAgentActivityPanel}
          focusAgentProfile={focusAgentProfile}
          readTargetAgentID={readTargetAgentID}
          readTargetAgentIDs={readTargetAgentIDs}
          readAutoRoutedTo={readAutoRoutedTo}
          readMetadataType={readMetadataType}
          formatRelativeTime={formatRelativeTime}
        />
      }
      composer={
        <ThreadComposerPanel
          threadStatus={thread.status}
          agentRoutingMode={agentRoutingMode}
          meetingMode={meetingMode}
          sending={sending}
          newMessage={newMessage}
          messageInputRef={messageInputRef}
          selectedDiscussionAgents={selectedDiscussionAgents}
          profileByID={profileByID}
          selectedFileRefs={selectedFileRefs}
          committedMentionTargetID={committedMentionTargetID}
          committedMentionProfile={committedMentionProfile}
          committedMentionSession={committedMentionSession}
          agentStatusColor={agentStatusColor}
          hashDraftActive={Boolean(hashDraft)}
          fileCandidates={fileCandidates}
          selectedHashIndex={selectedHashIndex}
          mentionDraftActive={Boolean(mentionDraft)}
          mentionCandidates={mentionCandidates}
          selectedMentionIndex={selectedMentionIndex}
          onFocusAgentProfile={focusAgentProfile}
          onRemoveSelectedDiscussionAgent={toggleDiscussionAgentSelection}
          onRemoveFileRef={removeFileRef}
          onChooseHashCandidate={applyHashCandidate}
          onChooseMentionCandidate={applyMentionCandidate}
          onInputChange={(event) =>
            handleMessageInputChange(
              event.target.value,
              event.target.selectionStart,
            )
          }
          onInputClick={(event) =>
            updateMentionDraft(
              event.currentTarget.value,
              event.currentTarget.selectionStart,
            )
          }
          onInputKeyUp={(event) => {
            if (
              event.key === "ArrowDown" ||
              event.key === "ArrowUp" ||
              event.key === "Tab"
            ) {
              return;
            }
            updateMentionDraft(
              event.currentTarget.value,
              event.currentTarget.selectionStart,
            );
          }}
          onInputBlur={() => {
            window.setTimeout(() => {
              setMentionDraft(null);
              setHashDraft(null);
              setFileCandidates([]);
            }, 120);
          }}
          onInputKeyDown={(event) => {
            if (hashDraft && fileCandidates.length > 0) {
              if (event.key === "ArrowDown") {
                event.preventDefault();
                setSelectedHashIndex(
                  (prev) => (prev + 1) % fileCandidates.length,
                );
                return;
              }
              if (event.key === "ArrowUp") {
                event.preventDefault();
                setSelectedHashIndex(
                  (prev) =>
                    (prev - 1 + fileCandidates.length) % fileCandidates.length,
                );
                return;
              }
              if (event.key === "Enter" || event.key === "Tab") {
                event.preventDefault();
                const selected = fileCandidates[selectedHashIndex];
                if (selected) applyHashCandidate(selected);
                return;
              }
              if (event.key === "Escape") {
                setHashDraft(null);
                setFileCandidates([]);
                return;
              }
            }
            if (mentionDraft && mentionCandidates.length > 0) {
              if (event.key === "ArrowDown") {
                event.preventDefault();
                setSelectedMentionIndex(
                  (prev) => (prev + 1) % mentionCandidates.length,
                );
                return;
              }
              if (event.key === "ArrowUp") {
                event.preventDefault();
                setSelectedMentionIndex(
                  (prev) =>
                    (prev - 1 + mentionCandidates.length) %
                    mentionCandidates.length,
                );
                return;
              }
              if (event.key === "Enter" || event.key === "Tab") {
                event.preventDefault();
                if (selectedMentionCandidate) {
                  applyMentionCandidate(selectedMentionCandidate.id);
                }
                return;
              }
              if (event.key === "Escape") {
                setMentionDraft(null);
                return;
              }
            }
            if (event.key === "Backspace" && selectedFileRefs.length > 0) {
              const input = event.currentTarget;
              if (input.selectionStart === 0 && input.selectionEnd === 0) {
                event.preventDefault();
                setSelectedFileRefs((prev) => prev.slice(0, -1));
                return;
              }
            }
            if (event.key === "Enter" && !event.shiftKey) {
              event.preventDefault();
              void handleSend();
            }
          }}
          onInputPaste={(event) => {
            const items = Array.from(event.clipboardData.items);
            const files = items
              .filter((item) => item.kind === "file")
              .map((item) => item.getAsFile())
              .filter((file): file is File => file !== null);
            if (files.length > 0) {
              event.preventDefault();
              files.forEach((file) => void handleUploadAttachment(file));
            }
          }}
          onUploadInputChange={(event) => {
            Array.from(event.target.files ?? []).forEach(
              (file) => void handleUploadAttachment(file),
            );
            event.target.value = "";
          }}
          onSend={() => {
            void handleSend();
          }}
        />
      }
      sidebar={
        <ThreadSidebar
          thread={thread}
          messagesCount={messages.length}
          inviteableProfiles={inviteableProfiles}
          selectedInviteIDs={selectedInviteIDs}
          invitingAgent={invitingAgent}
          onToggleInviteSelection={toggleInviteSelection}
          onInviteAgent={() => {
            void handleInviteAgent();
          }}
          onClearInviteSelection={() => setSelectedInviteIDs(new Set())}
          agentSessionsWithProfileID={agentSessionsWithProfileID}
          selectedDiscussionAgentIDs={selectedDiscussionAgentIDs}
          profileByID={profileByID}
          highlightedAgentProfileID={highlightedAgentProfileID}
          agentCardRefs={agentCardRefs}
          removingAgentID={removingAgentID}
          onRemoveAgent={(id) => {
            void handleRemoveAgent(id);
          }}
          onToggleDiscussionAgentSelection={toggleDiscussionAgentSelection}
          onStartDiscussionWithAgents={startDiscussionWithSelectedAgents}
          onClearDiscussionAgents={() =>
            setSelectedDiscussionAgentIDs(new Set())
          }
          canStartDiscussionWithAgent={canStartDiscussionWithAgent}
          agentStatusColor={agentStatusColor}
          participants={participants}
          proposals={orderedProposals}
          proposalsLoading={proposalsLoading}
          showProposalEditor={showProposalEditor}
          proposalEditor={proposalEditor}
          savingProposal={savingProposal}
          proposalActionLoadingID={proposalActionLoadingID}
          proposalReviewInputs={proposalReviewInputs}
          onOpenCreateProposal={handleOpenCreateProposal}
          onOpenEditProposal={handleOpenEditProposal}
          onShowProposalEditorChange={(open) => {
            setShowProposalEditor(open);
            if (!open) {
              setProposalEditor(createProposalEditorState(thread.owner_id));
            }
          }}
          onProposalEditorFieldChange={handleProposalEditorFieldChange}
          onProposalDraftChange={handleProposalDraftChange}
          onAddProposalDraft={handleAddProposalDraft}
          onRemoveProposalDraft={handleRemoveProposalDraft}
          onSaveProposal={handleSaveProposal}
          onProposalReviewInputChange={handleProposalReviewInputChange}
          onSubmitProposal={(proposalId) => {
            void runProposalAction(proposalId, "submit");
          }}
          onApproveProposal={(proposalId) => {
            void runProposalAction(proposalId, "approve");
          }}
          onRejectProposal={(proposalId) => {
            void runProposalAction(proposalId, "reject");
          }}
          onReviseProposal={(proposalId) => {
            void runProposalAction(proposalId, "revise");
          }}
          workItemLinks={workItemLinks}
          orderedWorkItemLinks={orderedWorkItemLinks}
          linkedWorkItems={linkedWorkItems}
          showCreateWI={showCreateWI}
          newWITitle={newWITitle}
          newWIBody={newWIBody}
          showLinkWI={showLinkWI}
          linkWIId={linkWIId}
          onOpenCreateWorkItem={handleOpenCreateWorkItem}
          onShowCreateWIChange={setShowCreateWI}
          onNewWITitleChange={setNewWITitle}
          onNewWIBodyChange={setNewWIBody}
          onCreateWorkItem={handleCreateWorkItem}
          onShowLinkWIChange={setShowLinkWI}
          onLinkWIIdChange={setLinkWIId}
          onLinkWorkItem={handleLinkWorkItem}
          onResetCreateWorkItemDraft={() => {
            setNewWITitle("");
            setNewWIBody("");
          }}
          attachments={attachments}
          attachmentsLoading={attachmentsLoading}
          onUploadAttachment={(file) => {
            void handleUploadAttachment(file);
          }}
          onDeleteAttachment={(attId) => {
            void handleDeleteAttachment(attId);
          }}
          getAttachmentDownloadUrl={apiClient.getThreadAttachmentDownloadUrl}
        />
      }
    />
  );
}
