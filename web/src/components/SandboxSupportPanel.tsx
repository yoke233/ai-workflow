import { RefreshCcw, ShieldCheck, ShieldOff } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { SandboxSupportResponse } from "@/types/system";

const providerLabels: Record<string, string> = {
  noop: "未启用",
  home_dir: "home_dir",
  litebox: "litebox",
  boxlite: "boxlite",
  docker: "docker",
  bwrap: "bwrap",
};

const statusLabel = (provider: string): string => providerLabels[provider] ?? provider;

interface SandboxSupportPanelProps {
  report: SandboxSupportResponse | null;
  loading: boolean;
  error: string | null;
  onRefresh: () => void;
}

export function SandboxSupportPanel({
  report,
  loading,
  error,
  onRefresh,
}: SandboxSupportPanelProps) {
  const providerEntries = Object.entries(report?.providers ?? {}).sort(([left], [right]) => {
    if (left === report?.current_provider) return -1;
    if (right === report?.current_provider) return 1;
    return left.localeCompare(right);
  });

  return (
    <Card>
      <CardHeader className="flex flex-row items-start justify-between gap-4">
        <div className="space-y-1">
          <CardTitle className="flex items-center gap-2 text-base">
            {report?.enabled ? <ShieldCheck className="h-5 w-5 text-emerald-600" /> : <ShieldOff className="h-5 w-5 text-slate-500" />}
            沙盒状态
          </CardTitle>
          <p className="text-sm text-muted-foreground">
            查看当前是否启用沙盒，以及各 provider 在当前运行环境中的支持和接入状态。
          </p>
        </div>
        <Button variant="outline" size="sm" onClick={onRefresh} disabled={loading}>
          <RefreshCcw className={loading ? "mr-2 h-4 w-4 animate-spin" : "mr-2 h-4 w-4"} />
          刷新
        </Button>
      </CardHeader>
      <CardContent className="space-y-4">
        {error ? <p className="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">{error}</p> : null}

        <div className="grid gap-3 md:grid-cols-4">
          <div className="rounded-xl border bg-muted/30 p-3">
            <div className="text-xs text-muted-foreground">运行环境</div>
            <div className="mt-1 font-medium">{report ? `${report.os} / ${report.arch}` : "-"}</div>
          </div>
          <div className="rounded-xl border bg-muted/30 p-3">
            <div className="text-xs text-muted-foreground">沙盒开关</div>
            <div className="mt-1">
              <Badge variant={report?.enabled ? "success" : "secondary"}>
                {report?.enabled ? "已开启" : "未开启"}
              </Badge>
            </div>
          </div>
          <div className="rounded-xl border bg-muted/30 p-3">
            <div className="text-xs text-muted-foreground">配置 provider</div>
            <div className="mt-1 font-medium">{report ? statusLabel(report.configured_provider) : "-"}</div>
          </div>
          <div className="rounded-xl border bg-muted/30 p-3">
            <div className="text-xs text-muted-foreground">当前生效 provider</div>
            <div className="mt-1 flex items-center gap-2">
              <span className="font-medium">{report ? statusLabel(report.current_provider) : "-"}</span>
              {report ? (
                <Badge variant={report.current_supported ? "success" : "warning"}>
                  {report.current_supported ? "可用" : "不可用"}
                </Badge>
              ) : null}
            </div>
          </div>
        </div>

        <div className="overflow-hidden rounded-xl border">
          <table className="w-full text-sm">
            <thead className="bg-muted/40 text-left text-muted-foreground">
              <tr>
                <th className="px-4 py-3 font-medium">Provider</th>
                <th className="px-4 py-3 font-medium">宿主支持</th>
                <th className="px-4 py-3 font-medium">项目已接入</th>
                <th className="px-4 py-3 font-medium">说明</th>
              </tr>
            </thead>
            <tbody>
              {providerEntries.length === 0 ? (
                <tr>
                  <td colSpan={4} className="px-4 py-6 text-center text-muted-foreground">
                    暂无数据
                  </td>
                </tr>
              ) : (
                providerEntries.map(([provider, support]) => (
                  <tr key={provider} className="border-t">
                    <td className="px-4 py-3 font-medium">{statusLabel(provider)}</td>
                    <td className="px-4 py-3">
                      <Badge variant={support.supported ? "success" : "secondary"}>
                        {support.supported ? "支持" : "未支持"}
                      </Badge>
                    </td>
                    <td className="px-4 py-3">
                      <Badge variant={support.implemented ? "success" : "outline"}>
                        {support.implemented ? "已接入" : "未接入"}
                      </Badge>
                    </td>
                    <td className="px-4 py-3 text-muted-foreground">{support.reason || "-"}</td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </CardContent>
    </Card>
  );
}
