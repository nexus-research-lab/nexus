"use client";

import { memo } from "react";
import { Hash, Users } from "lucide-react";

interface RoomConversationHeaderProps {
  current_agent_name: string | null;
  current_conversation_id: string | null;
  current_room_title: string | null;
  current_conversation_title: string | null;
  is_loading: boolean;
}

function getInitials(name: string | null): string {
  if (!name) {
    return "AG";
  }

  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) {
    return "AG";
  }
  if (parts.length === 1) {
    return parts[0].slice(0, 2).toUpperCase();
  }
  return `${parts[0][0] ?? ""}${parts[1][0] ?? ""}`.toUpperCase();
}

const RoomConversationHeaderView = memo(({
  current_agent_name,
  current_conversation_id,
  current_room_title,
  current_conversation_title,
  is_loading,
}: RoomConversationHeaderProps) => {
  return (
    <div className="z-10 flex min-w-0 items-center justify-between overflow-hidden border-b workspace-divider bg-transparent px-6 py-4 xl:px-8">
      <div className="flex min-w-0 flex-1 items-center gap-3">
        <div className="workspace-chip flex h-9 w-9 shrink-0 items-center justify-center rounded-2xl">
          <Hash size={14} className="text-slate-800/72" />
        </div>
        <div className="min-w-0 flex-1 overflow-hidden">
          <div className="truncate text-[22px] font-black tracking-[-0.04em] text-slate-950/90">
            {current_room_title?.trim() || "未命名协作"}
          </div>
          <div className="mt-1 flex min-w-0 items-center gap-2 text-[12px] text-slate-700/52">
            <Users className="h-3.5 w-3.5 shrink-0" />
            <span className="truncate">
            {current_conversation_title?.trim()
              ? current_conversation_title
              : current_conversation_id
                ? "当前对话已连接"
                : "还没有对话"}
            </span>
          </div>
        </div>
      </div>

      <div className="ml-3 flex shrink-0 items-center gap-2">
        <div className="hidden items-center -space-x-2 lg:flex">
          <div className="workspace-chip flex h-8 w-8 items-center justify-center rounded-full text-[10px] font-bold text-slate-900/82">
            YOU
          </div>
          <div className="workspace-chip flex h-8 w-8 items-center justify-center rounded-full text-[10px] font-bold text-slate-900/82">
            {getInitials(current_agent_name)}
          </div>
        </div>
        <div className="workspace-chip inline-flex items-center gap-2 rounded-full px-3 py-1.5 text-[11px] font-semibold text-slate-700/60">
          <span className={is_loading ? "text-emerald-500" : "text-sky-600"}>●</span>
          {is_loading ? "协作中" : "在线"}
        </div>
      </div>
    </div>
  );
});

RoomConversationHeaderView.displayName = "RoomConversationHeaderView";

export function RoomConversationHeader(props: RoomConversationHeaderProps) {
  return <RoomConversationHeaderView {...props} />;
}
