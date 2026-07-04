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
} from "@/types/conversation/conversation";
import {
  ApiAgentSession as ApiAgentSessionRecord,
  AgentSession as AgentSessionRecord,
} from "@/types/agent/agent";
import type {
  ApiRoomConversationMessagePage,
  ApiSessionRoundIndex,
  RoomConversationMessagePage,
  SessionRoundIndexItem,
} from "@/types/conversation/room";
import { getAgentApiBaseUrl } from "@/config/options";
import { requestApi } from "@/lib/api/http";
import { toTimestamp } from "@/lib/api/timestamp-utils";
import { assertStructuredSessionKey } from "@/lib/conversation/session-key";

const AGENT_API_BASE_URL = getAgentApiBaseUrl();

// ==================== 类型转换 ====================

/** 将 API 响应转换为前端标准格式 */
function transformApiConversation(api: ApiConversation): Conversation {
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

function transformApiAgentSession(
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
    created_at: toTimestamp(api.created_at),
    last_activity_at: toTimestamp(api.last_activity),
    title: api.title || "未命名会话",
    message_count: api.message_count,
    options: api.options || {},
  };
}

// ==================== 对话 API ====================

export const getConversations = async (): Promise<Conversation[]> => {
  const result = await requestApi<ApiConversation[]>(
    `${AGENT_API_BASE_URL}/sessions`,
    {
      method: "GET",
    },
  );
  return result.map(transformApiConversation);
};

export const getAgentSessionsApi = async (
  agentId: string,
): Promise<AgentSessionRecord[]> => {
  const result = await requestApi<ApiAgentSessionRecord[]>(
    `${AGENT_API_BASE_URL}/agents/${encodeURIComponent(agentId)}/sessions`,
    {
      method: "GET",
    },
  );
  return result.map(transformApiAgentSession);
};

export async function getSessionMessagesApi(
  sessionKey: string,
  options: {
    limit?: number;
    before_round_id?: string | null;
    before_round_timestamp?: number | null;
    around_round_id?: string | null;
    around_limit?: number | null;
  } = {},
): Promise<RoomConversationMessagePage> {
  const normalizedSessionKey = assertStructuredSessionKey(sessionKey);
  const params = new URLSearchParams();
  params.set("session_key", normalizedSessionKey);
  if (options.limit && options.limit > 0) {
    params.set("limit", String(options.limit));
  }
  if (options.before_round_id) {
    params.set("before_round_id", options.before_round_id);
  }
  if (options.before_round_timestamp && options.before_round_timestamp > 0) {
    params.set("before_round_timestamp", String(options.before_round_timestamp));
  }
  if (options.around_round_id) {
    params.set("around_round_id", options.around_round_id);
  }
  if (options.around_limit && options.around_limit > 0) {
    params.set("around_limit", String(options.around_limit));
  }
  const query = params.toString();
  const result = await requestApi<ApiRoomConversationMessagePage>(
    `${AGENT_API_BASE_URL}/sessions/messages?${query}`,
    {
      method: "GET",
    },
  );
  return {
    items: result.items ?? [],
    has_more: result.has_more ?? false,
    next_before_round_id: result.next_before_round_id ?? null,
    next_before_round_timestamp: result.next_before_round_timestamp ?? null,
  };
}

export async function getSessionRoundIndexApi(
  sessionKey: string,
): Promise<SessionRoundIndexItem[]> {
  const normalizedSessionKey = assertStructuredSessionKey(sessionKey);
  const params = new URLSearchParams();
  params.set("session_key", normalizedSessionKey);
  const result = await requestApi<ApiSessionRoundIndex>(
    `${AGENT_API_BASE_URL}/sessions/rounds?${params.toString()}`,
    {
      method: "GET",
    },
  );
  return (result.items ?? [])
    .filter((item) => item.round_id.trim() !== "")
    .map((item) => ({
      roundId: item.round_id,
      title: item.title?.trim() || "",
      timestamp: item.timestamp && item.timestamp > 0 ? item.timestamp : null,
      status: item.status?.trim() || null,
      durationMs: item.duration_ms && item.duration_ms > 0 ? item.duration_ms : null,
      isLive: item.is_live ?? false,
    }));
}
