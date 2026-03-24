import { Grid2X2, RefreshCw } from "lucide-react";

import { cn, truncate } from "@/lib/utils";

interface RoomSidebarHeaderProps {
  active_room_title: string;
  current_agent_name: string;
  is_refreshing: boolean;
  on_open_directory: () => void;
  on_refresh: () => void;
}

export function RoomSidebarHeader({
  active_room_title,
  current_agent_name,
  is_refreshing,
  on_open_directory,
  on_refresh,
}: RoomSidebarHeaderProps) {
  return (
    <div className="flex items-center justify-between border-b workspace-divider px-6 py-5">
      <div className="min-w-0">
        <p className="text-[11px] font-semibold uppercase tracking-[0.24em] text-slate-700/48">
          NEXUS
        </p>
        <p className="mt-1 truncate text-[22px] font-black tracking-[-0.04em] text-slate-950/90">
          {truncate(active_room_title, 22)}
        </p>
        <p className="mt-1 truncate text-[12px] text-slate-700/52">
          当前 room · {current_agent_name} 正在参与
        </p>
      </div>
      <div className="flex items-center gap-2">
        <button
          aria-label="返回目录"
          className="workspace-chip flex h-9 w-9 items-center justify-center rounded-2xl text-slate-700/58 transition-colors hover:text-slate-950 focus-visible:ring-2 focus-visible:ring-primary/40 focus-visible:ring-offset-1"
          onClick={on_open_directory}
          type="button"
        >
          <Grid2X2 className="h-3.5 w-3.5" />
        </button>
        <button
          aria-label="刷新文件列表"
          className="workspace-chip flex h-9 w-9 items-center justify-center rounded-2xl text-slate-700/58 transition-colors hover:text-slate-950 focus-visible:ring-2 focus-visible:ring-primary/40 focus-visible:ring-offset-1"
          onClick={on_refresh}
          type="button"
        >
          <RefreshCw className={cn("h-3.5 w-3.5", is_refreshing && "animate-spin")} />
        </button>
      </div>
    </div>
  );
}
