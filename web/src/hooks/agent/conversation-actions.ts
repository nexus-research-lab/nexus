import { resolveAgentId } from '@/config/options';
import { WebSocketMessage } from '@/types/system/websocket';
import { isStructuredSessionKey } from '@/lib/conversation/session-key';
import { generateUuid } from '@/lib/uuid';
import { Message } from '@/types';
import {
  AgentConversationActionContext,
  AgentConversationDeliveryPolicy,
  AgentConversationSendOptions,
} from '@/types/agent/agent-conversation';
import { PermissionDecisionPayload } from '@/types/conversation/permission';

import { upsertMessage } from './message-helpers';

function failSend(setError: AgentConversationActionContext["set_error"], message: string): never {
  setError(message);
  throw new Error(message);
}

export function buildSessionBindMessage({
  session_key: sessionKey,
  last_seen_session_seq: lastSeenSessionSeq,
  agent_id: agentId,
  room_id: roomId,
  conversation_id: conversationId,
}: {
  session_key: string;
  last_seen_session_seq?: number;
  agent_id?: string | null;
  room_id?: string | null;
  conversation_id?: string | null;
}): WebSocketMessage {
  return {
    type: 'bind_session',
    session_key: sessionKey,
    ...(lastSeenSessionSeq && lastSeenSessionSeq > 0
      ? { last_seen_session_seq: lastSeenSessionSeq }
      : {}),
    ...(agentId ? { agent_id: agentId } : {}),
    ...(roomId ? { room_id: roomId } : {}),
    ...(conversationId ? { conversation_id: conversationId } : {}),
  };
}

export function buildRoomSubscriptionMessage({
  type,
  room_id: roomId,
  conversation_id: conversationId,
  last_seen_room_seq: lastSeenRoomSeq,
}: {
  type: 'subscribe_room' | 'unsubscribe_room';
  room_id: string;
  conversation_id?: string | null;
  last_seen_room_seq?: number;
}): WebSocketMessage {
  return {
    type,
    room_id: roomId,
    ...(conversationId ? { conversation_id: conversationId } : {}),
    ...(type === 'subscribe_room' && lastSeenRoomSeq && lastSeenRoomSeq > 0
      ? { last_seen_room_seq: lastSeenRoomSeq }
      : {}),
  };
}

export interface OutboundChatRequest {
  client_request_id: string;
  client_message_id: string;
}

/**
 * 发送用户消息并建立当前轮次的本地状态。
 * round_id 由后端 mint；前端只生成 client_request_id / client_message_id。
 */
export async function sendSessionMessage(
  content: string,
  context: AgentConversationActionContext,
  options: AgentConversationSendOptions = {},
): Promise<OutboundChatRequest | null> {
  const {
    identity,
    session_key: sessionKey,
    ws_state: wsState,
    ws_send: wsSend,
    active_session_key_ref: activeSessionKeyRef,
    set_error: setError,
    set_messages: setMessages,
    set_pending_permissions: setPendingPermissions,
  } = context;
  const agentId = identity?.agent_id;
  const roomId = identity?.room_id;
  const conversationId = identity?.conversation_id;
  const chatType = identity?.chat_type;
  const resolvedSessionKey = sessionKey || activeSessionKeyRef.current;
  const attachments = options.attachments ?? [];

  if (!content.trim() && attachments.length === 0) {
    return null;
  }
  if (!resolvedSessionKey) {
    failSend(setError, '请先选择或创建会话');
  }
  if (!isStructuredSessionKey(resolvedSessionKey)) {
    failSend(setError, '当前会话的 session_key 非法，请刷新后重试');
  }
  if (wsState !== 'connected') {
    failSend(setError, 'WebSocket未连接，请稍候重试');
  }

  const clientRequestId = `req_${generateUuid()}`;
  const clientMessageId = `local_msg_${generateUuid()}`;
  const deliveryPolicy = options.delivery_policy ?? 'queue';
  activeSessionKeyRef.current = resolvedSessionKey;
  // optimistic user message：message_id 用本地 id，ack 后由 canonical id 替换。
  const userMessage: Message = {
    message_id: clientMessageId,
    session_key: resolvedSessionKey,
    round_id: clientMessageId,
    agent_id: resolveAgentId(agentId),
    role: 'user',
    content,
    timestamp: Date.now(),
    delivery_policy: deliveryPolicy,
    ...(attachments.length > 0 ? { attachments } : {}),
    ...(chatType === 'group' ? { room_id: roomId ?? undefined, conversation_id: conversationId ?? undefined } : {}),
  };

  const wsPayload: Record<string, unknown> = {
    type: 'chat',
    content,
    session_key: resolvedSessionKey,
    agent_id: resolveAgentId(agentId),
    client_request_id: clientRequestId,
    client_message_id: clientMessageId,
    delivery_policy: deliveryPolicy,
  };
  if (attachments.length > 0) {
    wsPayload.attachments = attachments;
  }

  // Room 消息附加 room 上下文
  if (chatType === 'group') {
    wsPayload.chat_type = 'group';
    if (roomId) wsPayload.room_id = roomId;
    if (conversationId) wsPayload.conversation_id = conversationId;
  }

  const sendResult = wsSend(wsPayload as WebSocketMessage);
  if (sendResult.disposition !== 'sent') {
    failSend(setError, '消息未发送到后端，请检查连接后重试');
  }

  setMessages((prev) => upsertMessage(prev, userMessage));
  setPendingPermissions([]);
  setError(null);
  return { client_request_id: clientRequestId, client_message_id: clientMessageId };
}

/**
 * 编辑最后一条用户消息，并请求后端按有效历史重新生成。
 */
export async function rewriteLastUserMessage(
  targetRoundId: string,
  content: string,
  context: AgentConversationActionContext,
): Promise<OutboundChatRequest | null> {
  const {
    identity,
    session_key: sessionKey,
    ws_state: wsState,
    ws_send: wsSend,
    active_session_key_ref: activeSessionKeyRef,
    set_error: setError,
  } = context;
  const resolvedSessionKey = sessionKey || activeSessionKeyRef.current;
  const agentId = identity?.agent_id;

  if (!content.trim()) {
    return null;
  }
  if (!targetRoundId.trim()) {
    failSend(setError, '找不到要编辑的消息，请刷新后重试');
  }
  if (!resolvedSessionKey) {
    failSend(setError, '请先选择或创建会话');
  }
  if (!isStructuredSessionKey(resolvedSessionKey)) {
    failSend(setError, '当前会话的 session_key 非法，请刷新后重试');
  }
  if (identity?.chat_type === 'group') {
    failSend(setError, 'Room 会话暂不支持编辑重跑');
  }
  if (wsState !== 'connected') {
    failSend(setError, 'WebSocket未连接，请稍候重试');
  }

  const clientRequestId = `req_${generateUuid()}`;
  const clientMessageId = `local_msg_${generateUuid()}`;
  activeSessionKeyRef.current = resolvedSessionKey;
  // replacement round_id 由后端 mint 并通过 chat_ack 回传。
  const sendResult = wsSend({
    type: 'chat_rewrite_last',
    content,
    session_key: resolvedSessionKey,
    agent_id: resolveAgentId(agentId),
    target_round_id: targetRoundId,
    client_request_id: clientRequestId,
    client_message_id: clientMessageId,
  } as WebSocketMessage);
  if (sendResult.disposition !== 'sent') {
    failSend(setError, '消息未发送到后端，请检查连接后重试');
  }

  setError(null);
  return { client_request_id: clientRequestId, client_message_id: clientMessageId };
}

function buildInputQueueBasePayload(
  context: AgentConversationActionContext,
): Record<string, unknown> {
  const { identity, session_key: sessionKey, active_session_key_ref: activeSessionKeyRef } = context;
  const resolvedSessionKey = sessionKey || activeSessionKeyRef.current;
  const agentId = identity?.agent_id;
  const roomId = identity?.room_id;
  const conversationId = identity?.conversation_id;
  const chatType = identity?.chat_type;

  if (!resolvedSessionKey) {
    failSend(context.set_error, '请先选择或创建会话');
  }
  if (!isStructuredSessionKey(resolvedSessionKey)) {
    failSend(context.set_error, '当前会话的 session_key 非法，请刷新后重试');
  }
  if (context.ws_state !== 'connected') {
    failSend(context.set_error, 'WebSocket未连接，请稍候重试');
  }

  const payload: Record<string, unknown> = {
    type: 'input_queue',
    session_key: resolvedSessionKey,
    agent_id: resolveAgentId(agentId),
  };
  if (chatType === 'group') {
    payload.chat_type = 'group';
    if (roomId) payload.room_id = roomId;
    if (conversationId) payload.conversation_id = conversationId;
  }
  return payload;
}

function sendInputQueuePayload(
  context: AgentConversationActionContext,
  payload: Record<string, unknown>,
): void {
  const sendResult = context.ws_send(payload as WebSocketMessage);
  if (sendResult.disposition !== 'sent') {
    failSend(context.set_error, '队列请求未发送到后端，请检查连接后重试');
  }
  context.set_error(null);
}

export function enqueueInputQueueMessage(
  content: string,
  context: AgentConversationActionContext,
  deliveryPolicy: AgentConversationDeliveryPolicy = 'queue',
  attachments: AgentConversationSendOptions["attachments"] = [],
): void {
  if (!content.trim() && attachments.length === 0) {
    return;
  }
  sendInputQueuePayload(context, {
    ...buildInputQueueBasePayload(context),
    action: 'enqueue',
    content,
    delivery_policy: deliveryPolicy,
    ...(attachments.length > 0 ? { attachments } : {}),
  });
}

export function deleteInputQueueMessage(
  itemId: string,
  context: AgentConversationActionContext,
): void {
  if (!itemId.trim()) {
    return;
  }
  sendInputQueuePayload(context, {
    ...buildInputQueueBasePayload(context),
    action: 'delete',
    item_id: itemId,
  });
}

export function guideInputQueueMessage(
  itemId: string,
  context: AgentConversationActionContext,
): void {
  if (!itemId.trim()) {
    return;
  }
  sendInputQueuePayload(context, {
    ...buildInputQueueBasePayload(context),
    action: 'guide',
    item_id: itemId,
  });
}

export function reorderInputQueueMessages(
  orderedIds: string[],
  context: AgentConversationActionContext,
): void {
  sendInputQueuePayload(context, {
    ...buildInputQueueBasePayload(context),
    action: 'reorder',
    ordered_ids: orderedIds,
  });
}

/**
 * 中断当前会话生成。
 * @param context - 会话上下文
 * @param agentRoundId - 可选，Room 并发场景下只停某个 agent slot
 */
export function stopSessionGeneration(
  context: AgentConversationActionContext,
  agentRoundId?: string,
): void {
  const {
    identity,
    session_key: sessionKey,
    ws_state: wsState,
    ws_send: wsSend,
    active_session_key_ref: activeSessionKeyRef,
    messages,
    set_error: setError,
    set_pending_permissions: setPendingPermissions,
  } = context;
  const agentId = identity?.agent_id;
  const roomId = identity?.room_id;
  const conversationId = identity?.conversation_id;
  const chatType = identity?.chat_type;
  const resolvedSessionKey = sessionKey || activeSessionKeyRef.current;

  if (!resolvedSessionKey || wsState !== 'connected') {
    return;
  }
  if (!isStructuredSessionKey(resolvedSessionKey)) {
    setError('当前会话的 session_key 非法，无法中断');
    return;
  }

  const latestUserRoundId = [...messages]
    .reverse()
    .find((message) => message.role === 'user')?.round_id;

  const payload: Record<string, unknown> = {
    type: 'interrupt',
    session_key: resolvedSessionKey,
    agent_id: resolveAgentId(agentId),
    round_id: latestUserRoundId,
  };
  if (agentRoundId) {
    payload.agent_round_id = agentRoundId;
  }
  if (chatType === 'group') {
    if (roomId) payload.room_id = roomId;
    if (conversationId) payload.conversation_id = conversationId;
  }

  const sendResult = wsSend(payload as WebSocketMessage);
  if (sendResult.disposition !== 'sent') {
    setError('中断请求发送失败，请稍后重试');
    return;
  }
  setPendingPermissions([]);
}

/**
 * 提交权限决策。
 */
export function sendSessionPermissionResponse(
  payload: PermissionDecisionPayload,
  context: AgentConversationActionContext,
): boolean {
  const {
    identity,
    session_key: sessionKey,
    ws_state: wsState,
    ws_send: wsSend,
    active_session_key_ref: activeSessionKeyRef,
    pending_permissions: pendingPermissions,
    set_error: setError,
    set_pending_permissions: setPendingPermissions,
  } = context;
  const resolvedSessionKey = sessionKey || activeSessionKeyRef.current;
  const agentId = identity?.agent_id;
  const pendingPermission = pendingPermissions.find(
    (item) => item.request_id === payload.request_id,
  );

  if (!pendingPermission) {
    return false;
  }
  if (!resolvedSessionKey || activeSessionKeyRef.current !== resolvedSessionKey) {
    setPendingPermissions((prev) => prev.filter((item) => item.request_id !== payload.request_id));
    return false;
  }
  if (!isStructuredSessionKey(resolvedSessionKey)) {
    setError('当前会话的 session_key 非法，无法提交权限决策');
    return false;
  }
  if (wsState !== 'connected') {
    setError('WebSocket未连接，无法提交权限决策');
    return false;
  }
  if (
    pendingPermission.interaction_mode === 'question' &&
    payload.decision === 'allow' &&
    !payload.user_answers?.length
  ) {
    setError('请先完成问题回答');
    return false;
  }

  const response: WebSocketMessage = {
    type: 'permission_response',
    request_id: payload.request_id,
    session_key: resolvedSessionKey,
    agent_id: resolveAgentId(pendingPermission.agent_id || agentId),
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

  const sendResult = wsSend(response);
  if (sendResult.disposition !== 'sent') {
    setError('权限决策发送失败，请稍后重试');
    return false;
  }
  setPendingPermissions((prev) => prev.filter((item) => item.request_id !== payload.request_id));
  setError(null);
  return true;
}
