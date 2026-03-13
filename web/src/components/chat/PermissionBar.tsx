import { cn } from "@/lib/utils";

interface PermissionOption {
  option_id: string;
  kind: string;
  name: string;
}

export interface PermissionRequest {
  permission_id: string;
  session_id: string;
  tool_call: {
    tool_call_id?: string;
    kind?: string;
    title?: string;
    locations?: { path: string }[];
    raw_input?: unknown;
  };
  options: PermissionOption[];
}

interface PermissionBarProps {
  permissions: PermissionRequest[];
  onResponse: (permissionId: string, optionId: string, cancel: boolean) => void;
}

export function PermissionBar({ permissions, onResponse }: PermissionBarProps) {
  if (permissions.length === 0) return null;

  return (
    <div className="space-y-2 border-t bg-amber-50/50 px-6 py-3">
      {permissions.map((perm) => {
        const title = perm.tool_call.title || perm.tool_call.kind || "Tool Call";
        const location = perm.tool_call.locations?.[0]?.path;
        return (
          <div key={perm.permission_id} className="flex items-center gap-3 rounded-lg border border-amber-200 bg-white px-4 py-2.5 text-sm">
            <div className="min-w-0 flex-1">
              <span className="font-medium text-amber-800">{title}</span>
              {location && <span className="ml-2 truncate font-mono text-xs text-muted-foreground">{location}</span>}
            </div>
            <div className="flex shrink-0 items-center gap-1.5">
              {perm.options.map((opt) => {
                const isAllow = opt.kind.startsWith("allow");
                return (
                  <button
                    key={opt.option_id}
                    type="button"
                    className={cn(
                      "rounded-md px-3 py-1 text-xs font-medium transition-colors",
                      isAllow
                        ? "bg-emerald-500 text-white hover:bg-emerald-600"
                        : "bg-muted text-muted-foreground hover:bg-muted/80",
                    )}
                    onClick={() => onResponse(perm.permission_id, opt.option_id, !isAllow)}
                  >
                    {opt.name}
                  </button>
                );
              })}
            </div>
          </div>
        );
      })}
    </div>
  );
}
