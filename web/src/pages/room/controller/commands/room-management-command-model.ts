export interface RoomMembershipPlan {
  addAgentIds: string[];
  removeAgentIds: string[];
}

export function buildRoomMembershipPlan(
  currentAgentIds: readonly string[],
  nextAgentIds: readonly string[],
): RoomMembershipPlan {
  const currentAgentIdSet = new Set(currentAgentIds);
  const nextAgentIdSet = new Set(nextAgentIds);

  return {
    addAgentIds: [...nextAgentIdSet].filter(
      (agentId) => !currentAgentIdSet.has(agentId),
    ),
    removeAgentIds: [...currentAgentIdSet].filter(
      (agentId) => !nextAgentIdSet.has(agentId),
    ),
  };
}
