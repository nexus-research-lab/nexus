"use client";

import { memo } from "react";
import { Bot, FolderTree, Hash, History, Info, MessageSquare, Users } from "lucide-react";

import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace-surface-header";
import { WorkspaceStatusBadge } from "@/shared/ui/workspace-status-badge";
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

  const subtitle = (
    <>
      <Users className="h-3.5 w-3.5 shrink-0" />
      <span className="truncate">
        {current_room_type === "dm"
          ? `${conversation_count} 段历史协作`
          : `${member_count} 位成员 · ${conversation_count} 段对话`}
      </span>
    </>
  );

  const trailing = (
    <>
      <div className="hidden items-center -space-x-2 lg:flex">
        <div className="flex h-8 w-8 items-center justify-center rounded-full border border-white bg-slate-100 text-[8px] font-bold text-slate-900/82 shadow-sm">
          YOU
        </div>
        <div className="flex h-8 w-8 items-center justify-center rounded-full border border-white bg-slate-100 text-[8px] font-bold text-slate-900/82 shadow-sm">
          {getInitials(current_agent_name)}
        </div>
      </div>
      <WorkspaceStatusBadge
        icon={<span className="text-current">●</span>}
        label={is_loading ? "协作中" : "在线"}
        tone={is_loading ? "running" : "active"}
      />
    </>
  );

  return (
    <WorkspaceSurfaceHeader
      active_tab={active_tab}
      badge={current_room_type === "dm" ? "DM" : "ROOM"}
      leading={
        current_room_type === "dm" ? (
          <Bot size={14} className="text-slate-800/72" />
        ) : (
          <Hash size={14} className="text-slate-800/72" />
        )
      }
      on_change_tab={on_change_tab}
      subtitle={subtitle}
      tabs={tabs}
      title={header_title}
      trailing={trailing}
    />
  );
});

RoomConversationHeaderView.displayName = "RoomConversationHeaderView";

export function RoomConversationHeader(props: RoomConversationHeaderProps) {
  return <RoomConversationHeaderView {...props} />;
}
