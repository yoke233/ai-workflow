import { useCallback, useEffect, useMemo, useState } from "react";
import type { ApiClient } from "../lib/apiClient";
import type { Pipeline } from "../types/workflow";
import type { PipelineActionRequest, PipelineCheckpoint } from "../types/api";

interface PipelineViewProps {
  apiClient: ApiClient;
  projectId: string;
  refreshToken: number;
}

const PIPELINE_STAGE_ORDER: Record<string, string[]> = {
  standard: [
    "requirements",
    "spec_gen",
    "spec_review",
    "worktree_setup",
    "implement",
    "code_review",
    "fixup",
    "e2e_test",
    "merge",
    "cleanup",
  ],
  quick: [
    "requirements",
    "worktree_setup",
    "implement",
    "code_review",
    "merge",
    "cleanup",
  ],
  hotfix: ["requirements", "worktree_setup", "implement", "merge", "cleanup"],
};

const getErrorMessage = (error: unknown): string => {
  if (error instanceof Error && error.message.trim().length > 0) {
    return error.message;
  }
  return "请求失败，请稍后重试";
};

const PAGE_LIMIT = 50;
const REFRESH_INTERVAL_MS = 10_000;

const getPipelineProgress = (pipeline: Pipeline) => {
  const stages = PIPELINE_STAGE_ORDER[pipeline.template] ?? [];
  const totalStages = stages.length;
  if (totalStages === 0) {
    return {
      percentage: 0,
      stageText: "未知模板，无法计算阶段进度",
    };
  }

  const currentIndex = stages.findIndex((stage) => stage === pipeline.current_stage);
  if (pipeline.status === "done" || pipeline.status === "failed" || pipeline.status === "aborted") {
    return {
      percentage: 100,
      stageText: `${totalStages}/${totalStages}`,
    };
  }
  if (pipeline.status === "created") {
    return {
      percentage: 0,
      stageText: `0/${totalStages}`,
    };
  }

  const safeIndex = currentIndex >= 0 ? currentIndex : 0;
  const completed = safeIndex + 0.5;
  const percentage = Math.min(100, Math.max(0, Math.round((completed / totalStages) * 100)));
  return {
    percentage,
    stageText: `${Math.max(0, safeIndex) + 1}/${totalStages}`,
  };
};

const PipelineView = ({ apiClient, projectId, refreshToken }: PipelineViewProps) => {
  const [pipelines, setPipelines] = useState<Pipeline[]>([]);
  const [selectedPipelineId, setSelectedPipelineId] = useState<string | null>(null);
  const [checkpoints, setCheckpoints] = useState<PipelineCheckpoint[]>([]);
  const [loading, setLoading] = useState(false);
  const [checkpointsLoading, setCheckpointsLoading] = useState(false);
  const [actionLoading, setActionLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [actionNotice, setActionNotice] = useState<string | null>(null);
  const [actionMessage, setActionMessage] = useState("");

  useEffect(() => {
    let cancelled = false;
    let inFlight = false;
    const loadPipelines = async () => {
      if (inFlight) {
        return;
      }
      inFlight = true;
      setLoading(true);
      setError(null);

      try {
        const allPipelines: Pipeline[] = [];
        let offset = 0;
        while (true) {
          const response = await apiClient.listPipelines(projectId, {
            limit: PAGE_LIMIT,
            offset,
          });
          if (cancelled) {
            return;
          }
          allPipelines.push(...response.items);
          const currentCount = response.items.length;
          if (currentCount === 0) {
            break;
          }
          offset += currentCount;
          if (currentCount < PAGE_LIMIT) {
            break;
          }
        }
        if (!cancelled) {
          setPipelines(allPipelines);
          setSelectedPipelineId((current) => {
            if (current && allPipelines.some((item) => item.id === current)) {
              return current;
            }
            return allPipelines[0]?.id ?? null;
          });
        }
      } catch (requestError) {
        if (!cancelled) {
          setPipelines([]);
          setSelectedPipelineId(null);
          setCheckpoints([]);
          setError(getErrorMessage(requestError));
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
        inFlight = false;
      }
    };

    void loadPipelines();
    // Fallback refresh for non-PlanView scenarios where WS events are not enough to keep list current.
    const intervalId = setInterval(() => {
      void loadPipelines();
    }, REFRESH_INTERVAL_MS);
    return () => {
      cancelled = true;
      clearInterval(intervalId);
    };
  }, [apiClient, projectId, refreshToken]);

  const loadCheckpoints = useCallback(
    async (pipelineId: string) => {
      setCheckpointsLoading(true);
      setActionError(null);
      try {
        const response = await apiClient.getPipelineCheckpoints(projectId, pipelineId);
        setCheckpoints(response);
      } catch (requestError) {
        setCheckpoints([]);
        setActionError(getErrorMessage(requestError));
      } finally {
        setCheckpointsLoading(false);
      }
    },
    [apiClient, projectId],
  );

  useEffect(() => {
    if (!selectedPipelineId) {
      setCheckpoints([]);
      return;
    }
    void loadCheckpoints(selectedPipelineId);
  }, [selectedPipelineId, loadCheckpoints]);

  const selectedPipeline = useMemo(
    () => pipelines.find((pipeline) => pipeline.id === selectedPipelineId) ?? null,
    [pipelines, selectedPipelineId],
  );
  const progress = selectedPipeline ? getPipelineProgress(selectedPipeline) : null;

  const handlePipelineAction = async (
    action: PipelineActionRequest["action"],
  ) => {
    if (!selectedPipeline) {
      return;
    }

    setActionLoading(true);
    setActionError(null);
    setActionNotice(null);
    try {
      const body: PipelineActionRequest = { action };
      const trimmedMessage = actionMessage.trim();
      if (action === "reject") {
        body.stage = selectedPipeline.current_stage || undefined;
        body.message = trimmedMessage || "人工驳回，请调整后重试。";
      } else if (trimmedMessage) {
        body.message = trimmedMessage;
      }

      const response = await apiClient.applyPipelineAction(
        projectId,
        selectedPipeline.id,
        body,
      );
      setPipelines((current) =>
        current.map((pipeline) =>
          pipeline.id === selectedPipeline.id
            ? {
                ...pipeline,
                status: response.status as Pipeline["status"],
                current_stage: response.current_stage ?? pipeline.current_stage,
              }
            : pipeline,
        ),
      );
      setActionNotice(`动作 ${action} 已提交，状态：${response.status}`);
      await loadCheckpoints(selectedPipeline.id);
    } catch (requestError) {
      setActionError(getErrorMessage(requestError));
    } finally {
      setActionLoading(false);
    }
  };

  return (
    <section className="flex flex-col gap-4">
      <header className="rounded-xl border border-slate-200 bg-white p-4 shadow-sm">
        <h1 className="text-xl font-bold">Pipeline</h1>
        <p className="mt-1 text-sm text-slate-600">
          增强视图：阶段进度、输出区、checkpoint 区与人工动作入口。
        </p>
      </header>

      {error ? (
        <p className="rounded-md border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700">
          {error}
        </p>
      ) : null}

      <section className="rounded-xl border border-slate-200 bg-white p-4 shadow-sm">
        {loading ? (
          <p className="text-sm text-slate-500">加载中...</p>
        ) : pipelines.length === 0 ? (
          <p className="text-sm text-slate-500">当前项目暂无流水线。</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="min-w-full table-auto border-collapse text-sm">
              <thead>
                <tr className="border-b border-slate-200 text-left text-xs text-slate-500">
                  <th className="px-2 py-2 font-semibold">ID</th>
                  <th className="px-2 py-2 font-semibold">Name</th>
                  <th className="px-2 py-2 font-semibold">Status</th>
                  <th className="px-2 py-2 font-semibold">Current Stage</th>
                  <th className="px-2 py-2 font-semibold">Updated</th>
                </tr>
              </thead>
              <tbody>
                {pipelines.map((pipeline) => (
                  <tr
                    key={pipeline.id}
                    data-testid="pipeline-row"
                    className={`cursor-pointer border-b border-slate-100 ${
                      selectedPipelineId === pipeline.id ? "bg-slate-50" : ""
                    }`}
                    onClick={() => {
                      setSelectedPipelineId(pipeline.id);
                    }}
                  >
                    <td className="px-2 py-2 font-mono text-xs">{pipeline.id}</td>
                    <td className="px-2 py-2">{pipeline.name}</td>
                    <td className="px-2 py-2">{pipeline.status}</td>
                    <td className="px-2 py-2">{pipeline.current_stage || "-"}</td>
                    <td className="px-2 py-2">
                      {pipeline.updated_at ? new Date(pipeline.updated_at).toLocaleString("zh-CN") : "-"}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <section className="rounded-xl border border-slate-200 bg-white p-4 shadow-sm">
        <h2 className="text-sm font-semibold">阶段进度</h2>
        {!selectedPipeline || !progress ? (
          <p className="mt-2 text-xs text-slate-500">请选择流水线查看阶段进度。</p>
        ) : (
          <>
            <div className="mt-2 h-3 overflow-hidden rounded-full bg-slate-200">
              <div
                data-testid="pipeline-progress-value"
                className="h-full bg-slate-900 transition-all"
                style={{ width: `${progress.percentage}%` }}
              />
            </div>
            <p className="mt-2 text-xs text-slate-600">
              stage={selectedPipeline.current_stage || "-"} · 进度 {progress.stageText} ·{" "}
              {progress.percentage}%
            </p>
          </>
        )}
      </section>

      <section className="grid gap-4 xl:grid-cols-2">
        <article className="rounded-xl border border-slate-200 bg-white p-4 shadow-sm">
          <h3 className="text-sm font-semibold">输出区</h3>
          {!selectedPipeline ? (
            <p className="mt-2 text-xs text-slate-500">请选择流水线查看输出。</p>
          ) : (
            <div className="mt-2 space-y-2 text-xs">
              <p className="text-slate-600">Artifacts</p>
              <pre className="max-h-52 overflow-auto rounded-md bg-slate-950 p-3 text-slate-100">
                {JSON.stringify(selectedPipeline.artifacts ?? {}, null, 2)}
              </pre>
              <p className="text-slate-600">Error</p>
              <pre className="max-h-24 overflow-auto rounded-md bg-slate-100 p-2 text-slate-800">
                {selectedPipeline.error_message || "-"}
              </pre>
            </div>
          )}
        </article>

        <article className="rounded-xl border border-slate-200 bg-white p-4 shadow-sm">
          <h3 className="text-sm font-semibold">Checkpoint 区</h3>
          {checkpointsLoading ? (
            <p className="mt-2 text-xs text-slate-500">checkpoint 加载中...</p>
          ) : checkpoints.length === 0 ? (
            <p className="mt-2 text-xs text-slate-500">暂无 checkpoint 数据。</p>
          ) : (
            <ul className="mt-2 space-y-1 text-xs text-slate-700">
              {checkpoints.map((checkpoint, index) => (
                <li
                  key={`${checkpoint.stage_name}-${checkpoint.started_at}-${index}`}
                  className="rounded border border-slate-200 px-2 py-1"
                >
                  <span className="font-medium">{checkpoint.stage_name}</span> ·{" "}
                  <span>{checkpoint.status}</span> · retry={checkpoint.retry_count}
                </li>
              ))}
            </ul>
          )}
        </article>
      </section>

      <section className="rounded-xl border border-slate-200 bg-white p-4 shadow-sm">
        <h3 className="text-sm font-semibold">人工动作</h3>
        <p className="mt-1 text-xs text-slate-500">
          最小可用：Approve/Reject/Skip/Abort，调用 Pipeline Action API。
        </p>
        <label htmlFor="pipeline-action-message" className="mt-2 block text-xs text-slate-700">
          动作备注（可选）
        </label>
        <input
          id="pipeline-action-message"
          className="mt-1 w-full rounded-md border border-slate-300 px-2 py-1 text-sm"
          value={actionMessage}
          onChange={(event) => {
            setActionMessage(event.target.value);
          }}
          disabled={!selectedPipeline || actionLoading}
        />
        <div className="mt-3 grid gap-2 sm:grid-cols-2 lg:grid-cols-4">
          <button
            type="button"
            className="rounded-md border border-emerald-300 px-3 py-2 text-sm text-emerald-700 disabled:opacity-50"
            disabled={!selectedPipeline || actionLoading}
            onClick={() => {
              void handlePipelineAction("approve");
            }}
          >
            Approve
          </button>
          <button
            type="button"
            className="rounded-md border border-rose-300 px-3 py-2 text-sm text-rose-700 disabled:opacity-50"
            disabled={!selectedPipeline || actionLoading}
            onClick={() => {
              void handlePipelineAction("reject");
            }}
          >
            Reject
          </button>
          <button
            type="button"
            className="rounded-md border border-amber-300 px-3 py-2 text-sm text-amber-700 disabled:opacity-50"
            disabled={!selectedPipeline || actionLoading}
            onClick={() => {
              void handlePipelineAction("skip");
            }}
          >
            Skip
          </button>
          <button
            type="button"
            className="rounded-md border border-slate-300 px-3 py-2 text-sm disabled:opacity-50"
            disabled={!selectedPipeline || actionLoading}
            onClick={() => {
              void handlePipelineAction("abort");
            }}
          >
            Abort
          </button>
        </div>
        {actionNotice ? (
          <p className="mt-2 rounded-md border border-emerald-200 bg-emerald-50 px-2 py-1 text-xs text-emerald-700">
            {actionNotice}
          </p>
        ) : null}
        {actionError ? (
          <p className="mt-2 rounded-md border border-rose-200 bg-rose-50 px-2 py-1 text-xs text-rose-700">
            {actionError}
          </p>
        ) : null}
      </section>
    </section>
  );
};

export default PipelineView;
