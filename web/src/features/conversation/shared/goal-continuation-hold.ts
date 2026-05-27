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
  host_agent_id: string | null | undefined,
  host_auto_reply_enabled: boolean,
): GoalContinuationHold | null {
  const target_agent = resolve_goal_continuation_target_agent(
    room_members,
    host_agent_id,
    host_auto_reply_enabled,
  );
  if (target_agent) {
    return goal_continuation_hold_for_agent(target_agent);
  }
  if (room_members.length <= 1) {
    return null;
  }
  return {
    detail:
      "房间有多个 Agent，但没有唯一默认目标；启用主持人自动回复后，隐藏 Goal 续跑才会自动启动",
    label: "等待目标 Agent",
  };
}

export function resolve_goal_continuation_target_agent(
  room_members: Agent[],
  host_agent_id: string | null | undefined,
  host_auto_reply_enabled: boolean,
): Agent | null {
  if (room_members.length === 1) {
    return room_members[0] ?? null;
  }
  if (!host_auto_reply_enabled) {
    return null;
  }
  const normalized_host_agent_id = host_agent_id?.trim();
  if (!normalized_host_agent_id) {
    return null;
  }
  return room_members.find((agent) => agent.agent_id === normalized_host_agent_id) ?? null;
}
