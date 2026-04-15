import { Check, Copy, Zap } from "lucide-react";
import { cn } from "@/lib/utils";

interface MessageStatsData {
  duration: string;
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
  return (
    <div
      className={cn(
        "flex min-w-0 items-center gap-x-2 gap-y-1 pt-2 text-[10.5px] text-(--text-muted)",
        compact ? "flex-wrap" : "flex-wrap sm:flex-nowrap sm:gap-2.5",
      )}>
      {stats?.duration ? <span className="shrink-0 tabular-nums">耗时 {stats.duration}</span> : null}
      {stats?.tokens && (
        <>
          {stats?.duration ? <span className="hidden text-(--text-soft) sm:inline">•</span> : null}
          <span className="min-w-0 truncate tabular-nums">Tokens {stats.tokens}</span>
        </>
      )}
      {stats?.cost && (
        <>
          {stats?.duration || stats?.tokens ? <span className="hidden text-(--text-soft) sm:inline">•</span> : null}
          <span className="shrink-0 tabular-nums">成本 {stats.cost}</span>
        </>
      )}
      {stats?.cache_hit && (
        <>
          {stats?.duration || stats?.tokens || stats?.cost ? <span className="hidden text-(--text-soft) sm:inline">•</span> : null}
          <span className="shrink-0">缓存 {stats.cache_hit}</span>
        </>
      )}

      <div className={cn("hidden flex-1 sm:block", compact && "sm:hidden")} />

      {/* 状态/操作 */}
      {show_cursor ? (
        <div className="ml-auto flex items-center gap-1">
          <Zap className="w-3 h-3 text-primary animate-pulse" />
        </div>
      ) : (
        <div className="ml-auto flex items-center gap-0.5 opacity-100 transition-opacity sm:opacity-0 sm:group-hover:opacity-100">
          {/* 复制 */}
          {on_copy_assistant && (
            <button
              onClick={on_copy_assistant}
              className={cn(
                "inline-flex h-6 w-6 items-center justify-center rounded-[10px] border border-transparent text-(--icon-default) transition-[color,border-color,background] duration-(--motion-duration-fast) hover:border-(--chip-default-border) hover:bg-(--chip-default-background) hover:text-(--icon-strong)",
                copied_assistant && "text-green-500",
              )}
              title="复制回答"
              type="button"
            >
              {copied_assistant ? <Check className="w-3 h-3" /> : <Copy className="w-3 h-3" />}
            </button>
          )}
        </div>
      )}
    </div>
  );
}
