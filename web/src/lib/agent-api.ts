/**
 * Conversation API 服务模块
 *
 * [INPUT]: 依赖 @/types/conversation, @/types/message, @/types/cost, @/types/api
 * [OUTPUT]: 对外提供 conversation CRUD、消息、成本等 API 函数
 * [POS]: lib 模块的 Conversation API 层
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import {
  ApiConversation,
  Conversation,
  CreateConversationParams,
  UpdateConversationParams,
} from '@/types/conversation';
import { Message as ChatMessage } from '@/types/message';
import { ConversationCostSummary } from '@/types/cost';
import { getAgentApiBaseUrl } from '@/config/options';
import { request_api } from '@/lib/http';
import { assertStructuredSessionKey } from '@/lib/session-key';

const AGENT_API_BASE_URL = getAgentApiBaseUrl();

// ==================== 类型转换 ====================

/** 将 API 响应转换为前端标准格式 */
export function transformApiConversation(api: ApiConversation): Conversation {
  return {
    session_key: api.session_key,
    agent_id: api.agent_id,
    session_id: api.session_id,
    room_session_id: api.room_session_id ?? null,
    room_id: api.room_id ?? null,
    conversation_id: api.conversation_id ?? null,
    title: api.title || '未命名会话',
    options: api.options || {},
    created_at: new Date(api.created_at).getTime(),
    last_activity_at: new Date(api.last_activity).getTime(),
    is_active: api.is_active,
    message_count: api.message_count,
  };
}

// ==================== 对话 API ====================

export const getConversations = async (): Promise<Conversation[]> => {
  const result = await request_api<ApiConversation[]>(`${AGENT_API_BASE_URL}/sessions`, {
    method: 'GET',
  });
  return result.map(transformApiConversation);
};

export const getConversationMessages = async (session_key: string): Promise<ChatMessage[]> => {
  const normalized_session_key = assertStructuredSessionKey(session_key);
  return request_api<ChatMessage[]>(`${AGENT_API_BASE_URL}/sessions/${normalized_session_key}/messages`, {
    method: 'GET',
  });
};

export const getConversationCostSummary = async (session_key: string): Promise<ConversationCostSummary> => {
  const normalized_session_key = assertStructuredSessionKey(session_key);
  return request_api<ConversationCostSummary>(`${AGENT_API_BASE_URL}/sessions/${normalized_session_key}/cost/summary`, {
    method: 'GET',
  });
};

export const deleteConversation = async (session_key: string): Promise<{ success: boolean }> => {
  const normalized_session_key = assertStructuredSessionKey(session_key);
  return request_api<{ success: boolean }>(`${AGENT_API_BASE_URL}/sessions/${normalized_session_key}`, {
    method: 'DELETE',
  });
};

export const createConversation = async (
  session_key: string,
  params: CreateConversationParams,
): Promise<Conversation> => {
  const normalized_session_key = assertStructuredSessionKey(session_key);
  const result = await request_api<ApiConversation>(`${AGENT_API_BASE_URL}/sessions`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      session_key: normalized_session_key,
      agent_id: params.agent_id,
      title: params.title,
    }),
  });
  return transformApiConversation(result);
};

export const updateConversation = async (
  session_key: string,
  params: UpdateConversationParams,
): Promise<Conversation> => {
  const normalized_session_key = assertStructuredSessionKey(session_key);
  const result = await request_api<ApiConversation>(`${AGENT_API_BASE_URL}/sessions/${normalized_session_key}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      title: params.title,
    }),
  });
  return transformApiConversation(result);
};
