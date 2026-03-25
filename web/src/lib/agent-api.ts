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
import { ApiResponse } from '@/types/api';
import { getAgentApiBaseUrl } from '@/config/options';

const AGENT_API_BASE_URL = getAgentApiBaseUrl();

// ==================== 类型转换 ====================

/** 将 API 响应转换为前端标准格式 */
export function transformApiConversation(api: ApiConversation): Conversation {
  return {
    session_key: api.session_key,
    agent_id: api.agent_id,
    session_id: api.session_id,
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
  const response = await fetch(`${AGENT_API_BASE_URL}/sessions`, {
    method: 'GET',
    headers: { 'Content-Type': 'application/json' },
  });
  if (!response.ok) {
    throw new Error(`获取会话列表失败: ${response.statusText}`);
  }
  const result: ApiResponse<ApiConversation[]> = await response.json();
  return result.data.map(transformApiConversation);
};

export const getConversationMessages = async (session_key: string): Promise<ChatMessage[]> => {
  const response = await fetch(`${AGENT_API_BASE_URL}/sessions/${session_key}/messages`, {
    method: 'GET',
    headers: { 'Content-Type': 'application/json' },
  });
  if (!response.ok) {
    throw new Error(`获取会话消息失败: ${response.statusText}`);
  }
  const result: ApiResponse<ChatMessage[]> = await response.json();
  return result.data;
};

export const getConversationCostSummary = async (session_key: string): Promise<ConversationCostSummary> => {
  const response = await fetch(`${AGENT_API_BASE_URL}/sessions/${session_key}/cost/summary`, {
    method: 'GET',
    headers: { 'Content-Type': 'application/json' },
  });
  if (!response.ok) {
    throw new Error(`获取会话成本失败: ${response.statusText}`);
  }
  const result: ApiResponse<ConversationCostSummary> = await response.json();
  return result.data;
};

export const deleteConversation = async (session_key: string): Promise<{ success: boolean }> => {
  if (!session_key) {
    throw new Error('session_key 不能为空');
  }
  const response = await fetch(`${AGENT_API_BASE_URL}/sessions/${session_key}`, {
    method: 'DELETE',
    headers: { 'Content-Type': 'application/json' },
  });
  if (!response.ok) {
    throw new Error(`删除会话失败: ${response.statusText}`);
  }
  const result: ApiResponse<{ success: boolean }> = await response.json();
  return result.data;
};

export const deleteRound = async (session_key: string, roundId: string): Promise<{ success: boolean; deleted_count: number }> => {
  const response = await fetch(`${AGENT_API_BASE_URL}/sessions/${session_key}/rounds/${roundId}`, {
    method: 'DELETE',
    headers: { 'Content-Type': 'application/json' },
  });
  if (!response.ok) {
    throw new Error(`删除轮次失败: ${response.statusText}`);
  }
  const result: ApiResponse<{ success: boolean; deleted_count: number }> = await response.json();
  return result.data;
};

export const createConversation = async (
  session_key: string,
  params: CreateConversationParams,
): Promise<Conversation> => {
  const response = await fetch(`${AGENT_API_BASE_URL}/sessions`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      session_key: session_key,
      agent_id: params.agent_id,
      title: params.title,
    }),
  });
  if (!response.ok) {
    throw new Error(`创建会话失败: ${response.statusText}`);
  }
  const result: ApiResponse<ApiConversation> = await response.json();
  return transformApiConversation(result.data);
};

export const updateConversation = async (
  session_key: string,
  params: UpdateConversationParams,
): Promise<Conversation> => {
  const response = await fetch(`${AGENT_API_BASE_URL}/sessions/${session_key}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      title: params.title,
    }),
  });
  if (!response.ok) {
    throw new Error(`更新会话失败: ${response.statusText}`);
  }
  const result: ApiResponse<ApiConversation> = await response.json();
  return transformApiConversation(result.data);
};
