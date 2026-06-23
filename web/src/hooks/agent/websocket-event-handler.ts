import {
  AssistantMessage,
  ChatAckData,
  EventMessage,
  Message,
  RoundStatusEventPayload,
  SessionStatusEventPayload,
  StreamMessage,
} from "@/types";
import {
  HandleAgentConversationWebSocketMessageParams,
  InputQueueEventPayload,
  RoomEventPayload,
} from "@/types/agent/agent-conversation";
import { WorkspaceEventPayload } from "@/types/app/workspace-live";
import {
  apply_stream_message,
  normalize_assistant_message,
  upsert_message,
} from "./message-helpers";
import {
  build_room_subscription_message,
  build_session_bind_message,
} from "./conversation-actions";

function is_event_message(data: unknown): data is EventMessage {
  if (typeof data !== "object" || data === null) {
    return false;
  }
  const msg = data as Record<string, unknown>;
  return (
    typeof msg.event_type === "string" &&
    typeof msg.protocol_version === "number"
  );
}

/**
 * 处理 Agent 会话的 WebSocket 事件。
 */
export function handle_agent_conversation_web_socket_message({
  backend_message,
  agent_id,
  room_id,
  conversation_id,
  session_key,
  session_seq_cursor_ref,
  room_seq_cursor_ref,
  ws_state_ref,
  ws_send_ref,
  apply_workspace_event,
  is_current_room_event,
  is_current_session_event,
  set_error,
  set_messages,
  set_input_queue_items,
  set_pending_permissions,
  enqueue_stream_payload,
  on_background_message,
  on_room_event,
  update_message_status,
  sync_session_status,
  apply_round_status,
  track_chat_ack,
  track_assistant_message,
  reload_current_session,
  settle_agent_workspace_writes,
}: HandleAgentConversationWebSocketMessageParams): void {
  if (!is_event_message(backend_message)) {
    console.warn("[websocket-event-handler] Received unexpected message shape:", backend_message);
    return;
  }
  const event = backend_message;
  const incoming_session_key = event.session_key || null;

  if (
    session_key &&
    event.session_key === session_key &&
    typeof event.session_seq === "number" &&
    session_seq_cursor_ref &&
    event.session_seq > session_seq_cursor_ref.current
  ) {
    session_seq_cursor_ref.current = event.session_seq;
  }

  if (
    room_id &&
    event.room_id === room_id &&
    typeof event.room_seq === "number" &&
    room_seq_cursor_ref &&
    event.room_seq > room_seq_cursor_ref.current
  ) {
    room_seq_cursor_ref.current = event.room_seq;
  }

  if (
    event.event_type === "room_resync_required" &&
    event.room_id === room_id
  ) {
    const latest_room_seq = event.data?.latest_room_seq;
    if (typeof latest_room_seq === "number" && room_seq_cursor_ref) {
      room_seq_cursor_ref.current = Math.max(
        room_seq_cursor_ref.current,
        latest_room_seq,
      );
    }
    on_room_event?.(event.event_type, event.data ?? {});
    void reload_current_session?.().finally(() => {
      if (
        !room_id ||
        ws_state_ref?.current !== "connected" ||
        !ws_send_ref ||
        !room_seq_cursor_ref
      ) {
        return;
      }
      ws_send_ref.current(build_room_subscription_message({
        type: "subscribe_room",
        room_id,
        conversation_id,
        last_seen_room_seq: room_seq_cursor_ref.current,
      }));
    });
    return;
  }

  if (
    event.event_type === "session_resync_required" &&
    event.session_key &&
    is_current_session_event(event.session_key)
  ) {
    const latest_session_seq = event.data?.latest_session_seq;
    if (typeof latest_session_seq === "number" && session_seq_cursor_ref) {
      session_seq_cursor_ref.current = Math.max(
        session_seq_cursor_ref.current,
        latest_session_seq,
      );
    }
    on_room_event?.(event.event_type, event.data ?? {});
    void reload_current_session?.().finally(() => {
      if (
        !session_key ||
        ws_state_ref?.current !== "connected" ||
        !ws_send_ref ||
        !session_seq_cursor_ref
      ) {
        return;
      }
      ws_send_ref.current(build_session_bind_message({
        session_key,
        last_seen_session_seq: session_seq_cursor_ref.current,
        agent_id,
        room_id,
        conversation_id,
      }));
    });
    return;
  }

  if (event.event_type === "agent_runtime_event") {
    const payload = event.data as
      | { agent_id?: string; running_task_count?: number; status?: string }
      | undefined;
    if (
      payload?.agent_id &&
      payload.agent_id === agent_id &&
      payload.running_task_count === 0 &&
      payload.status !== "running"
    ) {
      settle_agent_workspace_writes?.(payload.agent_id);
    }
    return;
  }

  if (event.event_type === "error") {
    if (
      incoming_session_key &&
      !is_current_session_event(incoming_session_key)
    ) {
      return;
    }
    if (event.message_id) {
      update_message_status?.(event.message_id, "error", event.caused_by);
    }
    set_error(event.data?.message || "Unknown error");
    return;
  }

  if (event.event_type === "permission_request") {
    if (!is_current_session_event(incoming_session_key)) {
      return;
    }
    const data = event.data || {};
    set_pending_permissions((prev) => {
      const next_permission = {
        request_id: data.request_id,
        tool_name: data.tool_name,
        tool_input: data.tool_input || {},
        session_key: incoming_session_key,
        agent_id: event.agent_id ?? null,
        message_id: event.message_id ?? null,
        caused_by: event.caused_by ?? null,
        interaction_mode:
          data.interaction_mode ??
          (data.tool_name === "AskUserQuestion" ? "question" : "permission"),
        risk_level: data.risk_level,
        risk_label: data.risk_label,
        summary: data.summary,
        suggestions: data.suggestions || [],
        expires_at: data.expires_at,
      };
      return [
        ...prev.filter((item) => item.request_id !== data.request_id),
        next_permission,
      ];
    });
    return;
  }

  if (event.event_type === "permission_request_resolved") {
    if (!is_current_session_event(incoming_session_key)) {
      return;
    }
    const request_id =
      typeof event.data?.request_id === "string" ? event.data.request_id : "";
    if (!request_id) {
      return;
    }
    set_pending_permissions((prev) => {
      const next = prev.filter((item) => item.request_id !== request_id);
      return next.length === prev.length ? prev : next;
    });
    return;
  }

  if (event.event_type === "workspace_event") {
    const payload = event.data as WorkspaceEventPayload;
    if (payload?.agent_id && payload?.path) {
      apply_workspace_event(payload);
    }
    return;
  }

  // Room-level events (member changes, room deleted, etc.)
  if (
    event.event_type === "room_member_added" ||
    event.event_type === "room_member_removed" ||
    event.event_type === "room_deleted" ||
    event.event_type === "room_directed_message" ||
    event.event_type === "room_directed_message_consumed" ||
    event.event_type === "room_resync_required" ||
    event.event_type === "session_resync_required"
  ) {
    if (is_current_room_event && !is_current_room_event(event.room_id)) {
      return;
    }
    on_room_event?.(event.event_type, (event.data ?? {}) as RoomEventPayload);
    return;
  }

  // session_status: 重连后后端告知该 session 是否仍在生成，恢复/收口 loading 态
  if (event.event_type === "session_status") {
    if (!is_current_session_event(incoming_session_key)) {
      return;
    }
    const payload = (event.data ?? {}) as SessionStatusEventPayload;
    sync_session_status?.(payload);
    return;
  }

  if (event.event_type === "input_queue") {
    if (!is_current_session_event(incoming_session_key)) {
      return;
    }
    const payload = (event.data ?? {}) as InputQueueEventPayload;
    const items = Array.isArray(payload.items) ? payload.items : [];
    set_input_queue_items?.(items);
    return;
  }

  if (is_goal_event(event.event_type)) {
    if (!is_current_session_event(incoming_session_key)) {
      return;
    }
    on_room_event?.(event.event_type, (event.data ?? {}) as RoomEventPayload);
    return;
  }

  if (event.event_type === "round_status") {
    if (!is_current_session_event(incoming_session_key)) {
      return;
    }
    const payload = (event.data ?? {}) as RoundStatusEventPayload;
    if (!payload.round_id || !payload.status) {
      return;
    }
    apply_round_status?.(payload.round_id, payload.status);
    return;
  }

  // chat_ack: 仅登记 Room 占位槽位，不再插入假 assistant 消息
  if (event.event_type === "chat_ack") {
    if (!is_current_session_event(incoming_session_key)) {
      return;
    }
    const ack = event.data as ChatAckData;
    if (!ack?.round_id) {
      return;
    }
    track_chat_ack?.(ack, incoming_session_key);
    return;
  }

  // stream_start: flip placeholder from pending → streaming
  if (event.event_type === "stream_start") {
    if (!is_current_session_event(incoming_session_key)) {
      return;
    }
    const msg_id = (event.message_id || event.data?.msg_id) as
      | string
      | undefined;
    if (msg_id) {
      update_message_status?.(msg_id, "streaming", event.data?.round_id);
    }
    return;
  }

  // stream_end: mark bubble done
  if (event.event_type === "stream_end") {
    if (!is_current_session_event(incoming_session_key)) {
      return;
    }
    const msg_id = (event.message_id || event.data?.msg_id) as
      | string
      | undefined;
    if (msg_id) {
      update_message_status?.(msg_id, "done", event.data?.round_id);
    }
    return;
  }

  // stream_cancelled: mark bubble cancelled, stop loading
  if (event.event_type === "stream_cancelled") {
    if (!is_current_session_event(incoming_session_key)) {
      return;
    }
    const msg_id = (event.message_id || event.data?.msg_id) as
      | string
      | undefined;
    if (msg_id) {
      update_message_status?.(msg_id, "cancelled", event.data?.round_id);
    }
    return;
  }

  if (event.event_type !== "message") {
    if (event.event_type !== "stream") {
      return;
    }

    const payload = event.data as StreamMessage;
    const message_session_key = payload?.session_key || incoming_session_key;
    if (
      !payload ||
      !message_session_key ||
      !is_current_session_event(message_session_key)
    ) {
      return;
    }

    // Route to rAF batch buffer when available (≤60 flushes/sec),
    // otherwise fall back to direct update (e.g. during tests).
    if (enqueue_stream_payload) {
      enqueue_stream_payload(payload);
    } else {
      set_messages((prev) => apply_stream_message(prev, payload));
    }
    return;
  }

  const payload = event.data as Message;
  const message_session_key = payload?.session_key || incoming_session_key;
  if (!payload || !message_session_key) {
    return;
  }

  const payload_with_delivery_mode: Message = event.delivery_mode
    ? {
        ...payload,
        delivery_mode: event.delivery_mode,
      }
    : payload;

  if (!is_current_session_event(message_session_key)) {
    // 只缓存 durable 消息，ephemeral 仅服务当前活跃轮次展示。
    if (event.delivery_mode !== "ephemeral" && on_background_message) {
      on_background_message(message_session_key, payload_with_delivery_mode);
    }
    return;
  }

  const normalized_payload =
    payload_with_delivery_mode.role === "assistant"
      ? normalize_assistant_message(
          payload_with_delivery_mode as AssistantMessage,
        )
      : payload_with_delivery_mode;

  set_messages((prev) => upsert_message(prev, normalized_payload));
  if (normalized_payload.role === "assistant") {
    track_assistant_message?.(normalized_payload as AssistantMessage);
  }
}

function is_goal_event(event_type: string): boolean {
  return (
    event_type === "goal_created" ||
    event_type === "goal_updated" ||
    event_type === "goal_status_changed" ||
    event_type === "goal_progress" ||
    event_type === "goal_continuation" ||
    event_type === "goal_cleared"
  );
}
