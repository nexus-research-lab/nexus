/** 私聊 Session、消息历史和轮次索引的 HTTP 边界。 */

import type {
  ApiConversation,
  Conversation,
} from "@/types/conversation/conversation";
import type {
  ApiAgentSession as ApiAgentSessionRecord,
  AgentSession as AgentSessionRecord,
} from "@/types/agent/agent";
import type {
  ApiConversationMessagePage,
  ApiSessionRoundIndex,
  ConversationMessagePage,
  ConversationMessagesQuery,
  SessionRoundIndexItem,
} from "@/types/conversation/history";
import { getAgentApiBaseUrl } from "@/config/runtime-endpoints";
import { requestApi } from "@/lib/api/core/http";
import { assertStructuredSessionKey } from "@/lib/conversation/session-key";

import {
  buildConversationMessagesQuerySuffix,
  normalizeConversationMessagePage,
} from "./message-page-model";
import {
  transformApiAgentSession,
  transformApiConversation,
  transformApiSessionRoundIndex,
} from "./session-api-model";

const AGENT_API_BASE_URL = getAgentApiBaseUrl();

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
  options: ConversationMessagesQuery = {},
): Promise<ConversationMessagePage> {
  const normalizedSessionKey = assertStructuredSessionKey(sessionKey);
  const querySuffix = buildConversationMessagesQuerySuffix(options, [
    ["session_key", normalizedSessionKey],
  ]);
  const result = await requestApi<ApiConversationMessagePage>(
    `${AGENT_API_BASE_URL}/sessions/messages${querySuffix}`,
    {
      method: "GET",
    },
  );
  return normalizeConversationMessagePage(result);
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
  return transformApiSessionRoundIndex(result);
}
