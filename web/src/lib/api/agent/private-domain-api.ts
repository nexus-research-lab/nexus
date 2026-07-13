import { getAgentApiBaseUrl } from "@/config/runtime-endpoints";
import { requestApi } from "@/lib/api/core/http";
import {
  AgentPrivateEventPage,
  AgentPrivateThreadPage,
} from "@/types/agent/private-domain";

const AGENT_API_BASE_URL = getAgentApiBaseUrl();

export interface AgentPrivateDomainQuery {
  room_id?: string | null;
  conversation_id?: string | null;
  limit?: number;
  room_limit?: number;
}

function buildPrivateDomainQuery(options: AgentPrivateDomainQuery = {}) {
  const params = new URLSearchParams();
  if (options.room_id) {
    params.set("room_id", options.room_id);
  }
  if (options.conversation_id) {
    params.set("conversation_id", options.conversation_id);
  }
  if (options.limit && options.limit > 0) {
    params.set("limit", String(options.limit));
  }
  if (options.room_limit && options.room_limit > 0) {
    params.set("room_limit", String(options.room_limit));
  }
  const query = params.toString();
  return query ? `?${query}` : "";
}

export async function listAgentPrivateThreadsApi(
  agentId: string,
  options: AgentPrivateDomainQuery = {},
): Promise<AgentPrivateThreadPage> {
  return requestApi<AgentPrivateThreadPage>(
    `${AGENT_API_BASE_URL}/agents/${encodeURIComponent(agentId)}/private-domain/threads${buildPrivateDomainQuery(options)}`,
    {
      method: "GET",
    },
  );
}

export async function listAgentPrivateEventsApi(
  agentId: string,
  threadId: string,
  options: AgentPrivateDomainQuery = {},
): Promise<AgentPrivateEventPage> {
  return requestApi<AgentPrivateEventPage>(
    `${AGENT_API_BASE_URL}/agents/${encodeURIComponent(agentId)}/private-domain/threads/${encodeURIComponent(threadId)}/events${buildPrivateDomainQuery(options)}`,
    {
      method: "GET",
    },
  );
}
