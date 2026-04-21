"use client";

import { useMemo, useState } from "react";
import { ArrowLeft, Check, ChevronDown, MessageSquare, X } from "lucide-react";

import { format_relative_time, get_icon_avatar_src, get_initials } from "@/lib/utils";
import { Agent } from "@/types/agent/agent";
import { AgentConversationIdentity } from "@/types/agent/agent-conversation";
import { ConversationSnapshotPayload, RoomConversationView } from "@/types/conversation/conversation";

import { DmChatPanel } from "@/features/conversation/room/dm/dm-chat-panel";
import { GroupChatPanel } from "../group/chat/group-chat-panel";
import { GroupThreadContextProvider } from "../group/thread/group-thread-context";
import { GroupThreadDetailPanel } from "../group/thread/group-thread-detail-panel";
import { useGroupThread, useGroupThreadPanelData } from "../group/thread/group-thread-state";

interface RoomMobileSurfaceProps {
  current_agent: Agent;
  current_room_type: string;
  room_id: string | null;
  room_members: Agent[];
  current_room_title: string;
  current_room_conversation: RoomConversationView | null;
  current_agent_session_identity: AgentConversationIdentity | null;
  conversation_id: string | null;
  current_room_conversations: RoomConversationView[];
  initial_draft?: string | null;
  on_initial_draft_consumed?: () => void;
  on_back_to_directory: () => void;
  on_create_conversation: (title?: string) => void | Promise<string | null>;
  on_select_conversation: (conversation_id: string) => void;
  on_loading_change: (is_loading: boolean) => void;
  on_conversation_snapshot_change: (snapshot: ConversationSnapshotPayload) => void;
  on_room_event?: (event_type: string, data: import("@/types/agent/agent-conversation").RoomEventPayload) => void;
}

export function RoomMobileSurface({
  current_agent,
  current_room_type,
  room_id,
  room_members,
  current_room_title,
  current_room_conversation,
  current_agent_session_identity,
  conversation_id,
  current_room_conversations,
  initial_draft = null,
  on_initial_draft_consumed,
  on_back_to_directory,
  on_create_conversation,
  on_select_conversation,
  on_loading_change,
  on_conversation_snapshot_change,
  on_room_event,
}: RoomMobileSurfaceProps) {
  const [is_conversation_sheet_open, setIsConversationSheetOpen] = useState(false);
  const is_dm = current_room_type === "dm";
  const current_agent_avatar_src = get_icon_avatar_src(current_agent.avatar);

  const current_room_conversation_title = useMemo(() => {
    if (current_room_conversation?.title?.trim()) {
      return current_room_conversation.title;
    }
    return "新会话";
  }, [current_room_conversation]);

  return (
    <section className="flex min-h-0 flex-1 flex-col overflow-hidden bg-background/90">
      <div className="px-2 pb-2 pt-2">
        <div className="radius-shell-lg flex items-center gap-2 px-2 py-2">
          <button
            className="inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-2xl text-(--text-strong) transition hover:bg-(--interaction-hover-background) hover:text-(--text-strong)"
            onClick={on_back_to_directory}
            type="button"
          >
            <ArrowLeft className="h-4 w-4" />
          </button>

          <button
            className="flex min-w-0 flex-1 items-center gap-3 rounded-[24px] border border-(--divider-subtle-color) px-3 py-2 text-left transition hover:bg-(--interaction-hover-background)"
            onClick={() => setIsConversationSheetOpen(true)}
            type="button"
          >
            <div className="flex h-8 w-8 shrink-0 items-center justify-center overflow-hidden rounded-full border border-(--surface-avatar-border) bg-(--surface-avatar-background) text-[11px] font-bold text-(--text-strong) shadow-(--surface-avatar-shadow)">
              {current_agent_avatar_src ? (
                <img
                  alt={current_agent.name}
                  className="h-full w-full object-cover"
                  src={current_agent_avatar_src}
                />
              ) : (
                get_initials(current_agent.name, "DM", 2)
              )}
            </div>

            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-semibold text-(--text-strong)">{current_agent.name}</p>
              <p className="truncate text-[12px] text-(--text-muted)">
                {current_room_title || current_room_conversation_title}
              </p>
            </div>

            <ChevronDown className="h-4 w-4 shrink-0 text-(--text-muted)" />
          </button>

          <div className="inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-2xl border border-(--divider-subtle-color) text-(--text-muted)">
            <MessageSquare className="h-4 w-4" />
          </div>
        </div>
      </div>

      <div className="min-h-0 min-w-0 flex-1">
        {is_dm ? (
          <DmChatPanel
            current_agent_name={current_agent.name}
            current_agent_avatar={current_agent.avatar ?? null}
            initial_draft={initial_draft}
            layout="mobile"
            on_conversation_snapshot_change={on_conversation_snapshot_change}
            on_initial_draft_consumed={on_initial_draft_consumed}
            on_loading_change={on_loading_change}
            on_room_event={on_room_event}
            session_identity={current_agent_session_identity}
          />
        ) : (
          <GroupThreadContextProvider>
            <GroupChatPanel
              agent_id={current_agent.agent_id}
              conversation_id={conversation_id}
              current_agent_name={current_agent.name}
              current_agent_avatar={current_agent.avatar ?? null}
              initial_draft={initial_draft}
              layout="mobile"
              on_conversation_snapshot_change={on_conversation_snapshot_change}
              on_create_conversation={on_create_conversation}
              on_initial_draft_consumed={on_initial_draft_consumed}
              on_loading_change={on_loading_change}
              on_room_event={on_room_event}
              room_id={room_id}
              room_members={room_members}
            />
            <MobileThreadOverlay />
          </GroupThreadContextProvider>
        )}
      </div>

      {is_conversation_sheet_open ? (
        <>
          <button
            aria-label="关闭会话列表"
            className="absolute inset-0 z-30 bg-(--dialog-backdrop-color)"
            onClick={() => setIsConversationSheetOpen(false)}
            type="button"
          />

          <div className="absolute inset-x-0 bottom-0 z-40 rounded-t-[28px] border-t border-(--surface-panel-border) bg-(--surface-panel-background) px-4 pb-6 pt-3 shadow-[0_-20px_40px_rgba(0,0,0,0.12)]">
            <div className="mx-auto mb-4 h-1.5 w-12 rounded-full bg-(--divider-strong-color)" />

            <div className="mb-4 flex items-center justify-between gap-3">
              <div>
                <p className="text-sm font-semibold text-(--text-strong)">切换会话</p>
                <p className="text-xs text-(--text-muted)">
                  {current_room_conversations.length} 个会话
                </p>
              </div>

              <button
                className="inline-flex h-9 w-9 items-center justify-center rounded-full text-(--text-muted) transition hover:bg-(--interaction-hover-background) hover:text-(--text-strong)"
                onClick={() => setIsConversationSheetOpen(false)}
                type="button"
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            <div className="max-h-[50vh] space-y-2 overflow-y-auto pr-1">
              {current_room_conversations.map((conversation) => {
                const is_active = conversation.conversation_id === conversation_id;
                return (
                  <button
                    key={conversation.conversation_id}
                    className="flex w-full items-start gap-3 rounded-2xl border border-(--divider-subtle-color) px-3 py-3 text-left transition hover:bg-(--interaction-hover-background)"
                    onClick={() => {
                      on_select_conversation(conversation.conversation_id);
                      setIsConversationSheetOpen(false);
                    }}
                    type="button"
                  >
                    <div className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-full border border-(--divider-subtle-color) text-(--text-strong)">
                      {is_active ? <Check className="h-4 w-4" /> : <MessageSquare className="h-4 w-4" />}
                    </div>

                    <div className="min-w-0 flex-1">
                      <p className="truncate text-sm font-medium text-(--text-strong)">
                        {conversation.title?.trim() || "未命名会话"}
                      </p>
                      <p className="mt-1 text-xs text-(--text-muted)">
                        {format_relative_time(conversation.last_activity_at)} · {conversation.message_count ?? 0} 条
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

/** 移动端 Thread 全屏覆盖 — 在 GroupThreadContextProvider 内部使用 */
function MobileThreadOverlay() {
  const { active_thread, close_thread } = useGroupThread();
  const { thread_panel_data } = useGroupThreadPanelData();

  if (!active_thread || !thread_panel_data) return null;

  return (
    <div className="fixed inset-0 z-50 bg-(--surface-panel-background)">
      <GroupThreadDetailPanel
        round_id={active_thread.round_id}
        agent_id={active_thread.agent_id}
        agent_name={thread_panel_data.agent_name ?? active_thread.agent_id}
        agent_avatar={thread_panel_data.agent_avatar}
        messages={thread_panel_data.messages}
        pending_permissions={thread_panel_data.pending_permissions}
        on_permission_response={thread_panel_data.on_permission_response}
        can_respond_to_permissions={thread_panel_data.can_respond_to_permissions}
        permission_read_only_reason={thread_panel_data.permission_read_only_reason}
        on_close={close_thread}
        on_stop_message={thread_panel_data.on_stop_message}
        on_open_workspace_file={thread_panel_data.on_open_workspace_file}
        is_loading={thread_panel_data.is_loading}
        layout="mobile"
      />
    </div>
  );
}
