import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { ArrowLeft, Loader2, Sparkles } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Select, SelectItem } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { useWorkbench } from "@/contexts/WorkbenchContext";
import { getErrorMessage } from "@/lib/v2Workbench";
import type { AgentProfile, AnalyzeRequirementResponse, Project } from "@/types/apiV2";

export function RequirementPage() {
  const navigate = useNavigate();
  const { apiClient } = useWorkbench();

  const [description, setDescription] = useState("");
  const [context, setContext] = useState("");
  const [analysis, setAnalysis] = useState<AnalyzeRequirementResponse | null>(null);
  const [projects, setProjects] = useState<Project[]>([]);
  const [profiles, setProfiles] = useState<AgentProfile[]>([]);
  const [selectedProjectIds, setSelectedProjectIds] = useState<number[]>([]);
  const [selectedAgentIds, setSelectedAgentIds] = useState<string[]>([]);
  const [threadTitle, setThreadTitle] = useState("");
  const [meetingMode, setMeetingMode] = useState<"direct" | "concurrent" | "group_chat">("direct");
  const [meetingMaxRounds, setMeetingMaxRounds] = useState(4);
  const [loadingMeta, setLoadingMeta] = useState(false);
  const [analyzing, setAnalyzing] = useState(false);
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      setLoadingMeta(true);
      try {
        const [projectList, profileList] = await Promise.all([
          apiClient.listProjects({ limit: 500, offset: 0 }),
          apiClient.listProfiles(),
        ]);
        if (cancelled) {
          return;
        }
        setProjects(projectList);
        setProfiles(profileList);
      } catch (loadError) {
        if (!cancelled) {
          setError(getErrorMessage(loadError));
        }
      } finally {
        if (!cancelled) {
          setLoadingMeta(false);
        }
      }
    };
    void load();
    return () => {
      cancelled = true;
    };
  }, [apiClient]);

  const projectMap = useMemo(
    () => new Map(projects.map((project) => [project.id, project])),
    [projects],
  );
  const profileMap = useMemo(
    () => new Map(profiles.map((profile) => [profile.id, profile])),
    [profiles],
  );

  const handleAnalyze = async () => {
    if (!description.trim()) {
      setError("需求描述不能为空。");
      return;
    }
    setAnalyzing(true);
    setError(null);
    try {
      const result = await apiClient.analyzeRequirement({
        description: description.trim(),
        context: context.trim() || undefined,
      });
      setAnalysis(result);
      setSelectedProjectIds(result.suggested_thread.context_refs?.map((item) => item.project_id) ?? []);
      setSelectedAgentIds(result.suggested_thread.agents ?? []);
      setThreadTitle(result.suggested_thread.title ?? "");
      setMeetingMode((result.suggested_thread.meeting_mode as "direct" | "concurrent" | "group_chat") ?? "direct");
      setMeetingMaxRounds(result.suggested_thread.meeting_max_rounds ?? 4);
    } catch (analysisError) {
      setError(getErrorMessage(analysisError));
    } finally {
      setAnalyzing(false);
    }
  };

  const toggleProject = (projectId: number) => {
    setSelectedProjectIds((current) =>
      current.includes(projectId) ? current.filter((id) => id !== projectId) : [...current, projectId],
    );
  };

  const toggleAgent = (profileId: string) => {
    setSelectedAgentIds((current) =>
      current.includes(profileId) ? current.filter((id) => id !== profileId) : [...current, profileId],
    );
  };

  const handleCreate = async () => {
    if (!description.trim()) {
      setError("需求描述不能为空。");
      return;
    }
    if (!threadTitle.trim()) {
      setError("Thread 标题不能为空。");
      return;
    }
    setCreating(true);
    setError(null);
    try {
      const selectedMatchedProjects = (analysis?.analysis.matched_projects ?? []).filter((item) =>
        selectedProjectIds.includes(item.project_id),
      );
      const selectedSuggestedAgents = (analysis?.analysis.suggested_agents ?? []).filter((item) =>
        selectedAgentIds.includes(item.profile_id),
      );
      const result = await apiClient.createThreadFromRequirement({
        description: description.trim(),
        context: context.trim() || undefined,
        owner_id: "human",
        analysis: analysis
          ? {
              ...analysis.analysis,
              matched_projects: selectedMatchedProjects,
              suggested_agents: selectedSuggestedAgents,
              suggested_meeting_mode: meetingMode,
            }
          : undefined,
        thread_config: {
          title: threadTitle.trim(),
          context_refs: selectedProjectIds.map((projectId) => ({ project_id: projectId, access: "read" })),
          agents: selectedAgentIds,
          meeting_mode: meetingMode,
          meeting_max_rounds: meetingMaxRounds,
        },
      });
      navigate(`/threads/${result.thread.id}`);
    } catch (createError) {
      setError(getErrorMessage(createError));
    } finally {
      setCreating(false);
    }
  };

  return (
    <div className="flex-1 space-y-6 p-8">
      <div className="flex items-center justify-between gap-4">
        <div>
          <div className="mb-2">
            <Link to="/threads" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
              <ArrowLeft className="h-4 w-4" />
              返回讨论
            </Link>
          </div>
          <h1 className="text-2xl font-bold tracking-tight">提交需求</h1>
          <p className="text-sm text-muted-foreground">
            先分析需求涉及的项目与参与者，再一键创建讨论 thread。
          </p>
        </div>
        {loadingMeta ? <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" /> : null}
      </div>

      {error ? (
        <p className="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">{error}</p>
      ) : null}

      <div className="grid gap-6 lg:grid-cols-[1.05fr_0.95fr]">
        <Card>
          <CardHeader>
            <CardTitle>需求输入</CardTitle>
            <CardDescription>输入原始需求与补充上下文。分析结果不会直接落库，确认后才创建 thread。</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-1.5">
              <label className="text-sm font-medium">需求描述</label>
              <Textarea
                value={description}
                onChange={(event) => setDescription(event.target.value)}
                placeholder="例如：给用户登录系统增加两步验证，后端要支持 OTP 校验，前端要补输入流程。"
                className="min-h-[160px]"
              />
            </div>
            <div className="space-y-1.5">
              <label className="text-sm font-medium">补充上下文</label>
              <Textarea
                value={context}
                onChange={(event) => setContext(event.target.value)}
                placeholder="例如：先兼容旧版短信通道；优先 Web 端。"
                className="min-h-[110px]"
              />
            </div>
            <Button onClick={() => void handleAnalyze()} disabled={analyzing || !description.trim()} className="gap-2">
              {analyzing ? <Loader2 className="h-4 w-4 animate-spin" /> : <Sparkles className="h-4 w-4" />}
              分析需求
            </Button>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>分析结果</CardTitle>
            <CardDescription>你可以调整项目、参与 agent 和讨论模式，再创建 thread。</CardDescription>
          </CardHeader>
          <CardContent className="space-y-5">
            {analysis ? (
              <>
                <div className="rounded-xl border bg-slate-50 px-4 py-3 text-sm text-slate-700">
                  <p className="font-medium text-slate-900">{analysis.analysis.summary}</p>
                  <p className="mt-2">
                    类型：{analysis.analysis.type} · 复杂度：{analysis.analysis.complexity ?? "medium"} · 推荐模式：
                    {" "}
                    {analysis.analysis.suggested_meeting_mode ?? "direct"}
                  </p>
                </div>

                <div className="space-y-2">
                  <label className="text-sm font-medium">Thread 标题</label>
                  <Input value={threadTitle} onChange={(event) => setThreadTitle(event.target.value)} />
                </div>

                <div className="grid gap-4 md:grid-cols-2">
                  <div className="space-y-2">
                    <label className="text-sm font-medium">讨论模式</label>
                    <Select value={meetingMode} onValueChange={(value) => setMeetingMode(value as "direct" | "concurrent" | "group_chat")}>
                      <SelectItem value="direct">direct</SelectItem>
                      <SelectItem value="concurrent">concurrent</SelectItem>
                      <SelectItem value="group_chat">group_chat</SelectItem>
                    </Select>
                  </div>
                  <div className="space-y-2">
                    <label className="text-sm font-medium">最大轮次</label>
                    <Input
                      type="number"
                      min={1}
                      max={12}
                      value={meetingMaxRounds}
                      onChange={(event) => {
                        const next = Number(event.target.value);
                        setMeetingMaxRounds(Number.isFinite(next) ? Math.min(12, Math.max(1, next)) : 4);
                      }}
                    />
                  </div>
                </div>

                <div className="space-y-3">
                  <div>
                    <h3 className="text-sm font-medium">关联项目</h3>
                    <p className="text-xs text-muted-foreground">勾选会被挂到 thread context refs 的项目。</p>
                  </div>
                  <div className="space-y-2">
                    {(analysis.analysis.matched_projects ?? []).map((item) => (
                      <label key={item.project_id} className="flex items-start gap-3 rounded-lg border p-3 text-sm">
                        <input
                          type="checkbox"
                          checked={selectedProjectIds.includes(item.project_id)}
                          onChange={() => toggleProject(item.project_id)}
                        />
                        <div>
                          <div className="font-medium">{projectMap.get(item.project_id)?.name ?? item.project_name}</div>
                          <div className="text-xs text-muted-foreground">{item.reason}</div>
                        </div>
                      </label>
                    ))}
                  </div>
                </div>

                <div className="space-y-3">
                  <div>
                    <h3 className="text-sm font-medium">参与 Agents</h3>
                    <p className="text-xs text-muted-foreground">初始消息会优先派发给这里勾选的 agents。</p>
                  </div>
                  <div className="space-y-2">
                    {(analysis.analysis.suggested_agents ?? []).map((item) => (
                      <label key={item.profile_id} className="flex items-start gap-3 rounded-lg border p-3 text-sm">
                        <input
                          type="checkbox"
                          checked={selectedAgentIds.includes(item.profile_id)}
                          onChange={() => toggleAgent(item.profile_id)}
                        />
                        <div>
                          <div className="font-medium">{profileMap.get(item.profile_id)?.name ?? item.profile_id}</div>
                          <div className="text-xs text-muted-foreground">{item.reason}</div>
                        </div>
                      </label>
                    ))}
                  </div>
                </div>

                {(analysis.analysis.risks ?? []).length > 0 ? (
                  <div className="space-y-2">
                    <h3 className="text-sm font-medium">风险提示</h3>
                    <ul className="space-y-1 text-sm text-muted-foreground">
                      {analysis.analysis.risks?.map((risk) => (
                        <li key={risk}>- {risk}</li>
                      ))}
                    </ul>
                  </div>
                ) : null}

                <Button onClick={() => void handleCreate()} disabled={creating} className="w-full">
                  {creating ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}
                  创建讨论 Thread
                </Button>
              </>
            ) : (
              <div className="rounded-xl border border-dashed px-4 py-12 text-center text-sm text-muted-foreground">
                提交需求后，这里会显示匹配项目、推荐 agents 与建议的会议模式。
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
