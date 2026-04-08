import { getAgentApiBaseUrl } from "@/config/options";
import { request_api } from "@/lib/http";
import {
  CreateRoomConversationParams,
  CreateRoomParams,
  RoomAggregate,
  RoomContextAggregate,
  UpdateRoomConversationParams,
  UpdateRoomParams,
} from "@/types/room";

const AGENT_API_BASE_URL = getAgentApiBaseUrl();

function normalizeConversationTitle(value: unknown): string | undefined {
  if (typeof value !== "string") {
    return undefined;
  }
  const normalized_title = value.trim();
  return normalized_title ? normalized_title : undefined;
}

export async function listRooms(limit = 50): Promise<RoomAggregate[]> {
  return request_api<RoomAggregate[]>(
    `${AGENT_API_BASE_URL}/rooms?limit=${encodeURIComponent(String(limit))}`,
    {
      method: "GET",
    },
  );
}

export async function getRoom(room_id: string): Promise<RoomAggregate> {
  return request_api<RoomAggregate>(`${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(room_id)}`, {
    method: "GET",
  });
}

export async function getRoomContexts(room_id: string): Promise<RoomContextAggregate[]> {
  return request_api<RoomContextAggregate[]>(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(room_id)}/contexts`,
    {
      method: "GET",
    },
  );
}

export async function createRoom(params: CreateRoomParams): Promise<RoomContextAggregate> {
  return request_api<RoomContextAggregate>(`${AGENT_API_BASE_URL}/rooms`, {
    method: "POST",
    headers: {"Content-Type": "application/json"},
    body: JSON.stringify({
      agent_ids: params.agent_ids,
      name: params.name,
      description: params.description ?? "",
      title: params.title,
    }),
  });
}

export async function updateRoom(
  room_id: string,
  params: UpdateRoomParams,
): Promise<RoomContextAggregate> {
  return request_api<RoomContextAggregate>(`${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(room_id)}`, {
    method: "PATCH",
    headers: {"Content-Type": "application/json"},
    body: JSON.stringify({
      name: params.name,
      description: params.description,
      title: params.title,
    }),
  });
}

export async function createRoomConversation(
  room_id: string,
  params: CreateRoomConversationParams = {},
): Promise<RoomContextAggregate> {
  const normalized_title = normalizeConversationTitle(params.title);
  return request_api<RoomContextAggregate>(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(room_id)}/conversations`,
    {
      method: "POST",
      headers: {"Content-Type": "application/json"},
      body: JSON.stringify({
        title: normalized_title,
      }),
    },
  );
}

export async function updateRoomConversation(
  room_id: string,
  conversation_id: string,
  params: UpdateRoomConversationParams,
): Promise<RoomContextAggregate> {
  return request_api<RoomContextAggregate>(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(room_id)}/conversations/${encodeURIComponent(conversation_id)}`,
    {
      method: "PATCH",
      headers: {"Content-Type": "application/json"},
      body: JSON.stringify({
        title: params.title,
      }),
    },
  );
}

export async function deleteRoomConversation(
  room_id: string,
  conversation_id: string,
): Promise<RoomContextAggregate> {
  return request_api<RoomContextAggregate>(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(room_id)}/conversations/${encodeURIComponent(conversation_id)}`,
    {
      method: "DELETE",
    },
  );
}

export async function addRoomMember(
  room_id: string,
  agent_id: string,
): Promise<RoomContextAggregate> {
  return request_api<RoomContextAggregate>(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(room_id)}/members`,
    {
      method: "POST",
      headers: {"Content-Type": "application/json"},
      body: JSON.stringify({
        agent_id,
      }),
    },
  );
}

export async function removeRoomMember(
  room_id: string,
  agent_id: string,
): Promise<RoomContextAggregate> {
  return request_api<RoomContextAggregate>(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(room_id)}/members/${encodeURIComponent(agent_id)}`,
    {
      method: "DELETE",
    },
  );
}

export async function deleteRoom(room_id: string): Promise<{success: boolean}> {
  return request_api<{success: boolean}>(`${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(room_id)}`, {
    method: "DELETE",
  });
}

export async function ensureDirectRoom(agent_id: string): Promise<RoomContextAggregate> {
  return request_api<RoomContextAggregate>(
    `${AGENT_API_BASE_URL}/rooms/dm/${encodeURIComponent(agent_id)}`,
    {
      method: "GET",
    },
  );
}
