"use client";

import { useCallback, useState } from "react";
import { Check, Clock3, MessageSquarePlus, Pencil, Trash2, X } from "lucide-react";

import { formatRelativeTime } from "@/lib/utils";
import { WorkspacePillButton } from "@/shared/ui/workspace/workspace-pill-button";
import { WorkspaceSurfaceView } from "@/shared/ui/workspace/workspace-surface-view";
import { RoomConversationView } from "@/types/conversation";

interface RoomConversationHistoryViewProps {
  can_manage_conversations?: boolean;
  conversations: RoomConversationView[];
  conversation_id: string | null;
  current_room_type: string;
  on_create_conversation: (title?: string) => Promise<string | null>;
  on_delete_conversation: (conversation_id: string) => Promise<string | null>;
  on_select_conversation: (conversation_id: string) => void;
  on_update_conversation_title?: (conversation_id: string, title: string) => Promise<void>;
}

export function RoomConversationHistoryView({
  can_manage_conversations = true,
  conversations,
  conversation_id,
  current_room_type,
  on_create_conversation,
  on_delete_conversation,
  on_select_conversation,
  on_update_conversation_title,
}: RoomConversationHistoryViewProps) {
  const action = can_manage_conversations ? (
    <WorkspacePillButton
      onClick={() => {
        void on_create_conversation();
      }}
      size="sm"
    >
      <MessageSquarePlus className="h-3.5 w-3.5" />
      新建对话
    </WorkspacePillButton>
  ) : null;

  return (
    <WorkspaceSurfaceView
      action={action}
      content_class_name="space-y-2"
      eyebrow="History"
      title={current_room_type === "dm" ? "历史对话" : "对话历史"}
    >
      {conversations.map((conversation) => {
        return (
        <ConversationHistoryItem
          key={conversation.conversation_id}
          can_delete={can_manage_conversations && conversation.conversation_type === "topic"}
          can_rename={can_manage_conversations && on_update_conversation_title !== undefined}
          conversation={conversation}
          is_active={conversation.conversation_id === conversation_id}
          on_delete={() => void on_delete_conversation(conversation.conversation_id)}
          on_rename={(title) => void on_update_conversation_title?.(conversation.conversation_id, title)}
          on_select={() => on_select_conversation(conversation.conversation_id)}
        />
        );
      })}
    </WorkspaceSurfaceView>
  );
}

/** 单条对话历史条目 — 支持内联重命名 */
function ConversationHistoryItem({
  can_delete,
  can_rename,
  conversation,
  is_active,
  on_delete,
  on_rename,
  on_select,
}: {
  can_delete: boolean;
  can_rename: boolean;
  conversation: RoomConversationView;
  is_active: boolean;
  on_delete: () => void;
  on_rename: (title: string) => void;
  on_select: () => void;
}) {
  const [is_editing, set_is_editing] = useState(false);
  const [edit_value, set_edit_value] = useState("");

  const start_edit = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
    set_edit_value(conversation.title?.trim() || "");
    set_is_editing(true);
  }, [conversation.title]);

  const confirm_edit = useCallback(() => {
    const trimmed = edit_value.trim();
    if (trimmed && trimmed !== conversation.title?.trim()) {
      on_rename(trimmed);
    }
    set_is_editing(false);
  }, [edit_value, conversation.title, on_rename]);

  const cancel_edit = useCallback(() => {
    set_is_editing(false);
  }, []);

  return (
    <div
      className={`group flex w-full items-center justify-between gap-4 rounded-[14px] border px-4 py-2.5 text-left transition-all duration-300 ${is_active
          ? "border-white/32 bg-white/22 shadow-[0_10px_18px_rgba(111,126,162,0.08)]"
          : "border-white/14 bg-white/8 hover:bg-white/12"
        }`}
    >
      <div className="min-w-0 flex-1">
        {is_editing ? (
          <div className="flex items-center gap-1.5">
            <input
              autoFocus
              className="min-w-0 flex-1 rounded-md border border-slate-200/60 bg-white/60 px-2 py-0.5 text-[13px] font-semibold text-slate-950/86 outline-none focus:border-primary/40"
              maxLength={64}
              onChange={(e) => set_edit_value(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") confirm_edit();
                if (e.key === "Escape") cancel_edit();
              }}
              value={edit_value}
            />
            <WorkspacePillButton
              aria-label="确认"
              class_name="rounded-lg p-1"
              onClick={confirm_edit}
              size="icon"
            >
              <Check className="h-3 w-3 text-emerald-600" />
            </WorkspacePillButton>
            <WorkspacePillButton
              aria-label="取消"
              class_name="rounded-lg p-1"
              onClick={cancel_edit}
              size="icon"
            >
              <X className="h-3 w-3" />
            </WorkspacePillButton>
          </div>
        ) : (
          <button
            className="block w-full text-left"
            onClick={on_select}
            type="button"
          >
            <p className="truncate text-[13px] font-semibold text-slate-950/86">
              {conversation.title?.trim() || "未命名对话"}
            </p>
            <div className="mt-1 flex items-center gap-2 text-[11px] text-slate-700/52">
              <Clock3 className="h-3.5 w-3.5" />
              <span>{formatRelativeTime(conversation.last_activity_at)}</span>
              <span>·</span>
              <span>{conversation.message_count ?? 0} 条消息</span>
            </div>
          </button>
        )}
        {is_editing ? (
          <div className="mt-1 flex items-center gap-2 text-[11px] text-slate-700/52">
            <Clock3 className="h-3.5 w-3.5" />
            <span>{formatRelativeTime(conversation.last_activity_at)}</span>
            <span>·</span>
            <span>{conversation.message_count ?? 0} 条消息</span>
          </div>
        ) : null}
      </div>

      <div className="flex shrink-0 items-center gap-1">
        {!is_editing && can_rename ? (
          <WorkspacePillButton
            aria-label="重命名"
            class_name="rounded-xl p-1.5 opacity-0 group-hover:opacity-100"
            onClick={start_edit}
            size="icon"
          >
            <Pencil className="h-3.5 w-3.5" />
          </WorkspacePillButton>
        ) : null}
        {!is_editing && can_delete ? (
          <WorkspacePillButton
            aria-label="删除对话"
            class_name="rounded-xl p-1.5 opacity-0 group-hover:opacity-100 hover:text-destructive"
            onClick={on_delete}
            size="icon"
          >
            <Trash2 className="h-3.5 w-3.5" />
          </WorkspacePillButton>
        ) : null}
      </div>
    </div>
  );
}
