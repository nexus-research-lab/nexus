import {
  AssistantMessage,
  ChatAckData,
  EventMessage,
  Message,
  RoundStatusEventPayload,
  SessionStatusEventPayload,
  StreamMessage,
} from '@/types';
import {
  HandleAgentConversationWebSocketMessageParams,
  RoomEventPayload,
} from '@/types/agent/agent-conversation';
import { WorkspaceEventPayload } from '@/types/app/workspace-live';

import { apply_stream_message, normalize_assistant_message, upsert_message } from './message-helpers';

/**
 * 处理 Agent 会话的 WebSocket 事件。
 */
export function handle_agent_conversation_web_socket_message({
  backend_message,
  apply_workspace_event,
  is_current_session_event,
  set_error,
  set_messages,
  set_pending_permissions,
  enqueue_stream_payload,
  on_background_message,
  on_room_event,
  update_message_status,
  sync_session_status,
  apply_round_status,
  track_chat_ack,
  track_assistant_message,
}: HandleAgentConversationWebSocketMessageParams): void {
  const event = backend_message as EventMessage;
  const incoming_session_key = event.session_key || null;

  if (event.event_type === 'error') {
    if (incoming_session_key && !is_current_session_event(incoming_session_key)) {
      return;
    }
    if (event.message_id) {
      update_message_status?.(event.message_id, 'error', event.caused_by);
    }
    set_error(event.data?.message || 'Unknown error');
    return;
  }

  if (event.event_type === 'permission_request') {
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
        interaction_mode: data.interaction_mode
          ?? (data.tool_name === 'AskUserQuestion' ? 'question' : 'permission'),
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

  if (event.event_type === 'workspace_event') {
    const payload = event.data as WorkspaceEventPayload;
    if (payload?.agent_id && payload?.path) {
      apply_workspace_event(payload);
    }
    return;
  }

  // Room-level events (member changes, room deleted, etc.)
  if (
    event.event_type === 'room_member_added' ||
    event.event_type === 'room_member_removed' ||
    event.event_type === 'room_deleted' ||
    event.event_type === 'room_resync_required' ||
    event.event_type === 'session_resync_required'
  ) {
    on_room_event?.(event.event_type, (event.data ?? {}) as RoomEventPayload);
    return;
  }

  // session_status: 重连后后端告知该 session 是否仍在生成，恢复/收口 loading 态
  if (event.event_type === 'session_status') {
    if (!is_current_session_event(incoming_session_key)) {
      return;
    }
    const payload = (event.data ?? {}) as SessionStatusEventPayload;
    sync_session_status?.(payload);
    return;
  }

  if (event.event_type === 'round_status') {
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
  if (event.event_type === 'chat_ack') {
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
  if (event.event_type === 'stream_start') {
    if (!is_current_session_event(incoming_session_key)) {
      return;
    }
    const msg_id = (event.message_id || event.data?.msg_id) as string | undefined;
    if (msg_id) {
      update_message_status?.(msg_id, 'streaming', event.data?.round_id);
    }
    return;
  }

  // stream_end: mark bubble done
  if (event.event_type === 'stream_end') {
    if (!is_current_session_event(incoming_session_key)) {
      return;
    }
    const msg_id = (event.message_id || event.data?.msg_id) as string | undefined;
    if (msg_id) {
      update_message_status?.(msg_id, 'done', event.data?.round_id);
    }
    return;
  }

  // stream_cancelled: mark bubble cancelled, stop loading
  if (event.event_type === 'stream_cancelled') {
    if (!is_current_session_event(incoming_session_key)) {
      return;
    }
    const msg_id = (event.message_id || event.data?.msg_id) as string | undefined;
    if (msg_id) {
      update_message_status?.(msg_id, 'cancelled', event.data?.round_id);
    }
    return;
  }

  if (event.event_type !== 'message') {
    if (event.event_type !== 'stream') {
      return;
    }

    const payload = event.data as StreamMessage;
    const message_session_key = payload?.session_key || incoming_session_key;
    if (!payload || !message_session_key || !is_current_session_event(message_session_key)) {
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

  const payload_with_delivery_mode: Message = (
    event.delivery_mode
      ? {
        ...payload,
        delivery_mode: event.delivery_mode,
      }
      : payload
  );

  if (!is_current_session_event(message_session_key)) {
    // 只缓存 durable 消息，ephemeral 仅服务当前活跃轮次展示。
    if (event.delivery_mode !== 'ephemeral' && on_background_message) {
      on_background_message(message_session_key, payload_with_delivery_mode);
    }
    return;
  }

  const normalized_payload = (
    payload_with_delivery_mode.role === 'assistant'
      ? normalize_assistant_message(payload_with_delivery_mode as AssistantMessage)
      : payload_with_delivery_mode
  );

  set_messages((prev) => upsert_message(prev, normalized_payload));
  if (normalized_payload.role === 'assistant') {
    track_assistant_message?.(normalized_payload as AssistantMessage);
  }
}
