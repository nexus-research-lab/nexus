import { getSessionMessages } from '@/lib/agent-api';
import { generateUuid } from '@/lib/uuid';

import { sortMessages } from './message-helpers';
import { AgentSessionLifecycleContext } from './session-context';

/**
 * 重置当前会话视图状态。
 */
export function resetSessionView(
  context: AgentSessionLifecycleContext,
  nextError: string | null = null,
): void {
  context.setMessages([]);
  context.setPendingPermission(null);
  context.setIsLoading(false);
  context.setError(nextError);
}

/**
 * 启动一个新的会话。
 */
export function startAgentSession(context: AgentSessionLifecycleContext): void {
  const newSessionKey = generateUuid();
  context.loadRequestIdRef.current += 1;
  context.activeSessionKeyRef.current = newSessionKey;
  context.setSessionKey(newSessionKey);
  resetSessionView(context);
}

/**
 * 加载现有会话消息。
 */
export async function loadAgentSession(
  sessionKey: string,
  context: AgentSessionLifecycleContext,
): Promise<void> {
  const requestId = context.loadRequestIdRef.current + 1;
  context.loadRequestIdRef.current = requestId;
  context.activeSessionKeyRef.current = sessionKey;
  context.setSessionKey(sessionKey);
  resetSessionView(context);

  try {
    const data = await getSessionMessages(sessionKey);
    if (
      context.loadRequestIdRef.current !== requestId ||
      context.activeSessionKeyRef.current !== sessionKey
    ) {
      return;
    }
    if (Array.isArray(data)) {
      context.setMessages(sortMessages(data));
    }
  } catch (err) {
    if (
      context.loadRequestIdRef.current !== requestId ||
      context.activeSessionKeyRef.current !== sessionKey
    ) {
      return;
    }
    console.error('[loadSession] 加载session失败:', err);
    context.setError(err instanceof Error ? err.message : 'Failed to load session');
  }
}

/**
 * 清空当前会话选择。
 */
export function clearAgentSession(context: AgentSessionLifecycleContext): void {
  context.loadRequestIdRef.current += 1;
  context.activeSessionKeyRef.current = null;
  context.setSessionKey(null);
  resetSessionView(context);
}

/**
 * 重置会话并创建新的会话键。
 */
export function resetAgentSession(context: AgentSessionLifecycleContext): void {
  startAgentSession(context);
}
