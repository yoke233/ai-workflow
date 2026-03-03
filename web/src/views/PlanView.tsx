
import { useEffect, useMemo, useRef, useState } from "react";
import type { ApiClient } from "../lib/apiClient";
import type {
  AdminAuditLogItem,
  ApiTaskPlan,
  IssueTimelineEntry,
  PlanChangeRecord,
  PlanRejectFeedbackCategory,
  PlanReviewRecord,
} from "../types/api";
import type { WsClient } from "../lib/wsClient";

interface PlanViewProps {
  apiClient: ApiClient;
  wsClient: WsClient;
  projectId: string;
  refreshToken: number;
}

type ReviewStepType = "source" | "review" | "change" | "timeline" | "audit";

interface ReviewStep {
  id: string;
  type: ReviewStepType;
  title: string;
  subtitle: string;
  createdAt?: string;
  changedFiles: string[];
  snapshot: Record<string, string>;
  review?: PlanReviewRecord;
  change?: PlanChangeRecord;
  timeline?: IssueTimelineEntry;
  audit?: AdminAuditLogItem;
}

interface FileUpdate {
  path: string;
  oldContent?: string;
  newContent?: string;
}

const PAGE_LIMIT = 50;

const REJECT_CATEGORIES: Array<{
  value: PlanRejectFeedbackCategory;
  label: string;
}> = [
  { value: "cycle", label: "循环依赖" },
  { value: "missing_node", label: "缺失节点" },
  { value: "bad_granularity", label: "粒度不合理" },
  { value: "coverage_gap", label: "覆盖不足" },
  { value: "other", label: "其他" },
];

const getErrorMessage = (error: unknown): string => {
  if (error instanceof Error && error.message.trim().length > 0) {
    return error.message;
  }
  return "请求失败，请稍后重试";
};

const toText = (value: unknown): string => {
  if (typeof value === "string") {
    return value;
  }
  if (typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  return "";
};

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === "object" && value !== null;

const formatTimestamp = (value?: string): string => {
  if (!value) {
    return "时间未知";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString("zh-CN", { hour12: false });
};

const parseTimestamp = (value?: string): number => {
  if (!value) {
    return Number.NEGATIVE_INFINITY;
  }
  const date = new Date(value);
  const time = date.getTime();
  return Number.isNaN(time) ? Number.NEGATIVE_INFINITY : time;
};

const uniqueStrings = (values: unknown[]): string[] => {
  const seen = new Set<string>();
  const out: string[] = [];
  values.forEach((value) => {
    const normalized = toText(value).trim();
    if (normalized.length === 0 || seen.has(normalized)) {
      return;
    }
    seen.add(normalized);
    out.push(normalized);
  });
  return out;
};

const asArray = <T,>(value: unknown): T[] => {
  if (!Array.isArray(value)) {
    return [];
  }
  return value as T[];
};

const asObjectArray = <T,>(value: unknown): T[] => {
  return asArray<unknown>(value).filter(isRecord) as unknown as T[];
};

const normalizeStringList = (value: unknown): string[] => {
  return uniqueStrings(
    asArray<unknown>(value).map((item) => toText(item).trim()),
  );
};

const normalizeFileContents = (value: unknown): Record<string, string> => {
  if (!isRecord(value)) {
    return {};
  }
  const out: Record<string, string> = {};
  Object.entries(value).forEach(([path, content]) => {
    if (typeof content === "string") {
      out[path] = content;
    }
  });
  return out;
};

const normalizePlan = (value: unknown, index: number): ApiTaskPlan | null => {
  if (!isRecord(value)) {
    return null;
  }
  const id = toText(value.id).trim() || `plan-${index + 1}`;
  const name = toText(value.name).trim() || toText(value.title).trim() || id;
  const status = toText(value.status).trim() || "draft";
  const autoMerge = typeof value.auto_merge === "boolean" ? value.auto_merge : true;

  return {
    ...(value as unknown as ApiTaskPlan),
    id,
    name,
    status: status as ApiTaskPlan["status"],
    auto_merge: autoMerge,
    wait_reason: toText(value.wait_reason).trim(),
    tasks: asObjectArray<ApiTaskPlan["tasks"][number]>(value.tasks),
    source_files: normalizeStringList(value.source_files),
    file_contents: normalizeFileContents(value.file_contents),
  };
};

const cloneMap = (value: Record<string, string>): Record<string, string> => {
  return { ...value };
};

const parseMapText = (raw: string): Record<string, string> | null => {
  const normalized = raw.trim();
  if (!normalized.startsWith("{")) {
    return null;
  }
  try {
    const parsed = JSON.parse(normalized);
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
      return null;
    }
    const out: Record<string, string> = {};
    Object.entries(parsed as Record<string, unknown>).forEach(([key, value]) => {
      if (typeof value === "string") {
        out[key] = value;
      }
    });
    return out;
  } catch {
    return null;
  }
};

const parseFilePathFromField = (field: string): string | null => {
  const trimmed = field.trim();
  if (trimmed.startsWith("file_contents.")) {
    return trimmed.slice("file_contents.".length).trim() || null;
  }
  const bracket = trimmed.match(/^file_contents\[(["']?)(.+)\1\]$/);
  if (bracket && bracket[2]) {
    return bracket[2].trim() || null;
  }
  return null;
};

const parseFileUpdatesFromChange = (change: PlanChangeRecord): FileUpdate[] => {
  const field = toText(change.field).trim();
  const oldValue = toText(change.old_value);
  const newValue = toText(change.new_value);

  const directPath = parseFilePathFromField(field);
  if (directPath) {
    return [
      {
        path: directPath,
        oldContent: oldValue,
        newContent: newValue,
      },
    ];
  }

  if (field !== "file_contents") {
    return [];
  }

  const oldMap = parseMapText(oldValue) ?? {};
  const newMap = parseMapText(newValue) ?? {};
  const allPaths = uniqueStrings([...Object.keys(oldMap), ...Object.keys(newMap)]);
  return allPaths.map((path) => ({
    path,
    oldContent: oldMap[path],
    newContent: newMap[path],
  }));
};

const extractFileHintsFromTimeline = (entry: IssueTimelineEntry): string[] => {
  const candidates = [
    toText(entry.meta?.file_path),
    toText(entry.meta?.path),
    toText(entry.meta?.file),
    toText(entry.meta?.target_file),
  ];
  return uniqueStrings(candidates);
};

const sortByCreatedAtAsc = <T extends { created_at?: string }>(items: T[]): T[] => {
  return [...items].sort((a, b) => {
    const left = parseTimestamp(a.created_at);
    const right = parseTimestamp(b.created_at);
    if (left === right) {
      return 0;
    }
    return left - right;
  });
};

const buildReviewSteps = (
  plan: ApiTaskPlan,
  reviews: PlanReviewRecord[],
  changes: PlanChangeRecord[],
  timeline: IssueTimelineEntry[],
  audits: AdminAuditLogItem[],
): ReviewStep[] => {
  const safeReviews = asObjectArray<PlanReviewRecord>(reviews);
  const safeChanges = asObjectArray<PlanChangeRecord>(changes);
  const safeTimeline = asObjectArray<IssueTimelineEntry>(timeline);
  const safeAudits = asObjectArray<AdminAuditLogItem>(audits);

  const sourceFiles = uniqueStrings([
    ...normalizeStringList(plan.source_files),
    ...Object.keys(normalizeFileContents(plan.file_contents)),
  ]);
  const sourceSnapshot = cloneMap(normalizeFileContents(plan.file_contents));

  const sourceStep: ReviewStep = {
    id: `${plan.id}:source`,
    type: "source",
    title: "步骤 0 · 原始输入文件",
    subtitle: `创建于 ${formatTimestamp(plan.created_at)}`,
    createdAt: plan.created_at,
    changedFiles: sourceFiles,
    snapshot: cloneMap(sourceSnapshot),
  };

  const reviewSteps: ReviewStep[] = sortByCreatedAtAsc(safeReviews).map((record) => ({
    id: `review:${record.id}`,
    type: "review",
    title: `Review Round ${toText(record.round) || "N/A"} · ${toText(record.verdict) || "unknown"}`,
    subtitle: `reviewer=${toText(record.reviewer) || "unknown"} · ${formatTimestamp(record.created_at)}`,
    createdAt: record.created_at,
    changedFiles: [],
    snapshot: {},
    review: record,
  }));

  const changeSteps: ReviewStep[] = sortByCreatedAtAsc(safeChanges).map((record) => ({
    id: `change:${record.id}`,
    type: "change",
    title: `Change · ${toText(record.field) || "unknown_field"}`,
    subtitle: `${toText(record.changed_by) || "system"} · ${formatTimestamp(record.created_at)}`,
    createdAt: record.created_at,
    changedFiles: [],
    snapshot: {},
    change: record,
  }));

  const timelineSteps: ReviewStep[] = sortByCreatedAtAsc(safeTimeline).map((entry) => ({
    id: `timeline:${entry.event_id}`,
    type: "timeline",
    title: toText(entry.title) || `Timeline · ${toText(entry.kind)}`,
    subtitle: `${toText(entry.actor_name) || toText(entry.actor_type) || "system"} · ${formatTimestamp(entry.created_at)}`,
    createdAt: entry.created_at,
    changedFiles: extractFileHintsFromTimeline(entry),
    snapshot: {},
    timeline: entry,
  }));

  const auditSteps: ReviewStep[] = sortByCreatedAtAsc(safeAudits).map((item) => ({
    id: `audit:${item.id}`,
    type: "audit",
    title: `Audit · ${toText(item.action) || "unknown_action"}`,
    subtitle: `${toText(item.user_id) || "system"} · ${formatTimestamp(item.created_at)}`,
    createdAt: item.created_at,
    changedFiles: [],
    snapshot: {},
    audit: item,
  }));

  const merged = [...reviewSteps, ...changeSteps, ...timelineSteps, ...auditSteps].sort((a, b) => {
    const left = parseTimestamp(a.createdAt);
    const right = parseTimestamp(b.createdAt);
    if (left !== right) {
      return left - right;
    }
    return a.id.localeCompare(b.id);
  });

  const out: ReviewStep[] = [sourceStep];
  let runningSnapshot = cloneMap(sourceSnapshot);
  merged.forEach((step) => {
    if (step.type === "change" && step.change) {
      const updates = parseFileUpdatesFromChange(step.change);
      const touched: string[] = [];
      updates.forEach((update) => {
        const normalizedPath = update.path.trim();
        if (!normalizedPath) {
          return;
        }
        touched.push(normalizedPath);
        if (typeof update.newContent === "string") {
          runningSnapshot[normalizedPath] = update.newContent;
        }
      });
      step.changedFiles = uniqueStrings([...step.changedFiles, ...touched]);
    }
    step.snapshot = cloneMap(runningSnapshot);
    out.push(step);
  });
  return out;
};

const PlanView = ({ apiClient, wsClient, projectId, refreshToken }: PlanViewProps) => {
  const [plans, setPlans] = useState<ApiTaskPlan[]>([]);
  const [loadingPlans, setLoadingPlans] = useState(true);
  const [loadingArtifacts, setLoadingArtifacts] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [selectedPlanId, setSelectedPlanId] = useState<string | null>(null);

  const [reviews, setReviews] = useState<PlanReviewRecord[]>([]);
  const [changes, setChanges] = useState<PlanChangeRecord[]>([]);
  const [timeline, setTimeline] = useState<IssueTimelineEntry[]>([]);
  const [audits, setAudits] = useState<AdminAuditLogItem[]>([]);

  const [actionLoading, setActionLoading] = useState(false);
  const [autoMergeLoading, setAutoMergeLoading] = useState(false);
  const [rejectCategory, setRejectCategory] = useState<PlanRejectFeedbackCategory>("coverage_gap");
  const [rejectDetail, setRejectDetail] = useState("");
  const [rejectDirection, setRejectDirection] = useState("");

  const [selectedStepId, setSelectedStepId] = useState<string | null>(null);
  const [selectedFilePath, setSelectedFilePath] = useState<string | null>(null);

  const listRequestIdRef = useRef(0);
  const artifactRequestIdRef = useRef(0);

  const activePlan = useMemo(
    () => plans.find((plan) => plan.id === selectedPlanId) ?? null,
    [plans, selectedPlanId],
  );

  useEffect(() => {
    listRequestIdRef.current += 1;
    artifactRequestIdRef.current += 1;
    setPlans([]);
    setLoadingPlans(true);
    setLoadingArtifacts(false);
    setError(null);
    setNotice(null);
    setSelectedPlanId(null);
    setReviews([]);
    setChanges([]);
    setTimeline([]);
    setAudits([]);
    setAutoMergeLoading(false);
    setSelectedStepId(null);
    setSelectedFilePath(null);
  }, [projectId]);

  useEffect(() => {
    let cancelled = false;
    const requestId = listRequestIdRef.current + 1;
    listRequestIdRef.current = requestId;
    setLoadingPlans(true);
    setError(null);

    const loadPlans = async () => {
      try {
        const all: ApiTaskPlan[] = [];
        let offset = 0;
        while (true) {
          const response = await apiClient.listPlans(projectId, { limit: PAGE_LIMIT, offset });
          if (cancelled || listRequestIdRef.current !== requestId) {
            return;
          }
          const pageItems = asArray<unknown>((response as { items?: unknown }).items)
            .map((item, index) => normalizePlan(item, offset + index))
            .filter((item): item is ApiTaskPlan => Boolean(item));
          all.push(...pageItems);
          const currentCount = pageItems.length;
          if (currentCount < PAGE_LIMIT) {
            break;
          }
          offset += currentCount;
        }
        all.sort((a, b) => parseTimestamp(b.updated_at) - parseTimestamp(a.updated_at));
        if (cancelled || listRequestIdRef.current !== requestId) {
          return;
        }
        setPlans(all);
        setSelectedPlanId((current) => {
          if (current && all.some((plan) => plan.id === current)) {
            return current;
          }
          return all[0]?.id ?? null;
        });
      } catch (requestError) {
        if (!cancelled && listRequestIdRef.current === requestId) {
          console.error("[PlanView] loadPlans failed:", requestError);
          setError(getErrorMessage(requestError));
          setPlans([]);
          setSelectedPlanId(null);
        }
      } finally {
        if (!cancelled && listRequestIdRef.current === requestId) {
          setLoadingPlans(false);
        }
      }
    };

    void loadPlans();
    return () => {
      cancelled = true;
    };
  }, [apiClient, projectId, refreshToken]);

  useEffect(() => {
    if (!activePlan) {
      return;
    }
    let subscribed = false;
    const subscribePlan = () => {
      if (subscribed) {
        return;
      }
      try {
        wsClient.send({
          type: "subscribe_plan",
          plan_id: activePlan.id,
        });
        subscribed = true;
      } catch (sendError) {
        console.warn("[PlanView] subscribe_plan skipped:", sendError);
      }
    };

    subscribePlan();
    const unsubscribe = wsClient.onStatusChange((statusValue) => {
      if (statusValue === "open") {
        subscribePlan();
      }
    });
    return () => {
      unsubscribe();
    };
  }, [activePlan?.id, wsClient]);

  useEffect(() => {
    if (!activePlan) {
      setReviews([]);
      setChanges([]);
      setTimeline([]);
      setAudits([]);
      return;
    }

    let cancelled = false;
    const requestId = artifactRequestIdRef.current + 1;
    artifactRequestIdRef.current = requestId;
    setLoadingArtifacts(true);
    setError(null);

    const loadArtifacts = async () => {
      try {
        const [
          reviewResult,
          changeResult,
          timelineResult,
          auditResult,
        ] = await Promise.allSettled([
          apiClient.listPlanReviews ? apiClient.listPlanReviews(projectId, activePlan.id) : Promise.resolve([]),
          apiClient.listPlanChanges ? apiClient.listPlanChanges(projectId, activePlan.id) : Promise.resolve([]),
          apiClient.listIssueTimeline(projectId, activePlan.id, { limit: 200, offset: 0 }),
          apiClient.listAdminAuditLog
            ? apiClient.listAdminAuditLog({ projectId, limit: 200, offset: 0 })
            : Promise.resolve({ items: [], total: 0, offset: 0 }),
        ]);

        if (cancelled || artifactRequestIdRef.current !== requestId) {
          return;
        }

        const nextReviews = reviewResult.status === "fulfilled"
          ? asObjectArray<PlanReviewRecord>(reviewResult.value)
          : [];
        const nextChanges = changeResult.status === "fulfilled"
          ? asObjectArray<PlanChangeRecord>(changeResult.value)
          : [];
        const nextTimeline = timelineResult.status === "fulfilled"
          ? asObjectArray<IssueTimelineEntry>(
              (timelineResult.value as { items?: unknown }).items ?? timelineResult.value,
            )
          : [];
        const rawAudits = auditResult.status === "fulfilled"
          ? asObjectArray<AdminAuditLogItem>((auditResult.value as { items?: unknown }).items)
          : [];
        const nextAudits = rawAudits.filter((item) => {
          const issueID = toText(item.issue_id).trim();
          if (issueID.length > 0 && issueID === activePlan.id) {
            return true;
          }
          const pipelineID = toText(item.pipeline_id).trim();
          const activePipelineID = toText(activePlan.pipeline_id).trim();
          if (activePipelineID.length > 0 && pipelineID === activePipelineID) {
            return true;
          }
          return false;
        });

        setReviews(nextReviews);
        setChanges(nextChanges);
        setTimeline(nextTimeline);
        setAudits(nextAudits);

        const failure = [reviewResult, changeResult, timelineResult, auditResult].find(
          (result) => result.status === "rejected",
        );
        if (failure && failure.status === "rejected") {
          setError(`部分审阅数据加载失败：${getErrorMessage(failure.reason)}`);
        }
      } catch (requestError) {
        if (!cancelled && artifactRequestIdRef.current === requestId) {
          console.error("[PlanView] loadArtifacts failed:", requestError);
          setError(getErrorMessage(requestError));
          setReviews([]);
          setChanges([]);
          setTimeline([]);
          setAudits([]);
        }
      } finally {
        if (!cancelled && artifactRequestIdRef.current === requestId) {
          setLoadingArtifacts(false);
        }
      }
    };

    void loadArtifacts();
    return () => {
      cancelled = true;
    };
  }, [apiClient, projectId, activePlan, refreshToken]);

  const reviewSteps = useMemo(() => {
    if (!activePlan) {
      return [] as ReviewStep[];
    }
    return buildReviewSteps(activePlan, reviews, changes, timeline, audits);
  }, [activePlan, reviews, changes, timeline, audits]);

  useEffect(() => {
    if (reviewSteps.length === 0) {
      setSelectedStepId(null);
      return;
    }
    setSelectedStepId((current) => {
      if (current && reviewSteps.some((step) => step.id === current)) {
        return current;
      }
      return reviewSteps[reviewSteps.length - 1]?.id ?? reviewSteps[0]?.id ?? null;
    });
  }, [reviewSteps]);

  const selectedStep = useMemo(
    () => reviewSteps.find((step) => step.id === selectedStepId) ?? null,
    [reviewSteps, selectedStepId],
  );

  const filePathsInStep = useMemo(() => {
    if (!activePlan || !selectedStep) {
      return [] as string[];
    }
    const source = Array.isArray(activePlan.source_files) ? activePlan.source_files : [];
    return uniqueStrings([
      ...selectedStep.changedFiles,
      ...source,
      ...Object.keys(selectedStep.snapshot),
    ]);
  }, [activePlan, selectedStep]);

  useEffect(() => {
    if (filePathsInStep.length === 0) {
      setSelectedFilePath(null);
      return;
    }
    setSelectedFilePath((current) => {
      if (current && filePathsInStep.includes(current)) {
        return current;
      }
      return filePathsInStep[0] ?? null;
    });
  }, [filePathsInStep]);

  const selectedFileContent = useMemo(() => {
    if (!selectedStep || !selectedFilePath) {
      return "";
    }
    return selectedStep.snapshot[selectedFilePath] ?? "";
  }, [selectedStep, selectedFilePath]);

  const runPlanAction = async (
    action:
      | "submit_review"
      | "approve"
      | "reject"
      | "abort",
  ) => {
    if (!activePlan || actionLoading) {
      return;
    }
    setActionLoading(true);
    setError(null);
    setNotice(null);

    try {
      if (action === "submit_review") {
        const response = await apiClient.submitPlanReview(projectId, activePlan.id);
        setNotice(`已提交审核，当前状态：${response.status}`);
      } else if (action === "approve") {
        const response = await apiClient.applyPlanAction(projectId, activePlan.id, {
          action: "approve",
        });
        setNotice(`已通过，当前状态：${response.status}`);
      } else if (action === "abort") {
        const response = await apiClient.applyPlanAction(projectId, activePlan.id, {
          action: "abort",
        });
        setNotice(`已放弃，当前状态：${response.status}`);
      } else {
        const detail = rejectDetail.trim();
        if (detail.length === 0) {
          setError("驳回说明不能为空。");
          return;
        }
        const response = await apiClient.applyPlanAction(projectId, activePlan.id, {
          action: "reject",
          feedback: {
            category: rejectCategory,
            detail,
            expected_direction: rejectDirection.trim() || undefined,
          },
        });
        setNotice(`已驳回，当前状态：${response.status}`);
      }
    } catch (requestError) {
      setError(getErrorMessage(requestError));
    } finally {
      setActionLoading(false);
    }
  };

  const setIssueAutoMerge = async (nextAutoMerge: boolean) => {
    if (!activePlan || autoMergeLoading) {
      return;
    }
    setAutoMergeLoading(true);
    setError(null);
    setNotice(null);
    try {
      const response = await apiClient.setIssueAutoMerge(projectId, activePlan.id, {
        auto_merge: nextAutoMerge,
      });
      setPlans((current) =>
        current.map((plan) =>
          plan.id === activePlan.id
            ? {
                ...plan,
                auto_merge: response.auto_merge,
                status: (toText(response.status).trim() || plan.status) as ApiTaskPlan["status"],
              }
            : plan,
        ),
      );
      setNotice(`自动合并已${response.auto_merge ? "开启" : "关闭"}，当前状态：${response.status}`);
    } catch (requestError) {
      setError(getErrorMessage(requestError));
    } finally {
      setAutoMergeLoading(false);
    }
  };

  const waitReason = toText(activePlan?.wait_reason).trim().toLowerCase();
  const status = toText(activePlan?.status).trim().toLowerCase();
  const autoMergeEnabled = activePlan?.auto_merge ?? true;
  const canSubmitReview = status === "draft";
  const canApprove =
    status === "reviewing" ||
    (status === "waiting_human" &&
      (waitReason === "final_approval" ||
        waitReason === "feedback_required" ||
        waitReason === "parse_failed"));
  const canReject =
    status === "reviewing" ||
    (status === "waiting_human" &&
      (waitReason === "final_approval" || waitReason === "feedback_required"));
  const canAbort = status !== "done" && status !== "failed" && status !== "abandoned";
  const canRetryParse = status === "waiting_human" && waitReason === "parse_failed";

  return (
    <section className="grid gap-4 lg:grid-cols-[280px_1fr]">
      <aside className="rounded-md border border-slate-200 bg-white p-3">
        <h2 className="text-sm font-semibold text-slate-900">Plans</h2>
        {loadingPlans ? <p className="mt-2 text-xs text-slate-500">加载中...</p> : null}
        {!loadingPlans && plans.length === 0 ? (
          <p className="mt-2 text-xs text-slate-500">暂无计划</p>
        ) : null}
        <ul className="mt-2 space-y-2">
          {plans.map((plan) => {
            const active = plan.id === selectedPlanId;
            return (
              <li key={plan.id}>
                <button
                  type="button"
                  data-testid="plan-item"
                  className={`w-full rounded-md border px-3 py-2 text-left text-xs ${
                    active
                      ? "border-slate-900 bg-slate-900 text-white"
                      : "border-slate-200 bg-white text-slate-700 hover:bg-slate-50"
                  }`}
                  onClick={() => {
                    setSelectedPlanId(plan.id);
                    setNotice(null);
                    setError(null);
                  }}
                >
                  <p className="truncate text-sm font-semibold">{plan.name || plan.id}</p>
                  <p className={`mt-1 ${active ? "text-slate-200" : "text-slate-500"}`}>
                    {plan.status}
                  </p>
                </button>
              </li>
            );
          })}
        </ul>
      </aside>

      <div className="space-y-4">
        {!activePlan ? (
          <section className="rounded-md border border-slate-200 bg-white p-4 text-sm text-slate-500">
            请选择一个计划。
          </section>
        ) : (
          <>
            <section className="rounded-md border border-slate-200 bg-white p-4">
              <h1 className="text-lg font-semibold text-slate-900">{activePlan.name || activePlan.id}</h1>
              <p className="mt-1 text-xs text-slate-600">
                plan_id={activePlan.id} · status={activePlan.status}
                {activePlan.wait_reason ? ` · wait_reason=${activePlan.wait_reason}` : ""}
              </p>
              <label className="mt-3 flex items-center gap-2 rounded-md border border-slate-200 bg-slate-50 px-3 py-2 text-xs text-slate-700">
                <input
                  type="checkbox"
                  checked={autoMergeEnabled}
                  disabled={autoMergeLoading}
                  onChange={(event) => {
                    void setIssueAutoMerge(event.target.checked);
                  }}
                />
                <span>自动合并（评审通过后自动进入 pipeline 执行合并）</span>
                {autoMergeLoading ? <span className="text-slate-500">保存中...</span> : null}
              </label>

              {canRetryParse ? (
                <p className="mt-2 rounded-md border border-amber-200 bg-amber-50 px-2 py-1 text-xs text-amber-700">
                  解析失败（parse_failed），请修正输入后点击“重试解析”。
                </p>
              ) : null}

              <div className="mt-3 grid gap-3 xl:grid-cols-[repeat(4,minmax(0,1fr))]">
                <button
                  type="button"
                  className="rounded-md border border-slate-900 px-3 py-2 text-xs font-medium text-slate-900 disabled:cursor-not-allowed disabled:border-slate-300 disabled:text-slate-400"
                  disabled={!canSubmitReview || actionLoading}
                  onClick={() => {
                    void runPlanAction("submit_review");
                  }}
                >
                  提交审核
                </button>
                <button
                  type="button"
                  className="rounded-md border border-emerald-700 px-3 py-2 text-xs font-medium text-emerald-700 disabled:cursor-not-allowed disabled:border-slate-300 disabled:text-slate-400"
                  disabled={!canApprove || actionLoading}
                  onClick={() => {
                    void runPlanAction("approve");
                  }}
                >
                  {canRetryParse ? "重试解析" : "通过"}
                </button>
                <button
                  type="button"
                  className="rounded-md border border-rose-700 px-3 py-2 text-xs font-medium text-rose-700 disabled:cursor-not-allowed disabled:border-slate-300 disabled:text-slate-400"
                  disabled={!canReject || actionLoading}
                  onClick={() => {
                    void runPlanAction("reject");
                  }}
                >
                  驳回
                </button>
                <button
                  type="button"
                  className="rounded-md border border-slate-400 px-3 py-2 text-xs font-medium text-slate-700 disabled:cursor-not-allowed disabled:border-slate-300 disabled:text-slate-400"
                  disabled={!canAbort || actionLoading}
                  onClick={() => {
                    void runPlanAction("abort");
                  }}
                >
                  放弃
                </button>
              </div>

              <div className="mt-3 grid gap-3 md:grid-cols-3">
                <label className="text-xs text-slate-700">
                  驳回类型
                  <select
                    className="mt-1 w-full rounded-md border border-slate-300 px-2 py-1 text-sm"
                    value={rejectCategory}
                    onChange={(event) => {
                      setRejectCategory(event.target.value as PlanRejectFeedbackCategory);
                    }}
                  >
                    {REJECT_CATEGORIES.map((item) => (
                      <option key={item.value} value={item.value}>
                        {item.label}
                      </option>
                    ))}
                  </select>
                </label>
                <label className="text-xs text-slate-700 md:col-span-2">
                  驳回说明
                  <input
                    className="mt-1 w-full rounded-md border border-slate-300 px-2 py-1 text-sm"
                    value={rejectDetail}
                    onChange={(event) => {
                      setRejectDetail(event.target.value);
                    }}
                  />
                </label>
              </div>
              <label className="mt-2 block text-xs text-slate-700">
                期望方向（可选）
                <input
                  className="mt-1 w-full rounded-md border border-slate-300 px-2 py-1 text-sm"
                  value={rejectDirection}
                  onChange={(event) => {
                    setRejectDirection(event.target.value);
                  }}
                />
              </label>

              {notice ? (
                <p className="mt-3 rounded-md border border-emerald-200 bg-emerald-50 px-2 py-1 text-xs text-emerald-700">
                  {notice}
                </p>
              ) : null}
              {error ? (
                <p className="mt-3 rounded-md border border-rose-200 bg-rose-50 px-2 py-1 text-xs text-rose-700">
                  {error}
                </p>
              ) : null}
            </section>

            <section className="rounded-md border border-slate-200 bg-white p-4">
              <div className="grid gap-4 xl:grid-cols-[280px_1fr]">
                <aside className="rounded-md border border-slate-200 bg-slate-50 p-3">
                  <h3 className="text-sm font-semibold text-slate-900">审阅步骤</h3>
                  {loadingArtifacts ? <p className="mt-2 text-xs text-slate-500">步骤加载中...</p> : null}
                  <ol className="mt-2 space-y-2">
                    {reviewSteps.map((step, index) => {
                      const active = step.id === selectedStepId;
                      return (
                        <li key={step.id}>
                          <button
                            type="button"
                            className={`w-full rounded-md border px-2 py-2 text-left text-xs ${
                              active
                                ? "border-slate-900 bg-slate-900 text-white"
                                : "border-slate-200 bg-white text-slate-700 hover:bg-slate-100"
                            }`}
                            onClick={() => {
                              setSelectedStepId(step.id);
                            }}
                          >
                            <p className="font-semibold">
                              {index}. {step.title}
                            </p>
                            <p className={`mt-1 ${active ? "text-slate-200" : "text-slate-500"}`}>
                              {step.subtitle}
                            </p>
                          </button>
                        </li>
                      );
                    })}
                  </ol>
                </aside>

                <div className="space-y-4">
                  {!selectedStep ? (
                    <p className="text-sm text-slate-500">暂无步骤详情。</p>
                  ) : (
                    <>
                      <header>
                        <h3 className="text-base font-semibold text-slate-900">{selectedStep.title}</h3>
                        <p className="text-xs text-slate-500">{selectedStep.subtitle}</p>
                      </header>

                      {selectedStep.review ? (
                        <section className="rounded-md border border-slate-200 bg-slate-50 p-3 text-xs text-slate-700">
                          {(() => {
                            const reviewIssues = asArray<{
                              issue_id?: string;
                              description?: string;
                              severity?: string;
                            }>(selectedStep.review?.issues);
                            const reviewFixes = asArray<{
                              issue_id?: string;
                              description?: string;
                            }>(selectedStep.review?.fixes);
                            const reviewSummary = toText(selectedStep.review?.summary).trim();
                            const reviewRawOutput = toText(selectedStep.review?.raw_output).trim();
                            const fallbackDetail = [
                              `verdict=${toText(selectedStep.review?.verdict) || "unknown"}`,
                              `score=${toText(selectedStep.review?.score) || "N/A"}`,
                            ].join("\n");
                            return (
                              <>
                          <p className="font-semibold text-slate-900">评审结论</p>
                          <pre className="mt-1 max-h-40 overflow-auto rounded border border-slate-200 bg-white p-2 whitespace-pre-wrap">
                            {reviewSummary || fallbackDetail}
                          </pre>
                          <p className="mt-2 font-semibold text-slate-900">完整审阅输出</p>
                          <pre className="mt-1 max-h-64 overflow-auto rounded border border-slate-200 bg-white p-2 whitespace-pre-wrap">
                            {reviewRawOutput || reviewSummary || fallbackDetail}
                          </pre>
                          <p className="mt-2 font-semibold text-slate-900">issues</p>
                          {reviewIssues.length === 0 ? (
                            <p className="mt-1 text-slate-500">无 issue</p>
                          ) : (
                            <ul className="mt-1 list-disc space-y-1 pl-4">
                              {reviewIssues.map((item, index) => (
                                <li key={`${item.issue_id ?? "issue"}-${index}`}>
                                  [{toText(item.severity) || "unknown"}] {toText(item.description) || "(empty)"}
                                </li>
                              ))}
                            </ul>
                          )}
                          <p className="mt-2 font-semibold text-slate-900">fixes</p>
                          {reviewFixes.length === 0 ? (
                            <p className="mt-1 text-slate-500">无 fixes</p>
                          ) : (
                            <ul className="mt-1 list-disc space-y-1 pl-4">
                              {reviewFixes.map((item, index) => (
                                <li key={`${item.issue_id ?? "fix"}-${index}`}>
                                  {toText(item.description) || "(empty)"}
                                </li>
                              ))}
                            </ul>
                          )}
                              </>
                            );
                          })()}
                        </section>
                      ) : null}

                      {selectedStep.change ? (
                        <section className="rounded-md border border-slate-200 bg-slate-50 p-3 text-xs text-slate-700">
                          <p>field={selectedStep.change.field}</p>
                          <p>reason={selectedStep.change.reason || "N/A"}</p>
                          <p>changed_by={selectedStep.change.changed_by || "system"}</p>
                          <div className="mt-2 grid gap-2 md:grid-cols-2">
                            <pre className="max-h-40 overflow-auto rounded border border-slate-200 bg-white p-2">
                              old:
                              {"\n"}
                              {selectedStep.change.old_value || "(empty)"}
                            </pre>
                            <pre className="max-h-40 overflow-auto rounded border border-slate-200 bg-white p-2">
                              new:
                              {"\n"}
                              {selectedStep.change.new_value || "(empty)"}
                            </pre>
                          </div>
                        </section>
                      ) : null}

                      {selectedStep.timeline ? (
                        <section className="rounded-md border border-slate-200 bg-slate-50 p-3 text-xs text-slate-700">
                          <p>kind={selectedStep.timeline.kind}</p>
                          <p>status={selectedStep.timeline.status}</p>
                          <pre className="mt-2 max-h-40 overflow-auto rounded border border-slate-200 bg-white p-2">
                            {selectedStep.timeline.body || "(no body)"}
                          </pre>
                        </section>
                      ) : null}

                      {selectedStep.audit ? (
                        <section className="rounded-md border border-slate-200 bg-slate-50 p-3 text-xs text-slate-700">
                          <p>action={selectedStep.audit.action}</p>
                          <p>source={selectedStep.audit.source}</p>
                          <p>user={selectedStep.audit.user_id}</p>
                          <pre className="mt-2 max-h-40 overflow-auto rounded border border-slate-200 bg-white p-2">
                            {selectedStep.audit.message || "(no message)"}
                          </pre>
                        </section>
                      ) : null}

                      <section className="grid gap-3 xl:grid-cols-[260px_1fr]">
                        <aside className="rounded-md border border-slate-200 bg-slate-50 p-2">
                          <h4 className="text-xs font-semibold text-slate-900">
                            {selectedStep.type === "source" ? "文件原文" : "该步骤后的文件结果"}
                          </h4>
                          {filePathsInStep.length === 0 ? (
                            <p className="mt-2 text-xs text-slate-500">当前步骤没有文件快照。</p>
                          ) : (
                            <ul className="mt-2 space-y-1">
                              {filePathsInStep.map((path) => (
                                <li key={path}>
                                  <button
                                    type="button"
                                    className={`w-full rounded border px-2 py-1 text-left text-xs ${
                                      selectedFilePath === path
                                        ? "border-slate-900 bg-slate-900 text-white"
                                        : "border-slate-200 bg-white text-slate-700 hover:bg-slate-100"
                                    }`}
                                    onClick={() => {
                                      setSelectedFilePath(path);
                                    }}
                                  >
                                    {path}
                                  </button>
                                </li>
                              ))}
                            </ul>
                          )}
                        </aside>
                        <pre className="max-h-[520px] overflow-auto rounded-md border border-slate-200 bg-slate-900 p-3 text-xs text-slate-100">
                          {selectedFilePath ? selectedFileContent || "(empty)" : "请选择文件查看内容。"}
                        </pre>
                      </section>
                    </>
                  )}
                </div>
              </div>
            </section>
          </>
        )}
      </div>
    </section>
  );
};

export default PlanView;
