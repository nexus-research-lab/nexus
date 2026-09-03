import { useEffect } from "react";
import {
  CornerDownRight,
  Info,
  LoaderCircle,
  RotateCcw,
  type LucideIcon,
} from "lucide-react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
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
      <div className="message-cjk-font min-w-0 max-w-full overflow-hidden break-words pt-1 text-base leading-6 text-(--text-default)">
        {block.subtype === "api_retry" ? (
          <ApiRetrySystemEventBody block={block} />
        ) : (
          block.content
        )}
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

function ApiRetrySystemEventBody({ block }: { block: SystemEventContent }) {
  const retryDelayMs =
    typeof block.retry_delay_ms === "number" && block.retry_delay_ms > 0
      ? block.retry_delay_ms
      : 0;
  const [nowMs, setNowMs] = useResettableState(
    Date.now(),
    `${block.timestamp}\x1f${retryDelayMs}`,
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
      ? `(attempt ${block.attempt}/${block.max_retries})`
      : null;
  const retryText = formatRetryText(retryDelayMs, retryInSeconds, attemptText);
  const content = block.content.length > MAX_API_RETRY_ERROR_CHARS
    ? `${block.content.slice(0, MAX_API_RETRY_ERROR_CHARS)}...`
    : block.content;

  return (
    <>
      <div>{content}</div>
      <div className="mt-0.5 text-sm leading-5 text-(--text-muted)">
        {retryText}
      </div>
    </>
  );
}

function formatRetryText(
  retryDelayMs: number,
  retryInSeconds: number,
  attemptText: string | null,
): string {
  const attemptSuffix = attemptText ? ` ${attemptText}` : "";
  if (retryDelayMs <= 0) {
    return `Retrying...${attemptSuffix}`;
  }
  const retryUnit = retryInSeconds === 1 ? "second" : "seconds";
  return `Retrying in ${retryInSeconds} ${retryUnit}...${attemptSuffix}`;
}
