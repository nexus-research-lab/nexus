"use client";

import { memo } from "react";
import { Bot, FolderTree, History, Info, MessageSquare, PanelRight } from "lucide-react";

import { cn } from "@/lib/utils";
import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace/workspace-surface-header";
import { WorkspaceStatusBadge } from "@/shared/ui/workspace/workspace-status-badge";
import { RoomSurfaceTabKey } from "@/types/room-surface";

interface DmConversationHeaderProps {
  current_agent_name: string | null;
  conversation_count: number;
  is_loading: boolean;
  is_detail_panel_open: boolean;
  active_tab: RoomSurfaceTabKey;
  on_change_tab: (tab: RoomSurfaceTabKey) => void;
  on_toggle_detail_panel: () => void;
}

const DM_TABS: { key: RoomSurfaceTabKey; label: string; icon: typeof MessageSquare }[] = [
  { key: "chat", label: "Chat", icon: MessageSquare },
  { key: "history", label: "History", icon: History },
  { key: "workspace", label: "Workspace", icon: FolderTree },
  { key: "about", label: "About", icon: Info },
];

const DmConversationHeaderView = memo(({
  current_agent_name,
  conversation_count,
  is_loading,
  is_detail_panel_open,
  active_tab,
  on_change_tab,
  on_toggle_detail_panel,
}: DmConversationHeaderProps) => {
  const header_title = current_agent_name?.trim() || "未命名 DM";

  const subtitle = (
    <span className="truncate text-slate-500">
      {conversation_count} 段历史协作
    </span>
  );

  const trailing = (
    <>
      <button
        className={cn(
          "hidden items-center gap-1.5 rounded-lg px-2 py-1 transition-colors lg:flex",
          "hover:bg-slate-100/60",
          is_detail_panel_open && "bg-slate-100/60",
        )}
        onClick={on_toggle_detail_panel}
        title={is_detail_panel_open ? "收起详情面板" : "展开详情面板"}
        type="button"
      >
        <PanelRight className={cn(
          "h-3.5 w-3.5 text-slate-400 transition-colors",
          is_detail_panel_open && "text-slate-600",
        )} />
      </button>
      <WorkspaceStatusBadge
        icon={<span className="text-current">●</span>}
        label={is_loading ? "回复中" : "在线"}
        tone={is_loading ? "running" : "active"}
      />
    </>
  );

  return (
    <WorkspaceSurfaceHeader
      active_tab={active_tab}
      badge="DM"
      leading={<Bot size={14} className="text-slate-800/72" />}
      on_change_tab={on_change_tab}
      subtitle={subtitle}
      tabs={DM_TABS}
      title={header_title}
      trailing={trailing}
    />
  );
});

DmConversationHeaderView.displayName = "DmConversationHeaderView";

export function DmConversationHeader(props: DmConversationHeaderProps) {
  return <DmConversationHeaderView {...props} />;
}
