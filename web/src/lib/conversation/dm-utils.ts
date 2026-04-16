import { Agent } from "@/types/agent/agent";
import { RoomAggregate } from "@/types/conversation/room";

const DEFAULT_FALLBACK = "未命名 DM";

/** Resolve a DM room's display name — prefers the agent member's name, then room name, then fallback. */
export function get_dm_display_name(
  room: RoomAggregate,
  agents: Agent[],
  fallback_label: string = DEFAULT_FALLBACK,
): string {
  const agent_member = room.members.find((m) => m.member_type === "agent");
  if (agent_member?.member_agent_id) {
    const matched = agents.find(
      (a) => a.agent_id === agent_member.member_agent_id,
    );
    if (matched) return matched.name;
  }
  return room.room.name?.trim() || fallback_label;
}
