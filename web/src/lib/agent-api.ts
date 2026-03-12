/**
 * Session API 服务模块
 *
 * [INPUT]: 依赖 @/types/session, @/types/message, @/types/cost, @/types/api
 * [OUTPUT]: 对外提供 getSessions、createSession、updateSession、deleteSession 等 API 函数
 * [POS]: lib 模块的 Session API 层
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import { ApiSession, CreateSessionParams, Session, UpdateSessionParams } from '@/types/session';
import { Message as ChatMessage } from '@/types/message';
import { SessionCostSummary } from '@/types/cost';
import { ApiResponse } from '@/types/api';
import { getAgentApiBaseUrl } from '@/lib/runtime-config';

const AGENT_API_BASE_URL = getAgentApiBaseUrl();

// ==================== 类型转换 ====================

/** 将 API 响应转换为前端标准格式 */
export function transformApiSession(api: ApiSession): Session {
  return {
    session_key: api.session_key,
    agent_id: api.agent_id,
    session_id: api.session_id,
    title: api.title || '未命名会话',
    options: api.options || {},
    created_at: new Date(api.created_at).getTime(),
    last_activity_at: new Date(api.last_activity).getTime(),
    is_active: api.is_active,
    message_count: api.message_count,
  };
}

// ==================== 会话 API ====================

export const getSessions = async (): Promise<Session[]> => {
  const response = await fetch(`${AGENT_API_BASE_URL}/sessions`, {
    method: 'GET',
    headers: { 'Content-Type': 'application/json' },
  });
  if (!response.ok) {
    throw new Error(`获取会话列表失败: ${response.statusText}`);
  }
  const result: ApiResponse<ApiSession[]> = await response.json();
  return result.data.map(transformApiSession);
};

export const getSessionMessages = async (session_key: string): Promise<ChatMessage[]> => {
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

export const getSessionCostSummary = async (session_key: string): Promise<SessionCostSummary> => {
  const response = await fetch(`${AGENT_API_BASE_URL}/sessions/${session_key}/cost/summary`, {
    method: 'GET',
    headers: { 'Content-Type': 'application/json' },
  });
  if (!response.ok) {
    throw new Error(`获取会话成本失败: ${response.statusText}`);
  }
  const result: ApiResponse<SessionCostSummary> = await response.json();
  return result.data;
};

export const deleteSession = async (session_key: string): Promise<{ success: boolean }> => {
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

export const createSession = async (session_key: string, params: CreateSessionParams): Promise<Session> => {
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
  const result: ApiResponse<ApiSession> = await response.json();
  return transformApiSession(result.data);
};

export const updateSession = async (session_key: string, params: UpdateSessionParams): Promise<Session> => {
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
  const result: ApiResponse<ApiSession> = await response.json();
  return transformApiSession(result.data);
};
