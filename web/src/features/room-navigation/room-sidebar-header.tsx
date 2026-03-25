import { useState } from "react";
import { Grid2X2, Pencil, RefreshCw, Trash2 } from "lucide-react";

import { cn, truncate } from "@/lib/utils";
import { ConfirmDialog, PromptDialog } from "@/shared/ui/confirm-dialog";

interface RoomSidebarHeaderProps {
  active_room_title: string;
  current_agent_name: string;
  is_refreshing: boolean;
  on_delete_room: () => Promise<void>;
  on_open_directory: () => void;
  on_refresh: () => void;
  on_rename_room: (name: string) => Promise<void>;
}

export function RoomSidebarHeader({
  active_room_title,
  current_agent_name,
  is_refreshing,
  on_delete_room,
  on_open_directory,
  on_refresh,
  on_rename_room,
}: RoomSidebarHeaderProps) {
  const [is_delete_dialog_open, set_is_delete_dialog_open] = useState(false);
  const [is_rename_dialog_open, set_is_rename_dialog_open] = useState(false);

  return (
    <>
      <div className="flex items-center justify-between border-b workspace-divider px-2 py-5">
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
            aria-label="重命名 room"
            className="workspace-chip flex h-9 w-9 items-center justify-center rounded-2xl text-slate-700/58 transition-colors hover:text-slate-950 focus-visible:ring-2 focus-visible:ring-primary/40 focus-visible:ring-offset-1"
            onClick={() => set_is_rename_dialog_open(true)}
            type="button"
          >
            <Pencil className="h-3.5 w-3.5" />
          </button>
          <button
            aria-label="删除 room"
            className="workspace-chip flex h-9 w-9 items-center justify-center rounded-2xl text-slate-700/58 transition-colors hover:text-destructive focus-visible:ring-2 focus-visible:ring-primary/40 focus-visible:ring-offset-1"
            onClick={() => set_is_delete_dialog_open(true)}
            type="button"
          >
            <Trash2 className="h-3.5 w-3.5" />
          </button>
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

      <PromptDialog
        default_value={active_room_title}
        is_open={is_rename_dialog_open}
        message="输入新的 room 名称"
        on_cancel={() => set_is_rename_dialog_open(false)}
        on_confirm={(name) => {
          const next_name = name.trim();
          if (next_name) {
            void on_rename_room(next_name);
          }
          set_is_rename_dialog_open(false);
        }}
        placeholder="为这个 room 命名"
        title="重命名 room"
      />

      <ConfirmDialog
        cancel_text="取消"
        confirm_text="删除"
        is_open={is_delete_dialog_open}
        message={`确定要删除 room「${active_room_title}」吗？删除后无法恢复。`}
        on_cancel={() => set_is_delete_dialog_open(false)}
        on_confirm={() => {
          void on_delete_room();
          set_is_delete_dialog_open(false);
        }}
        title="删除 room"
        variant="danger"
      />
    </>
  );
}
