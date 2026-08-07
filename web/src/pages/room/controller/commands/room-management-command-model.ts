/**
 * INPUT: 当前 Room 成员状态、管理弹窗的成员集合与 participation 草稿。
 * OUTPUT: 有序成员增删计划与仅包含实际变化的暂停/恢复命令。
 * POS: Room 管理提交跨多个 API 前的纯差异模型。
 */
export interface RoomMembershipPlan {
  addAgentIds: string[];
  removeAgentIds: string[];
}

export interface RoomParticipationMutation {
  agentId: string;
  paused: boolean;
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

export function buildRoomParticipationPlan(
  currentMembers: readonly {
    agent_id: string;
    room_participation_paused?: boolean;
  }[],
  nextAgentIds: readonly string[],
  pausedAgentIds: readonly string[],
): RoomParticipationMutation[] {
  const currentPausedByAgentID = new Map(
    currentMembers.map((member) => [
      member.agent_id,
      member.room_participation_paused ?? false,
    ]),
  );
  const pausedAgentIdSet = new Set(pausedAgentIds);
  return [...new Set(nextAgentIds)].flatMap((agentId) => {
    const paused = pausedAgentIdSet.has(agentId);
    if ((currentPausedByAgentID.get(agentId) ?? false) === paused) {
      return [];
    }
    return [{ agentId, paused }];
  });
}
