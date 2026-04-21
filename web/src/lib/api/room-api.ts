import { get_agent_api_base_url } from "@/config/options";
import { request_api } from "@/lib/api/http";
import {
  ApiRoomContextAggregate,
  ApiRoomConversationMessagePage,
  CreateRoomConversationParams,
  CreateRoomParams,
  RoomAggregate,
  RoomContextAggregate,
  RoomConversationMessagePage,
  UpdateRoomConversationParams,
  UpdateRoomParams,
} from "@/types/conversation/room";
import { Agent, ApiAgent } from "@/types/agent/agent";

const AGENT_API_BASE_URL = get_agent_api_base_url();
const ROOM_DIRECTORY_UPDATED_EVENT_NAME = "nexus:room-directory-updated";

export function notify_room_directory_updated() {
  if (typeof window === "undefined") {
    return;
  }

  // Room / DM 的列表数据目前被多个页面各自缓存。
  // 统一从 API 层发出变更事件，避免每个创建入口都手写 refresh。
  window.dispatchEvent(new CustomEvent(ROOM_DIRECTORY_UPDATED_EVENT_NAME));
}

export function subscribe_room_directory_updates(listener: () => void): () => void {
  if (typeof window === "undefined") {
    return () => {};
  }

  const handle_update = () => {
    listener();
  };

  window.addEventListener(ROOM_DIRECTORY_UPDATED_EVENT_NAME, handle_update);
  return () => {
    window.removeEventListener(ROOM_DIRECTORY_UPDATED_EVENT_NAME, handle_update);
  };
}

function normalize_conversation_title(value: unknown): string | undefined {
  if (typeof value !== "string") {
    return undefined;
  }
  const normalized_title = value.trim();
  return normalized_title ? normalized_title : undefined;
}

function transform_api_agent(api_agent: ApiAgent): Agent {
  return {
    agent_id: api_agent.agent_id,
    name: api_agent.name,
    workspace_path: api_agent.workspace_path,
    display_name: api_agent.display_name ?? null,
    headline: api_agent.headline ?? null,
    profile_markdown: api_agent.profile_markdown ?? null,
    options: api_agent.options || {},
    created_at: new Date(api_agent.created_at).getTime(),
    status: api_agent.status,
    avatar: api_agent.avatar ?? null,
    description: api_agent.description ?? null,
    vibe_tags: api_agent.vibe_tags ?? [],
    skills_count: api_agent.skills_count ?? null,
  };
}

function transform_room_context(api_context: ApiRoomContextAggregate): RoomContextAggregate {
  return {
    room: api_context.room,
    members: api_context.members,
    member_agents: (api_context.member_agents ?? []).map(transform_api_agent),
    conversation: api_context.conversation,
    sessions: api_context.sessions,
  };
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
  const result = await request_api<ApiRoomContextAggregate[]>(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(room_id)}/contexts`,
    {
      method: "GET",
    },
  );
  return result.map(transform_room_context);
}

export async function get_room_conversation_messages(
  room_id: string,
  conversation_id: string,
  options: {
    limit?: number;
    before_round_id?: string | null;
    before_round_timestamp?: number | null;
  } = {},
): Promise<RoomConversationMessagePage> {
  const params = new URLSearchParams();
  if (options.limit && options.limit > 0) {
    params.set("limit", String(options.limit));
  }
  if (options.before_round_id) {
    params.set("before_round_id", options.before_round_id);
  }
  if (options.before_round_timestamp && options.before_round_timestamp > 0) {
    params.set("before_round_timestamp", String(options.before_round_timestamp));
  }
  const query = params.toString();
  const result = await request_api<ApiRoomConversationMessagePage>(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(room_id)}/conversations/${encodeURIComponent(conversation_id)}/messages${query ? `?${query}` : ""}`,
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

export async function create_room(params: CreateRoomParams): Promise<RoomContextAggregate> {
  const context = await request_api<ApiRoomContextAggregate>(`${AGENT_API_BASE_URL}/rooms`, {
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
  notify_room_directory_updated();
  return transform_room_context(context);
}

export async function update_room(
  room_id: string,
  params: UpdateRoomParams,
): Promise<RoomContextAggregate> {
  const context = await request_api<ApiRoomContextAggregate>(`${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(room_id)}`, {
    method: "PATCH",
    headers: {"Content-Type": "application/json"},
    body: JSON.stringify({
      name: params.name,
      description: params.description,
      title: params.title,
      avatar: params.avatar ?? null,
    }),
  });
  notify_room_directory_updated();
  return transform_room_context(context);
}

export async function create_room_conversation(
  room_id: string,
  params: CreateRoomConversationParams = {},
): Promise<RoomContextAggregate> {
  const normalized_title = normalize_conversation_title(params.title);
  const context = await request_api<ApiRoomContextAggregate>(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(room_id)}/conversations`,
    {
      method: "POST",
      headers: {"Content-Type": "application/json"},
      body: JSON.stringify({
        title: normalized_title,
      }),
    },
  );
  notify_room_directory_updated();
  return transform_room_context(context);
}

export async function update_room_conversation(
  room_id: string,
  conversation_id: string,
  params: UpdateRoomConversationParams,
): Promise<RoomContextAggregate> {
  const context = await request_api<ApiRoomContextAggregate>(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(room_id)}/conversations/${encodeURIComponent(conversation_id)}`,
    {
      method: "PATCH",
      headers: {"Content-Type": "application/json"},
      body: JSON.stringify({
        title: params.title,
      }),
    },
  );
  notify_room_directory_updated();
  return transform_room_context(context);
}

export async function delete_room_conversation(
  room_id: string,
  conversation_id: string,
): Promise<RoomContextAggregate> {
  const context = await request_api<ApiRoomContextAggregate>(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(room_id)}/conversations/${encodeURIComponent(conversation_id)}`,
    {
      method: "DELETE",
    },
  );
  notify_room_directory_updated();
  return transform_room_context(context);
}

export async function add_room_member(
  room_id: string,
  agent_id: string,
): Promise<RoomContextAggregate> {
  const context = await request_api<ApiRoomContextAggregate>(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(room_id)}/members`,
    {
      method: "POST",
      headers: {"Content-Type": "application/json"},
      body: JSON.stringify({
        agent_id,
      }),
    },
  );
  notify_room_directory_updated();
  return transform_room_context(context);
}

export async function remove_room_member(
  room_id: string,
  agent_id: string,
): Promise<RoomContextAggregate> {
  const context = await request_api<ApiRoomContextAggregate>(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(room_id)}/members/${encodeURIComponent(agent_id)}`,
    {
      method: "DELETE",
    },
  );
  notify_room_directory_updated();
  return transform_room_context(context);
}

export async function delete_room(room_id: string): Promise<{success: boolean}> {
  const result = await request_api<{success: boolean}>(`${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(room_id)}`, {
    method: "DELETE",
  });
  notify_room_directory_updated();
  return result;
}

export async function ensure_direct_room(agent_id: string): Promise<RoomContextAggregate> {
  const context = await request_api<ApiRoomContextAggregate>(
    `${AGENT_API_BASE_URL}/rooms/dm/${encodeURIComponent(agent_id)}`,
    {
      method: "GET",
    },
  );
  notify_room_directory_updated();
  return transform_room_context(context);
}
