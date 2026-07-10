import type { Agent } from "@/types/agent/agent";

export const ROOM_GOAL_SCOPE_LABEL = "房间 Goal";

export interface GoalContinuationHold {
  detail: string;
  label: string;
}

export function goalContinuationHoldForPermission(
  agentName: string | null | undefined,
  permissionMode: string | null | undefined,
): GoalContinuationHold | null {
  if ((permissionMode ?? "").trim() !== "plan") {
    return null;
  }
  const name = agentName?.trim();
  return {
    detail: name
      ? `${name} 处于 Plan 模式，隐藏 Goal 续跑不会自动启动`
      : "目标 Agent 处于 Plan 模式，隐藏 Goal 续跑不会自动启动",
    label: "Plan 模式暂停",
  };
}

function goalContinuationHoldForAgent(
  agent: Pick<Agent, "name" | "options"> | null | undefined,
): GoalContinuationHold | null {
  return goalContinuationHoldForPermission(
    agent?.name,
    agent?.options?.permission_mode,
  );
}

export function goalContinuationHoldForRoomTarget(
  roomMembers: Agent[],
  leadAgentId: string | null | undefined,
  roomHostAutoReplyEnabled = true,
): GoalContinuationHold | null {
  const targetAgent = resolveGoalContinuationTargetAgent(
    roomMembers,
    leadAgentId,
  );
  if (targetAgent) {
    return goalContinuationHoldForAgent(targetAgent);
  }
  if (roomMembers.length <= 1) {
    return null;
  }
  if (!roomHostAutoReplyEnabled) {
    return {
      detail:
        "房间有多个 Agent，但还没有指定 Room Goal 负责人；群主接管未开启，请先选择一个 Agent 负责推进",
      label: "等待负责人",
    };
  }
  return {
    detail:
      "房间有多个 Agent，但还没有指定 Room Goal 负责人；请选择一个 Agent 负责推进",
    label: "等待目标 Agent",
  };
}

function resolveGoalContinuationTargetAgent(
  roomMembers: Agent[],
  leadAgentId: string | null | undefined,
): Agent | null {
  if (roomMembers.length === 1) {
    return roomMembers[0] ?? null;
  }
  const normalizedLeadAgentId = leadAgentId?.trim();
  if (!normalizedLeadAgentId) {
    return null;
  }
  return roomMembers.find((agent) => agent.agent_id === normalizedLeadAgentId) ?? null;
}
