import { resolve_agent_id } from '@/config/options';
import { WebSocketMessage } from '@/types/websocket';
import { is_structured_session_key } from '@/lib/session-key';
import { generate_uuid } from '@/lib/uuid';
import { Message } from '@/types';
import { AgentConversationActionContext } from '@/types/agent-conversation';
import { PermissionDecisionPayload } from '@/types/permission';

import { upsert_message } from './message-helpers';

/**
 * 发送用户消息并建立当前轮次的本地状态。
 */
export async function send_session_message(
  content: string,
  context: AgentConversationActionContext,
): Promise<string | null> {
  const {
    identity,
    session_key,
    ws_state,
    session_control_state,
    ws_send,
    active_session_key_ref,
    set_error,
    set_messages,
    set_pending_permissions,
  } = context;
  const agent_id = identity?.agent_id;
  const room_id = identity?.room_id;
  const conversation_id = identity?.conversation_id;
  const chat_type = identity?.chat_type;
  const resolved_session_key = session_key || active_session_key_ref.current;

  if (!content.trim()) {
    return null;
  }
  if (!resolved_session_key) {
    set_error('请先选择或创建会话');
    return null;
  }
  if (!is_structured_session_key(resolved_session_key)) {
    set_error('当前会话的 session_key 非法，请刷新后重试');
    return null;
  }
  if (ws_state !== 'connected') {
    set_error('WebSocket未连接,请稍候重试');
    return null;
  }
  if (session_control_state === 'observer') {
    set_error('当前窗口是观察视图，无法发送消息');
    return null;
  }

  const round_id = generate_uuid();
  active_session_key_ref.current = resolved_session_key;
  const userMessage: Message = {
    message_id: round_id,
    session_key: resolved_session_key,
    round_id,
    agent_id: resolve_agent_id(agent_id),
    role: 'user',
    content,
    timestamp: Date.now(),
    ...(chat_type === 'group' ? { room_id: room_id ?? undefined, conversation_id: conversation_id ?? undefined } : {}),
  };

  set_messages((prev) => upsert_message(prev, userMessage));
  set_pending_permissions([]);
  set_error(null);

  const ws_payload: Record<string, unknown> = {
    type: 'chat',
    content,
    session_key: resolved_session_key,
    agent_id: resolve_agent_id(agent_id),
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
export function stop_session_generation(
  context: AgentConversationActionContext,
  msg_id?: string,
): void {
  const {
    identity,
    session_key,
    ws_state,
    session_control_state,
    ws_send,
    active_session_key_ref,
    messages,
    pending_agent_slots,
    set_error,
    set_pending_permissions,
  } = context;
  const agent_id = identity?.agent_id;
  const room_id = identity?.room_id;
  const conversation_id = identity?.conversation_id;
  const chat_type = identity?.chat_type;
  const resolved_session_key = session_key || active_session_key_ref.current;

  if (!resolved_session_key || ws_state !== 'connected') {
    return;
  }
  if (!is_structured_session_key(resolved_session_key)) {
    set_error('当前会话的 session_key 非法，无法中断');
    return;
  }
  if (session_control_state === 'observer') {
    set_error('当前窗口是观察视图，无法停止生成');
    return;
  }

  const latest_user_round_id = [...messages]
    .reverse()
    .find((message) => message.role === 'user')?.round_id;

  const payload: Record<string, unknown> = {
    type: 'interrupt',
    session_key: resolved_session_key,
    agent_id: resolve_agent_id(agent_id),
    round_id: latest_user_round_id,
  };

  // per-msg_id interrupt for Room multi-agent scenario
  if (msg_id) {
    payload.msg_id = msg_id;
    // 中文注释：Room 的占位槽位不再写入 messages，需要同时查本地 slot 状态。
    const target_message = messages.find((m) => m.message_id === msg_id);
    const target_slot = pending_agent_slots.find((slot) => slot.msg_id === msg_id);
    if (target_message?.agent_id || target_slot?.agent_id) {
      payload.target_agent_id = target_message?.agent_id ?? target_slot?.agent_id;
    }
  }
  if (chat_type === 'group') {
    if (room_id) payload.room_id = room_id;
    if (conversation_id) payload.conversation_id = conversation_id;
  }

  ws_send(payload as WebSocketMessage);
  set_pending_permissions([]);
}

/**
 * 提交权限决策。
 */
export function send_session_permission_response(
  payload: PermissionDecisionPayload,
  context: AgentConversationActionContext,
): boolean {
  const {
    identity,
    session_key,
    ws_state,
    session_control_state,
    ws_send,
    active_session_key_ref,
    pending_permissions,
    set_error,
    set_pending_permissions,
  } = context;
  const resolved_session_key = session_key || active_session_key_ref.current;
  const agent_id = identity?.agent_id;
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
  if (!is_structured_session_key(resolved_session_key)) {
    set_error('当前会话的 session_key 非法，无法提交权限决策');
    return false;
  }
  if (ws_state !== 'connected') {
    set_error('WebSocket未连接，无法提交权限决策');
    return false;
  }
  if (session_control_state === 'observer') {
    set_error('当前窗口是观察视图，无法确认权限');
    return false;
  }
  if (
    pending_permission.interaction_mode === 'question' &&
    payload.decision === 'allow' &&
    !payload.user_answers?.length
  ) {
    set_error('请先完成问题回答');
    return false;
  }

  const response: WebSocketMessage = {
    type: 'permission_response',
    request_id: payload.request_id,
    session_key: resolved_session_key,
    agent_id: resolve_agent_id(pending_permission.agent_id || agent_id),
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
  set_error(null);
  return true;
}
