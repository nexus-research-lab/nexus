import { transformApiAgent } from "@/lib/api/agent/agent-transform";
import type {
  ApiRoomContextAggregate,
  CreateRoomParams,
  RoomContextAggregate,
  UpdateRoomParams,
} from "@/types/conversation/room";

export interface RoomMessagesQuery {
  around_limit?: number | null;
  around_round_id?: string | null;
  before_round_id?: string | null;
  before_round_timestamp?: number | null;
  limit?: number;
}

type RoomMutationParams = CreateRoomParams | UpdateRoomParams;

function appendDefinedRoomSettings(
  body: Record<string, unknown>,
  params: RoomMutationParams,
): Record<string, unknown> {
  const entries: Array<[string, unknown]> = [
    ["skill_names", params.skill_names],
    [
      "host_agent_id",
      params.host_agent_id === undefined ? undefined : params.host_agent_id ?? "",
    ],
    ["host_auto_reply_enabled", params.host_auto_reply_enabled],
    ["private_messages_enabled", params.private_messages_enabled],
  ];
  for (const [key, value] of entries) {
    if (value !== undefined) {
      body[key] = value;
    }
  }
  return body;
}

export function buildCreateRoomBody(
  params: CreateRoomParams,
): Record<string, unknown> {
  return appendDefinedRoomSettings({
    agent_ids: params.agent_ids,
    avatar: params.avatar ?? null,
    description: params.description ?? "",
    name: params.name,
    title: params.title,
  }, params);
}

export function buildUpdateRoomBody(
  params: UpdateRoomParams,
): Record<string, unknown> {
  return appendDefinedRoomSettings({
    avatar: params.avatar ?? null,
    description: params.description,
    name: params.name,
    title: params.title,
  }, params);
}

export function buildRoomMessagesQuery(options: RoomMessagesQuery): string {
  const values: Array<[string, string | null]> = [
    ["limit", options.limit && options.limit > 0 ? String(options.limit) : null],
    ["before_round_id", options.before_round_id ?? null],
    [
      "before_round_timestamp",
      options.before_round_timestamp && options.before_round_timestamp > 0
        ? String(options.before_round_timestamp)
        : null,
    ],
    ["around_round_id", options.around_round_id ?? null],
    [
      "around_limit",
      options.around_limit && options.around_limit > 0
        ? String(options.around_limit)
        : null,
    ],
  ];
  const params = new URLSearchParams();
  for (const [key, value] of values) {
    if (value) {
      params.set(key, value);
    }
  }
  const query = params.toString();
  return query ? `?${query}` : "";
}

export function normalizeConversationTitle(value: unknown): string | undefined {
  return typeof value === "string" && value.trim() ? value.trim() : undefined;
}

export function transformRoomContext(
  apiContext: ApiRoomContextAggregate,
): RoomContextAggregate {
  return {
    conversation: apiContext.conversation,
    member_agents: (apiContext.member_agents ?? []).map(transformApiAgent),
    members: apiContext.members,
    room: {
      ...apiContext.room,
      host_agent_id: apiContext.room.host_agent_id ?? null,
      host_auto_reply_enabled: apiContext.room.host_auto_reply_enabled ?? false,
      private_messages_enabled: apiContext.room.private_messages_enabled ?? false,
      skill_names: apiContext.room.skill_names ?? [],
    },
    sessions: apiContext.sessions,
  };
}
