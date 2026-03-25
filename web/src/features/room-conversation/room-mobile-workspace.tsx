"use client";

import { useMemo, useState } from "react";
import { ArrowLeft, Check, ChevronDown, MessageSquare, Search, X } from "lucide-react";

import { formatRelativeTime } from "@/lib/utils";
import { Agent } from "@/types/agent";
import { Conversation, ConversationSnapshotPayload } from "@/types/conversation";

import { RoomChatPanel } from "./room-chat-panel";

interface RoomMobileWorkspaceProps {
  current_agent: Agent;
  current_room_title: string;
  current_conversation: Conversation | null;
  current_conversation_id: string | null;
  current_room_conversations: Conversation[];
  on_back_to_directory: () => void;
  on_create_conversation: () => void;
  on_select_conversation: (conversation_id: string) => void;
  on_loading_change: (is_loading: boolean) => void;
  on_conversation_snapshot_change: (snapshot: ConversationSnapshotPayload) => void;
}

export function RoomMobileWorkspace({
  current_agent,
  current_room_title,
  current_conversation,
  current_conversation_id,
  current_room_conversations,
  on_back_to_directory,
  on_select_conversation,
  on_loading_change,
  on_conversation_snapshot_change,
}: RoomMobileWorkspaceProps) {
  const [is_conversation_sheet_open, setIsConversationSheetOpen] = useState(false);

  const current_conversation_title = useMemo(() => {
    if (current_conversation?.title?.trim()) {
      return current_conversation.title;
    }
    return "新会话";
  }, [current_conversation]);

  return (
    <section className="flex min-h-0 flex-1 flex-col overflow-hidden bg-background/90">
      <div className="px-2 pb-2 pt-2">
        <div className="workspace-shell radius-shell-lg flex items-center gap-2 px-2 py-2">
          <button
            className="workspace-chip inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-2xl text-slate-900/82 transition hover:text-slate-950"
            onClick={on_back_to_directory}
            type="button"
          >
            <ArrowLeft className="h-4 w-4" />
          </button>

          <button
            className="workspace-card flex min-w-0 flex-1 items-center gap-3 rounded-[24px] px-3 py-2 text-left transition hover:bg-white/18"
            onClick={() => setIsConversationSheetOpen(true)}
            type="button"
          >
            <div className="workspace-chip flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-slate-700/56">
              <Search className="h-4 w-4" />
            </div>

            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-semibold text-slate-900/84">{current_agent.name}</p>
              <p className="truncate text-[12px] text-slate-700/54">
                {current_room_title || current_conversation_title}
              </p>
            </div>

            <ChevronDown className="h-4 w-4 shrink-0 text-slate-700/50" />
          </button>

          <div className="workspace-chip inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-2xl text-slate-700/44">
            <MessageSquare className="h-4 w-4" />
          </div>
        </div>
      </div>

      <div className="min-h-0 min-w-0 flex-1">
        <RoomChatPanel
          agent_id={current_agent.agent_id}
          current_agent_name={current_agent.name}
          current_room_title={current_room_title}
          layout="mobile"
          on_conversation_snapshot_change={on_conversation_snapshot_change}
          on_loading_change={on_loading_change}
          session_key={current_conversation_id}
          session_title={current_conversation?.title ?? null}
        />
      </div>

      {is_conversation_sheet_open ? (
        <>
          <button
            aria-label="关闭会话列表"
            className="absolute inset-0 z-30 bg-black/20 backdrop-blur-[1px]"
            onClick={() => setIsConversationSheetOpen(false)}
            type="button"
          />

          <div className="workspace-shell absolute inset-x-0 bottom-0 z-40 rounded-t-[28px] border-t border-white/60 px-4 pb-6 pt-3 shadow-[0_-20px_40px_rgba(0,0,0,0.12)] backdrop-blur-md">
            <div className="mx-auto mb-4 h-1.5 w-12 rounded-full bg-black/10" />

            <div className="mb-4 flex items-center justify-between gap-3">
              <div>
                <p className="text-sm font-semibold text-slate-900/84">切换会话</p>
                <p className="text-xs text-slate-700/54">
                  {current_room_conversations.length} 个会话
                </p>
              </div>

              <button
                className="workspace-chip inline-flex h-9 w-9 items-center justify-center rounded-full text-slate-700/54 transition hover:text-slate-950"
                onClick={() => setIsConversationSheetOpen(false)}
                type="button"
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            <div className="max-h-[50vh] space-y-2 overflow-y-auto pr-1">
              {current_room_conversations.map((conversation) => {
                const is_active = conversation.session_key === current_conversation_id;
                return (
                  <button
                    key={conversation.session_key}
                    className="workspace-card flex w-full items-start gap-3 rounded-2xl px-3 py-3 text-left transition hover:bg-white/18"
                    onClick={() => {
                      on_select_conversation(conversation.session_key);
                      setIsConversationSheetOpen(false);
                    }}
                    type="button"
                  >
                    <div className="workspace-chip mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-slate-900/76">
                      {is_active ? <Check className="h-4 w-4" /> : <MessageSquare className="h-4 w-4" />}
                    </div>

                    <div className="min-w-0 flex-1">
                      <p className="truncate text-sm font-medium text-slate-900/84">
                        {conversation.title?.trim() || "未命名会话"}
                      </p>
                      <p className="mt-1 text-xs text-slate-700/54">
                        {formatRelativeTime(conversation.last_activity_at)} · {conversation.message_count ?? 0} 条
                      </p>
                    </div>
                  </button>
                );
              })}
            </div>
          </div>
        </>
      ) : null}
    </section>
  );
}
