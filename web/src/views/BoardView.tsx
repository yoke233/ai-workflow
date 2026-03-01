import { useEffect, useMemo, useState } from "react";
import type { ApiClient } from "../lib/apiClient";
import type { TaskItemStatus, TaskPlan } from "../types/workflow";

interface BoardViewProps {
  apiClient: ApiClient;
  projectId: string;
  refreshToken: number;
}

export type BoardStatus = "pending" | "ready" | "running" | "done" | "failed";

export interface BoardTask {
  id: string;
  plan_id: string;
  plan_name: string;
  title: string;
  status: BoardStatus;
  pipeline_id: string;
  github_issue_number?: number;
  github_issue_url?: string;
}

type TaskActionType = "retry" | "skip" | "abort";

interface ContextMenuState {
  taskId: string;
  x: number;
  y: number;
}

export const BOARD_COLUMNS: BoardStatus[] = [
  "pending",
  "ready",
  "running",
  "done",
  "failed",
];

const BOARD_STATUS_LABELS: Record<BoardStatus, string> = {
  pending: "Pending",
  ready: "Ready",
  running: "Running",
  done: "Done",
  failed: "Failed",
};

const DROP_ACTION_MAP: Record<BoardStatus, TaskActionType | null> = {
  pending: null,
  ready: "retry",
  running: "retry",
  done: "skip",
  failed: "abort",
};

const TASK_ACTIONS: TaskActionType[] = ["retry", "skip", "abort"];

export const toBoardStatus = (status: TaskItemStatus): BoardStatus => {
  switch (status) {
    case "pending":
      return "pending";
    case "ready":
      return "ready";
    case "running":
      return "running";
    case "done":
      return "done";
    case "failed":
      return "failed";
    case "blocked_by_failure":
      return "failed";
    case "skipped":
      return "done";
    default:
      return "pending";
  }
};

const toBoardStatusFromUnknown = (status: string, fallback: BoardStatus): BoardStatus => {
  const known: TaskItemStatus[] = [
    "pending",
    "ready",
    "running",
    "done",
    "failed",
    "skipped",
    "blocked_by_failure",
  ];
  if (!known.includes(status as TaskItemStatus)) {
    return fallback;
  }
  return toBoardStatus(status as TaskItemStatus);
};

export const groupBoardTasks = (
  tasks: BoardTask[],
): Record<BoardStatus, BoardTask[]> => {
  const grouped: Record<BoardStatus, BoardTask[]> = {
    pending: [],
    ready: [],
    running: [],
    done: [],
    failed: [],
  };
  tasks.forEach((task) => {
    grouped[task.status].push(task);
  });
  return grouped;
};

const getErrorMessage = (error: unknown): string => {
  if (error instanceof Error && error.message.trim().length > 0) {
    return error.message;
  }
  return "请求失败，请稍后重试";
};

const PAGE_LIMIT = 50;
const REFRESH_INTERVAL_MS = 10_000;

const BoardView = ({ apiClient, projectId, refreshToken }: BoardViewProps) => {
  const [tasks, setTasks] = useState<BoardTask[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [selectedTaskId, setSelectedTaskId] = useState<string | null>(null);
  const [draggingTaskId, setDraggingTaskId] = useState<string | null>(null);
  const [contextMenu, setContextMenu] = useState<ContextMenuState | null>(null);
  const [actionLoadingTaskId, setActionLoadingTaskId] = useState<string | null>(null);
  const [actionNotice, setActionNotice] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    let inFlight = false;
    const loadTasks = async () => {
      if (inFlight) {
        return;
      }
      inFlight = true;
      setLoading(true);
      setError(null);
      try {
        const allPlans: TaskPlan[] = [];
        let offset = 0;
        while (true) {
          const response = await apiClient.listPlans(projectId, {
            limit: PAGE_LIMIT,
            offset,
          });
          if (cancelled) {
            return;
          }
          allPlans.push(...response.items);
          const currentCount = response.items.length;
          if (currentCount === 0) {
            break;
          }
          offset += currentCount;
          if (currentCount < PAGE_LIMIT) {
            break;
          }
        }
        const flattened: BoardTask[] = allPlans.flatMap((plan) =>
          (plan.tasks ?? []).map((task) => ({
            id: task.id,
            plan_id: plan.id,
            plan_name: plan.name || plan.id,
            title: task.title,
            status: toBoardStatus(task.status),
            pipeline_id: task.pipeline_id,
            github_issue_number: task.github?.issue_number,
            github_issue_url: task.github?.issue_url,
          })),
        );
        if (!cancelled) {
          setTasks(flattened);
          setSelectedTaskId((current) =>
            current && flattened.some((task) => task.id === current) ? current : null,
          );
        }
      } catch (requestError) {
        if (!cancelled) {
          setTasks([]);
          setSelectedTaskId(null);
          setError(getErrorMessage(requestError));
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
        inFlight = false;
      }
    };

    void loadTasks();
    // Fallback refresh for non-PlanView scenarios where plan-scoped WS events may be missed.
    const intervalId = setInterval(() => {
      void loadTasks();
    }, REFRESH_INTERVAL_MS);
    return () => {
      cancelled = true;
      clearInterval(intervalId);
    };
  }, [apiClient, projectId, refreshToken]);

  useEffect(() => {
    const closeMenu = () => {
      setContextMenu(null);
    };
    window.addEventListener("click", closeMenu);
    return () => {
      window.removeEventListener("click", closeMenu);
    };
  }, []);

  const groupedTasks = useMemo(() => groupBoardTasks(tasks), [tasks]);
  const selectedTask = selectedTaskId
    ? tasks.find((task) => task.id === selectedTaskId) ?? null
    : null;

  const runTaskAction = async (task: BoardTask, action: TaskActionType) => {
    setActionLoadingTaskId(task.id);
    setActionError(null);
    setActionNotice(null);
    try {
      const response = await apiClient.applyTaskAction(projectId, task.plan_id, task.id, {
        action,
      });
      const nextStatus = toBoardStatusFromUnknown(String(response.status), task.status);
      setTasks((current) =>
        current.map((item) =>
          item.id === task.id
            ? {
                ...item,
                status: nextStatus,
              }
            : item,
        ),
      );
      setSelectedTaskId(task.id);
      setActionNotice(`任务 ${task.title} 已执行 ${action}，当前状态：${nextStatus}`);
    } catch (requestError) {
      setActionError(getErrorMessage(requestError));
    } finally {
      setActionLoadingTaskId(null);
    }
  };

  const handleColumnDrop = (column: BoardStatus) => {
    if (!draggingTaskId) {
      return;
    }
    const task = tasks.find((item) => item.id === draggingTaskId);
    setDraggingTaskId(null);
    if (!task) {
      return;
    }
    const action = DROP_ACTION_MAP[column];
    if (!action) {
      setActionNotice("拖拽仅支持到 Ready/Running（retry）、Done（skip）、Failed（abort）。");
      return;
    }
    void runTaskAction(task, action);
  };

  return (
    <section className="flex flex-col gap-4">
      <header className="rounded-xl border border-slate-200 bg-white p-4 shadow-sm">
        <h1 className="text-xl font-bold">Board</h1>
        <p className="mt-1 text-sm text-slate-600">
          基于真实 plans/tasks 的看板分组（pending / ready / running / done / failed）。
        </p>
        <p className="mt-2 text-xs text-slate-500">
          数据来源: GET /api/v1/projects/:projectID/plans
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
        ) : selectedTask ? (
          <div className="text-sm text-slate-700">
            <span className="font-semibold text-slate-900">当前选中：</span>
            {selectedTask.title} · plan={selectedTask.plan_name} · status=
            {selectedTask.status}
          </div>
        ) : (
          <p className="text-sm text-slate-500">点击任务卡片查看详情。</p>
        )}
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

      <section className="grid gap-3 md:grid-cols-2 xl:grid-cols-5">
        {BOARD_COLUMNS.map((column) => (
          <article
            key={column}
            data-testid={`board-column-${column}`}
            className={`flex min-h-72 flex-col rounded-xl border bg-white p-3 shadow-sm ${
              draggingTaskId ? "border-slate-400" : "border-slate-200"
            }`}
            onDragOver={(event) => {
              if (DROP_ACTION_MAP[column]) {
                event.preventDefault();
              }
            }}
            onDrop={(event) => {
              event.preventDefault();
              handleColumnDrop(column);
            }}
          >
            <header className="mb-3 flex items-center justify-between">
              <h2 className="text-sm font-semibold">{BOARD_STATUS_LABELS[column]}</h2>
              <span className="rounded-full bg-slate-100 px-2 py-0.5 text-xs text-slate-700">
                {groupedTasks[column].length}
              </span>
            </header>
            <div className="flex flex-1 flex-col gap-2">
              {groupedTasks[column].length === 0 ? (
                <p className="rounded-lg border border-dashed border-slate-300 bg-slate-50 px-2 py-3 text-xs text-slate-500">
                  空列
                </p>
              ) : (
                groupedTasks[column].map((task) => (
                  <button
                    key={task.id}
                    type="button"
                    data-testid="board-task"
                    draggable
                    className={`rounded-lg border px-2 py-2 text-left text-xs transition ${
                      selectedTaskId === task.id
                        ? "border-slate-900 bg-slate-900 text-white"
                        : "border-slate-300 bg-white text-slate-800 hover:bg-slate-50"
                    } ${actionLoadingTaskId === task.id ? "opacity-60" : ""}`}
                    onClick={() => {
                      setSelectedTaskId((current) =>
                        current === task.id ? null : task.id,
                      );
                    }}
                    onDragStart={() => {
                      setDraggingTaskId(task.id);
                      setSelectedTaskId(task.id);
                    }}
                    onDragEnd={() => {
                      setDraggingTaskId(null);
                    }}
                    onContextMenu={(event) => {
                      event.preventDefault();
                      setSelectedTaskId(task.id);
                      setContextMenu({
                        taskId: task.id,
                        x: event.clientX,
                        y: event.clientY,
                      });
                    }}
                  >
                    <p className="font-semibold">{task.title}</p>
                    <p className="mt-1 opacity-80">plan={task.plan_name}</p>
                    {task.pipeline_id ? (
                      <p className="mt-1 opacity-80">pipeline={task.pipeline_id}</p>
                    ) : null}
                    {task.github_issue_url ? (
                      <a
                        href={task.github_issue_url}
                        target="_blank"
                        rel="noreferrer"
                        data-testid="board-github-issue-icon"
                        className="mt-1 inline-flex rounded border border-blue-200 bg-blue-50 px-1.5 py-0.5 text-[10px] font-medium text-blue-700"
                      >
                        {task.github_issue_number ? `GH #${task.github_issue_number}` : "GH Issue"}
                      </a>
                    ) : null}
                  </button>
                ))
              )}
            </div>
          </article>
        ))}
      </section>

      {contextMenu ? (
        <div
          role="menu"
          className="fixed z-50 min-w-28 rounded-md border border-slate-300 bg-white p-1 shadow-lg"
          style={{
            left: contextMenu.x,
            top: contextMenu.y,
          }}
          onClick={(event) => {
            event.stopPropagation();
          }}
        >
          {TASK_ACTIONS.map((action) => {
            const task = tasks.find((item) => item.id === contextMenu.taskId);
            return (
              <button
                key={action}
                type="button"
                className="block w-full rounded px-2 py-1 text-left text-xs hover:bg-slate-100"
                onClick={() => {
                  setContextMenu(null);
                  if (!task) {
                    return;
                  }
                  void runTaskAction(task, action);
                }}
              >
                {action}
              </button>
            );
          })}
        </div>
      ) : null}
    </section>
  );
};

export default BoardView;
