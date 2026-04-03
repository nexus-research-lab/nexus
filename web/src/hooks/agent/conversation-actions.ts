import { resolveAgentId } from '@/config/options';
import { WebSocketMessage } from '@/types/websocket';
import { isStructuredSessionKey } from '@/lib/session-key';
import { generateUuid } from '@/lib/uuid';
import { Message } from '@/types';
import { AgentConversationActionContext } from '@/types/agent-conversation';
import { PermissionDecisionPayload } from '@/types/permission';

import { upsertMessage } from './message-helpers';

/**
 * 发送用户消息并建立当前轮次的本地状态。
 */
export async function sendSessionMessage(
  content: string,
  context: AgentConversationActionContext,
): Promise<string | null> {
  const {
    agent_id,
    session_key,
    room_id,
    conversation_id,
    chat_type,
    ws_state,
    ws_send,
    active_session_key_ref,
    set_error,
    set_is_loading,
    set_messages,
    set_pending_permissions,
  } = context;
  const resolved_session_key = session_key || active_session_key_ref.current;

  if (!content.trim()) {
    return null;
  }
  if (!resolved_session_key) {
    set_error('请先选择或创建会话');
    return null;
  }
  if (!isStructuredSessionKey(resolved_session_key)) {
    set_error('当前会话的 session_key 非法，请刷新后重试');
    return null;
  }
  if (ws_state !== 'connected') {
    set_error('WebSocket未连接,请稍候重试');
    return null;
  }

  const round_id = generateUuid();
  active_session_key_ref.current = resolved_session_key;
  const userMessage: Message = {
    message_id: round_id,
    session_key: resolved_session_key,
    round_id,
    agent_id: resolveAgentId(agent_id),
    role: 'user',
    content,
    timestamp: Date.now(),
    ...(chat_type === 'group' ? { room_id: room_id ?? undefined, conversation_id: conversation_id ?? undefined } : {}),
  };

  set_messages((prev) => upsertMessage(prev, userMessage));
  set_pending_permissions([]);
  set_is_loading(true);
  set_error(null);

  const ws_payload: Record<string, unknown> = {
    type: 'chat',
    content,
    session_key: resolved_session_key,
    agent_id: resolveAgentId(agent_id),
    round_id,
    req_id: round_id,  // echo'd back in chat_ack for correlation
  };

  // Room 消息附加 room 上下文
  if (chat_type === 'group') {
    ws_payload.chat_type = 'group';
    if (room_id) ws_payload.room_id = room_id;
    if (conversation_id) ws_payload.conversation_id = conversation_id;
  }

  ws_send(ws_payload as WebSocketMessage);
  return round_id;
}

/**
 * 中断当前会话生成。
 * @param context - 会话上下文
 * @param msg_id - 可选，指定只取消某个 Agent 气泡（Room 并发场景）
 */
export function stopSessionGeneration(
  context: AgentConversationActionContext,
  msg_id?: string,
): void {
  const {
    agent_id,
    session_key,
    room_id,
    conversation_id,
    chat_type,
    ws_state,
    ws_send,
    active_session_key_ref,
    messages,
    set_error,
    set_is_loading,
    set_pending_permissions,
  } = context;
  const resolved_session_key = session_key || active_session_key_ref.current;

  if (!resolved_session_key || ws_state !== 'connected') {
    set_is_loading(false);
    return;
  }
  if (!isStructuredSessionKey(resolved_session_key)) {
    set_error('当前会话的 session_key 非法，无法中断');
    set_is_loading(false);
    return;
  }

  const latest_user_round_id = [...messages]
    .reverse()
    .find((message) => message.role === 'user')?.round_id;

  const payload: Record<string, unknown> = {
    type: 'interrupt',
    session_key: resolved_session_key,
    agent_id: resolveAgentId(agent_id),
    round_id: latest_user_round_id,
  };

  // per-msg_id interrupt for Room multi-agent scenario
  if (msg_id) {
    payload.msg_id = msg_id;
    // 从消息列表中查找目标 agent_id，用于后端精确定位中断目标
    const target_message = messages.find((m) => m.message_id === msg_id);
    if (target_message?.agent_id) {
      payload.target_agent_id = target_message.agent_id;
    }
  }
  if (chat_type === 'group') {
    if (room_id) payload.room_id = room_id;
    if (conversation_id) payload.conversation_id = conversation_id;
  }

  ws_send(payload as WebSocketMessage);

  set_is_loading(false);
  set_pending_permissions([]);
}

/**
 * 提交权限决策。
 */
export function sendSessionPermissionResponse(
  payload: PermissionDecisionPayload,
  context: AgentConversationActionContext,
): boolean {
  const {
    agent_id,
    session_key,
    ws_state,
    ws_send,
    active_session_key_ref,
    pending_permissions,
    set_error,
    set_is_loading,
    set_pending_permissions,
  } = context;
  const resolved_session_key = session_key || active_session_key_ref.current;
  const pending_permission = pending_permissions.find(
    (item) => item.request_id === payload.request_id,
  );

  if (!pending_permission) {
    return false;
  }
  if (!resolved_session_key || active_session_key_ref.current !== resolved_session_key) {
    set_pending_permissions((prev) => prev.filter((item) => item.request_id !== payload.request_id));
    return false;
  }
  if (!isStructuredSessionKey(resolved_session_key)) {
    set_error('当前会话的 session_key 非法，无法提交权限决策');
    return false;
  }
  if (ws_state !== 'connected') {
    set_error('WebSocket未连接，无法提交权限决策');
    return false;
  }

  const response: WebSocketMessage = {
    type: 'permission_response',
    request_id: payload.request_id,
    session_key: resolved_session_key,
    agent_id: resolveAgentId(agent_id),
    decision: payload.decision,
    message: payload.message || (payload.decision === 'deny' ? 'User denied permission' : ''),
    interrupt: payload.interrupt ?? false,
  };

  if (payload.user_answers?.length) {
    response.user_answers = payload.user_answers;
  }
  if (payload.updated_permissions?.length) {
    response.updated_permissions = payload.updated_permissions;
  }

  ws_send(response);
  set_pending_permissions((prev) => prev.filter((item) => item.request_id !== payload.request_id));
  set_is_loading(true);
  set_error(null);
  return true;
}
