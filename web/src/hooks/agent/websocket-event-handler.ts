import { AssistantMessage, AssistantMessageStatus, ChatAckData, EventMessage, Message, StreamMessage } from '@/types';
import {
  AgentThinkingPayload,
  HandleAgentConversationWebSocketMessageParams,
  RoomEventPayload,
} from '@/types/agent-conversation';
import { WorkspaceEventPayload } from '@/types/workspace-live';

import { applyStreamMessage, upsertMessage } from './message-helpers';

/**
 * 处理 Agent 会话的 WebSocket 事件。
 */
export function handleAgentConversationWebSocketMessage({
  backend_message,
  apply_workspace_event,
  is_current_session_event,
  set_error,
  set_is_loading,
  set_messages,
  set_pending_permission,
  enqueue_stream_payload,
  on_background_message,
  set_agent_thinking,
  on_room_event,
  update_message_status,
}: HandleAgentConversationWebSocketMessageParams): void {
  const event = backend_message as EventMessage;
  const incoming_session_key = event.session_key || null;

  if (event.event_type === 'error') {
    if (incoming_session_key && !is_current_session_event(incoming_session_key)) {
      return;
    }
    set_error(event.data?.message || 'Unknown error');
    set_is_loading(false);
    return;
  }

  if (event.event_type === 'permission_request') {
    if (!is_current_session_event(incoming_session_key)) {
      return;
    }
    const data = event.data || {};
    set_pending_permission({
      request_id: data.request_id,
      tool_name: data.tool_name,
      tool_input: data.tool_input || {},
      risk_level: data.risk_level,
      risk_label: data.risk_label,
      summary: data.summary,
      suggestions: data.suggestions || [],
      expires_at: data.expires_at,
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

  // Agent thinking/done status in multi-agent rooms
  if (event.event_type === 'agent_thinking') {
    set_agent_thinking?.(event.data as AgentThinkingPayload);
    return;
  }

  if (event.event_type === 'agent_done') {
    set_agent_thinking?.(null);
    return;
  }

  // Room-level events (member changes, room deleted, etc.)
  if (
    event.event_type === 'room_member_added' ||
    event.event_type === 'room_member_removed' ||
    event.event_type === 'room_deleted'
  ) {
    on_room_event?.(event.event_type, (event.data ?? {}) as RoomEventPayload);
    return;
  }

  // chat_ack: server pre-allocated msg_ids; insert pending placeholder bubbles immediately
  if (event.event_type === 'chat_ack') {
    if (!is_current_session_event(incoming_session_key)) {
      return;
    }
    const ack = event.data as ChatAckData;
    if (!ack?.pending?.length) {
      return;
    }
    const now = Date.now();
    set_messages((prev) => {
      let next = prev;
      for (const slot of ack.pending) {
        const placeholder: AssistantMessage = {
          message_id: slot.msg_id,
          session_key: incoming_session_key ?? '',
          agent_id: slot.agent_id,
          round_id: ack.round_id,
          role: 'assistant',
          content: [],
          is_complete: false,
          stream_status: 'pending',
          timestamp: now,
        };
        next = upsertMessage(next, placeholder);
      }
      return next;
    });
    set_is_loading(true);
    return;
  }

  // stream_start: flip placeholder from pending → streaming
  if (event.event_type === 'stream_start') {
    if (!is_current_session_event(incoming_session_key)) {
      return;
    }
    const msg_id = event.data?.msg_id as string | undefined;
    if (msg_id) {
      update_message_status?.(msg_id, 'streaming');
    }
    return;
  }

  // stream_end: mark bubble done
  if (event.event_type === 'stream_end') {
    if (!is_current_session_event(incoming_session_key)) {
      return;
    }
    const msg_id = event.data?.msg_id as string | undefined;
    if (msg_id) {
      update_message_status?.(msg_id, 'done');
    }
    return;
  }

  // stream_cancelled: mark bubble cancelled, stop loading
  if (event.event_type === 'stream_cancelled') {
    if (!is_current_session_event(incoming_session_key)) {
      return;
    }
    const msg_id = event.data?.msg_id as string | undefined;
    if (msg_id) {
      update_message_status?.(msg_id, 'cancelled');
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
      set_messages((prev) => applyStreamMessage(prev, payload));
      set_is_loading(true);
    }
    return;
  }

  const payload = event.data as Message;
  const message_session_key = payload?.session_key || incoming_session_key;
  if (!payload || !message_session_key) {
    return;
  }

  if (!is_current_session_event(message_session_key)) {
    // Cache complete messages for non-active sessions so they aren't lost
    // when the user switches conversations and switches back.
    if (on_background_message) {
      on_background_message(message_session_key, payload);
    }
    return;
  }

  set_messages((prev) => upsertMessage(prev, payload));
  if (payload.role === 'result') {
    set_pending_permission(null);
    set_is_loading(false);
    return;
  }
  if (payload.role === 'assistant') {
    set_is_loading(true);
  }
}
