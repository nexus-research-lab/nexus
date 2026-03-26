"use client";

import { Clock3, MessageSquarePlus, Trash2 } from "lucide-react";

import { formatRelativeTime } from "@/lib/utils";
import { Conversation } from "@/types/conversation";

interface RoomConversationHistoryViewProps {
  can_manage_conversations?: boolean;
  conversations: Conversation[];
  current_conversation_id: string | null;
  current_room_type: string;
  on_create_conversation: (title?: string) => Promise<string | null>;
  on_delete_conversation: (conversation_id: string) => Promise<string | null>;
  on_select_conversation: (conversation_id: string) => void;
}

export function RoomConversationHistoryView({
  can_manage_conversations = true,
  conversations,
  current_conversation_id,
  current_room_type,
  on_create_conversation,
  on_delete_conversation,
  on_select_conversation,
}: RoomConversationHistoryViewProps) {
  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden bg-transparent">
      <div className="border-b workspace-divider px-5 py-2.5 xl:px-6">
        <div className="mx-auto flex w-full max-w-[760px] items-center justify-between gap-3">
          <div>
            <p className="text-[10px] font-semibold uppercase tracking-[0.18em] text-slate-700/44">
              History
            </p>
            <h2 className="mt-1 text-[16px] font-black tracking-[-0.04em] text-slate-950/88">
              {current_room_type === "dm" ? "历史对话" : "对话历史"}
            </h2>
          </div>
          {can_manage_conversations ? (
            <button
              className="workspace-chip inline-flex items-center gap-2 rounded-full px-3 py-1.5 text-[11px] font-semibold text-slate-900/78"
              onClick={() => {
                void on_create_conversation();
              }}
              type="button"
            >
              <MessageSquarePlus className="h-3.5 w-3.5" />
              新建对话
            </button>
          ) : null}
        </div>
      </div>

      <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto px-4 py-3 sm:px-5 xl:px-6">
        <div className="mx-auto w-full max-w-[760px] space-y-2">
          {conversations.map((conversation) => {
            const is_active = conversation.session_key === current_conversation_id;
            const can_delete = conversation.conversation_type === "topic";
            return (
              <button
                key={conversation.session_key}
                className={`group flex w-full items-center justify-between gap-4 rounded-[14px] border px-4 py-2.5 text-left transition-all duration-300 ${
                  is_active
                    ? "border-white/32 bg-white/22 shadow-[0_10px_18px_rgba(111,126,162,0.08)]"
                    : "border-white/14 bg-white/8 hover:bg-white/12"
                }`}
                onClick={() => on_select_conversation(conversation.session_key)}
                type="button"
              >
                <div className="min-w-0 flex-1">
                  <p className="truncate text-[13px] font-semibold text-slate-950/86">
                    {conversation.title?.trim() || "未命名对话"}
                  </p>
                  <div className="mt-1 flex items-center gap-2 text-[11px] text-slate-700/52">
                    <Clock3 className="h-3.5 w-3.5" />
                    <span>{formatRelativeTime(conversation.last_activity_at)}</span>
                    <span>·</span>
                    <span>{conversation.message_count ?? 0} 条消息</span>
                  </div>
                </div>

                {can_manage_conversations && can_delete ? (
                  <button
                    aria-label="删除对话"
                    className="workspace-chip rounded-xl p-1.5 text-slate-700/54 opacity-0 transition-all group-hover:opacity-100 hover:text-destructive"
                    onClick={(event) => {
                      event.stopPropagation();
                      void on_delete_conversation(conversation.session_key);
                    }}
                    type="button"
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </button>
                ) : null}
              </button>
            );
          })}
        </div>
      </div>
    </div>
  );
}
