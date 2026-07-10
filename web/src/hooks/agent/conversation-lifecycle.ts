import type { Dispatch, RefObject, SetStateAction } from 'react';

import { getMessageHistoryRoundPageSize } from "@/config/options";
import { getSessionMessagesApi } from "@/lib/api/agent-api";
import { getRoomConversationMessages } from '@/lib/api/room-api';
import { buildRoomSharedSessionKey, buildSessionKey } from '@/lib/conversation/session-key';
import { generateUuid } from '@/lib/uuid';
import type { Message, RoomPendingAgentSlotState } from '@/types';
import type {
  AgentConversationIdentity,
  InputQueueItem,
} from '@/types/agent/agent-conversation';
import type { PendingPermission } from '@/types/conversation/permission';

import { mergeLoadedMessages, sortMessages } from './message-helpers';

/** 生命周期层只接收加载和切换会话所需的状态能力。 */
export interface AgentConversationLifecycleContext {
  active_session_key_ref: RefObject<string | null>;
  load_request_id_ref: RefObject<number>;
  identity: AgentConversationIdentity | null;
  set_session_key: Dispatch<SetStateAction<string | null>>;
  set_is_session_loading: Dispatch<SetStateAction<boolean>>;
  set_messages: Dispatch<SetStateAction<Message[]>>;
  set_pending_agent_slots: Dispatch<SetStateAction<RoomPendingAgentSlotState[]>>;
  set_input_queue_items: Dispatch<SetStateAction<InputQueueItem[]>>;
  set_pending_permissions: Dispatch<SetStateAction<PendingPermission[]>>;
  set_error: Dispatch<SetStateAction<string | null>>;
  bg_message_cache_ref: RefObject<Map<string, Message[]>>;
  restore_volatile_session_snapshot: (sessionKey: string) => boolean;
  on_session_messages_loaded: (
    messages: Message[],
    meta: {
      session_key: string;
      is_reload: boolean;
      has_more_history: boolean;
      next_before_round_id: string | null;
      next_before_round_timestamp: number | null;
    },
  ) => void;
}

/**
 * 重置当前会话视图状态。
 * preserveLoading=true 时保留 isLoading 态（重连 reload 场景下由后端 roundStatus / sessionStatus 控制）。
 */
function resetSessionView(
  context: AgentConversationLifecycleContext,
  nextError: string | null = null,
): void {
  context.set_messages([]);
  context.set_pending_agent_slots([]);
  context.set_input_queue_items([]);
  context.set_pending_permissions([]);
  context.set_error(nextError);
}

/**
 * 启动一个新的会话。
 */
export function startAgentSession(context: AgentConversationLifecycleContext): void {
  const chatType = context.identity?.chat_type ?? 'dm';
  const conversationId = context.identity?.conversation_id;
  const agentId = context.identity?.agent_id;
  const newSessionKey = (
    chatType === 'group' && conversationId
      ? buildRoomSharedSessionKey(conversationId)
      : buildSessionKey({
        channel: 'ws',
        chat_type: 'dm',
        ref: generateUuid(),
        agent_id: agentId,
      })
  );
  context.load_request_id_ref.current += 1;
  context.active_session_key_ref.current = newSessionKey;
  context.set_session_key(newSessionKey);
  context.set_is_session_loading(false);
  resetSessionView(context);
}

/**
 * 加载现有会话消息。
 * 如果 bgMessageCacheRef 中有该 session 的缓存消息，先用缓存预填充（避免 loading 闪烁）。
 * API 返回后用服务端数据覆盖，并清除 cache。
 * isReload=true 时只刷新消息快照，运行态由 hook 内的状态机继续维护。
 */
export async function loadAgentSession(
  sessionKey: string,
  context: AgentConversationLifecycleContext,
  isReload: boolean = false,
): Promise<void> {
  const requestId = context.load_request_id_ref.current + 1;
  context.load_request_id_ref.current = requestId;
  context.active_session_key_ref.current = sessionKey;
  context.set_session_key(sessionKey);
  if (!isReload) {
    context.set_is_session_loading(true);
  }

  // 同 session 重拉只刷新消息快照，不要顺手清空运行时状态，
  // 否则执行中的轮次会在前端闪断成“可输入”后再恢复。
  if (isReload) {
    context.set_error(null);
  } else {
    // API 返回前先展示后台收到的消息，避免会话切换时闪回空态。
    const cached = context.bg_message_cache_ref.current.get(sessionKey);
    if (cached && cached.length > 0) {
      context.set_messages(sortMessages(cached));
      context.set_pending_permissions([]);
      context.set_error(null);
    } else {
      resetSessionView(context);
    }
    context.restore_volatile_session_snapshot(sessionKey);
  }

  try {
    const data = context.identity?.room_id && context.identity?.conversation_id
      ? await getRoomConversationMessages(
        context.identity.room_id,
        context.identity.conversation_id,
        {
          limit: getMessageHistoryRoundPageSize(),
        },
      )
      : await getSessionMessagesApi(sessionKey, {
        limit: getMessageHistoryRoundPageSize(),
      });
    if (
      context.load_request_id_ref.current !== requestId ||
      context.active_session_key_ref.current !== sessionKey
    ) {
      return;
    }
    const sortedMessages = sortMessages(data.items ?? []);
    let mergedMessages = sortedMessages;
    context.set_messages((currentMessages) => {
      mergedMessages = mergeLoadedMessages(sortedMessages, currentMessages);
      return mergedMessages;
    });
    context.on_session_messages_loaded(mergedMessages, {
      session_key: sessionKey,
      is_reload: isReload,
      has_more_history: data.has_more ?? false,
      next_before_round_id: data.next_before_round_id ?? null,
      next_before_round_timestamp: data.next_before_round_timestamp ?? null,
    });
    // 服务端快照合并完成后，后台缓存已完成使命。
    context.bg_message_cache_ref.current.delete(sessionKey);
  } catch (err) {
    if (
      context.load_request_id_ref.current !== requestId ||
      context.active_session_key_ref.current !== sessionKey
    ) {
      return;
    }
    console.error('[loadSession] 加载 session 失败:', err);
    context.set_error(err instanceof Error ? err.message : 'Failed to load session');
  } finally {
    if (
      !isReload &&
      context.load_request_id_ref.current === requestId &&
      context.active_session_key_ref.current === sessionKey
    ) {
      context.set_is_session_loading(false);
    }
  }
}

/**
 * 清空当前会话选择。
 */
export function clearAgentSession(context: AgentConversationLifecycleContext): void {
  context.load_request_id_ref.current += 1;
  context.active_session_key_ref.current = null;
  context.set_session_key(null);
  context.set_is_session_loading(false);
  resetSessionView(context);
}

/**
 * 重置会话并创建新的会话键。
 */
export function resetAgentSession(context: AgentConversationLifecycleContext): void {
  startAgentSession(context);
}
