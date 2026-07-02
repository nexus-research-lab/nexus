import { get_agent_api_base_url } from "@/config/options";
import { transform_api_agent } from "@/lib/api/agent-transform";
import { ApiRequestError, request_api } from "@/lib/api/http";
import { useAgentStore } from "@/store/agent";
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

export function subscribe_room_directory_updates(
  listener: () => void,
): () => void {
  if (typeof window === "undefined") {
    return () => {};
  }

  const handleUpdate = () => {
    listener();
  };

  window.addEventListener(ROOM_DIRECTORY_UPDATED_EVENT_NAME, handleUpdate);
  return () => {
    window.removeEventListener(
      ROOM_DIRECTORY_UPDATED_EVENT_NAME,
      handleUpdate,
    );
  };
}

function normalizeConversationTitle(value: unknown): string | undefined {
  if (typeof value !== "string") {
    return undefined;
  }
  const normalizedTitle = value.trim();
  return normalizedTitle ? normalizedTitle : undefined;
}

function transformRoomContext(
  apiContext: ApiRoomContextAggregate,
): RoomContextAggregate {
  return {
    room: {
      ...apiContext.room,
      skill_names: apiContext.room.skill_names ?? [],
      host_agent_id: apiContext.room.host_agent_id ?? null,
      host_auto_reply_enabled: apiContext.room.host_auto_reply_enabled ?? false,
      private_messages_enabled: apiContext.room.private_messages_enabled ?? false,
    },
    members: apiContext.members,
    member_agents: (apiContext.member_agents ?? []).map(transform_api_agent),
    conversation: apiContext.conversation,
    sessions: apiContext.sessions,
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

export async function get_room_contexts(
  roomId: string,
): Promise<RoomContextAggregate[]> {
  const result = await request_api<ApiRoomContextAggregate[]>(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(roomId)}/contexts`,
    {
      method: "GET",
    },
  );
  return result.map(transformRoomContext);
}

export async function get_room_conversation_messages(
  roomId: string,
  conversationId: string,
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
    params.set(
      "before_round_timestamp",
      String(options.before_round_timestamp),
    );
  }
  const query = params.toString();
  const result = await request_api<ApiRoomConversationMessagePage>(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(roomId)}/conversations/${encodeURIComponent(conversationId)}/messages${query ? `?${query}` : ""}`,
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

export async function upload_room_conversation_attachment_api(
  roomId: string,
  conversationId: string,
  file: File,
  path?: string,
): Promise<{ path: string; name: string; size: number }> {
  const formData = new FormData();
  formData.append("file", file);
  if (path) {
    formData.append("path", path);
  }

  return request_api<{ path: string; name: string; size: number }>(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(roomId)}/conversations/${encodeURIComponent(conversationId)}/attachments/upload`,
    {
      method: "POST",
      body: formData,
    },
  );
}

export async function create_room(
  params: CreateRoomParams,
): Promise<RoomContextAggregate> {
  const body: Record<string, unknown> = {
    agent_ids: params.agent_ids,
    name: params.name,
    description: params.description ?? "",
    title: params.title,
    avatar: params.avatar ?? null,
  };
  if (params.skill_names !== undefined) {
    body.skill_names = params.skill_names;
  }
  if (params.host_agent_id !== undefined) {
    body.host_agent_id = params.host_agent_id ?? "";
  }
  if (params.host_auto_reply_enabled !== undefined) {
    body.host_auto_reply_enabled = params.host_auto_reply_enabled;
  }
  if (params.private_messages_enabled !== undefined) {
    body.private_messages_enabled = params.private_messages_enabled;
  }
  const context = await request_api<ApiRoomContextAggregate>(
    `${AGENT_API_BASE_URL}/rooms`,
    {
      method: "POST",
      body: JSON.stringify(body),
    },
  );
  notify_room_directory_updated();
  return transformRoomContext(context);
}

export async function update_room(
  roomId: string,
  params: UpdateRoomParams,
): Promise<RoomContextAggregate> {
  const body: Record<string, unknown> = {
    name: params.name,
    description: params.description,
    title: params.title,
    avatar: params.avatar ?? null,
  };
  if (params.skill_names !== undefined) {
    body.skill_names = params.skill_names;
  }
  if (params.host_agent_id !== undefined) {
    body.host_agent_id = params.host_agent_id ?? "";
  }
  if (params.host_auto_reply_enabled !== undefined) {
    body.host_auto_reply_enabled = params.host_auto_reply_enabled;
  }
  if (params.private_messages_enabled !== undefined) {
    body.private_messages_enabled = params.private_messages_enabled;
  }
  const context = await request_api<ApiRoomContextAggregate>(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(roomId)}`,
    {
      method: "PATCH",
      body: JSON.stringify(body),
    },
  );
  notify_room_directory_updated();
  return transformRoomContext(context);
}

export async function create_room_conversation(
  roomId: string,
  params: CreateRoomConversationParams = {},
): Promise<RoomContextAggregate> {
  const normalizedTitle = normalizeConversationTitle(params.title);
  const context = await request_api<ApiRoomContextAggregate>(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(roomId)}/conversations`,
    {
      method: "POST",
      body: JSON.stringify({
        title: normalizedTitle,
      }),
    },
  );
  notify_room_directory_updated();
  return transformRoomContext(context);
}

export async function update_room_conversation(
  roomId: string,
  conversationId: string,
  params: UpdateRoomConversationParams,
): Promise<RoomContextAggregate> {
  const context = await request_api<ApiRoomContextAggregate>(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(roomId)}/conversations/${encodeURIComponent(conversationId)}`,
    {
      method: "PATCH",
      body: JSON.stringify({
        title: params.title,
      }),
    },
  );
  notify_room_directory_updated();
  return transformRoomContext(context);
}

export async function delete_room_conversation(
  roomId: string,
  conversationId: string,
): Promise<RoomContextAggregate> {
  const context = await request_api<ApiRoomContextAggregate>(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(roomId)}/conversations/${encodeURIComponent(conversationId)}`,
    {
      method: "DELETE",
    },
  );
  notify_room_directory_updated();
  return transformRoomContext(context);
}

export async function close_room_conversation_runtime(
  roomId: string,
  conversationId: string,
): Promise<void> {
  await request_api<{ closed: boolean }>(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(roomId)}/conversations/${encodeURIComponent(conversationId)}/close`,
    {
      method: "POST",
    },
  );
}

export async function add_room_member(
  roomId: string,
  agentId: string,
): Promise<RoomContextAggregate> {
  const context = await request_api<ApiRoomContextAggregate>(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(roomId)}/members`,
    {
      method: "POST",
      body: JSON.stringify({
        agent_id: agentId,
      }),
    },
  );
  notify_room_directory_updated();
  return transformRoomContext(context);
}

export async function remove_room_member(
  roomId: string,
  agentId: string,
): Promise<RoomContextAggregate> {
  const context = await request_api<ApiRoomContextAggregate>(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(roomId)}/members/${encodeURIComponent(agentId)}`,
    {
      method: "DELETE",
    },
  );
  notify_room_directory_updated();
  return transformRoomContext(context);
}

export async function delete_room(
  roomId: string,
): Promise<{ success: boolean }> {
  const result = await request_api<{ success: boolean }>(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(roomId)}`,
    {
      method: "DELETE",
    },
  );
  notify_room_directory_updated();
  return result;
}

export async function ensure_direct_room(
  agentId: string,
): Promise<RoomContextAggregate> {
  let context: ApiRoomContextAggregate;
  try {
    context = await request_api<ApiRoomContextAggregate>(
      `${AGENT_API_BASE_URL}/rooms/dm/${encodeURIComponent(agentId)}`,
      {
        method: "GET",
      },
    );
  } catch (error) {
    if (error instanceof ApiRequestError && error.status === 404) {
      const store = useAgentStore.getState();
      if (store.current_agent_id === agentId) {
        store.set_current_agent(null);
      }
      void store.load_agents_from_server();
    }
    throw error;
  }
  notify_room_directory_updated();
  return transformRoomContext(context);
}
