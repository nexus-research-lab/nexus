import { getAgentApiBaseUrl } from "@/config/runtime-endpoints";
import { requestApi } from "@/lib/api/core/http";
import type {
  ApiConversationMessagePage,
  ConversationMessagePage,
  ConversationMessagesQuery,
} from "@/types/conversation/history";
import type {
  ApiRoomContextAggregate,
  RoomAggregate,
  RoomContextAggregate,
} from "@/types/conversation/room";

import {
  buildConversationMessagesQuerySuffix,
  normalizeConversationMessagePage,
} from "./message-page-model";
import { transformRoomContext } from "./room-api-model";

const AGENT_API_BASE_URL = getAgentApiBaseUrl();

export async function listRooms(limit = 50): Promise<RoomAggregate[]> {
  return requestApi<RoomAggregate[]>(
    `${AGENT_API_BASE_URL}/rooms?limit=${encodeURIComponent(String(limit))}`,
    { method: "GET" },
  );
}

export async function getRoomContexts(
  roomId: string,
): Promise<RoomContextAggregate[]> {
  const result = await requestApi<ApiRoomContextAggregate[]>(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(roomId)}/contexts`,
    { method: "GET" },
  );
  return result.map(transformRoomContext);
}

export async function getRoomConversationMessages(
  roomId: string,
  conversationId: string,
  options: ConversationMessagesQuery = {},
): Promise<ConversationMessagePage> {
  const result = await requestApi<ApiConversationMessagePage>(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(roomId)}/conversations/${encodeURIComponent(conversationId)}/messages${buildConversationMessagesQuerySuffix(options)}`,
    { method: "GET" },
  );
  return normalizeConversationMessagePage(result);
}
