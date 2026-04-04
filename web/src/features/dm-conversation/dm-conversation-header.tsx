"use client";

import { memo, useRef, useState } from "react";
import {
  Bot,
  Check,
  ChevronDown,
  FolderTree,
  History,
  Info,
  MessageSquare,
  MessageSquarePlus,
} from "lucide-react";

import { cn } from "@/lib/utils";
import {
  WorkspaceSurfaceHeader,
  WorkspaceTaskStrip,
} from "@/shared/ui/workspace/workspace-surface-header";
import { WorkspaceStatusBadge } from "@/shared/ui/workspace/workspace-status-badge";
import { RoomSurfaceTabKey } from "@/types/room-surface";
import { TodoItem } from "@/types/todo";
import { RoomConversationView } from "@/types/conversation";

interface DmConversationHeaderProps {
  conversation_id: string | null;
  conversations: RoomConversationView[];
  current_agent_name: string | null;
  is_loading: boolean;
  todos: TodoItem[];
  active_tab: RoomSurfaceTabKey;
  on_change_tab: (tab: RoomSurfaceTabKey) => void;
  on_select_conversation: (conversation_id: string) => void;
  on_create_conversation?: (title?: string) => Promise<string | null>;
}

const DM_TABS: { key: RoomSurfaceTabKey; label: string; icon: typeof MessageSquare }[] = [
  { key: "chat", label: "Chat", icon: MessageSquare },
  { key: "history", label: "History", icon: History },
  { key: "workspace", label: "Workspace", icon: FolderTree },
  { key: "about", label: "About", icon: Info },
];

function ConversationSwitcher({
  conversations,
  conversation_id,
  on_select_conversation,
  on_create_conversation,
}: {
  conversations: RoomConversationView[];
  conversation_id: string | null;
  on_select_conversation: (conversation_id: string) => void;
  on_create_conversation?: (title?: string) => Promise<string | null>;
}) {
  const [is_open, set_is_open] = useState(false);
  const [is_creating, set_is_creating] = useState(false);
  const trigger_ref = useRef<HTMLButtonElement>(null);

  const current_title =
    conversations.find((conversation) => conversation.conversation_id === conversation_id)?.title
    ?? "选择对话";

  const handle_create = async () => {
    if (!on_create_conversation || is_creating) return;
    set_is_creating(true);
    set_is_open(false);
    try {
      await on_create_conversation();
    } finally {
      set_is_creating(false);
    }
  };

  return (
    <div className="relative">
      <button
        ref={trigger_ref}
        className={cn(
          "flex h-7 max-w-[168px] items-center gap-1 rounded-full border border-white/60 bg-white/72 px-2.5 text-[11px] font-medium text-slate-600 shadow-sm transition-colors",
          "hover:bg-slate-100/70 hover:text-slate-800",
          is_open && "bg-slate-100/80 text-slate-800",
        )}
        onClick={() => set_is_open((prev) => !prev)}
        type="button"
      >
        <span className="max-w-[124px] truncate">{current_title}</span>
        <ChevronDown className={cn("h-3 w-3 transition-transform", is_open && "rotate-180")} />
      </button>

      {is_open ? (
        <>
          <div
            className="fixed inset-0 z-40"
            onClick={() => set_is_open(false)}
          />
          <div className="absolute left-0 top-full z-50 mt-1 w-56 rounded-xl border border-slate-200/60 bg-white/95 py-1 shadow-lg backdrop-blur-md">
            {conversations.length > 0 ? (
              <>
                {conversations.map((conversation) => {
                  const is_active = conversation.conversation_id === conversation_id;
                  return (
                    <button
                      key={conversation.conversation_id}
                      className={cn(
                        "flex w-full items-center gap-2 px-3 py-2 text-left text-[12px] transition-colors",
                        is_active
                          ? "bg-slate-100/80 font-semibold text-slate-900"
                          : "text-slate-600 hover:bg-slate-50",
                      )}
                      onClick={() => {
                        on_select_conversation(conversation.conversation_id);
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
                })}
                {on_create_conversation ? (
                  <div className="mx-3 my-1 border-t border-slate-200/60" />
                ) : null}
                {on_create_conversation ? (
                  <button
                    className="flex w-full items-center gap-2 px-3 py-2 text-left text-[12px] font-medium text-emerald-600 transition-colors hover:bg-emerald-50/80 disabled:opacity-60"
                    disabled={is_creating}
                    onClick={handle_create}
                    type="button"
                  >
                    <MessageSquarePlus className={cn("h-3.5 w-3.5 shrink-0", is_creating && "animate-spin")} />
                    <span className="min-w-0 flex-1">
                      {is_creating ? "创建中..." : "新建对话"}
                    </span>
                  </button>
                ) : null}
              </>
            ) : (
              <div className="px-3 py-2 text-[11px] text-slate-400">暂无对话</div>
            )}
          </div>
        </>
      ) : null}
    </div>
  );
}

const DmConversationHeaderView = memo(({
  conversation_id,
  conversations,
  current_agent_name,
  is_loading,
  todos,
  active_tab,
  on_change_tab,
  on_select_conversation,
  on_create_conversation,
}: DmConversationHeaderProps) => {
  const header_title = current_agent_name?.trim() || "未命名 DM";

  const title_trailing = (
    <ConversationSwitcher
      conversations={conversations}
      conversation_id={conversation_id}
      on_select_conversation={on_select_conversation}
      on_create_conversation={on_create_conversation}
    />
  );

  const trailing = (
    <>
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
      tabs_trailing={<WorkspaceTaskStrip todos={todos} />}
      tabs={DM_TABS}
      title={header_title}
      title_trailing={title_trailing}
      trailing={trailing}
    />
  );
});

DmConversationHeaderView.displayName = "DmConversationHeaderView";

export function DmConversationHeader(props: DmConversationHeaderProps) {
  return <DmConversationHeaderView {...props} />;
}
