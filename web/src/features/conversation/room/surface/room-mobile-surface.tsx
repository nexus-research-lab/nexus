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
import { useGroupThread } from "../group/thread/group-thread-state";
import { useRoomThreadPanel } from "../group/chat/use-room-thread-panel-data";

interface RoomMobileSurfaceProps {
  current_agent: Agent;
  current_room_type: string;
  room_id: string | null;
  room_members: Agent[];
  room_host_agent_id?: string | null;
  room_host_auto_reply_enabled: boolean;
  current_room_title: string;
  current_room_conversation: RoomConversationView | null;
  current_agent_session_identity: AgentConversationIdentity | null;
  conversation_id: string | null;
  current_room_conversations: RoomConversationView[];
  initial_draft?: string | null;
  on_initial_draft_consumed?: () => void;
  on_back_to_directory: () => void;
  on_create_conversation: (title?: string) => void | Promise<string | null>;
  on_select_conversation: (conversationId: string) => void;
  on_loading_change: (isLoading: boolean) => void;
  on_conversation_snapshot_change: (snapshot: ConversationSnapshotPayload) => void;
  on_room_event?: (eventType: string, data: import("@/types/agent/agent-conversation").RoomEventPayload) => void;
}

export function RoomMobileSurface({
  current_agent: currentAgent,
  current_room_type: currentRoomType,
  room_id: roomId,
  room_members: roomMembers,
  room_host_agent_id: roomHostAgentId,
  room_host_auto_reply_enabled: roomHostAutoReplyEnabled,
  current_room_title: currentRoomTitle,
  current_room_conversation: currentRoomConversation,
  current_agent_session_identity: currentAgentSessionIdentity,
  conversation_id: conversationId,
  current_room_conversations: currentRoomConversations,
  initial_draft: initialDraft = null,
  on_initial_draft_consumed: onInitialDraftConsumed,
  on_back_to_directory: onBackToDirectory,
  on_create_conversation: onCreateConversation,
  on_select_conversation: onSelectConversation,
  on_loading_change: onLoadingChange,
  on_conversation_snapshot_change: onConversationSnapshotChange,
  on_room_event: onRoomEvent,
}: RoomMobileSurfaceProps) {
  const [isConversationSheetOpen, setIsConversationSheetOpen] = useState(false);
  const isDm = currentRoomType === "dm";
  const currentAgentAvatarSrc = get_icon_avatar_src(currentAgent.avatar);

  const currentRoomConversationTitle = useMemo(() => {
    if (currentRoomConversation?.title?.trim()) {
      return currentRoomConversation.title;
    }
    return "新会话";
  }, [currentRoomConversation]);

  return (
    <section className="flex min-h-0 flex-1 flex-col overflow-hidden bg-background/90">
      <div className="px-2 pb-2 pt-2">
        <div className="surface-radius-lg flex items-center gap-2 px-2 py-2">
          <button
            className="inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-2xl text-(--text-strong) transition hover:bg-(--interaction-hover-background) hover:text-(--text-strong)"
            onClick={onBackToDirectory}
            type="button"
          >
            <ArrowLeft className="h-4 w-4" />
          </button>

          <button
            className="flex min-w-0 flex-1 items-center gap-3 rounded-[12px] border border-(--divider-subtle-color) px-3 py-2 text-left transition hover:bg-(--interaction-hover-background)"
            onClick={() => setIsConversationSheetOpen(true)}
            type="button"
          >
            <div className="flex h-8 w-8 shrink-0 items-center justify-center overflow-hidden rounded-full border border-(--surface-avatar-border) bg-(--surface-avatar-background) text-[11px] font-bold text-(--text-strong) shadow-(--surface-avatar-shadow)">
              {currentAgentAvatarSrc ? (
                <img
                  alt={currentAgent.name}
                  className="h-full w-full object-cover"
                  src={currentAgentAvatarSrc}
                />
              ) : (
                get_initials(currentAgent.name, "DM", 2)
              )}
            </div>

            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-semibold text-(--text-strong)">{currentAgent.name}</p>
              <p className="truncate text-[12px] text-(--text-muted)">
                {currentRoomTitle || currentRoomConversationTitle}
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
        {isDm ? (
          <DmChatPanel
            current_agent_name={currentAgent.name}
            current_agent_avatar={currentAgent.avatar ?? null}
            current_agent_permission_mode={currentAgent.options.permission_mode ?? null}
            initial_draft={initialDraft}
            layout="mobile"
            on_conversation_snapshot_change={onConversationSnapshotChange}
            on_initial_draft_consumed={onInitialDraftConsumed}
            on_loading_change={onLoadingChange}
            on_room_event={onRoomEvent}
            session_identity={currentAgentSessionIdentity}
          />
        ) : (
          <GroupThreadContextProvider>
            <GroupChatPanel
              agent_id={currentAgent.agent_id}
              conversation_id={conversationId}
              current_agent_name={currentAgent.name}
              current_agent_avatar={currentAgent.avatar ?? null}
              initial_draft={initialDraft}
              layout="mobile"
              on_conversation_snapshot_change={onConversationSnapshotChange}
              on_create_conversation={onCreateConversation}
              on_initial_draft_consumed={onInitialDraftConsumed}
              on_loading_change={onLoadingChange}
              on_room_event={onRoomEvent}
              room_host_agent_id={roomHostAgentId}
              room_host_auto_reply_enabled={roomHostAutoReplyEnabled}
              room_id={roomId}
              room_members={roomMembers}
            />
            <MobileThreadOverlay />
          </GroupThreadContextProvider>
        )}
      </div>

      {isConversationSheetOpen ? (
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
                  {currentRoomConversations.length} 个会话
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
              {currentRoomConversations.map((conversation) => {
                const isActive = conversation.conversation_id === conversationId;
                return (
                  <button
                    key={conversation.conversation_id}
                    className="flex w-full items-start gap-3 rounded-2xl border border-(--divider-subtle-color) px-3 py-3 text-left transition hover:bg-(--interaction-hover-background)"
                    onClick={() => {
                      onSelectConversation(conversation.conversation_id);
                      setIsConversationSheetOpen(false);
                    }}
                    type="button"
                  >
                    <div className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-full border border-(--divider-subtle-color) text-(--text-strong)">
                      {isActive ? <Check className="h-4 w-4" /> : <MessageSquare className="h-4 w-4" />}
                    </div>

                    <div className="min-w-0 flex-1">
                      <p className="truncate text-sm font-medium text-(--text-strong)">
                        {conversation.title?.trim() || "未命名会话"}
                      </p>
                      <p className="mt-1 text-xs text-(--text-muted)">
                        {format_relative_time(conversation.last_activity_at)}
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
  const { active_thread: activeThread, close_thread: closeThread } = useGroupThread();
  const threadPanelData = useRoomThreadPanel();

  if (!activeThread || !threadPanelData) return null;

  return (
    <div className="fixed inset-0 z-50 bg-(--surface-panel-background)">
      <GroupThreadDetailPanel
        round_id={activeThread.round_id}
        agent_id={activeThread.agent_id}
        agent_name={threadPanelData.agent_name ?? activeThread.agent_id}
        agent_avatar={threadPanelData.agent_avatar}
        user_avatar={threadPanelData.user_avatar}
        messages={threadPanelData.messages}
        pending_permissions={threadPanelData.pending_permissions}
        on_permission_response={threadPanelData.on_permission_response}
        can_respond_to_permissions={threadPanelData.can_respond_to_permissions}
        permission_read_only_reason={threadPanelData.permission_read_only_reason}
        on_close={closeThread}
        on_stop_message={threadPanelData.on_stop_message}
        on_open_workspace_file={threadPanelData.on_open_workspace_file}
        is_loading={threadPanelData.is_loading}
        layout="mobile"
      />
    </div>
  );
}
