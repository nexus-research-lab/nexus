import { getAgentApiBaseUrl } from "@/config/options";
import { requestApi } from "@/lib/api/core/http";
import type {
  ApiRoomContextAggregate,
  ApiRoomConversationMessagePage,
  RoomAggregate,
  RoomContextAggregate,
  RoomConversationMessagePage,
} from "@/types/conversation/room";

import {
  buildRoomMessagesQuery,
  type RoomMessagesQuery,
  transformRoomContext,
} from "./room-api-model";

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
  options: RoomMessagesQuery = {},
): Promise<RoomConversationMessagePage> {
  const result = await requestApi<ApiRoomConversationMessagePage>(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(roomId)}/conversations/${encodeURIComponent(conversationId)}/messages${buildRoomMessagesQuery(options)}`,
    { method: "GET" },
  );
  return {
    has_more: result.has_more ?? false,
    items: result.items ?? [],
    next_before_round_id: result.next_before_round_id ?? null,
    next_before_round_timestamp: result.next_before_round_timestamp ?? null,
  };
}
