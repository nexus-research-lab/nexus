import {
  AssistantMessage,
  AssistantMessageStatus,
  ChatAckData,
  EventMessage,
  Message,
  ResultMessage,
  StreamMessage,
} from '@/types';
import {
  AgentThinkingPayload,
  HandleAgentConversationWebSocketMessageParams,
  RoomEventPayload,
} from '@/types/agent-conversation';
import { WorkspaceEventPayload } from '@/types/workspace-live';
import { buildPermissionSignature } from '@/types/permission';

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
  set_pending_permissions,
  enqueue_stream_payload,
  on_background_message,
  set_agent_thinking,
  on_room_event,
  update_message_status,
  clear_round_tracking,
  reset_loading_tracking,
  track_chat_ack,
  track_assistant_message,
  track_result_message,
}: HandleAgentConversationWebSocketMessageParams): void {
  const clearTimedOutQuestionPermissions = (incoming_session: string | null) => {
    set_pending_permissions((prev) => prev.filter((item) => !(
      (item.interaction_mode === 'question' || item.tool_name === 'AskUserQuestion') &&
      (item.session_key ?? null) === incoming_session
    )));
  };
  const event = backend_message as EventMessage;
  const incoming_session_key = event.session_key || null;

  if (event.event_type === 'error') {
    if (incoming_session_key && !is_current_session_event(incoming_session_key)) {
      return;
    }
    if (event.message_id) {
      update_message_status?.(event.message_id, 'error', event.caused_by);
      clear_round_tracking?.(event.caused_by ?? event.data?.round_id ?? null);
    } else {
      reset_loading_tracking?.();
      if (!reset_loading_tracking) {
        set_is_loading(false);
      }
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
      const next_signature = buildPermissionSignature(
        next_permission.tool_name,
        next_permission.tool_input,
      );
      return [
        ...prev.filter((item) => {
          if (item.request_id === data.request_id) {
            return false;
          }
          if (
            (next_permission.interaction_mode !== 'question' && next_permission.tool_name !== 'AskUserQuestion') ||
            (item.interaction_mode !== 'question' && item.tool_name !== 'AskUserQuestion')
          ) {
            return true;
          }
          if ((item.session_key ?? null) !== (next_permission.session_key ?? null)) {
            return true;
          }
          if ((item.agent_id ?? null) !== (next_permission.agent_id ?? null)) {
            return true;
          }
          return buildPermissionSignature(item.tool_name, item.tool_input) !== next_signature;
        }),
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

  // Agent thinking/done status in multi-agent rooms
  // 只处理当前会话的 thinking 状态，避免跨 room 污染
  if (event.event_type === 'agent_thinking') {
    if (incoming_session_key && !is_current_session_event(incoming_session_key)) {
      return;
    }
    set_agent_thinking?.(event.data as AgentThinkingPayload);
    return;
  }

  if (event.event_type === 'agent_done') {
    if (incoming_session_key && !is_current_session_event(incoming_session_key)) {
      return;
    }
    set_agent_thinking?.(null);
    return;
  }

  // Room-level events (member changes, room deleted, etc.)
  if (
    event.event_type === 'room_member_added' ||
    event.event_type === 'room_member_removed' ||
    event.event_type === 'room_deleted' ||
    event.event_type === 'room_resync_required'
  ) {
    on_room_event?.(event.event_type, (event.data ?? {}) as RoomEventPayload);
    return;
  }

  // session_status: 重连后后端告知该 session 是否仍在生成，恢复 loading 态
  if (event.event_type === 'session_status') {
    if (!is_current_session_event(incoming_session_key)) {
      return;
    }
    if (event.data?.is_generating) {
      set_is_loading(true);
    }
    return;
  }

  // chat_ack: 仅登记 Room 占位槽位，不再插入假 assistant 消息
  if (event.event_type === 'chat_ack') {
    if (!is_current_session_event(incoming_session_key)) {
      return;
    }
    const ack = event.data as ChatAckData;
    if (!ack?.pending?.length) {
      return;
    }
    track_chat_ack?.(ack, incoming_session_key);
    set_is_loading(true);
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
    clear_round_tracking?.(event.data?.round_id ?? event.caused_by ?? null);
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
  const tool_use_result = (payload as { tool_use_result?: string }).tool_use_result;
  if (payload.role === 'user' && tool_use_result === 'Error: Permission request timeout') {
    clearTimedOutQuestionPermissions(message_session_key);
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
    set_pending_permissions([]);
    track_result_message?.(payload as ResultMessage);
    if (!track_result_message) {
      set_is_loading(false);
    }
    return;
  }
  if (payload.role === 'assistant') {
    track_assistant_message?.(payload as AssistantMessage);
    if (!track_assistant_message) {
      set_is_loading(!(payload.is_complete || payload.stop_reason));
    }
  }
}
