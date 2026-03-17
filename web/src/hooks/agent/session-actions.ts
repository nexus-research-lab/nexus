import { WebSocketMessage } from '@/lib/websocket';
import { deleteRound as deleteRoundApi } from '@/lib/agent-api';
import { generateUuid } from '@/lib/uuid';
import { Message, UserMessage } from '@/types';
import { PermissionDecisionPayload } from '@/types/permission';

import { upsertMessage } from './message-helpers';
import { AgentSessionActionContext } from './session-context';

/**
 * 发送用户消息并建立当前轮次的本地状态。
 */
export async function sendSessionMessage(
  content: string,
  context: AgentSessionActionContext,
): Promise<void> {
  const {
    agentId,
    sessionKey,
    wsState,
    wsSend,
    activeSessionKeyRef,
    setError,
    setIsLoading,
    setMessages,
    setPendingPermission,
  } = context;

  if (!content.trim()) {
    return;
  }
  if (!sessionKey) {
    setError('请先选择或创建会话');
    return;
  }
  if (wsState !== 'connected') {
    setError('WebSocket未连接,请稍候重试');
    return;
  }

  const roundId = generateUuid();
  activeSessionKeyRef.current = sessionKey;
  const userMessage: Message = {
    message_id: roundId,
    session_key: sessionKey,
    round_id: roundId,
    agent_id: agentId || 'main',
    role: 'user',
    content,
    timestamp: Date.now(),
  };

  setMessages((prev) => upsertMessage(prev, userMessage));
  setPendingPermission(null);
  setIsLoading(true);
  setError(null);

  wsSend({
    type: 'chat',
    content,
    session_key: sessionKey,
    agent_id: agentId || 'main',
    round_id: roundId,
  });
}

/**
 * 中断当前会话生成。
 */
export function stopSessionGeneration(context: AgentSessionActionContext): void {
  const {
    agentId,
    sessionKey,
    wsState,
    wsSend,
    messages,
    setIsLoading,
    setPendingPermission,
  } = context;

  if (!sessionKey || wsState !== 'connected') {
    setIsLoading(false);
    return;
  }

  const latestUserRoundId = [...messages]
    .reverse()
    .find((message) => message.role === 'user')?.round_id;

  wsSend({
    type: 'interrupt',
    session_key: sessionKey,
    agent_id: agentId || 'main',
    round_id: latestUserRoundId,
  });

  setIsLoading(false);
  setPendingPermission(null);
}

/**
 * 提交权限决策。
 */
export function sendSessionPermissionResponse(
  payload: PermissionDecisionPayload,
  context: AgentSessionActionContext,
): void {
  const {
    agentId,
    sessionKey,
    wsState,
    wsSend,
    activeSessionKeyRef,
    pendingPermission,
    setError,
    setPendingPermission,
  } = context;

  if (!pendingPermission) {
    return;
  }
  if (!sessionKey || activeSessionKeyRef.current !== sessionKey) {
    setPendingPermission(null);
    return;
  }
  if (wsState !== 'connected') {
    setError('WebSocket未连接，无法提交权限决策');
    return;
  }

  const response: WebSocketMessage = {
    type: 'permission_response',
    request_id: pendingPermission.request_id,
    session_key: sessionKey,
    agent_id: agentId || 'main',
    decision: payload.decision,
    message: payload.message || (payload.decision === 'deny' ? 'User denied permission' : ''),
    interrupt: payload.interrupt ?? false,
  };

  if (payload.userAnswers?.length) {
    response.user_answers = payload.userAnswers;
  }
  if (payload.updatedPermissions?.length) {
    response.updated_permissions = payload.updatedPermissions;
  }

  wsSend(response);
  setPendingPermission(null);
}

/**
 * 删除指定轮次的消息。
 */
export async function deleteSessionRound(
  roundId: string,
  context: AgentSessionActionContext,
): Promise<void> {
  const {
    sessionKey,
    setError,
    setMessages,
  } = context;

  if (!sessionKey) {
    return;
  }

  try {
    await deleteRoundApi(sessionKey, roundId);
    setMessages((prev) => prev.filter((message) => message.round_id !== roundId));
  } catch (err) {
    console.error('[deleteRound] 删除失败:', err);
    setError(err instanceof Error ? err.message : 'Failed to delete round');
  }
}

/**
 * 基于指定轮次重新生成回复。
 */
export async function regenerateSessionRound(
  roundId: string,
  context: AgentSessionActionContext,
): Promise<void> {
  const {
    sessionKey,
    messages,
    setError,
    setIsLoading,
  } = context;

  if (!sessionKey) {
    return;
  }

  const lastUserMessage = messages.findLast(
    (message) => message.role === 'user' && message.message_id === roundId,
  ) as UserMessage | undefined;
  if (!lastUserMessage?.content) {
    return;
  }

  try {
    await deleteSessionRound(roundId, context);
    await sendSessionMessage(lastUserMessage.content, context);
  } catch (err) {
    console.error('[regenerate] 重新生成失败:', err);
    setError(err instanceof Error ? err.message : 'Failed to regenerate');
    setIsLoading(false);
  }
}
