import { Grid2X2, RefreshCw } from "lucide-react";

import { cn, truncate } from "@/lib/utils";

interface RoomSidebarHeaderProps {
  activeRoomTitle: string;
  currentAgentName: string;
  isRefreshing: boolean;
  onOpenDirectory: () => void;
  onRefresh: () => void;
}

export function RoomSidebarHeader({
  activeRoomTitle,
  currentAgentName,
  isRefreshing,
  onOpenDirectory,
  onRefresh,
}: RoomSidebarHeaderProps) {
  return (
    <div className="flex items-center justify-between border-b workspace-divider px-6 py-5">
      <div className="min-w-0">
        <p className="text-[11px] font-semibold uppercase tracking-[0.24em] text-slate-700/48">
          NEXUS
        </p>
        <p className="mt-1 truncate text-[22px] font-black tracking-[-0.04em] text-slate-950/90">
          {truncate(activeRoomTitle, 22)}
        </p>
        <p className="mt-1 truncate text-[12px] text-slate-700/52">
          当前 room · {currentAgentName} 正在参与
        </p>
      </div>
      <div className="flex items-center gap-2">
        <button
          aria-label="返回目录"
          className="workspace-chip flex h-9 w-9 items-center justify-center rounded-2xl text-slate-700/58 transition-colors hover:text-slate-950 focus-visible:ring-2 focus-visible:ring-primary/40 focus-visible:ring-offset-1"
          onClick={onOpenDirectory}
          type="button"
        >
          <Grid2X2 className="h-3.5 w-3.5" />
        </button>
        <button
          aria-label="刷新文件列表"
          className="workspace-chip flex h-9 w-9 items-center justify-center rounded-2xl text-slate-700/58 transition-colors hover:text-slate-950 focus-visible:ring-2 focus-visible:ring-primary/40 focus-visible:ring-offset-1"
          onClick={onRefresh}
          type="button"
        >
          <RefreshCw className={cn("h-3.5 w-3.5", isRefreshing && "animate-spin")} />
        </button>
      </div>
    </div>
  );
}
