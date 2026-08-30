import type { Agent } from "@/types/agent/agent";
import type { RoomContextAggregate } from "@/types/conversation/room";

function buildFallbackRoomMemberAgent(
  agentId: string,
  roomContexts: RoomContextAggregate[],
): Agent {
  const primaryContext = roomContexts[0] ?? null;
  const fallbackName = primaryContext?.room.room_type === "dm"
    ? primaryContext.room.name?.trim()
      || primaryContext.conversation.title?.trim()
      || agentId
    : agentId;

  return {
    agent_id: agentId,
    name: fallbackName,
    workspace_path: "",
    options: {},
    created_at: 0,
    status: "active",
    avatar: null,
    business_tags: [],
    description: null,
    vibe_tags: [],
    skills_count: null,
  };
}

export function resolveRoomMemberAgents(roomContexts: RoomContextAggregate[]): Agent[] {
  const memberAgents = roomContexts[0]?.member_agents ?? [];
  if (memberAgents.length > 0) {
    return memberAgents;
  }

  const memberAgentIds = roomContexts[0]?.members
    .filter((member) => member.member_type === "agent")
    .map((member) => member.member_agent_id)
    .filter((agentId): agentId is string => Boolean(agentId)) ?? [];

  return memberAgentIds.map((agentId) => (
    buildFallbackRoomMemberAgent(agentId, roomContexts)
  ));
}

export function resolveCurrentRoomMemberAgents(
  roomContexts: RoomContextAggregate[],
  agentDirectory: Agent[],
): Agent[] {
  const directoryByID = new Map(
    agentDirectory.map((agent) => [agent.agent_id, agent]),
  );
  const participationByID = new Map(
    (roomContexts[0]?.members ?? [])
      .filter((member) => member.member_type === "agent")
      .map((member) => [
        member.member_agent_id,
        member.participation_paused,
      ]),
  );
  return resolveRoomMemberAgents(roomContexts).map(
    (agent) => ({
      ...(directoryByID.get(agent.agent_id) ?? agent),
      room_participation_paused:
        participationByID.get(agent.agent_id) ?? false,
    }),
  );
}
