import { Check, Copy } from "lucide-react";
import { cn } from "@/lib/utils";

interface MessageStatsData {
  duration: string | null;
  tokens: string | null;
  cost: string | null;
  cache_hit: string | null;
}

interface MessageStatsProps {
  stats?: MessageStatsData;
  show_cursor?: boolean;
  compact?: boolean;
  copied_assistant?: boolean;
  on_copy_assistant?: () => void;
}

export function MessageStats(
  {
    stats,
    show_cursor,
    compact = false,
    copied_assistant,
    on_copy_assistant,
  }: MessageStatsProps) {
  const stat_items = [
    stats?.duration ?? null,
    stats?.tokens ?? null,
    stats?.cost ?? null,
    stats?.cache_hit ?? null,
  ].filter((item): item is string => Boolean(item));

  return (
    <div
      className={cn(
        "flex min-w-0 items-start justify-between gap-3 pt-1.5 text-(--text-muted)",
        compact ? "text-[10.5px]" : "text-[11px]",
      )}>
      <div
        className={cn(
          "flex min-w-0 flex-wrap items-center gap-x-1.5 gap-y-1 leading-none",
          compact ? "max-w-full" : "max-w-[calc(100%-2.5rem)]",
        )}>
        {stat_items.map((item, index) => (
          <span key={`${item}-${index}`} className="contents">
            {index > 0 ? (
              <span className="shrink-0 text-(--text-soft)/70">•</span>
            ) : null}
            <span className="min-w-0 truncate tabular-nums text-(--text-muted)">
              {item}
            </span>
          </span>
        ))}
      </div>

      <div className="ml-auto shrink-0">
        {show_cursor ? (
          <span
            aria-hidden="true"
            className="mt-[2px] inline-flex h-1.5 w-1.5 rounded-full bg-(--text-soft) opacity-70 animate-pulse"
          />
        ) : (
          <div className="flex items-center gap-0.5 opacity-0 transition-opacity duration-(--motion-duration-fast) sm:group-hover:opacity-100">
            {on_copy_assistant ? (
              <button
                onClick={on_copy_assistant}
                className={cn(
                  "inline-flex h-5 w-5 items-center justify-center rounded-md text-(--icon-muted) transition-[color,background] duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-strong)",
                  copied_assistant && "text-emerald-500",
                )}
                title="复制回答"
                type="button"
              >
                {copied_assistant ? <Check className="h-3 w-3" /> : <Copy className="h-3 w-3" />}
              </button>
            ) : null}
          </div>
        )}
      </div>
    </div>
  );
}
