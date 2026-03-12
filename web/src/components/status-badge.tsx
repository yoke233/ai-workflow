import { useTranslation } from "react-i18next";
import { Badge, type BadgeProps } from "@/components/ui/badge";

type Status = "done" | "succeeded" | "running" | "in_progress" | "pending" | "queued" | "ready"
  | "failed" | "cancelled" | "blocked" | "waiting_gate" | "created" | string;

const statusVariant: Record<string, BadgeProps["variant"]> = {
  done: "success",
  succeeded: "success",
  running: "info",
  in_progress: "info",
  pending: "secondary",
  queued: "secondary",
  ready: "info",
  failed: "destructive",
  cancelled: "secondary",
  blocked: "warning",
  waiting_gate: "warning",
  created: "secondary",
};

export function StatusBadge({ status }: { status: Status }) {
  const { t } = useTranslation();
  const variant = statusVariant[status] ?? ("outline" as const);
  const label = t(`status.${status}`, status);
  return <Badge variant={variant}>{label}</Badge>;
}
