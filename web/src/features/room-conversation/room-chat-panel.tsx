"use client";

import { useEffect, useMemo } from "react";

import { useAgentConversation } from "@/hooks/agent";
import { useConversationLoader } from "@/hooks/use-conversation-loader";
import { useExtractTodos } from "@/hooks/use-extract-todos";
import { useFollowScroll } from "@/hooks/use-follow-scroll";
import { ConversationSnapshotPayload, Conversation } from "@/types/conversation";
import { TodoItem } from "@/types/todo";
import { Agent } from "@/types/agent";
import { RoomSurfaceTabKey } from "@/types/room-surface";

import { ComposerPanel } from "@/features/conversation-shared/composer-panel";
import { ConversationFeed } from "@/features/conversation-shared/conversation-feed";
import { ScrollToLatestButton } from "@/features/conversation-shared/scroll-to-latest-button";
import { groupMessagesByRound, get_latest_reply_timestamp } from "@/features/conversation-shared/utils";
import { RoomConversationEmptyState } from "./room-conversation-empty-state";
import { RoomConversationHeader } from "./room-conversation-header";

/** 构建 Room 共享 session_key，与后端 room_chat_service.py 一致 */
function build_room_session_key(conversation_id: string): string {
  return `room:group:${conversation_id}`;
}

export interface RoomChatPanelProps {
  agent_id: string | null;
  current_agent_name?: string | null;
  current_room_title?: string | null;
  /** Room conversation id — used to derive the shared session_key */
  conversation_id: string | null;
  room_id?: string | null;
  room_members: Agent[];
  conversations: Conversation[];
  session_title?: string | null;
  /** Controlled tab — caller manages which surface tab is active */
  active_tab?: RoomSurfaceTabKey;
  on_change_tab?: (tab: RoomSurfaceTabKey) => void;
  is_detail_panel_open?: boolean;
  on_toggle_detail_panel?: () => void;
  layout?: "desktop" | "mobile";
  initial_draft?: string | null;
  hide_header?: boolean;
  on_open_workspace_file?: (path: string) => void;
  on_todos_change?: (todos: TodoItem[]) => void;
  on_loading_change?: (is_loading: boolean) => void;
  on_conversation_snapshot_change?: (snapshot: ConversationSnapshotPayload) => void;
  on_create_conversation?: (title?: string) => void | Promise<string | null>;
  on_select_conversation?: (conversation_id: string) => void;
  on_room_event?: (event_type: string, data: import("@/types/agent-conversation").RoomEventPayload) => void;
}

export function RoomChatPanel({
  agent_id,
  current_agent_name,
  current_room_title,
  conversation_id,
  room_id = null,
  room_members,
  conversations,
  session_title,
  active_tab = "chat",
  on_change_tab,
  is_detail_panel_open = false,
  on_toggle_detail_panel,
  layout = "desktop",
  initial_draft = null,
  hide_header = false,
  on_open_workspace_file,
  on_todos_change,
  on_loading_change,
  on_conversation_snapshot_change,
  on_create_conversation,
  on_select_conversation,
  on_room_event,
}: RoomChatPanelProps) {
  const is_mobile_layout = layout === "mobile";

  const session_key = conversation_id ? build_room_session_key(conversation_id) : null;

  const agent_name_map = useMemo(() => {
    if (room_members.length === 0) return undefined;
    const map: Record<string, string> = {};
    for (const member of room_members) {
      map[member.agent_id] = member.name;
    }
    return map;
  }, [room_members]);

  const {
    error,
    messages,
    is_loading,
    pending_permission,
    send_message,
    stop_generation,
    load_conversation,
    send_permission_response,
    delete_round,
    regenerate,
  } = useAgentConversation({
    agent_id,
    room_id,
    conversation_id,
    chat_type: "group",
    on_error: (err) => {
      console.error("Room conversation error:", err);
    },
    on_room_event,
  });

  const {
    scroll_ref,
    feed_ref,
    bottom_anchor_ref,
    show_scroll_to_bottom,
    scroll_to_bottom,
    on_scroll,
    on_wheel,
    on_touch_start,
    on_touch_move,
    on_touch_end,
  } = useFollowScroll({
    trigger_deps: [messages, is_loading] as const,
    session_key,
  });

  const todos = useExtractTodos(messages, session_key);

  useEffect(() => { on_todos_change?.(todos); }, [on_todos_change, todos]);
  useEffect(() => { on_loading_change?.(is_loading); }, [is_loading, on_loading_change]);

  useEffect(() => {
    if (!session_key || messages.length === 0) return;
    const last = messages[messages.length - 1];
    const latest_reply_timestamp = get_latest_reply_timestamp(messages);
    on_conversation_snapshot_change?.({
      conversation_id: conversation_id ?? session_key,
      message_count: messages.length,
      ...(latest_reply_timestamp ? { last_activity_at: latest_reply_timestamp } : {}),
      session_id: last?.session_id ?? null,
    });
  }, [conversation_id, session_key, messages, on_conversation_snapshot_change]);

  useConversationLoader({
    conversation_id: session_key,
    load_conversation,
    debug_name: "RoomChatPanel",
  });

  const message_groups = useMemo(() => groupMessagesByRound(messages), [messages]);
  const round_ids = Array.from(message_groups.keys());

  const handle_send_message = async (content: string) => {
    if (!content.trim() || is_loading) return;
    scroll_to_bottom("auto");
    try {
      await send_message(content);
    } catch (err) {
      console.error("Failed to send message:", err);
    }
  };

  const handle_stop = () => stop_generation();
  const handle_stop_message = (msg_id: string) => stop_generation(msg_id);

  return (
    <div className="relative flex h-full min-w-0 flex-1 flex-col overflow-hidden bg-transparent">

      {error && error.includes("服务器") ? (
        <div className="absolute left-1/2 top-4 z-50 max-w-md -translate-x-1/2">
          <div className="rounded-2xl border border-destructive/20 bg-destructive/8 p-3">
            <div className="flex items-start gap-3">
              <svg className="mt-0.5 h-5 w-5 shrink-0 text-destructive" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path
                  d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                />
              </svg>
              <div className="flex-1">
                <p className="text-sm font-medium text-destructive">无法连接到后端服务</p>
                <p className="mt-1 text-xs text-muted-foreground">
                  请确保后端服务正在运行 (端口 8010)
                </p>
              </div>
            </div>
          </div>
        </div>
      ) : null}

      {!session_key ? (
        <RoomConversationEmptyState on_create_conversation={on_create_conversation ?? (() => { })} />
      ) : (
        <>
          {!is_mobile_layout && !hide_header ? (
            <RoomConversationHeader
              active_tab={active_tab}
              conversations={conversations}
              current_conversation_id={session_key}
              current_conversation_title={session_title ?? null}
              current_room_title={current_room_title ?? null}
              is_detail_panel_open={is_detail_panel_open}
              is_loading={is_loading}
              on_change_tab={on_change_tab ?? (() => { })}
              on_create_conversation={
                on_create_conversation
                  ? async (title) => {
                    const result = await on_create_conversation(title);
                    return typeof result === "string" ? result : null;
                  }
                  : undefined
              }
              on_select_conversation={on_select_conversation ?? (() => { })}
              on_toggle_detail_panel={on_toggle_detail_panel ?? (() => { })}
              room_members={room_members}
            />
          ) : null}

          <div
            ref={scroll_ref}
            className={
              is_mobile_layout
                ? "soft-scrollbar relative z-0 min-w-0 flex-1 space-y-4 overflow-x-hidden overflow-y-auto px-1 py-2"
                : "soft-scrollbar relative z-0 min-w-0 flex-1 overflow-x-hidden overflow-y-auto bg-[#fcfcfd] px-4 py-5 sm:px-6 sm:py-6 xl:px-8 xl:py-7"
            }
            style={{ overflowAnchor: "none" }}
            onScroll={on_scroll}
            onTouchEnd={on_touch_end}
            onTouchMove={on_touch_move}
            onTouchStart={on_touch_start}
            onWheel={on_wheel}
          >
            <ConversationFeed
              agent_name_map={agent_name_map}
              bottom_anchor_ref={bottom_anchor_ref}
              feed_ref={feed_ref}
              scroll_ref={scroll_ref}
              current_agent_name={current_agent_name ?? null}
              is_last_round_pending_permission={pending_permission}
              is_loading={is_loading}
              is_mobile_layout={is_mobile_layout}
              message_groups={message_groups}
              on_delete_round={delete_round}
              on_open_workspace_file={on_open_workspace_file}
              on_permission_response={send_permission_response}
              on_regenerate_round={regenerate}
              on_stop_message={handle_stop_message}
              round_ids={round_ids}
            />
          </div>

          {show_scroll_to_bottom ? (
            <ScrollToLatestButton
              is_loading={is_loading}
              is_mobile_layout={is_mobile_layout}
              on_click={() => scroll_to_bottom("smooth")}
            />
          ) : null}

          <ComposerPanel
            compact={is_mobile_layout}
            current_agent_name={current_agent_name ?? null}
            initial_draft={initial_draft}
            is_loading={is_loading}
            on_send_message={handle_send_message}
            on_stop={handle_stop}
            room_members={room_members}
          />
        </>
      )}
    </div>
  );
}
