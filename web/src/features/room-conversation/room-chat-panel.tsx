"use client";

import { useCallback, useEffect, useMemo, useRef } from "react";

import { useAgentConversation } from "@/hooks/agent";
import { useSessionLoader } from "@/hooks/use-session-loader";
import { useExtractTodos } from "@/hooks/use-extract-todos";
import { useFollowScroll } from "@/hooks/use-follow-scroll";
import { buildRoomSharedSessionKey } from "@/lib/session-key";
import { AgentConversationIdentity, getSessionControlStatusText } from "@/types/agent-conversation";
import { RoomConversationSnapshotPayload } from "@/types/conversation";
import { PendingPermission } from "@/types/permission";
import { TodoItem } from "@/types/todo";
import { Agent } from "@/types/agent";

import { ScrollToLatestButton } from "@/features/conversation-shared/scroll-to-latest-button";
import {
  buildRoomAgentRoundEntries,
  getRoomAgentRoundEntry,
  getRoomBaseRoundId,
  getRoomThreadMessages,
  get_latest_reply_timestamp,
  groupRoomPendingPermissionsByRound,
  groupRoomPendingSlotsByRound,
  groupRoomMessagesByRound,
  isAgentRoundActive,
} from "@/features/conversation-shared/utils";
import { RoomConversationFeed } from "./room-conversation-feed";
import { useRoomThread, useSetThreadPanelData } from "./thread/room-thread-state";
import { RoomComposerPanel } from "./room-composer-panel";
import { RoomConversationEmptyState } from "./room-conversation-empty-state";

export interface RoomChatPanelProps {
  agent_id: string | null;
  current_agent_name?: string | null;
  /** Room conversation id — used to derive the shared session_key */
  conversation_id: string | null;
  room_id?: string | null;
  room_members: Agent[];
  layout?: "desktop" | "mobile";
  initial_draft?: string | null;
  on_initial_draft_consumed?: () => void;
  on_open_workspace_file?: (path: string) => void;
  on_todos_change?: (todos: TodoItem[]) => void;
  on_loading_change?: (is_loading: boolean) => void;
  on_conversation_snapshot_change?: (snapshot: RoomConversationSnapshotPayload) => void;
  on_create_conversation?: (title?: string) => void | Promise<string | null>;
  on_room_event?: (event_type: string, data: import("@/types/agent-conversation").RoomEventPayload) => void;
}

function get_thread_pending_permissions(
  round_id: string,
  agent_id: string,
  pending_permissions: PendingPermission[],
): PendingPermission[] {
  if (pending_permissions.length === 0) {
    return [];
  }

  return pending_permissions.filter((permission) => {
    if (permission.agent_id !== agent_id) {
      return false;
    }
    if (!permission.caused_by) {
      return false;
    }
    if (getRoomBaseRoundId(permission.caused_by, permission.agent_id) !== round_id) {
      return false;
    }
    // 中文注释：Room 的权限请求在很多场景下绑定的是占位槽位 msg_id，
    // 不是 assistant 真正的 message_id。Thread 已经按 round_id + agent_id 收口，
    // 这里不能再按 message_id 二次过滤，否则问答/权限会被错误吞掉。
    return true;
  });
}

/**
 * RoomChatPanel — 必须在 RoomThreadContextProvider 内部使用。
 * Provider 由 RoomWorkspaceLayout / RoomMobileWorkspace 提供。
 */
export function RoomChatPanel({
  agent_id,
  current_agent_name,
  conversation_id,
  room_id = null,
  room_members,
  layout = "desktop",
  initial_draft = null,
  on_initial_draft_consumed,
  on_open_workspace_file,
  on_todos_change,
  on_loading_change,
  on_conversation_snapshot_change,
  on_create_conversation,
  on_room_event,
}: RoomChatPanelProps) {
  const is_mobile_layout = layout === "mobile";
  const { active_thread, close_thread } = useRoomThread();
  const { set_thread_panel_data } = useSetThreadPanelData();
  const thread_loading_ref = useRef(false);
  const consumed_initial_draft_ref = useRef<string | null>(null);

  const session_key = conversation_id ? buildRoomSharedSessionKey(conversation_id) : null;
  const session_identity = useMemo<AgentConversationIdentity | null>(() => {
    if (!conversation_id) {
      return null;
    }

    return {
      session_key,
      agent_id,
      room_id,
      conversation_id,
      chat_type: "group",
    };
  }, [agent_id, conversation_id, room_id, session_key]);

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
    session_control_state,
    session_observer_count,
    pending_agent_slots,
    pending_permissions,
    send_message,
    stop_generation,
    load_session,
    send_permission_response,
  } = useAgentConversation({
    identity: session_identity,
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
  const can_control_session = session_control_state !== "observer";
  const observer_read_only_reason = "当前窗口是观察视图，控制权在另一窗口";
  const session_control_text = useMemo(
    () => getSessionControlStatusText(session_control_state, session_observer_count),
    [session_control_state, session_observer_count],
  );

  useEffect(() => { on_todos_change?.(todos); }, [on_todos_change, todos]);
  useEffect(() => { on_loading_change?.(is_loading); }, [is_loading, on_loading_change]);

  // 切换对话时自动关闭 Thread 面板
  useEffect(() => { close_thread(); }, [conversation_id, close_thread]);

  useEffect(() => {
    if (!conversation_id || messages.length === 0) return;
    const last = messages[messages.length - 1];
    const latest_reply_timestamp = get_latest_reply_timestamp(messages);
    on_conversation_snapshot_change?.({
      conversation_id,
      message_count: messages.length,
      ...(latest_reply_timestamp ? { last_activity_at: latest_reply_timestamp } : {}),
      session_id: last?.session_id ?? null,
    });
  }, [conversation_id, messages, on_conversation_snapshot_change]);

  useSessionLoader({
    session_key,
    load_session,
    debug_name: "RoomChatPanel",
  });

  const message_groups = useMemo(() => groupRoomMessagesByRound(messages), [messages]);
  const pending_slot_groups = useMemo(
    () => groupRoomPendingSlotsByRound(pending_agent_slots),
    [pending_agent_slots],
  );
  const pending_permission_groups = useMemo(
    () => groupRoomPendingPermissionsByRound(pending_permissions),
    [pending_permissions],
  );
  const round_ids = Array.from(message_groups.keys());
  const mention_unavailable_agent_ids = useMemo(() => {
    const next_ids = new Set<string>();
    for (const round_id of round_ids) {
      const round_messages = message_groups.get(round_id) ?? [];
      const round_pending_slots = pending_slot_groups.get(round_id) ?? [];
      for (const entry of buildRoomAgentRoundEntries(round_messages, round_pending_slots)) {
        if (isAgentRoundActive(entry.status)) {
          next_ids.add(entry.agent_id);
        }
      }
    }
    return Array.from(next_ids);
  }, [message_groups, pending_slot_groups, round_ids]);

  const handle_send_message = async (content: string) => {
    if (!content.trim()) return;
    scroll_to_bottom("auto");
    try {
      await send_message(content);
    } catch (err) {
      console.error("Failed to send message:", err);
    }
  };

  const handle_stop_message = useCallback((msg_id: string) => stop_generation(msg_id), [stop_generation]);

  useEffect(() => {
    const normalized_draft = initial_draft?.trim() ?? "";
    if (!session_key || !normalized_draft || is_loading || !can_control_session) {
      return;
    }

    const initial_draft_signature = `${session_key}:${normalized_draft}`;
    if (consumed_initial_draft_ref.current === initial_draft_signature) {
      return;
    }

    consumed_initial_draft_ref.current = initial_draft_signature;
    scroll_to_bottom("auto");
    void send_message(normalized_draft)
      .then(() => {
        on_initial_draft_consumed?.();
      })
      .catch((error) => {
        consumed_initial_draft_ref.current = null;
        console.error("Failed to auto send initial room prompt:", error);
      });
  }, [can_control_session, initial_draft, is_loading, on_initial_draft_consumed, scroll_to_bottom, send_message, session_key]);

  // Thread 面板数据：推送到 Context，由 Layout 读取渲染 inspector
  const thread_round_messages = useMemo(
    () => active_thread ? message_groups.get(active_thread.round_id) ?? [] : [],
    [active_thread, message_groups],
  );
  const thread_messages = useMemo(() => {
    if (!active_thread) {
      return [];
    }

    return getRoomThreadMessages(thread_round_messages, active_thread.agent_id);
  }, [active_thread, thread_round_messages]);
  const thread_entry = useMemo(
    () => active_thread
      ? getRoomAgentRoundEntry(
        thread_round_messages,
        active_thread.agent_id,
        pending_slot_groups.get(active_thread.round_id) ?? [],
      )
      : null,
    [active_thread, pending_slot_groups, thread_round_messages],
  );
  const thread_is_loading = useMemo(
    () => Boolean(thread_entry && isAgentRoundActive(thread_entry.status)),
    [thread_entry],
  );
  const thread_agent_name = active_thread && agent_name_map
    ? agent_name_map[active_thread.agent_id] ?? active_thread.agent_id
    : null;
  const thread_pending_permissions = useMemo(
    () => active_thread
      ? get_thread_pending_permissions(
        active_thread.round_id,
        active_thread.agent_id,
        pending_permission_groups.get(active_thread.round_id) ?? [],
      )
      : [],
    [active_thread, pending_permission_groups],
  );
  const thread_panel_data = useMemo(() => {
    if (!active_thread) {
      return null;
    }

    return {
      messages: thread_messages,
      agent_name: thread_agent_name,
      is_loading: thread_is_loading,
      pending_permissions: thread_pending_permissions,
      on_permission_response: send_permission_response,
      can_respond_to_permissions: can_control_session,
      permission_read_only_reason: observer_read_only_reason,
      on_stop_message: can_control_session ? handle_stop_message : undefined,
      on_open_workspace_file,
    };
  }, [
    active_thread,
    can_control_session,
    handle_stop_message,
    on_open_workspace_file,
    observer_read_only_reason,
    send_permission_response,
    thread_agent_name,
    thread_is_loading,
    thread_messages,
    thread_pending_permissions,
  ]);

  useEffect(() => {
    if (!active_thread) {
      thread_loading_ref.current = false;
      return;
    }

    if (
      active_thread.auto_close_on_finish &&
      thread_loading_ref.current &&
      !thread_is_loading
    ) {
      thread_loading_ref.current = false;
      close_thread();
      return;
    }

    thread_loading_ref.current = thread_is_loading;
  }, [active_thread, close_thread, thread_is_loading]);

  useEffect(() => {
    set_thread_panel_data(thread_panel_data);
  }, [set_thread_panel_data, thread_panel_data]);

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
          <div
            ref={scroll_ref}
            className={
              is_mobile_layout
                ? "soft-scrollbar relative z-0 min-w-0 flex-1 overflow-x-hidden overflow-y-auto px-1 py-2"
                : "soft-scrollbar relative z-0 min-w-0 flex-1 overflow-x-hidden overflow-y-auto bg-[var(--surface-canvas-background)] px-4 py-5 sm:px-6 sm:py-6 xl:px-8 xl:py-7"
            }
            style={{ overflowAnchor: "none" }}
            onScroll={on_scroll}
            onTouchEnd={on_touch_end}
            onTouchMove={on_touch_move}
            onTouchStart={on_touch_start}
            onWheel={on_wheel}
          >
            <RoomConversationFeed
              agent_name_map={agent_name_map}
              bottom_anchor_ref={bottom_anchor_ref}
              feed_ref={feed_ref}
              scroll_ref={scroll_ref}
              current_agent_name={current_agent_name ?? null}
              is_last_round_pending_permissions={pending_permissions}
              is_loading={is_loading}
              is_mobile_layout={is_mobile_layout}
              message_groups={message_groups}
              pending_permission_groups={pending_permission_groups}
              pending_slot_groups={pending_slot_groups}
              on_open_workspace_file={on_open_workspace_file}
              on_permission_response={send_permission_response}
              can_respond_to_permissions={can_control_session}
              permission_read_only_reason={observer_read_only_reason}
              on_stop_message={can_control_session ? handle_stop_message : undefined}
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

          <RoomComposerPanel
            compact={is_mobile_layout}
            control_status_text={session_control_text}
            mention_unavailable_agent_ids={mention_unavailable_agent_ids}
            on_send_message={handle_send_message}
            room_members={room_members}
            disabled={!can_control_session}
          />
        </>
      )}
    </div>
  );
}
