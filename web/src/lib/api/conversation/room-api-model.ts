import { transformApiAgent } from "@/lib/api/agent/agent-transform";
import type {
  ApiRoomContextAggregate,
  CreateRoomParams,
  RoomContextAggregate,
  UpdateRoomParams,
} from "@/types/conversation/room";

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
