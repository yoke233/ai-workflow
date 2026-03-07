import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import ChatView from "./views/ChatView";
import A2AChatView from "./views/A2AChatView";
import BoardView from "./views/BoardView";
import ProjectAdminPanel from "./components/ProjectAdminPanel";
import { SettingsPanel } from "./components/SettingsPanel";
import SystemEventBanner from "./components/SystemEventBanner";
import { createApiClient, type ApiClient } from "./lib/apiClient";
import { createA2AClient, type A2AClient } from "./lib/a2aClient";
import { createWsClient, type WsClient } from "./lib/wsClient";
import type { WsEnvelope } from "./types/ws";
import type { Project } from "./types/workflow";

type AppView = "chat" | "board";

const VIEW_LABELS: Record<AppView, string> = {
  chat: "Chat",
  board: "Issues",
};

const ISSUE_RUN_EVENT_TYPES = new Set([
  "run_started",
  "run_update",
  "run_completed",
  "run_failed",
  "run_cancelled",
  "auto_merged",
  "issue_created",
  "issue_reviewing",
  "review_done",
  "issue_approved",
  "issue_queued",
  "issue_ready",
  "issue_executing",
  "issue_done",
  "issue_failed",
  "issue_dependency_changed",
]);

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || "/api/v1";
const TOKEN_STORAGE_KEY = "ai-workflow-api-token";

type TokenSource = "query" | "storage" | "missing";

interface ResolvedToken {
  token: string | null;
  source: TokenSource;
}

const resolveA2AEnabledFromEnv = (): boolean => {
  const raw = String(import.meta.env.VITE_A2A_ENABLED ?? "").trim().toLowerCase();
  return raw === "true" || raw === "1" || raw === "on";
};

const parseViewFromLocation = (): AppView => {
  if (typeof window === "undefined") {
    return "chat";
  }
  const params = new URLSearchParams(window.location.search);
  const view = params.get("view");
  if (view === "chat" || view === "board") {
    return view;
  }
  if (params.get("issue")) {
    return "board";
  }
  return "chat";
};

const readTokenFromStorage = (): string | null => {
  if (typeof window === "undefined") {
    return null;
  }
  const raw = window.localStorage.getItem(TOKEN_STORAGE_KEY);
  if (!raw) {
    return null;
  }
  const token = raw.trim();
  return token.length > 0 ? token : null;
};

const persistTokenToStorage = (token: string): void => {
  if (typeof window === "undefined") {
    return;
  }
  window.localStorage.setItem(TOKEN_STORAGE_KEY, token);
};

const resolveTokenFromLocation = (): ResolvedToken => {
  if (typeof window === "undefined") {
    return { token: null, source: "missing" };
  }

  const params = new URLSearchParams(window.location.search);
  const queryToken = (params.get("token") ?? "").trim();
  if (queryToken.length > 0) {
    return { token: queryToken, source: "query" };
  }

  const storageToken = readTokenFromStorage();
  if (storageToken) {
    return { token: storageToken, source: "storage" };
  }

  return { token: null, source: "missing" };
};

const cleanupTokenFromUrlToHome = (): void => {
  if (typeof window === "undefined") {
    return;
  }

  const url = new URL(window.location.href);
  url.searchParams.delete("token");
  url.searchParams.delete("issue");
  url.searchParams.set("view", "chat");
  window.history.replaceState(null, "", `${url.pathname}${url.search}${url.hash}`);
};

const getErrorMessage = (error: unknown): string => {
  if (error instanceof Error && error.message.trim().length > 0) {
    return error.message;
  }
  return "请求失败，请稍后重试";
};

interface ViewProps {
  apiClient: ApiClient;
  a2aClient: A2AClient;
  wsClient: WsClient;
  projectId: string;
  refreshToken: number;
  a2aEnabled: boolean;
}

const renderView = ({ apiClient, a2aClient, wsClient, projectId, refreshToken, a2aEnabled }: ViewProps, view: AppView) => {
  switch (view) {
    case "chat":
      return a2aEnabled ? (
        <A2AChatView a2aClient={a2aClient} wsClient={wsClient} projectId={projectId} />
      ) : (
        <ChatView apiClient={apiClient} wsClient={wsClient} projectId={projectId} />
      );
    case "board":
      return (
        <BoardView
          apiClient={apiClient}
          projectId={projectId}
          refreshToken={refreshToken}
        />
      );
    default:
      return null;
  }
};

interface AppProps {
  a2aEnabledOverride?: boolean;
}

const App = ({ a2aEnabledOverride }: AppProps = {}) => {
  const a2aEnabled = a2aEnabledOverride ?? resolveA2AEnabledFromEnv();
  const tokenRef = useRef<string | null>(null);
  const apiClient = useMemo(
    () =>
      createApiClient({
        baseUrl: API_BASE_URL,
        getToken: () => tokenRef.current,
      }),
    [],
  );
  const wsClient = useMemo(
    () =>
      createWsClient({
        baseUrl: API_BASE_URL,
        getToken: () => tokenRef.current,
      }),
    [],
  );
  const a2aClient = useMemo(
    () =>
      createA2AClient({
        baseUrl: import.meta.env.VITE_A2A_BASE_URL || "/api/v1",
        getToken: () => tokenRef.current,
      }),
    [],
  );

  const [authStatus, setAuthStatus] = useState<"checking" | "ready" | "error">("checking");
  const [authError, setAuthError] = useState<string | null>(null);
  const [projects, setProjects] = useState<Project[]>([]);
  const [projectsLoading, setProjectsLoading] = useState(false);
  const [projectsError, setProjectsError] = useState<string | null>(null);
  const [selectedProjectId, setSelectedProjectId] = useState<string | null>(null);
  const [activeView, setActiveView] = useState<AppView>(() => parseViewFromLocation());
  const [refreshToken, setRefreshToken] = useState(0);
  const [wsStatus, setWsStatus] = useState(wsClient.getStatus());
  const [settingsOpen, setSettingsOpen] = useState(false);

  const selectedProjectIdRef = useRef<string | null>(selectedProjectId);
  useEffect(() => {
    selectedProjectIdRef.current = selectedProjectId;
  }, [selectedProjectId]);

  const applyProjects = useCallback((nextProjects: Project[], preferredProjectId?: string | null) => {
    setProjects(nextProjects);
    setSelectedProjectId((current) => {
      if (
        preferredProjectId &&
        nextProjects.some((project) => project.id === preferredProjectId)
      ) {
        return preferredProjectId;
      }
      if (current && nextProjects.some((project) => project.id === current)) {
        return current;
      }
      return nextProjects[0]?.id ?? null;
    });
  }, []);

  const fetchProjects = useCallback(async (): Promise<Project[]> => {
    const listedProjects = await apiClient.listProjects();
    return Array.isArray(listedProjects) ? listedProjects : [];
  }, [apiClient]);

  const loadProjects = useCallback(async (preferredProjectId?: string | null) => {
    if (authStatus !== "ready") {
      return;
    }

    setProjectsLoading(true);
    setProjectsError(null);

    try {
      const nextProjects = await fetchProjects();
      applyProjects(nextProjects, preferredProjectId);
    } catch (error) {
      setProjectsError(getErrorMessage(error));
    } finally {
      setProjectsLoading(false);
    }
  }, [applyProjects, authStatus, fetchProjects]);

  useEffect(() => {
    const resolvedToken = resolveTokenFromLocation();
    if (!resolvedToken.token) {
      setAuthStatus("error");
      setAuthError("缺少访问 token，请使用 ?token=xxxx 访问。");
      return;
    }

    tokenRef.current = resolvedToken.token;

    let cancelled = false;
    const bootstrap = async (): Promise<void> => {
      setAuthStatus("checking");
      setAuthError(null);
      setProjectsLoading(true);
      setProjectsError(null);

      try {
        const nextProjects = await fetchProjects();
        if (cancelled) {
          return;
        }
        applyProjects(nextProjects);
        if (resolvedToken.source === "query" && resolvedToken.token) {
          persistTokenToStorage(resolvedToken.token);
          setActiveView("chat");
          cleanupTokenFromUrlToHome();
        }
        setAuthStatus("ready");
      } catch (error) {
        if (cancelled) {
          return;
        }
        setProjects([]);
        setSelectedProjectId(null);
        setAuthStatus("error");
        setAuthError(`Token 校验失败：${getErrorMessage(error)}`);
      } finally {
        if (!cancelled) {
          setProjectsLoading(false);
        }
      }
    };

    void bootstrap();

    return () => {
      cancelled = true;
    };
  }, [applyProjects, fetchProjects]);

  useEffect(() => {
    if (authStatus !== "ready") {
      return;
    }

    const unsubscribeStatus = wsClient.onStatusChange((status) => {
      setWsStatus(status);
    });
    const unsubscribeAll = wsClient.subscribe<WsEnvelope>("*", (payload) => {
      const envelope = payload as WsEnvelope;
      if (!ISSUE_RUN_EVENT_TYPES.has(envelope.type)) {
        return;
      }
      const projectID = selectedProjectIdRef.current;
      if (
        projectID &&
        envelope.project_id &&
        envelope.project_id.trim().length > 0 &&
        envelope.project_id !== projectID
      ) {
        return;
      }
      setRefreshToken((current) => current + 1);
    });

    wsClient.connect();

    return () => {
      unsubscribeAll();
      unsubscribeStatus();
      wsClient.disconnect(1000, "app_unmount");
    };
  }, [authStatus, wsClient]);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }
    const onPopState = () => {
      setActiveView(parseViewFromLocation());
    };
    window.addEventListener("popstate", onPopState);
    return () => {
      window.removeEventListener("popstate", onPopState);
    };
  }, []);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }
    const url = new URL(window.location.href);
    const current = url.searchParams.get("view");
    if (current === activeView) {
      return;
    }
    url.searchParams.set("view", activeView);
    if (activeView !== "board") {
      url.searchParams.delete("issue");
    }
    window.history.replaceState(null, "", `${url.pathname}${url.search}${url.hash}`);
  }, [activeView]);

  const selectedProject = selectedProjectId
    ? projects.find((project) => project.id === selectedProjectId) ?? null
    : null;

  if (authStatus !== "ready") {
    return (
      <main className="min-h-screen bg-slate-100 px-4 py-6 text-slate-900 md:px-6">
        <div className="mx-auto flex w-full max-w-3xl flex-col gap-4">
          <section className="rounded-xl border border-slate-200 bg-white p-6 shadow-sm">
            <h1 className="text-2xl font-bold">AI Workflow Workbench</h1>
            <p className="mt-2 text-sm text-slate-600">
              {authStatus === "checking" ? "正在验证访问 token..." : authError ?? "Token 校验失败"}
            </p>
            {authStatus === "error" ? (
              <p className="mt-3 rounded-md border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700">
                请使用带 token 的访问链接重新进入，例如：<code>?token=xxxx</code>
              </p>
            ) : null}
          </section>
        </div>
      </main>
    );
  }

  return (
    <main className="min-h-screen bg-slate-100 px-4 py-6 text-slate-900 md:px-6">
      <SystemEventBanner wsClient={wsClient} />
      <div className="mx-auto flex w-full max-w-7xl flex-col gap-4">
        <header className="rounded-xl border border-slate-200 bg-white p-4 shadow-sm">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div>
              <h1 className="text-2xl font-bold">AI Workflow Workbench</h1>
              <p className="mt-1 text-sm text-slate-600">
                API: <code>{API_BASE_URL}</code> · WS 状态:{" "}
                <span className="font-semibold">{wsStatus}</span>
              </p>
            </div>
            <div className="flex items-center gap-2">
              <label htmlFor="project-select" className="text-sm font-medium">
                当前项目
              </label>
              <select
                id="project-select"
                className="min-w-64 rounded-md border border-slate-300 bg-white px-3 py-2 text-sm"
                value={selectedProjectId ?? ""}
                onChange={(event) => {
                  const value = event.target.value;
                  setSelectedProjectId(value.length > 0 ? value : null);
                }}
                disabled={projectsLoading}
              >
                {projects.length === 0 ? (
                  <option value="">暂无项目</option>
                ) : (
                  projects.map((project) => (
                    <option key={project.id} value={project.id}>
                      {project.name}
                    </option>
                  ))
                )}
              </select>
              <button
                type="button"
                className="rounded-md border border-slate-300 bg-white px-3 py-2 text-sm hover:bg-slate-50"
                onClick={() => {
                  void loadProjects();
                }}
                disabled={projectsLoading}
              >
                刷新项目
              </button>
              <div className="relative">
                <button
                  type="button"
                  title="外观设置"
                  aria-label="外观设置"
                  onClick={() => setSettingsOpen((v) => !v)}
                  className="rounded-md border border-slate-300 bg-white px-3 py-2 text-sm hover:bg-slate-50"
                >
                  ⚙️
                </button>
                <SettingsPanel open={settingsOpen} onClose={() => setSettingsOpen(false)} />
              </div>
            </div>
          </div>

          {selectedProject ? (
            <p className="mt-2 text-xs text-slate-500">
              项目 ID: {selectedProject.id} · Repo: {selectedProject.repo_path}
            </p>
          ) : null}

          {projectsError ? (
            <p className="mt-2 rounded-md border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700">
              加载项目失败：{projectsError}
            </p>
          ) : null}

          <ProjectAdminPanel
            apiClient={apiClient}
            wsClient={wsClient}
            wsStatus={wsStatus}
            onProjectCreated={async (projectId) => {
              await loadProjects(projectId);
            }}
          />
        </header>

        <nav className="rounded-xl border border-slate-200 bg-white p-2 shadow-sm">
          <div className="flex flex-wrap gap-2">
            {(Object.keys(VIEW_LABELS) as AppView[]).map((view) => (
              <button
                key={view}
                type="button"
                onClick={() => {
                  setActiveView(view);
                }}
                className={`rounded-lg px-4 py-2 text-sm font-semibold transition ${
                  activeView === view
                    ? "accent-bg"
                    : "bg-slate-100 text-slate-700 hover:bg-slate-200"
                }`}
              >
                {VIEW_LABELS[view]}
              </button>
            ))}
          </div>
        </nav>

        {!selectedProjectId ? (
          <section className="rounded-xl border border-slate-200 bg-white p-6 text-sm text-slate-600 shadow-sm">
            暂无可用项目。请先在后端创建项目，或点击“刷新项目”重试。
          </section>
        ) : (
          renderView(
            {
              apiClient,
              a2aClient,
              wsClient,
              projectId: selectedProjectId,
              refreshToken,
              a2aEnabled,
            },
            activeView,
          )
        )}
      </div>
    </main>
  );
};

export default App;
