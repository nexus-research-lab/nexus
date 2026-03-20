import { Check, Copy, RefreshCw, Trash2, Zap } from "lucide-react";
import { cn } from "@/lib/utils";

interface MessageStatsProps {
  stats?: {
    duration: string;
    tokens: string | null;
    cost: string | null;
    cacheHit: string | null;
  };
  showCursor?: boolean;
  copiedAssistant?: boolean;
  isRegenerating?: boolean;
  isDeleting?: boolean;
  onCopyAssistant?: () => void;
  onRegenerate?: () => void;
  onDelete?: () => void;
}

export function MessageStats(
  {
    stats,
    showCursor,
    copiedAssistant,
    isRegenerating,
    isDeleting,
    onCopyAssistant,
    onRegenerate,
    onDelete,
  }: MessageStatsProps) {
  return (
    <div
      className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 border-t border-white/55 px-3 py-2 text-[10px] font-mono text-muted-foreground/50 sm:h-8 sm:flex-nowrap sm:gap-3 sm:px-4 sm:py-0">
      <span className="shrink-0 tabular-nums">{stats?.duration}</span>
      {stats?.tokens && (
        <>
          <span className="hidden text-muted-foreground/20 sm:inline">•</span>
          <span className="min-w-0 truncate tabular-nums">{stats.tokens}</span>
        </>
      )}
      {stats?.cost && (
        <>
          <span className="hidden text-muted-foreground/20 sm:inline">•</span>
          <span className="shrink-0 tabular-nums">{stats.cost}</span>
        </>
      )}
      {stats?.cacheHit && (
        <>
          <span className="hidden text-muted-foreground/20 sm:inline">•</span>
          <span className="shrink-0">{stats.cacheHit}</span>
        </>
      )}

      <div className="hidden flex-1 sm:block" />

      {/* 状态/操作 */}
      {showCursor ? (
        <div className="ml-auto flex items-center gap-1">
          <Zap className="w-3 h-3 text-primary animate-pulse"/>
        </div>
      ) : (
        <div className="ml-auto flex items-center gap-0.5 opacity-100 transition-opacity sm:opacity-0 sm:group-hover:opacity-100">
          {/* 复制 */}
          <button
            onClick={onCopyAssistant}
            className={cn(
              "neo-pill radius-shell-sm p-1 transition-colors",
              copiedAssistant ? "text-green-500" : "text-muted-foreground/50 hover:text-foreground"
            )}
            title="复制回答"
          >
            {copiedAssistant ? <Check className="w-3 h-3"/> : <Copy className="w-3 h-3"/>}
          </button>
          {/* 重新生成 */}
          {onRegenerate && (
            <button
              onClick={onRegenerate}
              disabled={isRegenerating}
              className="neo-pill radius-shell-sm p-1 text-muted-foreground/50 transition-colors hover:text-foreground disabled:opacity-50"
              title="重新生成"
            >
              <RefreshCw className={cn("w-3 h-3", isRegenerating && "animate-spin")}/>
            </button>
          )}
          {/* 删除 */}
          {onDelete && (
            <button
              onClick={onDelete}
              disabled={isDeleting}
              className="neo-pill radius-shell-sm p-1 text-muted-foreground/50 transition-colors hover:text-red-500 disabled:opacity-50"
              title="删除"
            >
              <Trash2 className="w-3 h-3"/>
            </button>
          )}
        </div>
      )}
    </div>
  );
}
