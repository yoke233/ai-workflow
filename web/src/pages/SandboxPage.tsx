import { useEffect, useMemo, useState } from "react";
import { Shield, RefreshCw, CheckCircle2, XCircle, Cpu, Loader2, Save } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Select } from "@/components/ui/select";
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table";
import { useWorkbench } from "@/contexts/WorkbenchContext";
import { getErrorMessage } from "@/lib/v2Workbench";
import type { SandboxSupportResponse } from "@/types/system";

const PROVIDER_LABELS: Record<string, string> = {
  noop: "未启用",
  home_dir: "home_dir",
  litebox: "litebox",
  boxlite: "boxlite",
  docker: "docker",
  bwrap: "bwrap",
};

export function SandboxPage() {
  const { apiClient } = useWorkbench();
  const [data, setData] = useState<SandboxSupportResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [enabled, setEnabled] = useState(false);
  const [provider, setProvider] = useState("home_dir");

  const providers = useMemo(
    () => Object.entries(data?.providers ?? {}).sort(([left], [right]) => left.localeCompare(right)),
    [data],
  );

  const hydrateForm = (next: SandboxSupportResponse) => {
    setData(next);
    setEnabled(next.enabled);
    setProvider(next.configured_provider || "home_dir");
  };

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const next = await apiClient.getSandboxSupport();
      hydrateForm(next);
    } catch (loadError) {
      setError(getErrorMessage(loadError));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
  }, []);

  const save = async () => {
    setSaving(true);
    setError(null);
    try {
      const next = await apiClient.updateSandboxSupport({
        enabled,
        provider,
      });
      hydrateForm(next);
    } catch (saveError) {
      setError(getErrorMessage(saveError));
    } finally {
      setSaving(false);
    }
  };

  const changed = data != null && (enabled !== data.enabled || provider !== data.configured_provider);
  const selectedSupport = data?.providers?.[provider];

  return (
    <div className="flex-1 space-y-6 p-8">
      <div className="flex items-center justify-between gap-4">
        <div>
          <div className="flex items-center gap-2">
            <Shield className="h-6 w-6 text-primary" />
            <h1 className="text-2xl font-bold tracking-tight">沙盒状态</h1>
          </div>
          <p className="mt-2 text-sm text-muted-foreground">查询当前运行环境支持哪些沙盒 provider，并直接在前端开启、关闭或切换。</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => void load()} disabled={loading || saving}>
            <RefreshCw className={`mr-2 h-4 w-4 ${loading ? "animate-spin" : ""}`} />
            刷新
          </Button>
          <Button onClick={() => void save()} disabled={loading || saving || !changed}>
            {saving ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <Save className="mr-2 h-4 w-4" />}
            保存配置
          </Button>
        </div>
      </div>

      {error ? <p className="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">{error}</p> : null}

      <div className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base">当前状态</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2">
            <div className="flex items-center justify-between gap-3">
              <span className="text-sm text-muted-foreground">沙盒开关</span>
              <Badge variant={data?.enabled ? "default" : "secondary"}>
                {data?.enabled ? "已开启" : "未开启"}
              </Badge>
            </div>
            <div className="flex items-center justify-between gap-3">
              <span className="text-sm text-muted-foreground">配置 provider</span>
              <Badge variant="outline">{data?.configured_provider ?? "-"}</Badge>
            </div>
            <div className="flex items-center justify-between gap-3">
              <span className="text-sm text-muted-foreground">当前 provider</span>
              <Badge variant="outline">{PROVIDER_LABELS[data?.current_provider ?? ""] ?? data?.current_provider ?? "-"}</Badge>
            </div>
            <div className="flex items-center justify-between gap-3">
              <span className="text-sm text-muted-foreground">当前 provider 可用</span>
              <Badge variant={data?.current_supported ? "success" : "destructive"}>
                {data?.current_supported ? "支持" : "不支持"}
              </Badge>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="flex items-center gap-2 text-base">
              <Cpu className="h-4 w-4" />
              运行平台
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-2">
            <div className="flex items-center justify-between gap-3">
              <span className="text-sm text-muted-foreground">OS</span>
              <span className="font-mono text-sm">{data?.os ?? "-"}</span>
            </div>
            <div className="flex items-center justify-between gap-3">
              <span className="text-sm text-muted-foreground">Arch</span>
              <span className="font-mono text-sm">{data?.arch ?? "-"}</span>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base">支持概览</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2">
            <div className="flex items-center justify-between gap-3">
              <span className="text-sm text-muted-foreground">已识别 provider</span>
              <Badge variant="secondary">{providers.length}</Badge>
            </div>
            <div className="flex items-center justify-between gap-3">
              <span className="text-sm text-muted-foreground">可用 provider</span>
              <Badge variant="outline">
                {providers.filter(([, support]) => support.supported).length}
              </Badge>
            </div>
            <div className="flex items-center justify-between gap-3">
              <span className="text-sm text-muted-foreground">已接入 provider</span>
              <Badge variant="outline">
                {providers.filter(([, support]) => support.implemented).length}
              </Badge>
            </div>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">前端开关</CardTitle>
          <CardDescription>修改后会写回 runtime 配置；后续新建的 ACP 进程将按新配置准备沙盒。</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-4 md:grid-cols-2">
          <label className="space-y-2">
            <span className="text-sm font-medium">启用沙盒</span>
            <button
              type="button"
              onClick={() => setEnabled((current) => !current)}
              className={[
                "flex h-11 w-full items-center justify-between rounded-[10px] border px-4 text-sm font-medium transition-colors",
                enabled
                  ? "border-emerald-200 bg-emerald-50 text-emerald-700"
                  : "border-slate-200 bg-white text-slate-600",
              ].join(" ")}
            >
              <span>{enabled ? "开启" : "关闭"}</span>
              <span>{enabled ? "ON" : "OFF"}</span>
            </button>
          </label>

          <label className="space-y-2">
            <span className="text-sm font-medium">配置 provider</span>
            <Select value={provider} onChange={(event) => setProvider(event.target.value)}>
              {providers.map(([name, support]) => (
                <option key={name} value={name}>
                  {name}
                  {support.supported ? "" : " (当前平台不支持)"}
                </option>
              ))}
            </Select>
          </label>

          <div className="rounded-xl border border-slate-200 bg-slate-50 p-4 md:col-span-2">
            <div className="flex items-center justify-between gap-3">
              <span className="text-sm text-slate-500">选中 provider 的平台支持情况</span>
              <Badge variant={selectedSupport?.supported ? "success" : "warning"}>
                {selectedSupport?.supported ? "可用" : "不可用"}
              </Badge>
            </div>
            <div className="mt-3 flex items-center justify-between gap-3">
              <span className="text-sm text-slate-500">选中 provider 的项目接入情况</span>
              <Badge variant={selectedSupport?.implemented ? "success" : "outline"}>
                {selectedSupport?.implemented ? "已接入" : "未接入"}
              </Badge>
            </div>
            <p className="mt-2 text-sm text-slate-600">{selectedSupport?.reason || "无附加说明"}</p>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Provider 列表</CardTitle>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Provider</TableHead>
                <TableHead>宿主支持</TableHead>
                <TableHead>项目接入</TableHead>
                <TableHead>说明</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {providers.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={4} className="text-center text-muted-foreground">
                    暂无 provider 信息
                  </TableCell>
                </TableRow>
              ) : providers.map(([name, support]) => (
                <TableRow key={name}>
                  <TableCell className="font-medium">
                    <div className="flex items-center gap-2">
                      <code className="rounded bg-muted px-1.5 py-0.5 text-xs font-mono">{name}</code>
                      {name === data?.current_provider ? <Badge variant="secondary">当前</Badge> : null}
                    </div>
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center gap-2">
                      {support.supported ? (
                        <CheckCircle2 className="h-4 w-4 text-emerald-600" />
                      ) : (
                        <XCircle className="h-4 w-4 text-rose-600" />
                      )}
                      <span>{support.supported ? "支持" : "不支持"}</span>
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge variant={support.implemented ? "success" : "outline"}>
                      {support.implemented ? "已接入" : "未接入"}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {support.reason || "无附加说明"}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  );
}
