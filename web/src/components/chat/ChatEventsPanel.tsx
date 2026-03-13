import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { EventLogRow } from "./EventLogRow";
import {
  computeEventLevel,
  EVENT_LEVEL_ORDER,
  toEventListItem,
  type EventLevel,
} from "./chatUtils";
import type { Event as ApiEvent } from "@/types/apiV2";

interface ChatEventsPanelProps {
  events: ApiEvent[];
}

export function ChatEventsPanel({ events }: ChatEventsPanelProps) {
  const { t } = useTranslation();
  const [minEventLevel, setMinEventLevel] = useState<EventLevel>("info");

  const sortedEventItems = useMemo(
    () =>
      [...events]
        .sort((left, right) => {
          const timeDiff = new Date(left.timestamp).getTime() - new Date(right.timestamp).getTime();
          if (timeDiff !== 0) return timeDiff;
          return left.id - right.id;
        })
        .map((event) => toEventListItem(event, t)),
    [events, t],
  );

  const filteredEventItems = useMemo(
    () =>
      sortedEventItems.filter(
        (item) => EVENT_LEVEL_ORDER[computeEventLevel(item.rawType)] >= EVENT_LEVEL_ORDER[minEventLevel],
      ),
    [sortedEventItems, minEventLevel],
  );

  return (
    <>
      {/* Event level filter */}
      <div className="mx-auto mb-3 flex w-full max-w-[1200px] items-center gap-2 text-xs text-muted-foreground">
        <span>{t("chat.showLevel", { defaultValue: "显示级别:" })}</span>
        {(["debug", "info", "warning", "error"] as EventLevel[]).map((level) => (
          <button
            key={level}
            type="button"
            onClick={() => setMinEventLevel(level)}
            className={[
              "rounded px-2 py-0.5 font-mono transition-colors",
              minEventLevel === level
                ? "bg-primary text-primary-foreground"
                : "bg-muted hover:bg-muted/80 text-muted-foreground",
            ].join(" ")}
          >
            {level}
          </button>
        ))}
        <span className="ml-auto text-[10px]">
          {filteredEventItems.length} / {sortedEventItems.length}
        </span>
      </div>
      {filteredEventItems.length === 0 ? (
        <div className="mx-auto w-full max-w-[1200px] rounded-xl border border-dashed bg-muted/20 px-5 py-6 text-sm text-muted-foreground">
          {t("chat.noEvents")}
        </div>
      ) : (
        <table className="mx-auto w-full max-w-[1200px] border-collapse text-left">
          <thead>
            <tr className="border-b border-border text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
              <th className="py-1.5 pl-3 pr-2">{t("chat.colTime", { defaultValue: "时间" })}</th>
              <th className="py-1.5 pr-2" />
              <th className="py-1.5 pr-3">{t("chat.colType", { defaultValue: "类型" })}</th>
              <th className="py-1.5 pr-2">{t("chat.colSummary", { defaultValue: "摘要" })}</th>
              <th className="py-1.5 pr-2" />
            </tr>
          </thead>
          <tbody>
            {filteredEventItems.map((item) => <EventLogRow key={item.id} item={item} />)}
          </tbody>
        </table>
      )}
    </>
  );
}
