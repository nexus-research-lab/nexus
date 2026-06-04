import type { Agent } from "@/types/agent/agent";

export const ROOM_GOAL_SCOPE_LABEL = "房间 Goal";

export interface GoalContinuationHold {
  detail: string;
  label: string;
}

export function goal_continuation_hold_for_permission(
  agent_name: string | null | undefined,
  permission_mode: string | null | undefined,
): GoalContinuationHold | null {
  if ((permission_mode ?? "").trim() !== "plan") {
    return null;
  }
  const name = agent_name?.trim();
  return {
    detail: name
      ? `${name} 处于 Plan 模式，隐藏 Goal 续跑不会自动启动`
      : "目标 Agent 处于 Plan 模式，隐藏 Goal 续跑不会自动启动",
    label: "Plan 模式暂停",
  };
}

export function goal_continuation_hold_for_agent(
  agent: Pick<Agent, "name" | "options"> | null | undefined,
): GoalContinuationHold | null {
  return goal_continuation_hold_for_permission(
    agent?.name,
    agent?.options?.permission_mode,
  );
}

export function goal_continuation_hold_for_room_target(
  room_members: Agent[],
  lead_agent_id: string | null | undefined,
  room_host_auto_reply_enabled = true,
): GoalContinuationHold | null {
  const target_agent = resolve_goal_continuation_target_agent(
    room_members,
    lead_agent_id,
  );
  if (target_agent) {
    return goal_continuation_hold_for_agent(target_agent);
  }
  if (room_members.length <= 1) {
    return null;
  }
  if (!room_host_auto_reply_enabled) {
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

export function resolve_goal_continuation_target_agent(
  room_members: Agent[],
  lead_agent_id: string | null | undefined,
): Agent | null {
  if (room_members.length === 1) {
    return room_members[0] ?? null;
  }
  const normalized_lead_agent_id = lead_agent_id?.trim();
  if (!normalized_lead_agent_id) {
    return null;
  }
  return room_members.find((agent) => agent.agent_id === normalized_lead_agent_id) ?? null;
}
