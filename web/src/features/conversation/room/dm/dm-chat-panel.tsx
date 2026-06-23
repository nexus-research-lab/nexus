"use client";

import { useCallback, useEffect, useMemo, useState } from "react";

import { useAgentConversation } from "@/hooks/agent";
import { useProviderAvailability } from "@/hooks/capability/use-provider-availability";
import { useExtractTodos } from "@/hooks/conversation/use-extract-todos";
import { useFollowScroll } from "@/hooks/conversation/use-follow-scroll";
import { useSessionLoader } from "@/hooks/conversation/use-session-loader";
import { useDefaultChatDeliveryPolicy } from "@/hooks/settings/use-default-chat-delivery-policy";
import { create_goal_api } from "@/lib/api/goal-api";
import { useAuth } from "@/shared/auth/auth-context";
import {
  AgentConversationIdentity,
} from "@/types/agent/agent-conversation";
import { SessionSnapshotPayload } from "@/types/conversation/conversation";
import { TodoItem } from "@/types/conversation/todo";

import { ComposerPanel } from "@/features/conversation/shared/composer-panel";
import {
  prepare_workspace_attachments,
} from "@/features/conversation/shared/composer-attachments";
import { ConversationErrorBubble } from "@/features/conversation/shared/conversation-error-bubble";
import { is_provider_error } from "@/features/conversation/shared/conversation-error-utils";
import { ConversationFeed } from "@/features/conversation/shared/conversation-feed";
import { goal_continuation_hold_for_permission } from "@/features/conversation/shared/goal-continuation-hold";
import { GoalPanel } from "@/features/conversation/shared/goal-panel";
import { ProviderUnavailableBanner } from "@/features/conversation/shared/provider-unavailable-banner";
import { ScrollToLatestButton } from "@/features/conversation/shared/scroll-to-latest-button";
import { build_timeline_round_ids } from "@/features/conversation/shared/timeline-rounds";
import { useConversationComposerHandlers } from "@/features/conversation/shared/use-conversation-composer-handlers";
import { useConversationHistoryLoader } from "@/features/conversation/shared/use-conversation-history-loader";
import {
  useConversationSnapshotReporter,
  type ConversationSnapshotBuildInput,
} from "@/features/conversation/shared/use-conversation-snapshot-reporter";
import {
  group_messages_by_round,
} from "@/features/conversation/shared/utils";
import { CONVERSATION_TOUR_ANCHORS } from "../room-tour";

export interface DmChatPanelProps {
  current_agent_name?: string | null;
  current_agent_avatar?: string | null;
  current_agent_permission_mode?: string | null;
  session_identity: AgentConversationIdentity | null;
  layout?: "desktop" | "mobile";
  initial_draft?: string | null;
  on_initial_draft_consumed?: () => void;
  on_open_agent_contact?: (agent_id: string) => void;
  on_open_workspace_file?: (path: string) => void;
  on_todos_change?: (todos: TodoItem[]) => void;
  on_loading_change?: (is_loading: boolean) => void;
  on_conversation_snapshot_change?: (snapshot: SessionSnapshotPayload) => void;
  on_room_event?: (
    event_type: string,
    data: import("@/types/agent/agent-conversation").RoomEventPayload,
  ) => void;
}

export function DmChatPanel({
  current_agent_name,
  current_agent_avatar,
  current_agent_permission_mode,
  session_identity,
  layout = "desktop",
  initial_draft = null,
  on_initial_draft_consumed,
  on_open_agent_contact,
  on_open_workspace_file,
  on_todos_change,
  on_loading_change,
  on_conversation_snapshot_change,
  on_room_event,
}: DmChatPanelProps) {
  const is_mobile_layout = layout === "mobile";
  const session_key = session_identity?.session_key ?? null;
  const default_delivery_policy = useDefaultChatDeliveryPolicy();
  const { status: auth_status } = useAuth();
  const current_user_avatar = auth_status?.avatar ?? null;
  const [goal_refresh_seq, set_goal_refresh_seq] = useState(0);
  const refresh_goal_panel = useCallback(() => {
    set_goal_refresh_seq((value) => value + 1);
  }, []);
  const goal_continuation_hold = useMemo(
    () =>
      goal_continuation_hold_for_permission(
        current_agent_name,
        current_agent_permission_mode,
      ),
    [current_agent_name, current_agent_permission_mode],
  );
  const can_control_session = true;
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

  const {
    error,
    messages,
    is_loading,
    is_history_loading,
    has_more_history,
    history_prepend_token,
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
      console.error("DM conversation error:", err);
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
    auxiliary_block_count: pending_permissions.length,
    auxiliary_block_key: system_error,
    is_loading,
    session_key,
    history_prepend_token,
  });
  const prepare_dm_attachments = useCallback(async (files: File[]) => {
    const target_agent_id = session_identity?.agent_id;
    if (!target_agent_id) {
      throw new Error("当前会话尚未准备好，暂时无法附加文件。");
    }
    return prepare_workspace_attachments(target_agent_id, files);
  }, [session_identity?.agent_id]);
  const { handle_prepare_attachments, handle_send_message } =
    useConversationComposerHandlers({
      initial_draft,
      initial_draft_log_label: "DM",
      is_loading,
      on_initial_draft_consumed,
      prepare_attachments: prepare_dm_attachments,
      scroll_to_bottom,
      send_message,
      session_key,
    });

  const build_dm_snapshot = useCallback(
    (input: ConversationSnapshotBuildInput): SessionSnapshotPayload => {
      const {
        scope_key,
        last_message,
        latest_reply_timestamp,
        should_report_last_activity,
      } = input;

      return {
        session_key: scope_key,
        agent_id: session_identity?.agent_id ?? null,
        room_id: session_identity?.room_id ?? null,
        conversation_id: session_identity?.conversation_id ?? null,
        room_session_id: session_identity?.room_session_id ?? null,
        ...(should_report_last_activity && latest_reply_timestamp !== null
          ? { last_activity_at: latest_reply_timestamp }
          : {}),
        session_id: last_message.session_id ?? null,
      };
    },
    [session_identity],
  );

  useEffect(() => {
    on_todos_change?.(todos);
  }, [on_todos_change, todos]);
  useEffect(() => {
    on_loading_change?.(is_loading);
  }, [is_loading, on_loading_change]);

  useConversationSnapshotReporter({
    scope_key: session_key,
    messages,
    build_snapshot: build_dm_snapshot,
    on_snapshot_change: on_conversation_snapshot_change,
  });

  useSessionLoader({
    session_key,
    load_session,
    debug_name: "DmChatPanel",
  });

  const message_groups = useMemo(
    () => group_messages_by_round(messages),
    [messages],
  );
  const round_ids = useMemo(
    () => build_timeline_round_ids(message_groups, live_round_ids),
    [live_round_ids, message_groups],
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

  const handle_stop = () => stop_generation();

  const handle_create_goal = useCallback(async (objective: string) => {
    if (!session_key) {
      throw new Error("当前会话尚未准备好，暂时无法启动 Goal。");
    }
    await create_goal_api({
      session_key,
      objective,
      token_budget: null,
    });
    refresh_goal_panel();
  }, [refresh_goal_panel, session_key]);

  return (
    <div className="relative flex h-full min-w-0 flex-1 flex-col overflow-hidden bg-transparent">

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
        <ConversationFeed
          bottom_anchor_ref={bottom_anchor_ref}
          feed_ref={feed_ref}
          scroll_ref={scroll_ref}
          current_agent_name={current_agent_name ?? null}
          current_agent_avatar={current_agent_avatar ?? null}
          workspace_agent_id={session_identity?.agent_id ?? null}
          current_user_avatar={current_user_avatar}
          is_last_round_pending_permissions={pending_permissions}
          is_loading={is_loading}
          runtime_phase={runtime_phase}
          live_round_ids={live_round_ids}
          is_mobile_layout={is_mobile_layout}
          message_groups={message_groups}
          on_open_agent_contact={on_open_agent_contact}
          on_open_workspace_file={on_open_workspace_file}
          on_permission_response={send_permission_response}
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

      <GoalPanel
        activity_key={`${messages.length}:${is_loading ? "loading" : "idle"}:${goal_refresh_seq}`}
        compact={is_mobile_layout}
        continuation_hold={goal_continuation_hold}
        disabled={!can_control_session}
        is_generating={is_loading}
        session_key={session_key}
        scope_label="会话 Goal"
      />

      <ComposerPanel
        allow_send_while_loading
        compact={is_mobile_layout}
        default_delivery_policy={default_delivery_policy}
        input_queue_items={input_queue_items}
        is_loading={is_loading}
        goal_scope_label="会话 Goal"
        runtime_phase={runtime_phase}
        on_delete_queued_message={delete_input_queue_message}
        on_enqueue_message={enqueue_input_queue_message}
        on_create_goal={session_key && can_control_session ? handle_create_goal : undefined}
        on_guide_queued_message={guide_input_queue_message}
        on_prepare_attachments={handle_prepare_attachments}
        on_reorder_queue_messages={reorder_input_queue_messages}
        on_send_message={handle_send_message}
        on_stop={handle_stop}
        tour_anchor={CONVERSATION_TOUR_ANCHORS.composer}
      />
    </div>
  );
}
