import { get_agent_api_base_url } from "@/config/options";
import { request_api } from "@/lib/api/http";
import {
  AgentPrivateEventPage,
  AgentPrivateThreadPage,
} from "@/types/agent/private-domain";

const AGENT_API_BASE_URL = get_agent_api_base_url();

export interface AgentPrivateDomainQuery {
  room_id?: string | null;
  conversation_id?: string | null;
  limit?: number;
  room_limit?: number;
}

function build_private_domain_query(options: AgentPrivateDomainQuery = {}) {
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

export async function list_agent_private_threads_api(
  agent_id: string,
  options: AgentPrivateDomainQuery = {},
): Promise<AgentPrivateThreadPage> {
  return request_api<AgentPrivateThreadPage>(
    `${AGENT_API_BASE_URL}/agents/${encodeURIComponent(agent_id)}/private-domain/threads${build_private_domain_query(options)}`,
    {
      method: "GET",
    },
  );
}

export async function list_agent_private_events_api(
  agent_id: string,
  thread_id: string,
  options: AgentPrivateDomainQuery = {},
): Promise<AgentPrivateEventPage> {
  return request_api<AgentPrivateEventPage>(
    `${AGENT_API_BASE_URL}/agents/${encodeURIComponent(agent_id)}/private-domain/threads/${encodeURIComponent(thread_id)}/events${build_private_domain_query(options)}`,
    {
      method: "GET",
    },
  );
}
