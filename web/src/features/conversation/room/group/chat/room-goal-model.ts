import type { Agent } from "@/types/agent/agent";
import type { Goal } from "@/types/conversation/goal";

export const ROOM_GOAL_LEAD_AGENT_ID_KEY = "room_goal_lead_agent_id";
export const ROOM_GOAL_LEAD_AGENT_NAME_KEY = "room_goal_lead_agent_name";
export const ROOM_GOAL_COLLABORATION_REQUIRED_KEY =
  "room_goal_collaboration_required";
export const ROOM_GOAL_SCOPE_KEY = "room_goal_scope";

function metadata_string(
  metadata: Record<string, unknown> | undefined,
  key: string,
): string {
  const value = metadata?.[key];
  return typeof value === "string" ? value.trim() : "";
}

export function resolve_default_room_goal_lead(
  room_members: Agent[],
  host_agent_id: string | null | undefined,
): string {
  const normalized_host_agent_id = host_agent_id?.trim();
  if (
    normalized_host_agent_id &&
    room_members.some((agent) => agent.agent_id === normalized_host_agent_id)
  ) {
    return normalized_host_agent_id;
  }
  if (room_members.length === 1) {
    return room_members[0]?.agent_id ?? "";
  }
  return "";
}

export function resolve_room_goal_lead_agent_id(
  goal: Goal | null,
  room_members: Agent[],
  fallback_agent_id: string,
): string {
  const metadata_agent_id = metadata_string(
    goal?.metadata,
    ROOM_GOAL_LEAD_AGENT_ID_KEY,
  );
  if (
    metadata_agent_id &&
    room_members.some((agent) => agent.agent_id === metadata_agent_id)
  ) {
    return metadata_agent_id;
  }
  return fallback_agent_id;
}

export function build_room_goal_metadata(
  room_members: Agent[],
  lead_agent_id: string,
): Record<string, unknown> {
  const lead_agent = room_members.find((agent) => agent.agent_id === lead_agent_id);
  return {
    [ROOM_GOAL_SCOPE_KEY]: "room",
    [ROOM_GOAL_LEAD_AGENT_ID_KEY]: lead_agent_id,
    [ROOM_GOAL_LEAD_AGENT_NAME_KEY]: lead_agent?.name ?? "",
    [ROOM_GOAL_COLLABORATION_REQUIRED_KEY]: room_members.length > 1,
  };
}
