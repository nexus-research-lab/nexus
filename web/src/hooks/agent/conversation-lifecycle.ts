import { getConversationMessages } from '@/lib/agent-api';
import { generateUuid } from '@/lib/uuid';
import { AgentConversationLifecycleContext } from '@/types/agent-conversation';

import { sortMessages } from './message-helpers';

/**
 * 重置当前对话视图状态。
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
 * 启动一个新的对话。
 */
export function startAgentConversation(context: AgentConversationLifecycleContext): void {
  const new_conversation_key = generateUuid();
  context.load_request_id_ref.current += 1;
  context.active_conversation_key_ref.current = new_conversation_key;
  context.set_conversation_key(new_conversation_key);
  resetConversationView(context);
}

/**
 * 加载现有对话消息。
 * 如果 bg_message_cache_ref 中有该 session 的缓存消息，先用缓存预填充（避免 loading 闪烁）。
 * API 返回后用服务端数据覆盖，并清除 cache。
 */
export async function loadAgentConversation(
  session_key: string,
  context: AgentConversationLifecycleContext,
): Promise<void> {
  const request_id = context.load_request_id_ref.current + 1;
  context.load_request_id_ref.current = request_id;
  context.active_conversation_key_ref.current = session_key;
  context.set_conversation_key(session_key);

  // Pre-fill with cached background messages before the API round-trip
  const cached = context.bg_message_cache_ref?.current.get(session_key);
  if (cached && cached.length > 0) {
    context.set_messages(sortMessages(cached));
    context.set_pending_permission(null);
    context.set_is_loading(false);
    context.set_error(null);
  } else {
    resetConversationView(context);
  }

  try {
    const data = await getConversationMessages(session_key);
    if (
      context.load_request_id_ref.current !== request_id ||
      context.active_conversation_key_ref.current !== session_key
    ) {
      return;
    }
    if (Array.isArray(data)) {
      context.set_messages(sortMessages(data));
    }
    // Cache is now stale — clear it
    context.bg_message_cache_ref?.current.delete(session_key);
  } catch (err) {
    if (
      context.load_request_id_ref.current !== request_id ||
      context.active_conversation_key_ref.current !== session_key
    ) {
      return;
    }
    console.error('[loadConversation] 加载 conversation 失败:', err);
    context.set_error(err instanceof Error ? err.message : 'Failed to load conversation');
  }
}

/**
 * 清空当前对话选择。
 */
export function clearAgentConversation(context: AgentConversationLifecycleContext): void {
  context.load_request_id_ref.current += 1;
  context.active_conversation_key_ref.current = null;
  context.set_conversation_key(null);
  resetConversationView(context);
}

/**
 * 重置对话并创建新的对话键。
 */
export function resetAgentConversation(context: AgentConversationLifecycleContext): void {
  startAgentConversation(context);
}
