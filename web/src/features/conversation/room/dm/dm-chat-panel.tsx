"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { useAgentConversation } from "@/hooks/agent";
import { useProviderAvailability } from "@/hooks/capability/use-provider-availability";
import { useExtractTodos } from "@/hooks/conversation/use-extract-todos";
import { useFollowScroll } from "@/hooks/conversation/use-follow-scroll";
import { useSessionLoader } from "@/hooks/conversation/use-session-loader";
import { useDefaultChatDeliveryPolicy } from "@/hooks/settings/use-default-chat-delivery-policy";
import { useAuth } from "@/shared/auth/auth-context";
import {
  AgentConversationIdentity,
  AgentConversationDeliveryPolicy,
  get_session_control_status_text,
} from "@/types/agent/agent-conversation";
import { SessionSnapshotPayload } from "@/types/conversation/conversation";
import { TodoItem } from "@/types/conversation/todo";

import { ComposerPanel } from "@/features/conversation/shared/composer-panel";
import {
  prepare_workspace_attachments,
  type PreparedComposerAttachment,
} from "@/features/conversation/shared/composer-attachments";
import { ConversationErrorBubble } from "@/features/conversation/shared/conversation-error-bubble";
import { is_provider_error } from "@/features/conversation/shared/conversation-error-utils";
import { ConversationFeed } from "@/features/conversation/shared/conversation-feed";
import { GoalPanel } from "@/features/conversation/shared/goal-panel";
import { ProviderUnavailableBanner } from "@/features/conversation/shared/provider-unavailable-banner";
import { ScrollToLatestButton } from "@/features/conversation/shared/scroll-to-latest-button";
import { useGoalCommandHandler } from "@/features/conversation/shared/use-goal-command-handler";
import {
  group_messages_by_round,
  get_latest_reply_timestamp,
} from "@/features/conversation/shared/utils";
import { CONVERSATION_TOUR_ANCHORS } from "../room-tour";

const HISTORY_LOAD_THRESHOLD_PX = 120;

export interface DmChatPanelProps {
  current_agent_name?: string | null;
  current_agent_avatar?: string | null;
  session_identity: AgentConversationIdentity | null;
  layout?: "desktop" | "mobile";
  initial_draft?: string | null;
  on_initial_draft_consumed?: () => void;
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
  session_identity,
  layout = "desktop",
  initial_draft = null,
  on_initial_draft_consumed,
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
  const [goal_edit_seq, set_goal_edit_seq] = useState(0);
  const refresh_goal_panel = useCallback(() => {
    set_goal_refresh_seq((value) => value + 1);
  }, []);
  const request_goal_edit = useCallback(() => {
    set_goal_edit_seq((value) => value + 1);
  }, []);
  const { try_handle_goal_command, goal_command_dialog } = useGoalCommandHandler({
    session_key,
    on_refresh: refresh_goal_panel,
    on_edit_goal: request_goal_edit,
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

  const {
    error,
    messages,
    is_loading,
    is_history_loading,
    has_more_history,
    history_prepend_token,
    session_control_state,
    session_observer_count,
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
  const last_snapshot_key_ref = useRef<string | null>(null);
  const consumed_initial_draft_ref = useRef<string | null>(null);
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
    if (!session_key || messages.length === 0) return;
    const last = messages[messages.length - 1];
    const latest_reply_timestamp = get_latest_reply_timestamp(messages);
    const snapshot = {
      session_key,
      agent_id: session_identity?.agent_id ?? null,
      room_id: session_identity?.room_id ?? null,
      conversation_id: session_identity?.conversation_id ?? null,
      room_session_id: session_identity?.room_session_id ?? null,
      message_count: messages.length,
      ...(latest_reply_timestamp
        ? { last_activity_at: latest_reply_timestamp }
        : {}),
      session_id: last?.session_id ?? null,
    };
    const snapshot_key = JSON.stringify(snapshot);

    // DM 与 Room 共用流式消息模式，这里同样需要阻断重复快照回写。
    if (last_snapshot_key_ref.current === snapshot_key) {
      return;
    }

    last_snapshot_key_ref.current = snapshot_key;
    on_conversation_snapshot_change?.(snapshot);
  }, [
    session_identity,
    session_key,
    messages,
    on_conversation_snapshot_change,
  ]);

  useSessionLoader({
    session_key,
    load_session,
    debug_name: "DmChatPanel",
  });

  const message_groups = useMemo(
    () => group_messages_by_round(messages),
    [messages],
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

  const handle_send_message = async (
    content: string,
    delivery_policy: AgentConversationDeliveryPolicy,
    attachments: PreparedComposerAttachment[] = [],
  ) => {
    if (!content.trim() && attachments.length === 0) return;
    if (await try_handle_goal_command(content)) {
      return;
    }
    scroll_to_bottom("auto");
    await send_message(content, { delivery_policy, attachments });
  };

  const handle_stop = () => stop_generation();

  const handle_prepare_attachments = async (files: File[]) => {
    const target_agent_id = session_identity?.agent_id;
    if (!target_agent_id) {
      throw new Error("当前会话尚未准备好，暂时无法附加文件。");
    }
    return prepare_workspace_attachments(target_agent_id, files);
  };

  useEffect(() => {
    const normalized_draft = initial_draft?.trim() ?? "";
    if (
      !session_key ||
      !normalized_draft ||
      is_loading ||
      !can_control_session
    ) {
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
        console.error("Failed to auto send initial DM prompt:", error);
      });
  }, [
    can_control_session,
    initial_draft,
    is_loading,
    on_initial_draft_consumed,
    scroll_to_bottom,
    send_message,
    session_key,
  ]);

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
          on_open_workspace_file={on_open_workspace_file}
          on_permission_response={send_permission_response}
          can_respond_to_permissions={can_control_session}
          permission_read_only_reason={observer_read_only_reason}
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
        disabled={!can_control_session}
        edit_request_key={goal_edit_seq}
        is_generating={is_loading}
        session_key={session_key}
      />

      <ComposerPanel
        allow_send_while_loading
        compact={is_mobile_layout}
        control_status_text={session_control_text}
        default_delivery_policy={default_delivery_policy}
        input_queue_items={input_queue_items}
        is_loading={is_loading}
        runtime_phase={runtime_phase}
        on_delete_queued_message={delete_input_queue_message}
        on_enqueue_message={enqueue_input_queue_message}
        on_guide_queued_message={guide_input_queue_message}
        on_prepare_attachments={handle_prepare_attachments}
        on_reorder_queue_messages={reorder_input_queue_messages}
        on_send_message={handle_send_message}
        on_stop={handle_stop}
        tour_anchor={CONVERSATION_TOUR_ANCHORS.composer}
        disabled={!can_control_session}
      />
      {goal_command_dialog}
    </div>
  );
}
