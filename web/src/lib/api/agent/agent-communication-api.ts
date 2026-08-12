/** Agent 通讯客户端的 owner 控制面 HTTP 边界。 */

import { getAgentApiBaseUrl } from "@/config/runtime-endpoints";
import { transformRoomContext } from "@/lib/api/conversation/room-api-model";
import { requestApi } from "@/lib/api/core/http";
import type { AgentCommunicationSendResult } from "@/types/agent/agent";
import type {
  ApiRoomContextAggregate,
  RoomContextAggregate,
} from "@/types/conversation/room";

const AGENT_API_BASE_URL = getAgentApiBaseUrl();

export async function openAgentContactChannelApi(
  agentId: string,
  contactAgentId: string,
): Promise<RoomContextAggregate> {
  const result = await requestApi<ApiRoomContextAggregate>(
    `${AGENT_API_BASE_URL}/agents/${encodeURIComponent(agentId)}/contacts/${encodeURIComponent(contactAgentId)}/channel`,
    { method: "POST" },
  );
  return transformRoomContext(result);
}

export function sendAgentCommunicationMessageApi(
  agentId: string,
  request: {
    content: string;
    conversation_id?: string;
    target_id: string;
    target_type: "agent" | "room";
  },
): Promise<AgentCommunicationSendResult> {
  return requestApi<AgentCommunicationSendResult>(
    `${AGENT_API_BASE_URL}/agents/${encodeURIComponent(agentId)}/communications/messages`,
    {
      body: JSON.stringify(request),
      method: "POST",
    },
  );
}
