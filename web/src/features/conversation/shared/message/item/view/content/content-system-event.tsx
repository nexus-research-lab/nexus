// INPUT: 已投影的系统事件内容、重试时序和本地化文案。
// OUTPUT: 紧凑系统事件，以及可独立展开错误详情的 Provider 重试活动行。
// POS: Conversation 消息内容视图；只解释系统事件展示，不拥有协议投影或重试状态。

import { useEffect } from "react";
import {
  ChevronRight,
  CornerDownRight,
  Info,
  LoaderCircle,
  RotateCcw,
  type LucideIcon,
} from "lucide-react";

import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import type { SystemEventContent } from "@/types/conversation/message/content";

const MAX_API_RETRY_ERROR_CHARS = 1000;
const SYSTEM_EVENT_ICONS: Record<SystemEventContent["icon"], LucideIcon> = {
  guide: CornerDownRight,
  progress: LoaderCircle,
  retry: RotateCcw,
  status: Info,
};
const SYSTEM_EVENT_STYLES: Record<
  SystemEventContent["tone"],
  { iconClassName: string; labelClassName: string }
> = {
  neutral: {
    iconClassName: "text-(--icon-muted)",
    labelClassName: "text-(--text-muted)",
  },
  warning: {
    iconClassName: "text-(--warning)",
    labelClassName: "text-(--warning)",
  },
};

export function ContentSystemEvent({ block }: { block: SystemEventContent }) {
  const Icon = SYSTEM_EVENT_ICONS[block.icon];
  const style = SYSTEM_EVENT_STYLES[block.tone];
  if (block.subtype === "api_retry") {
    return <ApiRetrySystemEvent block={block} Icon={Icon} />;
  }
  if (block.subtype === "memory_saved") {
    return <CompactSystemEvent block={block} Icon={Icon} />;
  }
  return (
    <div
      className="min-w-0 max-w-full overflow-hidden border-l-2 pl-4"
      style={{ borderColor: "color-mix(in srgb, var(--foreground) 18%, transparent)" }}
    >
      <div className={cn(
        "flex min-w-0 flex-1 items-center gap-2 text-xs font-medium text-(--text-muted)",
        style.labelClassName,
      )}>
        <span
          className="flex h-4 w-4 shrink-0 items-center justify-center"
          data-timeline-anchor
          data-timeline-anchor-mode="box"
        >
          <Icon className={cn("h-3 w-3", style.iconClassName)} />
        </span>
        <span>{block.label}</span>
      </div>
      <div className="message-cjk-font ui-type-body min-w-0 max-w-full overflow-hidden break-words pt-1 text-(--text-default)">
        {block.content}
      </div>
    </div>
  );
}

function CompactSystemEvent({
  block,
  Icon,
}: {
  block: SystemEventContent;
  Icon: LucideIcon;
}) {
  return (
    <div
      className="grid min-h-7 min-w-0 grid-cols-[20px_minmax(0,1fr)] items-center gap-1.5 px-1.5 py-0.5 text-sm font-normal leading-5 text-(--text-soft)"
      data-activity-row="system-event"
      data-system-event-subtype={block.subtype}
    >
      <span
        className="flex h-5 w-5 shrink-0 items-center justify-center text-(--icon-muted)"
        data-timeline-anchor
        data-timeline-anchor-mode="box"
      >
        <Icon aria-hidden className="h-3.5 w-3.5" strokeWidth={1.8} />
      </span>
      <span className="flex min-w-0 items-baseline gap-1.5">
        <span className="shrink-0">{block.label}</span>
        <span
          className="min-w-0 flex-1 truncate"
          data-system-event-detail="inline"
        >
          {block.content}
        </span>
      </span>
    </div>
  );
}

function ApiRetrySystemEvent({
  block,
  Icon,
}: {
  block: SystemEventContent;
  Icon: LucideIcon;
}) {
  const { t } = useI18n();
  const retryDelayMs =
    typeof block.retry_delay_ms === "number" && block.retry_delay_ms > 0
      ? block.retry_delay_ms
      : 0;
  const [nowMs, setNowMs] = useResettableState(
    Date.now(),
    `${block.timestamp}\x1f${retryDelayMs}`,
  );
  const [expanded, setExpanded] = useResettableState(
    true,
    `${block.source_message_id}\x1f${block.attempt ?? ""}`,
  );

  useEffect(() => {
    if (retryDelayMs <= 0) {
      return;
    }
    const intervalId = window.setInterval(() => setNowMs(Date.now()), 1000);
    return () => window.clearInterval(intervalId);
  }, [block.timestamp, retryDelayMs, setNowMs]);

  const retryInSeconds = Math.max(
    0,
    Math.round((block.timestamp + retryDelayMs - nowMs) / 1000),
  );
  const attemptText =
    typeof block.attempt === "number" && typeof block.max_retries === "number"
      ? `${block.attempt}/${block.max_retries}`
      : null;
  const waitText = retryDelayMs > 0 && retryInSeconds > 0
    ? t("message.api_retry_waiting", { seconds: retryInSeconds })
    : null;
  const content = block.content.length > MAX_API_RETRY_ERROR_CHARS
    ? `${block.content.slice(0, MAX_API_RETRY_ERROR_CHARS)}…`
    : block.content;

  return (
    <div
      className="min-w-0 px-1.5 py-0.5 text-sm font-normal leading-5"
      data-activity-row="system-event"
      data-system-event-subtype={block.subtype}
    >
      <button
        aria-expanded={expanded}
        className="grid min-w-0 grid-cols-[20px_minmax(0,1fr)] items-center gap-1.5 text-left text-(--text-soft)"
        onClick={() => setExpanded((current) => !current)}
        type="button"
      >
        <span
          className="flex h-5 w-5 shrink-0 items-center justify-center text-(--icon-muted)"
          data-timeline-anchor
          data-timeline-anchor-mode="box"
        >
          <Icon aria-hidden className="h-3.5 w-3.5" strokeWidth={1.8} />
        </span>
        <span className="flex min-w-0 items-center gap-1.5">
          <span>{t("message.api_retrying")}</span>
          {attemptText ? <span className="shrink-0">{attemptText}</span> : null}
          {waitText ? <span className="shrink-0 text-(--text-muted)">· {waitText}</span> : null}
          <ChevronRight
            aria-hidden
            className={cn(
              "h-3.5 w-3.5 shrink-0 transition-transform",
              expanded && "rotate-90",
            )}
            strokeWidth={1.8}
          />
        </span>
      </button>
      {expanded ? (
        <span className="message-cjk-font ml-[26px] mt-0.5 block min-w-0 break-words text-xs leading-5 text-(--text-muted)">
          {content}
        </span>
      ) : null}
    </div>
  );
}
