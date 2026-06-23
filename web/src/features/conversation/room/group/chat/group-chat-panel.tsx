"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { UserRound } from "lucide-react";

import { useAgentConversation } from "@/hooks/agent";
import { useProviderAvailability } from "@/hooks/capability/use-provider-availability";
import { useExtractTodos } from "@/hooks/conversation/use-extract-todos";
import { useFollowScroll } from "@/hooks/conversation/use-follow-scroll";
import { useSessionLoader } from "@/hooks/conversation/use-session-loader";
import { useDefaultChatDeliveryPolicy } from "@/hooks/settings/use-default-chat-delivery-policy";
import { create_goal_api } from "@/lib/api/goal-api";
import { build_room_shared_session_key } from "@/lib/conversation/session-key";
import { useAuth } from "@/shared/auth/auth-context";
import { AgentConversationIdentity } from "@/types/agent/agent-conversation";
import { RoomConversationSnapshotPayload } from "@/types/conversation/conversation";
import { TodoItem } from "@/types/conversation/todo";
import { Agent } from "@/types/agent/agent";
import type { LoopCatalogItem } from "@/types/capability/loop";

import { ScrollToLatestButton } from "@/features/conversation/shared/scroll-to-latest-button";
import { ComposerPanel } from "@/features/conversation/shared/composer-panel";
import { prepare_room_conversation_attachments } from "@/features/conversation/shared/composer-attachments";
import { ConversationErrorBubble } from "@/features/conversation/shared/conversation-error-bubble";
import { is_provider_error } from "@/features/conversation/shared/conversation-error-utils";
import { ProviderUnavailableBanner } from "@/features/conversation/shared/provider-unavailable-banner";
import { SubagentStatusStrip } from "@/features/conversation/shared/subagent-status-strip";
import { ROOM_GOAL_SCOPE_LABEL } from "@/features/conversation/shared/goal-continuation-hold";
import { build_timeline_round_ids } from "@/features/conversation/shared/timeline-rounds";
import { useConversationComposerHandlers } from "@/features/conversation/shared/use-conversation-composer-handlers";
import { useConversationHistoryLoader } from "@/features/conversation/shared/use-conversation-history-loader";
import {
  useConversationSnapshotReporter,
  type ConversationSnapshotBuildInput,
} from "@/features/conversation/shared/use-conversation-snapshot-reporter";
import {
  group_room_pending_permissions_by_round,
  group_room_pending_slots_by_round,
  group_room_messages_by_round,
} from "@/features/conversation/shared/utils";
import { GroupConversationFeed } from "./group-conversation-feed";
import { useRoomThreadPanelData } from "./use-room-thread-panel-data";
import { GroupConversationEmptyState } from "./group-conversation-empty-state";
import { RoomGoalPanel } from "./room-goal-panel";
import {
  build_room_goal_metadata,
  build_room_loop_goal_metadata,
  build_room_loop_goal_objective,
  resolve_default_room_goal_lead,
} from "./room-goal-model";
import { CONVERSATION_TOUR_ANCHORS } from "../../room-tour";

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
  room_host_agent_id = null,
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
  const refresh_goal_panel = useCallback(() => {
    set_goal_refresh_seq((value) => value + 1);
  }, []);
  const default_room_goal_lead_agent_id = useMemo(
    () => resolve_default_room_goal_lead(room_members, room_host_agent_id),
    [room_host_agent_id, room_members],
  );
  const [room_goal_lead_agent_id, set_room_goal_lead_agent_id] = useState(
    default_room_goal_lead_agent_id,
  );
  useEffect(() => {
    set_room_goal_lead_agent_id((current) => {
      if (current && room_members.some((agent) => agent.agent_id === current)) {
        return current;
      }
      return default_room_goal_lead_agent_id;
    });
  }, [default_room_goal_lead_agent_id, room_members]);
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
  const can_control_session = true;
  const observer_read_only_reason = "";

  const build_room_snapshot = useCallback(
    (input: ConversationSnapshotBuildInput): RoomConversationSnapshotPayload => {
      const {
        scope_key,
        last_message,
        latest_reply_timestamp,
        should_report_last_activity,
      } = input;

      return {
        conversation_id: scope_key,
        ...(should_report_last_activity && latest_reply_timestamp !== null
          ? { last_activity_at: latest_reply_timestamp }
          : {}),
        session_id: last_message.session_id ?? null,
      };
    },
    [],
  );

  useEffect(() => {
    on_todos_change?.(todos);
  }, [on_todos_change, todos]);
  useEffect(() => {
    on_loading_change?.(is_loading);
  }, [is_loading, on_loading_change]);

  useConversationSnapshotReporter({
    scope_key: conversation_id,
    messages,
    build_snapshot: build_room_snapshot,
    on_snapshot_change: on_conversation_snapshot_change,
  });

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
  const round_ids = useMemo(
    () =>
      build_timeline_round_ids(message_groups, live_round_ids, [
        ...pending_slot_groups.keys(),
        ...pending_permission_groups.keys(),
      ]),
    [
      live_round_ids,
      message_groups,
      pending_permission_groups,
      pending_slot_groups,
    ],
  );
  const { handle_scroll } = useConversationHistoryLoader({
    scroll_ref,
    message_count: messages.length,
    has_more_history,
    is_history_loading,
    is_loading,
    load_older_messages,
    prepare_history_prepend_restore,
    cancel_history_prepend_restore,
    on_scroll,
  });

  const handle_stop_message = useCallback(
    (msg_id: string) => stop_generation(msg_id),
    [stop_generation],
  );
  const prepare_room_attachments = useCallback(async (files: File[]) => {
    if (!room_id || !conversation_id) {
      throw new Error("当前 Room 会话尚未就绪，暂时无法附加文件。");
    }
    return prepare_room_conversation_attachments(room_id, conversation_id, files);
  }, [conversation_id, room_id]);
  const { handle_prepare_attachments, handle_send_message } =
    useConversationComposerHandlers({
      can_send_initial_draft: can_control_session,
      initial_draft,
      initial_draft_log_label: "room",
      is_loading,
      on_initial_draft_consumed,
      prepare_attachments: prepare_room_attachments,
      scroll_to_bottom,
      send_message,
      session_key,
    });
  const room_goal_create_disabled_reason =
    room_members.length === 0
      ? "房间还没有可指派的 Agent"
      : room_goal_lead_agent_id.trim() === ""
        ? "请选择 Room Goal 负责人"
        : null;
  const room_goal_lead_control = (
    <label
      className="pointer-events-auto inline-flex h-5 min-w-0 max-w-[190px] items-center gap-1 rounded-[7px] border border-(--surface-canvas-border) bg-(--surface-elevated-background) px-1.5 text-[10px] font-medium text-(--text-muted)"
      title="选择 Room Goal 负责人"
    >
      <UserRound className="h-3 w-3 shrink-0" />
      <select
        className="min-w-0 flex-1 bg-transparent text-[10px] font-semibold text-(--text-default) outline-none disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)"
        disabled={!can_control_session || is_loading || room_members.length === 0}
        value={room_goal_lead_agent_id}
        onChange={(event) => set_room_goal_lead_agent_id(event.target.value)}
      >
        <option value="">负责人</option>
        {room_members.map((agent) => (
          <option key={agent.agent_id} value={agent.agent_id}>
            {agent.name}
          </option>
        ))}
      </select>
    </label>
  );
  const handle_create_goal = useCallback(async (objective: string) => {
    if (!session_key) {
      throw new Error("当前房间会话尚未准备好，暂时无法启动 Goal。");
    }
    const lead_agent_id = room_goal_lead_agent_id.trim();
    if (!lead_agent_id) {
      throw new Error("请选择 Room Goal 负责人。");
    }
    await create_goal_api({
      session_key,
      objective,
      token_budget: null,
      metadata: build_room_goal_metadata(room_members, lead_agent_id),
    });
    refresh_goal_panel();
  }, [
    refresh_goal_panel,
    room_goal_lead_agent_id,
    room_members,
    session_key,
  ]);
  const handle_create_loop_goal = useCallback(async (loop: LoopCatalogItem) => {
    if (!session_key) {
      throw new Error("当前房间会话尚未准备好，暂时无法启动 Loop。");
    }
    const lead_agent_id = room_goal_lead_agent_id.trim();
    if (!lead_agent_id) {
      throw new Error("请选择 Room Goal 负责人。");
    }
    await create_goal_api({
      session_key,
      objective: build_room_loop_goal_objective(loop),
      token_budget: null,
      metadata: build_room_loop_goal_metadata(room_members, lead_agent_id, loop),
    });
    refresh_goal_panel();
  }, [
    refresh_goal_panel,
    room_goal_lead_agent_id,
    room_members,
    session_key,
  ]);
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

          <SubagentStatusStrip
            compact={is_mobile_layout}
            live_round_ids={live_round_ids}
            messages={messages}
          />

          <RoomGoalPanel
            activity_key={`${messages.length}:${is_loading ? "loading" : "idle"}:${goal_refresh_seq}`}
            can_control_session={can_control_session}
            is_loading={is_loading}
            is_mobile_layout={is_mobile_layout}
            room_host_agent_id={room_host_agent_id}
            room_host_auto_reply_enabled={Boolean(room_host_auto_reply_enabled)}
            room_members={room_members}
            session_key={session_key}
          />

          <ComposerPanel
            allow_send_while_loading
            compact={is_mobile_layout}
            default_delivery_policy={default_delivery_policy}
            enable_loops
            goal_create_disabled_reason={room_goal_create_disabled_reason}
            goal_mode_extra={room_goal_lead_control}
            goal_scope_label={ROOM_GOAL_SCOPE_LABEL}
            input_queue_items={input_queue_items}
            is_loading={is_loading}
            queue_when_session_busy={false}
            runtime_phase={runtime_phase}
            on_create_loop_goal={session_key && can_control_session ? handle_create_loop_goal : undefined}
            on_create_goal={session_key && can_control_session ? handle_create_goal : undefined}
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
        </>
      )}
    </div>
  );
}
