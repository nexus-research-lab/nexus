/**
 * Conversation API 服务模块
 *
 * [INPUT]: 依赖 @/types/conversation/conversation, @/types/system/api
 * [OUTPUT]: 对外提供 conversation CRUD API 函数
 * [POS]: lib 模块的 Conversation API 层
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import {
  ApiConversation,
  Conversation,
  UpdateConversationParams,
} from "@/types/conversation/conversation";
import {
  ApiAgentSession as ApiAgentSessionRecord,
  AgentSession as AgentSessionRecord,
} from "@/types/agent/agent";
import { get_agent_api_base_url } from "@/config/options";
import { request_api } from "@/lib/api/http";
import { to_timestamp } from "@/lib/api/timestamp-utils";
import { assert_structured_session_key } from "@/lib/conversation/session-key";

const AGENT_API_BASE_URL = get_agent_api_base_url();

// ==================== 类型转换 ====================

/** 将 API 响应转换为前端标准格式 */
export function transform_api_conversation(api: ApiConversation): Conversation {
  return {
    session_key: api.session_key,
    agent_id: api.agent_id,
    session_id: api.session_id,
    room_session_id: api.room_session_id ?? null,
    room_id: api.room_id ?? null,
    conversation_id: api.conversation_id ?? null,
    title: api.title || "未命名会话",
    options: api.options || {},
    created_at: new Date(api.created_at).getTime(),
    last_activity_at: new Date(api.last_activity).getTime(),
    is_active: api.is_active,
    message_count: api.message_count,
  };
}

export function transform_api_agent_session(
  api: ApiAgentSessionRecord,
): AgentSessionRecord {
  return {
    session_key: api.session_key,
    agent_id: api.agent_id,
    session_id: api.session_id,
    room_session_id: api.room_session_id ?? null,
    room_id: api.room_id ?? null,
    conversation_id: api.conversation_id ?? null,
    channel_type: api.channel_type,
    chat_type: api.chat_type,
    status: api.status,
    created_at: to_timestamp(api.created_at),
    last_activity_at: to_timestamp(api.last_activity),
    title: api.title || "未命名会话",
    message_count: api.message_count,
    options: api.options || {},
  };
}

// ==================== 对话 API ====================

export const get_conversations = async (): Promise<Conversation[]> => {
  const result = await request_api<ApiConversation[]>(
    `${AGENT_API_BASE_URL}/sessions`,
    {
      method: "GET",
    },
  );
  return result.map(transform_api_conversation);
};

export const get_agent_sessions_api = async (
  agent_id: string,
): Promise<AgentSessionRecord[]> => {
  const result = await request_api<ApiAgentSessionRecord[]>(
    `${AGENT_API_BASE_URL}/agents/${encodeURIComponent(agent_id)}/sessions`,
    {
      method: "GET",
    },
  );
  return result.map(transform_api_agent_session);
};

export const delete_conversation = async (
  session_key: string,
): Promise<{ success: boolean }> => {
  const normalized_session_key = assert_structured_session_key(session_key);
  return request_api<{ success: boolean }>(
    `${AGENT_API_BASE_URL}/sessions/${normalized_session_key}`,
    {
      method: "DELETE",
    },
  );
};

export const update_conversation = async (
  session_key: string,
  params: UpdateConversationParams,
): Promise<Conversation> => {
  const normalized_session_key = assert_structured_session_key(session_key);
  const result = await request_api<ApiConversation>(
    `${AGENT_API_BASE_URL}/sessions/${normalized_session_key}`,
    {
      method: "PATCH",
      body: JSON.stringify({
        title: params.title,
      }),
    },
  );
  return transform_api_conversation(result);
};
