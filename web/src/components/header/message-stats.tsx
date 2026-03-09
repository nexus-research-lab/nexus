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
      className="h-7 px-4 flex items-center gap-3 border-t border-primary/10 text-[10px] text-muted-foreground/50 font-mono">
      <span className="tabular-nums">{stats?.duration}</span>
      {stats?.tokens && (
        <>
          <span className="text-muted-foreground/20">•</span>
          <span className="tabular-nums">{stats.tokens}</span>
        </>
      )}
      {stats?.cost && (
        <>
          <span className="text-muted-foreground/20">•</span>
          <span className="tabular-nums">{stats.cost}</span>
        </>
      )}
      {stats?.cacheHit && (
        <>
          <span className="text-muted-foreground/20">•</span>
          <span>{stats.cacheHit}</span>
        </>
      )}

      <div className="flex-1"/>

      {/* 状态/操作 */}
      {showCursor ? (
        <div className="flex items-center gap-1">
          <Zap className="w-3 h-3 text-primary animate-pulse"/>
        </div>
      ) : (
        <div className="flex items-center gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
          {/* 复制 */}
          <button
            onClick={onCopyAssistant}
            className={cn(
              "p-1 rounded transition-colors",
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
              className="p-1 rounded text-muted-foreground/50 hover:text-foreground transition-colors disabled:opacity-50"
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
              className="p-1 rounded text-muted-foreground/50 hover:text-red-500 transition-colors disabled:opacity-50"
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