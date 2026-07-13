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

import {
  MessageRail,
  MessageRailBody,
  MessageRailLabel,
} from "../../../ui/message-rail";

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
    labelClassName: "text-amber-800/80",
  },
};

export function ContentSystemEvent({ block }: { block: SystemEventContent }) {
  const Icon = SYSTEM_EVENT_ICONS[block.icon];
  const style = SYSTEM_EVENT_STYLES[block.tone];
  return (
    <MessageRail className="min-w-0">
      <MessageRailLabel className={cn("flex-1", style.labelClassName)}>
        <span
          className="flex h-4 w-4 shrink-0 items-center justify-center"
          data-timeline-anchor
          data-timeline-anchor-mode="box"
        >
          <Icon className={cn("h-3 w-3", style.iconClassName)} />
        </span>
        <span>{block.label}</span>
      </MessageRailLabel>
      <MessageRailBody className="pt-1 text-[14px] leading-6 text-(--text-default)">
        {block.subtype === "api_retry" ? (
          <ApiRetrySystemEventBody block={block} />
        ) : (
          block.content
        )}
      </MessageRailBody>
    </MessageRail>
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
      <div className="mt-0.5 text-[13px] leading-5 text-(--text-muted)">
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
