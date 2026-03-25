import { getAgentApiBaseUrl } from "@/config/options";
import { ApiResponse } from "@/types/api";
import {
  CreateRoomParams,
  RoomAggregate,
  RoomContextAggregate,
} from "@/types/room";

const AGENT_API_BASE_URL = getAgentApiBaseUrl();

export async function listRooms(limit = 50): Promise<RoomAggregate[]> {
  const response = await fetch(
    `${AGENT_API_BASE_URL}/rooms?limit=${encodeURIComponent(String(limit))}`,
    {
      method: "GET",
      headers: {"Content-Type": "application/json"},
    },
  );
  if (!response.ok) {
    throw new Error(`获取 room 列表失败: ${response.statusText}`);
  }
  const result: ApiResponse<RoomAggregate[]> = await response.json();
  return result.data;
}

export async function getRoom(room_id: string): Promise<RoomAggregate> {
  const response = await fetch(`${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(room_id)}`, {
    method: "GET",
    headers: {"Content-Type": "application/json"},
  });
  if (!response.ok) {
    throw new Error(`获取 room 失败: ${response.statusText}`);
  }
  const result: ApiResponse<RoomAggregate> = await response.json();
  return result.data;
}

export async function getRoomContexts(room_id: string): Promise<RoomContextAggregate[]> {
  const response = await fetch(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(room_id)}/contexts`,
    {
      method: "GET",
      headers: {"Content-Type": "application/json"},
    },
  );
  if (!response.ok) {
    throw new Error(`获取 room 上下文失败: ${response.statusText}`);
  }
  const result: ApiResponse<RoomContextAggregate[]> = await response.json();
  return result.data;
}

export async function createRoom(params: CreateRoomParams): Promise<RoomContextAggregate> {
  const response = await fetch(`${AGENT_API_BASE_URL}/rooms`, {
    method: "POST",
    headers: {"Content-Type": "application/json"},
    body: JSON.stringify({
      agent_ids: params.agent_ids,
      name: params.name,
      description: params.description ?? "",
      title: params.title,
    }),
  });
  if (!response.ok) {
    throw new Error(`创建 room 失败: ${response.statusText}`);
  }
  const result: ApiResponse<RoomContextAggregate> = await response.json();
  return result.data;
}

export async function ensureDirectRoom(agent_id: string): Promise<RoomContextAggregate> {
  const rooms = await listRooms(200);
  const matched_room = rooms.find((item) => {
    if (item.room.room_type !== "dm") {
      return false;
    }

    const agent_members = item.members.filter((member) => member.member_type === "agent");
    return (
      agent_members.length === 1 &&
      agent_members[0]?.member_agent_id === agent_id
    );
  });

  if (matched_room) {
    const contexts = await getRoomContexts(matched_room.room.id);
    if (contexts.length > 0) {
      return contexts[0];
    }
  }

  return createRoom({
    agent_ids: [agent_id],
  });
}
