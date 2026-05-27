"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { useAgentConversation } from "@/hooks/agent";
import { useProviderAvailability } from "@/hooks/capability/use-provider-availability";
import { useExtractTodos } from "@/hooks/conversation/use-extract-todos";
import { useFollowScroll } from "@/hooks/conversation/use-follow-scroll";
import { useSessionLoader } from "@/hooks/conversation/use-session-loader";
import { useDefaultChatDeliveryPolicy } from "@/hooks/settings/use-default-chat-delivery-policy";
import { build_room_shared_session_key } from "@/lib/conversation/session-key";
import { useAuth } from "@/shared/auth/auth-context";
import {
  AgentConversationIdentity,
  get_session_control_status_text,
} from "@/types/agent/agent-conversation";
import { RoomConversationSnapshotPayload } from "@/types/conversation/conversation";
import { TodoItem } from "@/types/conversation/todo";
import { Agent } from "@/types/agent/agent";

import { ScrollToLatestButton } from "@/features/conversation/shared/scroll-to-latest-button";
import { ComposerPanel } from "@/features/conversation/shared/composer-panel";
import { ConversationErrorBubble } from "@/features/conversation/shared/conversation-error-bubble";
import { is_provider_error } from "@/features/conversation/shared/conversation-error-utils";
import { GOAL_COMMAND_HINT_ITEMS } from "@/features/conversation/shared/goal-command-hints";
import { ProviderUnavailableBanner } from "@/features/conversation/shared/provider-unavailable-banner";
import { useGoalCommandHandler } from "@/features/conversation/shared/use-goal-command-handler";
import { useGoalPanelEditRequest } from "@/features/conversation/shared/use-goal-panel-edit-request";
import {
  build_conversation_activity_snapshot,
  get_latest_reply_timestamp,
  group_room_pending_permissions_by_round,
  group_room_pending_slots_by_round,
  group_room_messages_by_round,
  should_emit_conversation_activity,
  type ConversationActivitySnapshot,
} from "@/features/conversation/shared/utils";
import { GroupConversationFeed } from "./group-conversation-feed";
import { RoomGoalPanel } from "./room-goal-panel";
import { useRoomComposerHandlers } from "./use-room-composer-handlers";
import { useRoomThreadPanelData } from "./use-room-thread-panel-data";
import { GroupConversationEmptyState } from "./group-conversation-empty-state";
import { CONVERSATION_TOUR_ANCHORS } from "../../room-tour";

const HISTORY_LOAD_THRESHOLD_PX = 120;

export interface GroupChatPanelProps {
  agent_id: string | null;
  current_agent_name?: string | null;
  current_agent_avatar?: string | null;
  /** Room conversation id — used to derive the shared session_key */
  conversation_id: string | null;
  room_id?: string | null;
  room_members: Agent[];
  room_host_agent_id?: string | null;
  room_host_auto_reply_enabled?: boolean;
  layout?: "desktop" | "mobile";
  initial_draft?: string | null;
  on_initial_draft_consumed?: () => void;
  on_open_agent_contact?: (agent_id: string) => void;
  on_open_workspace_file?: (path: string) => void;
  on_todos_change?: (todos: TodoItem[]) => void;
  on_loading_change?: (is_loading: boolean) => void;
  on_conversation_snapshot_change?: (
    snapshot: RoomConversationSnapshotPayload,
  ) => void;
  on_create_conversation?: (title?: string) => void | Promise<string | null>;
  on_room_event?: (
    event_type: string,
    data: import("@/types/agent/agent-conversation").RoomEventPayload,
  ) => void;
}

/**
 * GroupChatPanel — 必须在 GroupThreadContextProvider 内部使用。
 * Provider 由 RoomSurfaceLayout / RoomMobileSurface 提供。
 */
export function GroupChatPanel({
  agent_id,
  current_agent_name,
  current_agent_avatar,
  conversation_id,
  room_id = null,
  room_members,
  room_host_agent_id,
  room_host_auto_reply_enabled = false,
  layout = "desktop",
  initial_draft = null,
  on_initial_draft_consumed,
  on_open_agent_contact,
  on_open_workspace_file,
  on_todos_change,
  on_loading_change,
  on_conversation_snapshot_change,
  on_create_conversation,
  on_room_event,
}: GroupChatPanelProps) {
  const is_mobile_layout = layout === "mobile";
  const { status: auth_status } = useAuth();
  const current_user_avatar = auth_status?.avatar ?? null;

  const session_key = conversation_id
    ? build_room_shared_session_key(conversation_id)
    : null;
  const default_delivery_policy = useDefaultChatDeliveryPolicy();
  const [goal_refresh_seq, set_goal_refresh_seq] = useState(0);
  const { goal_edit_request, request_goal_edit } = useGoalPanelEditRequest();
  const refresh_goal_panel = useCallback(() => {
    set_goal_refresh_seq((value) => value + 1);
  }, []);
  const { try_handle_goal_command, goal_command_dialog } = useGoalCommandHandler({
    on_edit: request_goal_edit,
    session_key,
    on_refresh: refresh_goal_panel,
  });
  const handle_conversation_event = useCallback(
    (
      event_type: string,
      data: import("@/types/agent/agent-conversation").RoomEventPayload,
    ) => {
      if (event_type.startsWith("goal_")) {
        refresh_goal_panel();
      }
      on_room_event?.(event_type, data);
    },
    [on_room_event, refresh_goal_panel],
  );
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

  const agent_avatar_map = useMemo(() => {
    if (room_members.length === 0) return undefined;
    const map: Record<string, string | null> = {};
    for (const member of room_members) {
      map[member.agent_id] = member.avatar ?? null;
    }
    return map;
  }, [room_members]);

  const {
    error,
    messages,
    is_loading,
    is_history_loading,
    has_more_history,
    history_prepend_token,
    session_control_state,
    session_observer_count,
    pending_agent_slots,
    pending_permissions,
    send_message,
    stop_generation,
    load_session,
    load_older_messages,
    send_permission_response,
    runtime_phase,
    live_round_ids,
    input_queue_items,
    enqueue_input_queue_message,
    delete_input_queue_message,
    guide_input_queue_message,
    reorder_input_queue_messages,
  } = useAgentConversation({
    identity: session_identity,
    on_error: (err) => {
      console.error("Room conversation error:", err);
    },
    on_room_event: handle_conversation_event,
  });

  const todos = useExtractTodos(messages, session_key);
  const { has_available_provider, is_ready: provider_ready } = useProviderAvailability();
  const show_provider_warning = provider_ready && !has_available_provider;
  const system_error = error && !is_provider_error(error) ? error : null;
  const {
    scroll_ref,
    feed_ref,
    bottom_anchor_ref,
    show_scroll_to_bottom,
    scroll_to_bottom,
    prepare_history_prepend_restore,
    cancel_history_prepend_restore,
    on_scroll,
    on_wheel,
    on_touch_start,
    on_touch_move,
    on_touch_end,
  } = useFollowScroll({
    message_count: messages.length,
    auxiliary_block_count:
      pending_agent_slots.length + pending_permissions.length,
    auxiliary_block_key: system_error,
    is_loading,
    session_key,
    history_prepend_token,
  });
  const last_snapshot_key_ref = useRef<string | null>(null);
  const last_activity_snapshot_ref = useRef<ConversationActivitySnapshot | null>(null);
  const can_control_session = session_control_state !== "observer";
  const observer_read_only_reason = "当前窗口是观察视图，控制权在另一窗口";
  const session_control_text = useMemo(
    () =>
      get_session_control_status_text(
        session_control_state,
        session_observer_count,
      ),
    [session_control_state, session_observer_count],
  );

  useEffect(() => {
    on_todos_change?.(todos);
  }, [on_todos_change, todos]);
  useEffect(() => {
    on_loading_change?.(is_loading);
  }, [is_loading, on_loading_change]);

  useEffect(() => {
    if (!conversation_id || messages.length === 0) return;
    const last = messages[messages.length - 1];
    const latest_reply_timestamp = get_latest_reply_timestamp(messages);
    const should_report_last_activity = should_emit_conversation_activity(
      last_activity_snapshot_ref.current,
      conversation_id,
      latest_reply_timestamp,
    );
    const snapshot: RoomConversationSnapshotPayload = {
      conversation_id,
      ...(should_report_last_activity && latest_reply_timestamp !== null
        ? { last_activity_at: latest_reply_timestamp }
        : {}),
      session_id: last?.session_id ?? null,
    };
    const snapshot_key = JSON.stringify(snapshot);
    const next_activity_snapshot = build_conversation_activity_snapshot(
      conversation_id,
      latest_reply_timestamp,
    );

    // Room 历史加载只同步快照，不应该因为切换视图刷新活跃时间。
    if (last_snapshot_key_ref.current === snapshot_key) {
      last_activity_snapshot_ref.current = next_activity_snapshot;
      return;
    }

    last_snapshot_key_ref.current = snapshot_key;
    last_activity_snapshot_ref.current = next_activity_snapshot;
    on_conversation_snapshot_change?.(snapshot);
  }, [
    conversation_id,
    messages,
    on_conversation_snapshot_change,
  ]);

  useSessionLoader({
    session_key,
    load_session,
    debug_name: "GroupChatPanel",
  });

  const message_groups = useMemo(
    () => group_room_messages_by_round(messages),
    [messages],
  );
  const pending_slot_groups = useMemo(
    () => group_room_pending_slots_by_round(pending_agent_slots),
    [pending_agent_slots],
  );
  const pending_permission_groups = useMemo(
    () => group_room_pending_permissions_by_round(pending_permissions),
    [pending_permissions],
  );
  const round_ids = Array.from(message_groups.keys());
  const maybe_load_older_messages = useCallback(async () => {
    const container = scroll_ref.current;
    if (
      !container ||
      !has_more_history ||
      is_history_loading ||
      container.scrollTop > HISTORY_LOAD_THRESHOLD_PX
    ) {
      return;
    }

    prepare_history_prepend_restore();
    const did_prepend = await load_older_messages();
    if (!did_prepend) {
      cancel_history_prepend_restore();
    }
  }, [
    cancel_history_prepend_restore,
    has_more_history,
    is_history_loading,
    load_older_messages,
    prepare_history_prepend_restore,
    scroll_ref,
  ]);

  const handle_scroll = useCallback(() => {
    on_scroll();
    void maybe_load_older_messages();
  }, [maybe_load_older_messages, on_scroll]);

  useEffect(() => {
    const container = scroll_ref.current;
    if (
      !container ||
      !has_more_history ||
      is_history_loading ||
      is_loading ||
      container.scrollHeight > container.clientHeight + 24
    ) {
      return;
    }
    void maybe_load_older_messages();
  }, [
    has_more_history,
    is_history_loading,
    is_loading,
    maybe_load_older_messages,
    messages.length,
    scroll_ref,
  ]);

  const handle_stop_message = useCallback(
    (msg_id: string) => stop_generation(msg_id),
    [stop_generation],
  );
  const { handle_prepare_attachments, handle_send_message } =
    useRoomComposerHandlers({
      can_control_session,
      conversation_id,
      initial_draft,
      is_loading,
      on_initial_draft_consumed,
      room_id,
      scroll_to_bottom,
      send_message,
      session_key,
      try_handle_goal_command,
    });
  useRoomThreadPanelData({
    agent_avatar_map,
    agent_name_map,
    can_control_session,
    conversation_id,
    current_user_avatar,
    is_loading,
    message_groups,
    observer_read_only_reason,
    on_open_workspace_file,
    on_stop_message: handle_stop_message,
    pending_permission_groups,
    pending_slot_groups,
    send_permission_response,
  });
  return (
    <div className="relative flex h-full min-w-0 flex-1 flex-col overflow-hidden bg-transparent">

      {!session_key ? (
        <GroupConversationEmptyState
          on_create_conversation={on_create_conversation ?? (() => {})}
        />
      ) : (
        <>
          <div
            data-tour-anchor={CONVERSATION_TOUR_ANCHORS.feed}
            ref={scroll_ref}
            className={
              is_mobile_layout
                ? "soft-scrollbar relative z-0 min-w-0 flex-1 overflow-x-hidden overflow-y-auto px-1 py-2"
                : "soft-scrollbar relative z-0 min-w-0 flex-1 overflow-x-hidden overflow-y-auto px-4 py-5 sm:px-6 sm:py-6 xl:px-8 xl:py-7"
            }
            style={{ overflowAnchor: "none" }}
            onScroll={handle_scroll}
            onTouchEnd={on_touch_end}
            onTouchMove={on_touch_move}
            onTouchStart={on_touch_start}
            onWheel={on_wheel}
          >
            {is_history_loading ? (
              <div className="mx-auto mb-3 flex w-full max-w-[980px] items-center justify-center text-xs text-muted-foreground">
                正在加载更早消息...
              </div>
            ) : null}
            <GroupConversationFeed
              agent_name_map={agent_name_map}
              agent_avatar_map={agent_avatar_map}
              bottom_anchor_ref={bottom_anchor_ref}
              feed_ref={feed_ref}
              scroll_ref={scroll_ref}
              current_agent_name={current_agent_name ?? null}
              current_agent_avatar={current_agent_avatar ?? null}
              current_user_avatar={current_user_avatar}
              is_last_round_pending_permissions={pending_permissions}
              is_loading={is_loading}
              runtime_phase={runtime_phase}
              live_round_ids={live_round_ids}
              is_mobile_layout={is_mobile_layout}
              message_groups={message_groups}
              pending_permission_groups={pending_permission_groups}
              pending_slot_groups={pending_slot_groups}
              on_open_agent_contact={on_open_agent_contact}
              on_open_workspace_file={on_open_workspace_file}
              on_permission_response={send_permission_response}
              can_respond_to_permissions={can_control_session}
              permission_read_only_reason={observer_read_only_reason}
              on_stop_message={
                can_control_session ? handle_stop_message : undefined
              }
              round_ids={round_ids}
            />
            {system_error ? (
              <div className={is_mobile_layout ? "mt-4" : "mx-auto mt-2 w-full max-w-[980px]"}>
                <ConversationErrorBubble
                  error={system_error}
                  compact={is_mobile_layout}
                />
              </div>
            ) : null}
          </div>

          {show_scroll_to_bottom ? (
            <ScrollToLatestButton
              is_loading={is_loading}
              is_mobile_layout={is_mobile_layout}
              on_click={() => scroll_to_bottom("smooth")}
            />
          ) : null}

          {show_provider_warning ? (
            <ProviderUnavailableBanner compact={is_mobile_layout} />
          ) : null}

          <RoomGoalPanel
            activity_key={`${messages.length}:${is_loading ? "loading" : "idle"}:${goal_refresh_seq}`}
            can_control_session={can_control_session}
            edit_request={goal_edit_request}
            is_loading={is_loading}
            is_mobile_layout={is_mobile_layout}
            room_host_agent_id={room_host_agent_id}
            room_host_auto_reply_enabled={room_host_auto_reply_enabled}
            room_members={room_members}
            session_key={session_key}
          />

          <ComposerPanel
            allow_send_while_loading
            compact={is_mobile_layout}
            command_hint_items={GOAL_COMMAND_HINT_ITEMS}
            control_status_text={session_control_text}
            default_delivery_policy={default_delivery_policy}
            input_queue_items={input_queue_items}
            is_loading={is_loading}
            queue_when_session_busy={false}
            runtime_phase={runtime_phase}
            on_delete_queued_message={delete_input_queue_message}
            on_enqueue_message={enqueue_input_queue_message}
            on_guide_queued_message={guide_input_queue_message}
            on_prepare_attachments={handle_prepare_attachments}
            on_reorder_queue_messages={reorder_input_queue_messages}
            on_send_message={handle_send_message}
            on_stop={can_control_session ? () => stop_generation() : undefined}
            room_members={room_members}
            tour_anchor={CONVERSATION_TOUR_ANCHORS.composer}
            disabled={!can_control_session}
          />
          {goal_command_dialog}
        </>
      )}
    </div>
  );
}
