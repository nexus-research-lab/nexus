import { Check, Copy, type LucideIcon } from "lucide-react";

import { cn } from "@/shared/ui/class-name";

import type { AssistantFooterStats } from "./assistant-message-model";

interface CopyActionPresentation {
  className?: string;
  icon: LucideIcon;
}

const COPY_ACTION_PRESENTATION: Record<"copied" | "idle", CopyActionPresentation> = {
  copied: { className: "text-(--success)", icon: Check },
  idle: { icon: Copy },
};

export function AssistantMessageStats({
  compact,
  copied,
  onCopy,
  stats,
  streaming,
}: {
  compact: boolean;
  copied: boolean;
  onCopy?: () => Promise<void>;
  stats: AssistantFooterStats | null;
  streaming: boolean;
}) {
  const items = [
    stats?.duration,
    stats?.tokens,
    stats?.cost,
    stats?.cacheHit,
  ].filter((item): item is string => Boolean(item));

  return (
    <div className={cn(
      "nexus-chat-message-stats flex min-w-0 items-start justify-between gap-3 pt-1.5 text-(--text-muted)",
      compact ? "text-[10.5px]" : "text-[11px]",
    )}>
      <div className={cn(
        "nexus-chat-message-stat-list flex min-w-0 flex-wrap items-center gap-x-1.5 gap-y-1 leading-none",
        compact ? "max-w-full" : "max-w-[calc(100%-2.5rem)]",
      )}>
        {items.map((item, index) => (
          <span className="contents" key={`${item}-${index}`}>
            {index > 0 ? (
              <span className="shrink-0 text-(--text-soft)/70">•</span>
            ) : null}
            <span className="min-w-0 truncate tabular-nums text-(--text-muted)">
              {item}
            </span>
          </span>
        ))}
      </div>

      <AssistantStatsTrailing
        copied={copied}
        onCopy={onCopy}
        streaming={streaming}
      />
    </div>
  );
}

function AssistantStatsTrailing({
  copied,
  onCopy,
  streaming,
}: {
  copied: boolean;
  onCopy?: () => Promise<void>;
  streaming: boolean;
}) {
  if (streaming) {
    return (
      <span
        aria-hidden="true"
        className="ml-auto mt-[2px] inline-flex h-1.5 w-1.5 shrink-0 rounded-full bg-(--text-soft) opacity-70"
      />
    );
  }

  return (
    <div className="ml-auto flex shrink-0 items-center gap-0.5 opacity-0 transition-opacity duration-(--motion-duration-fast) sm:group-hover:opacity-100">
      {onCopy ? <AssistantCopyAction copied={copied} onCopy={onCopy} /> : null}
    </div>
  );
}

function AssistantCopyAction({
  copied,
  onCopy,
}: {
  copied: boolean;
  onCopy: () => Promise<void>;
}) {
  const presentation = COPY_ACTION_PRESENTATION[copied ? "copied" : "idle"];
  const Icon = presentation.icon;
  return (
    <button
      aria-label="复制回答"
      className={cn(
        "inline-flex h-5 w-5 items-center justify-center rounded-md text-(--icon-muted) transition-[color,background] duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-strong)",
        presentation.className,
      )}
      onClick={onCopy}
      title="复制回答"
      type="button"
    >
      <Icon className="h-3 w-3" />
    </button>
  );
}
