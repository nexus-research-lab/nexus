import { get_conversation_messages } from '@/lib/api/agent-api';
import { build_room_shared_session_key, build_ws_dm_session_key } from '@/lib/conversation/session-key';
import { generate_uuid } from '@/lib/uuid';
import { AgentConversationLifecycleContext } from '@/types/agent/agent-conversation';

import { merge_loaded_messages, sort_messages } from './message-helpers';

/**
 * 重置当前会话视图状态。
 * preserve_loading=true 时保留 is_loading 态（重连 reload 场景下由后端 round_status / session_status 控制）。
 */
export function reset_session_view(
  context: AgentConversationLifecycleContext,
  next_error: string | null = null,
): void {
  context.set_messages([]);
  context.set_pending_agent_slots([]);
  context.set_pending_permissions([]);
  context.set_error(next_error);
}

/**
 * 启动一个新的会话。
 */
export function start_agent_session(context: AgentConversationLifecycleContext): void {
  const chat_type = context.identity?.chat_type ?? 'dm';
  const conversation_id = context.identity?.conversation_id;
  const agent_id = context.identity?.agent_id;
  const new_session_key = (
    chat_type === 'group' && conversation_id
      ? build_room_shared_session_key(conversation_id)
      : build_ws_dm_session_key(generate_uuid(), agent_id)
  );
  context.load_request_id_ref.current += 1;
  context.active_session_key_ref.current = new_session_key;
  context.set_session_key(new_session_key);
  context.set_is_session_loading(false);
  reset_session_view(context);
}

/**
 * 加载现有会话消息。
 * 如果 bg_message_cache_ref 中有该 session 的缓存消息，先用缓存预填充（避免 loading 闪烁）。
 * API 返回后用服务端数据覆盖，并清除 cache。
 * is_reload=true 时只刷新消息快照，运行态由 hook 内的状态机继续维护。
 */
export async function load_agent_session(
  session_key: string,
  context: AgentConversationLifecycleContext,
  is_reload: boolean = false,
): Promise<void> {
  const request_id = context.load_request_id_ref.current + 1;
  context.load_request_id_ref.current = request_id;
  context.active_session_key_ref.current = session_key;
  context.set_session_key(session_key);
  if (!is_reload) {
    context.set_is_session_loading(true);
  }

  // 同 session 重拉只刷新消息快照，不要顺手清空运行时状态，
  // 否则执行中的轮次会在前端闪断成“可输入”后再恢复。
  if (is_reload) {
    context.set_error(null);
  } else {
    // Pre-fill with cached background messages before the API round-trip
    const cached = context.bg_message_cache_ref?.current.get(session_key);
    if (cached && cached.length > 0) {
      context.set_messages(sort_messages(cached));
      context.set_pending_permissions([]);
      context.set_error(null);
    } else {
      reset_session_view(context);
    }
  }

  try {
    const data = await get_conversation_messages(session_key);
    if (
      context.load_request_id_ref.current !== request_id ||
      context.active_session_key_ref.current !== session_key
    ) {
      return;
    }
    if (Array.isArray(data)) {
      const sorted_messages = sort_messages(data);
      let merged_messages = sorted_messages;
      context.set_messages((current_messages) => {
        merged_messages = merge_loaded_messages(sorted_messages, current_messages);
        return merged_messages;
      });
      context.on_session_messages_loaded?.(merged_messages, {
        session_key,
        is_reload,
      });
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
  } finally {
    if (
      !is_reload &&
      context.load_request_id_ref.current === request_id &&
      context.active_session_key_ref.current === session_key
    ) {
      context.set_is_session_loading(false);
    }
  }
}

/**
 * 清空当前会话选择。
 */
export function clear_agent_session(context: AgentConversationLifecycleContext): void {
  context.load_request_id_ref.current += 1;
  context.active_session_key_ref.current = null;
  context.set_session_key(null);
  context.set_is_session_loading(false);
  reset_session_view(context);
}

/**
 * 重置会话并创建新的会话键。
 */
export function reset_agent_session(context: AgentConversationLifecycleContext): void {
  start_agent_session(context);
}
