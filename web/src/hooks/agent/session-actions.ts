import { WebSocketMessage } from '@/types/websocket';
import { deleteRound as deleteRoundApi } from '@/lib/agent-api';
import { generateUuid } from '@/lib/uuid';
import { Message, UserMessage } from '@/types';
import { AgentSessionActionContext } from '@/types/agent-session';
import { PermissionDecisionPayload } from '@/types/permission';

import { upsertMessage } from './message-helpers';

/**
 * 发送用户消息并建立当前轮次的本地状态。
 */
export async function sendSessionMessage(
  content: string,
  context: AgentSessionActionContext,
): Promise<void> {
  const {
    agent_id,
    session_key,
    ws_state,
    ws_send,
    active_session_key_ref,
    set_error,
    set_is_loading,
    set_messages,
    set_pending_permission,
  } = context;

  if (!content.trim()) {
    return;
  }
  if (!session_key) {
    set_error('请先选择或创建会话');
    return;
  }
  if (ws_state !== 'connected') {
    set_error('WebSocket未连接,请稍候重试');
    return;
  }

  const round_id = generateUuid();
  active_session_key_ref.current = session_key;
  const userMessage: Message = {
    message_id: round_id,
    session_key,
    round_id,
    agent_id: agent_id || 'main',
    role: 'user',
    content,
    timestamp: Date.now(),
  };

  set_messages((prev) => upsertMessage(prev, userMessage));
  set_pending_permission(null);
  set_is_loading(true);
  set_error(null);

  ws_send({
    type: 'chat',
    content,
    session_key,
    agent_id: agent_id || 'main',
    round_id,
  });
}

/**
 * 中断当前会话生成。
 */
export function stopSessionGeneration(context: AgentSessionActionContext): void {
  const {
    agent_id,
    session_key,
    ws_state,
    ws_send,
    messages,
    set_is_loading,
    set_pending_permission,
  } = context;

  if (!session_key || ws_state !== 'connected') {
    set_is_loading(false);
    return;
  }

  const latest_user_round_id = [...messages]
    .reverse()
    .find((message) => message.role === 'user')?.round_id;

  ws_send({
    type: 'interrupt',
    session_key,
    agent_id: agent_id || 'main',
    round_id: latest_user_round_id,
  });

  set_is_loading(false);
  set_pending_permission(null);
}

/**
 * 提交权限决策。
 */
export function sendSessionPermissionResponse(
  payload: PermissionDecisionPayload,
  context: AgentSessionActionContext,
): void {
  const {
    agent_id,
    session_key,
    ws_state,
    ws_send,
    active_session_key_ref,
    pending_permission,
    set_error,
    set_pending_permission,
  } = context;

  if (!pending_permission) {
    return;
  }
  if (!session_key || active_session_key_ref.current !== session_key) {
    set_pending_permission(null);
    return;
  }
  if (ws_state !== 'connected') {
    set_error('WebSocket未连接，无法提交权限决策');
    return;
  }

  const response: WebSocketMessage = {
    type: 'permission_response',
    request_id: pending_permission.request_id,
    session_key,
    agent_id: agent_id || 'main',
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
  set_pending_permission(null);
}

/**
 * 删除指定轮次的消息。
 */
export async function deleteSessionRound(
  round_id: string,
  context: AgentSessionActionContext,
): Promise<void> {
  const {
    session_key,
    set_error,
    set_messages,
  } = context;

  if (!session_key) {
    return;
  }

  try {
    await deleteRoundApi(session_key, round_id);
    set_messages((prev) => prev.filter((message) => message.round_id !== round_id));
  } catch (err) {
    console.error('[deleteRound] 删除失败:', err);
    set_error(err instanceof Error ? err.message : 'Failed to delete round');
  }
}

/**
 * 基于指定轮次重新生成回复。
 */
export async function regenerateSessionRound(
  round_id: string,
  context: AgentSessionActionContext,
): Promise<void> {
  const {
    session_key,
    messages,
    set_error,
    set_is_loading,
  } = context;

  if (!session_key) {
    return;
  }

  const last_user_message = messages.findLast(
    (message) => message.role === 'user' && message.message_id === round_id,
  ) as UserMessage | undefined;
  if (!last_user_message?.content) {
    return;
  }

  try {
    await deleteSessionRound(round_id, context);
    await sendSessionMessage(last_user_message.content, context);
  } catch (err) {
    console.error('[regenerate] 重新生成失败:', err);
    set_error(err instanceof Error ? err.message : 'Failed to regenerate');
    set_is_loading(false);
  }
}
