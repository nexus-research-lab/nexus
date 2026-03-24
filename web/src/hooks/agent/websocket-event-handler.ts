import { EventMessage, Message, StreamMessage } from '@/types';
import { HandleAgentConversationWebSocketMessageParams } from '@/types/agent-conversation';
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

  if (event.event_type !== 'message') {
    if (event.event_type !== 'stream') {
      return;
    }

    const payload = event.data as StreamMessage;
    const message_session_key = payload?.session_key || incoming_session_key;
    if (!payload || !message_session_key || !is_current_session_event(message_session_key)) {
      return;
    }

    set_messages((prev) => applyStreamMessage(prev, payload));
    set_is_loading(true);
    return;
  }

  const payload = event.data as Message;
  const message_session_key = payload?.session_key || incoming_session_key;
  if (!payload || !message_session_key || !is_current_session_event(message_session_key)) {
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
