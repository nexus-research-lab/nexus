"use client";

import { memo, useRef, useState } from "react";
import {
  Bot,
  Check,
  ChevronDown,
  FolderTree,
  Hash,
  History,
  Info,
  MessageSquare,
  PanelRight,
} from "lucide-react";

import { cn } from "@/lib/utils";
import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace-surface-header";
import { WorkspaceStatusBadge } from "@/shared/ui/workspace-status-badge";
import { Agent } from "@/types/agent";
import { Conversation } from "@/types/conversation";
import { RoomSurfaceTabKey } from "@/types/room-surface";

interface RoomConversationHeaderProps {
  current_agent_name: string | null;
  current_conversation_id: string | null;
  current_room_title: string | null;
  current_conversation_title: string | null;
  current_room_type: string;
  conversation_count: number;
  conversations: Conversation[];
  is_loading: boolean;
  is_detail_panel_open: boolean;
  member_count: number;
  room_members: Agent[];
  active_tab: RoomSurfaceTabKey;
  on_change_tab: (tab: RoomSurfaceTabKey) => void;
  on_select_conversation: (conversation_id: string) => void;
  on_toggle_detail_panel: () => void;
}

/** 获取名称首字母缩写 */
function getInitials(name: string | null): string {
  if (!name) return "AG";
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return "AG";
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
  return `${parts[0][0] ?? ""}${parts[1][0] ?? ""}`.toUpperCase();
}

/** 对话切换下拉菜单 */
function ConversationSwitcher({
  conversations,
  current_conversation_id,
  on_select_conversation,
}: {
  conversations: Conversation[];
  current_conversation_id: string | null;
  on_select_conversation: (conversation_id: string) => void;
}) {
  const [is_open, set_is_open] = useState(false);
  const trigger_ref = useRef<HTMLButtonElement>(null);

  // 当前对话标题
  const current_title =
    conversations.find((c) => c.session_key === current_conversation_id)?.title
    ?? "选择对话";

  return (
    <div className="relative">
      <button
        ref={trigger_ref}
        className={cn(
          "flex items-center gap-1 rounded-md px-2 py-1 text-[12px] font-medium text-slate-600 transition-colors",
          "hover:bg-slate-100/60 hover:text-slate-800",
          is_open && "bg-slate-100/60 text-slate-800",
        )}
        onClick={() => set_is_open((prev) => !prev)}
        type="button"
      >
        <span className="max-w-[140px] truncate">{current_title}</span>
        <ChevronDown className={cn("h-3 w-3 transition-transform", is_open && "rotate-180")} />
      </button>

      {is_open ? (
        <>
          {/* 点击外部关闭 */}
          <div
            className="fixed inset-0 z-40"
            onClick={() => set_is_open(false)}
          />
          {/* 下拉菜单 */}
          <div className="absolute left-0 top-full z-50 mt-1 w-56 rounded-xl border border-slate-200/60 bg-white/95 py-1 shadow-lg backdrop-blur-md">
            {conversations.length > 0 ? (
              conversations.map((conversation) => {
                const is_active = conversation.session_key === current_conversation_id;
                return (
                  <button
                    key={conversation.session_key}
                    className={cn(
                      "flex w-full items-center gap-2 px-3 py-2 text-left text-[12px] transition-colors",
                      is_active
                        ? "bg-slate-100/80 font-semibold text-slate-900"
                        : "text-slate-600 hover:bg-slate-50",
                    )}
                    onClick={() => {
                      on_select_conversation(conversation.session_key);
                      set_is_open(false);
                    }}
                    type="button"
                  >
                    <MessageSquare className="h-3.5 w-3.5 shrink-0 text-slate-400" />
                    <span className="min-w-0 flex-1 truncate">
                      {conversation.title || "未命名对话"}
                    </span>
                    {is_active ? (
                      <Check className="h-3.5 w-3.5 shrink-0 text-emerald-500" />
                    ) : null}
                  </button>
                );
              })
            ) : (
              <div className="px-3 py-2 text-[11px] text-slate-400">暂无对话</div>
            )}
          </div>
        </>
      ) : null}
    </div>
  );
}

/** 成员头像堆叠组件 */
function MemberAvatarStack({
  room_members,
  is_detail_panel_open,
  on_toggle_detail_panel,
}: {
  room_members: Agent[];
  is_detail_panel_open: boolean;
  on_toggle_detail_panel: () => void;
}) {
  // 最多显示 5 个头像
  const MAX_VISIBLE = 5;
  const visible_members = room_members.slice(0, MAX_VISIBLE);
  const overflow_count = room_members.length - MAX_VISIBLE;

  return (
    <button
      className={cn(
        "flex items-center gap-1.5 rounded-lg px-2 py-1 transition-colors",
        "hover:bg-slate-100/60",
        is_detail_panel_open && "bg-slate-100/60",
      )}
      onClick={on_toggle_detail_panel}
      title={is_detail_panel_open ? "收起详情面板" : "展开详情面板"}
      type="button"
    >
      {/* 头像堆叠 */}
      <div className="flex items-center -space-x-2">
        {/* 用户自己的头像 */}
        <div className="flex h-7 w-7 items-center justify-center rounded-full border-2 border-white bg-slate-100 text-[8px] font-bold text-slate-900/82 shadow-sm">
          YOU
        </div>
        {/* Agent 成员头像 */}
        {visible_members.map((member) => (
          <div
            key={member.agent_id}
            className="flex h-7 w-7 items-center justify-center rounded-full border-2 border-white bg-gradient-to-b from-slate-100 to-slate-200 text-[8px] font-bold text-slate-700 shadow-sm"
            title={member.name}
          >
            {getInitials(member.name)}
          </div>
        ))}
        {/* 溢出计数 */}
        {overflow_count > 0 ? (
          <div className="flex h-7 w-7 items-center justify-center rounded-full border-2 border-white bg-slate-200 text-[9px] font-semibold text-slate-600 shadow-sm">
            +{overflow_count}
          </div>
        ) : null}
      </div>

      {/* 面板切换图标 */}
      <PanelRight className={cn(
        "h-3.5 w-3.5 text-slate-400 transition-colors",
        is_detail_panel_open && "text-slate-600",
      )} />
    </button>
  );
}

const RoomConversationHeaderView = memo(({
  current_agent_name,
  current_conversation_id,
  current_room_title,
  current_conversation_title,
  current_room_type,
  conversation_count,
  conversations,
  is_loading,
  is_detail_panel_open,
  member_count,
  room_members,
  active_tab,
  on_change_tab,
  on_select_conversation,
  on_toggle_detail_panel,
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

  // 副标题：对话切换下拉（Room 模式）或对话数量（DM 模式）
  const subtitle = current_room_type === "dm" ? (
    <>
      <span className="truncate text-slate-500">
        {conversation_count} 段历史协作
      </span>
    </>
  ) : (
    <ConversationSwitcher
      conversations={conversations}
      current_conversation_id={current_conversation_id}
      on_select_conversation={on_select_conversation}
    />
  );

  // 右侧：成员头像堆叠 + 状态徽章
  const trailing = (
    <>
      {/* 成员头像堆叠（桌面端） */}
      <div className="hidden lg:flex">
        <MemberAvatarStack
          is_detail_panel_open={is_detail_panel_open}
          on_toggle_detail_panel={on_toggle_detail_panel}
          room_members={room_members}
        />
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
