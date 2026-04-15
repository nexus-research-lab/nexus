import { get_agent_api_base_url } from "@/config/options";
import { request_api } from "@/lib/http";
import {
  CreateRoomConversationParams,
  CreateRoomParams,
  RoomAggregate,
  RoomContextAggregate,
  UpdateRoomConversationParams,
  UpdateRoomParams,
} from "@/types/room";

const AGENT_API_BASE_URL = get_agent_api_base_url();
const ROOM_LIST_UPDATED_EVENT_NAME = "nexus:room-list-updated";

export function notify_room_list_updated() {
  if (typeof window === "undefined") {
    return;
  }

  // 中文注释：Room / DM 的列表数据目前被多个页面各自缓存。
  // 统一从 API 层发出变更事件，避免每个创建入口都手写 refresh。
  window.dispatchEvent(new CustomEvent(ROOM_LIST_UPDATED_EVENT_NAME));
}

export function subscribe_room_list_updates(listener: () => void): () => void {
  if (typeof window === "undefined") {
    return () => {};
  }

  const handle_update = () => {
    listener();
  };

  window.addEventListener(ROOM_LIST_UPDATED_EVENT_NAME, handle_update);
  return () => {
    window.removeEventListener(ROOM_LIST_UPDATED_EVENT_NAME, handle_update);
  };
}

function normalize_conversation_title(value: unknown): string | undefined {
  if (typeof value !== "string") {
    return undefined;
  }
  const normalized_title = value.trim();
  return normalized_title ? normalized_title : undefined;
}

export async function list_rooms(limit = 50): Promise<RoomAggregate[]> {
  return request_api<RoomAggregate[]>(
    `${AGENT_API_BASE_URL}/rooms?limit=${encodeURIComponent(String(limit))}`,
    {
      method: "GET",
    },
  );
}

export async function get_room(room_id: string): Promise<RoomAggregate> {
  return request_api<RoomAggregate>(`${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(room_id)}`, {
    method: "GET",
  });
}

export async function get_room_contexts(room_id: string): Promise<RoomContextAggregate[]> {
  return request_api<RoomContextAggregate[]>(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(room_id)}/contexts`,
    {
      method: "GET",
    },
  );
}

export async function create_room(params: CreateRoomParams): Promise<RoomContextAggregate> {
  const context = await request_api<RoomContextAggregate>(`${AGENT_API_BASE_URL}/rooms`, {
    method: "POST",
    headers: {"Content-Type": "application/json"},
    body: JSON.stringify({
      agent_ids: params.agent_ids,
      name: params.name,
      description: params.description ?? "",
      title: params.title,
      avatar: params.avatar ?? null,
    }),
  });
  notify_room_list_updated();
  return context;
}

export async function update_room(
  room_id: string,
  params: UpdateRoomParams,
): Promise<RoomContextAggregate> {
  const context = await request_api<RoomContextAggregate>(`${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(room_id)}`, {
    method: "PATCH",
    headers: {"Content-Type": "application/json"},
    body: JSON.stringify({
      name: params.name,
      description: params.description,
      title: params.title,
      avatar: params.avatar ?? null,
    }),
  });
  notify_room_list_updated();
  return context;
}

export async function create_room_conversation(
  room_id: string,
  params: CreateRoomConversationParams = {},
): Promise<RoomContextAggregate> {
  const normalized_title = normalize_conversation_title(params.title);
  const context = await request_api<RoomContextAggregate>(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(room_id)}/conversations`,
    {
      method: "POST",
      headers: {"Content-Type": "application/json"},
      body: JSON.stringify({
        title: normalized_title,
      }),
    },
  );
  notify_room_list_updated();
  return context;
}

export async function update_room_conversation(
  room_id: string,
  conversation_id: string,
  params: UpdateRoomConversationParams,
): Promise<RoomContextAggregate> {
  const context = await request_api<RoomContextAggregate>(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(room_id)}/conversations/${encodeURIComponent(conversation_id)}`,
    {
      method: "PATCH",
      headers: {"Content-Type": "application/json"},
      body: JSON.stringify({
        title: params.title,
      }),
    },
  );
  notify_room_list_updated();
  return context;
}

export async function delete_room_conversation(
  room_id: string,
  conversation_id: string,
): Promise<RoomContextAggregate> {
  const context = await request_api<RoomContextAggregate>(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(room_id)}/conversations/${encodeURIComponent(conversation_id)}`,
    {
      method: "DELETE",
    },
  );
  notify_room_list_updated();
  return context;
}

export async function add_room_member(
  room_id: string,
  agent_id: string,
): Promise<RoomContextAggregate> {
  const context = await request_api<RoomContextAggregate>(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(room_id)}/members`,
    {
      method: "POST",
      headers: {"Content-Type": "application/json"},
      body: JSON.stringify({
        agent_id,
      }),
    },
  );
  notify_room_list_updated();
  return context;
}

export async function remove_room_member(
  room_id: string,
  agent_id: string,
): Promise<RoomContextAggregate> {
  const context = await request_api<RoomContextAggregate>(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(room_id)}/members/${encodeURIComponent(agent_id)}`,
    {
      method: "DELETE",
    },
  );
  notify_room_list_updated();
  return context;
}

export async function delete_room(room_id: string): Promise<{success: boolean}> {
  const result = await request_api<{success: boolean}>(`${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(room_id)}`, {
    method: "DELETE",
  });
  notify_room_list_updated();
  return result;
}

export async function ensure_direct_room(agent_id: string): Promise<RoomContextAggregate> {
  const context = await request_api<RoomContextAggregate>(
    `${AGENT_API_BASE_URL}/rooms/dm/${encodeURIComponent(agent_id)}`,
    {
      method: "GET",
    },
  );
  notify_room_list_updated();
  return context;
}
