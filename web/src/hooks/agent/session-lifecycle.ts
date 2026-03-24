import { getConversationMessages } from '@/lib/agent-api';
import { generateUuid } from '@/lib/uuid';
import { AgentConversationLifecycleContext } from '@/types/agent-conversation';

import { sortMessages } from './message-helpers';

/**
 * 重置当前会话视图状态。
 */
export function resetConversationView(
  context: AgentConversationLifecycleContext,
  next_error: string | null = null,
): void {
  context.set_messages([]);
  context.set_pending_permission(null);
  context.set_is_loading(false);
  context.set_error(next_error);
}

/**
 * 启动一个新的会话。
 */
export function startAgentConversation(context: AgentConversationLifecycleContext): void {
  const new_session_key = generateUuid();
  context.load_request_id_ref.current += 1;
  context.active_session_key_ref.current = new_session_key;
  context.set_session_key(new_session_key);
  resetConversationView(context);
}

/**
 * 加载现有会话消息。
 */
export async function loadAgentConversation(
  session_key: string,
  context: AgentConversationLifecycleContext,
): Promise<void> {
  const request_id = context.load_request_id_ref.current + 1;
  context.load_request_id_ref.current = request_id;
  context.active_session_key_ref.current = session_key;
  context.set_session_key(session_key);
  resetConversationView(context);

  try {
    const data = await getConversationMessages(session_key);
    if (
      context.load_request_id_ref.current !== request_id ||
      context.active_session_key_ref.current !== session_key
    ) {
      return;
    }
    if (Array.isArray(data)) {
      context.set_messages(sortMessages(data));
    }
  } catch (err) {
    if (
      context.load_request_id_ref.current !== request_id ||
      context.active_session_key_ref.current !== session_key
    ) {
      return;
    }
    console.error('[loadConversation] 加载 conversation 失败:', err);
    context.set_error(err instanceof Error ? err.message : 'Failed to load session');
  }
}

/**
 * 清空当前会话选择。
 */
export function clearAgentConversation(context: AgentConversationLifecycleContext): void {
  context.load_request_id_ref.current += 1;
  context.active_session_key_ref.current = null;
  context.set_session_key(null);
  resetConversationView(context);
}

/**
 * 重置会话并创建新的会话键。
 */
export function resetAgentConversation(context: AgentConversationLifecycleContext): void {
  startAgentConversation(context);
}
