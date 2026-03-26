"use client";

import { memo } from "react";
import { Bot, FolderTree, Hash, History, Info, MessageSquare, Users } from "lucide-react";

import { RoomSurfaceTabKey } from "@/types/room-surface";

interface RoomConversationHeaderProps {
  current_agent_name: string | null;
  current_conversation_id: string | null;
  current_room_title: string | null;
  current_conversation_title: string | null;
  current_room_type: string;
  conversation_count: number;
  is_loading: boolean;
  member_count: number;
  active_tab: RoomSurfaceTabKey;
  on_change_tab: (tab: RoomSurfaceTabKey) => void;
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
  current_room_type,
  conversation_count,
  is_loading,
  member_count,
  active_tab,
  on_change_tab,
}: RoomConversationHeaderProps) => {
  const tabs: { key: RoomSurfaceTabKey; label: string; icon: typeof MessageSquare }[] =
    current_room_type === "dm"
      ? [
        { key: "chat", label: "Chat", icon: MessageSquare },
        { key: "history", label: "History", icon: History },
        { key: "workspace", label: "Workspace", icon: FolderTree },
        { key: "about", label: "About", icon: Info },
      ]
      : [
        { key: "chat", label: "Chat", icon: MessageSquare },
        { key: "history", label: "History", icon: History },
        { key: "workspace", label: "Workspace", icon: FolderTree },
      ];

  const header_title = current_room_type === "dm"
    ? current_agent_name?.trim() || current_room_title?.trim() || "未命名 DM"
    : current_room_title?.trim() || "未命名协作";

  return (
    <div className="z-10 overflow-hidden border-b border-slate-200/80 bg-white/92">
      <div className="flex min-w-0 items-center justify-between px-5 py-3 xl:px-6">
        <div className="flex min-w-0 flex-1 items-center gap-3">
          <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full border border-slate-200 bg-slate-50 text-slate-700">
            {current_room_type === "dm" ? (
              <Bot size={14} className="text-slate-800/72" />
            ) : (
              <Hash size={14} className="text-slate-800/72" />
            )}
          </div>
          <div className="min-w-0 flex-1 overflow-hidden">
            <div className="flex min-w-0 items-center gap-2">
              <div className="truncate text-[17px] font-black tracking-[-0.04em] text-slate-950/86">
                {header_title}
              </div>
              <span className="hidden rounded-full border border-slate-200 bg-slate-100 px-2 py-0.5 text-[10px] font-semibold text-slate-500 md:inline-flex">
                {current_room_type === "dm" ? "DM" : "ROOM"}
              </span>
            </div>
          </div>
        </div>

        <div className="ml-3 flex shrink-0 items-center gap-2">
          <div className="hidden items-center -space-x-2 lg:flex">
            <div className="flex h-8 w-8 items-center justify-center rounded-full border border-white bg-slate-100 text-[8px] font-bold text-slate-900/82 shadow-sm">
              YOU
            </div>
            <div className="flex h-8 w-8 items-center justify-center rounded-full border border-white bg-slate-100 text-[8px] font-bold text-slate-900/82 shadow-sm">
              {getInitials(current_agent_name)}
            </div>
          </div>
          <div className="inline-flex items-center gap-2 rounded-full border border-slate-200 bg-white px-3 py-1 text-[11px] font-semibold text-slate-600 shadow-sm">
            <span className={is_loading ? "text-emerald-500" : "text-sky-600"}>●</span>
            {is_loading ? "协作中" : "在线"}
          </div>
        </div>
      </div>

      <div className="flex items-center gap-1 px-5 pb-2 xl:px-6">
        {tabs.map((tab) => {
          const Icon = tab.icon;
          const is_active = active_tab === tab.key;
          return (
            <button
              key={tab.key}
              className={`inline-flex items-center gap-1.5 rounded-md px-2.5 py-1 text-[11px] font-semibold transition-all ${
                is_active
                  ? "border border-slate-200 bg-white text-slate-950 shadow-sm"
                  : "text-slate-600 hover:bg-slate-100 hover:text-slate-950"
              }`}
              onClick={() => on_change_tab(tab.key)}
              type="button"
            >
              <Icon className="h-3.5 w-3.5" />
              {tab.label}
            </button>
          );
        })}
      </div>
    </div>
  );
});

RoomConversationHeaderView.displayName = "RoomConversationHeaderView";

export function RoomConversationHeader(props: RoomConversationHeaderProps) {
  return <RoomConversationHeaderView {...props} />;
}
