import { getConversationMessages } from '@/lib/agent-api';
import { buildRoomSharedSessionKey, buildWsDmSessionKey } from '@/lib/session-key';
import { generateUuid } from '@/lib/uuid';
import { AgentConversationLifecycleContext } from '@/types/agent-conversation';

import { sortMessages } from './message-helpers';

/**
 * 重置当前会话视图状态。
 */
export function resetSessionView(
  context: AgentConversationLifecycleContext,
  next_error: string | null = null,
): void {
  context.set_messages([]);
  context.set_pending_agent_slots([]);
  context.set_pending_permissions([]);
  context.set_is_loading(false);
  context.set_error(next_error);
}

/**
 * 启动一个新的会话。
 */
export function startAgentSession(context: AgentConversationLifecycleContext): void {
  const new_session_key = (
    context.chat_type === 'group' && context.conversation_id
      ? buildRoomSharedSessionKey(context.conversation_id)
      : buildWsDmSessionKey(generateUuid(), context.agent_id)
  );
  context.load_request_id_ref.current += 1;
  context.active_session_key_ref.current = new_session_key;
  context.set_session_key(new_session_key);
  resetSessionView(context);
}

/**
 * 加载现有会话消息。
 * 如果 bg_message_cache_ref 中有该 session 的缓存消息，先用缓存预填充（避免 loading 闪烁）。
 * API 返回后用服务端数据覆盖，并清除 cache。
 */
export async function loadAgentSession(
  session_key: string,
  context: AgentConversationLifecycleContext,
): Promise<void> {
  const request_id = context.load_request_id_ref.current + 1;
  context.load_request_id_ref.current = request_id;
  context.active_session_key_ref.current = session_key;
  context.set_session_key(session_key);

  // Pre-fill with cached background messages before the API round-trip
  const cached = context.bg_message_cache_ref?.current.get(session_key);
  if (cached && cached.length > 0) {
    context.set_messages(sortMessages(cached));
    context.set_pending_permissions([]);
    context.set_is_loading(false);
    context.set_error(null);
  } else {
    resetSessionView(context);
  }

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
    // Cache is now stale — clear it
    context.bg_message_cache_ref?.current.delete(session_key);
  } catch (err) {
    if (
      context.load_request_id_ref.current !== request_id ||
      context.active_session_key_ref.current !== session_key
    ) {
      return;
    }
    console.error('[loadSession] 加载 session 失败:', err);
    context.set_error(err instanceof Error ? err.message : 'Failed to load session');
  }
}

/**
 * 清空当前会话选择。
 */
export function clearAgentSession(context: AgentConversationLifecycleContext): void {
  context.load_request_id_ref.current += 1;
  context.active_session_key_ref.current = null;
  context.set_session_key(null);
  resetSessionView(context);
}

/**
 * 重置会话并创建新的会话键。
 */
export function resetAgentSession(context: AgentConversationLifecycleContext): void {
  startAgentSession(context);
}
