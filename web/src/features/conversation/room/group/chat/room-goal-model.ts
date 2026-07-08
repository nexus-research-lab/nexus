import type { Agent } from "@/types/agent/agent";
import type { LoopCatalogItem } from "@/types/capability/loop";
import type { Goal } from "@/types/conversation/goal";

const ROOM_GOAL_LEAD_AGENT_ID_KEY = "room_goal_lead_agent_id";
const ROOM_GOAL_LEAD_AGENT_NAME_KEY = "room_goal_lead_agent_name";
const ROOM_GOAL_COLLABORATION_REQUIRED_KEY =
  "room_goal_collaboration_required";
const ROOM_GOAL_SCOPE_KEY = "room_goal_scope";
const ROOM_GOAL_LOOP_SLUG_KEY = "room_goal_loop_slug";
const ROOM_GOAL_LOOP_TITLE_KEY = "room_goal_loop_title";

const ROOM_LOOP_GOAL_MAX_OBJECTIVE_LENGTH = 3900;

function metadataString(
  metadata: Record<string, unknown> | undefined,
  key: string,
): string {
  const value = metadata?.[key];
  return typeof value === "string" ? value.trim() : "";
}

export function resolveDefaultRoomGoalLead(
  roomMembers: Agent[],
  hostAgentId: string | null | undefined,
): string {
  const normalizedHostAgentId = hostAgentId?.trim();
  if (
    normalizedHostAgentId &&
    roomMembers.some((agent) => agent.agent_id === normalizedHostAgentId)
  ) {
    return normalizedHostAgentId;
  }
  if (roomMembers.length === 1) {
    return roomMembers[0]?.agent_id ?? "";
  }
  return "";
}

export function resolveRoomGoalLeadAgentId(
  goal: Goal | null,
  roomMembers: Agent[],
  fallbackAgentId: string,
): string {
  const metadataAgentId = metadataString(
    goal?.metadata,
    ROOM_GOAL_LEAD_AGENT_ID_KEY,
  );
  if (
    metadataAgentId &&
    roomMembers.some((agent) => agent.agent_id === metadataAgentId)
  ) {
    return metadataAgentId;
  }
  return fallbackAgentId;
}

export function buildRoomGoalMetadata(
  roomMembers: Agent[],
  leadAgentId: string,
): Record<string, unknown> {
  const leadAgent = roomMembers.find((agent) => agent.agent_id === leadAgentId);
  return {
    [ROOM_GOAL_SCOPE_KEY]: "room",
    [ROOM_GOAL_LEAD_AGENT_ID_KEY]: leadAgentId,
    [ROOM_GOAL_LEAD_AGENT_NAME_KEY]: leadAgent?.name ?? "",
    [ROOM_GOAL_COLLABORATION_REQUIRED_KEY]: roomMembers.length > 1,
  };
}

export function buildRoomLoopGoalMetadata(
  roomMembers: Agent[],
  leadAgentId: string,
  loop: LoopCatalogItem,
): Record<string, unknown> {
  return {
    ...buildRoomGoalMetadata(roomMembers, leadAgentId),
    [ROOM_GOAL_LOOP_SLUG_KEY]: loop.slug,
    [ROOM_GOAL_LOOP_TITLE_KEY]: loop.title,
  };
}

export function buildRoomLoopGoalObjective(loop: LoopCatalogItem): string {
  const lines = [
    `按 Loop「${loop.title}」推进这个 Room Goal。`,
    "",
    "目标",
    firstNonEmpty(loop.kickoff_prompt, loop.description),
    "",
    "步骤",
    ...loop.steps.map((step, index) => {
      const shellCheck = step.shell_check?.trim();
      return `${index + 1}. ${step.name}: ${step.prompt}${shellCheck ? `\n   验证: ${shellCheck}` : ""}`;
    }),
    "",
    "退出条件",
    `- ${loop.exit_condition.description}`,
    loop.exit_condition.command ? `- 验证命令: ${loop.exit_condition.command}` : "",
    loop.exit_condition.max_iterations
      ? `- 最大轮数: ${loop.exit_condition.max_iterations}`
      : "",
    "",
    "护栏",
    ...(loop.guardrails.length > 0 ? loop.guardrails.map((item) => `- ${item}`) : ["- 每轮先检查退出条件；满足后再标记 Goal complete。"]),
    "",
    "Room 协作规则",
    "- 负责人推进整体闭环；需要其他成员时，用 Room @ 委派具体交付物。",
    "- 验证失败时，把失败信息作为反馈继续修；不要把未验证的进展当完成。",
    "- 完成前必须有当前证据证明退出条件成立。",
  ].filter((line) => line.trim() !== "");

  return truncateObjective(lines.join("\n"));
}

function firstNonEmpty(...values: string[]): string {
  return values.map((value) => value.trim()).find(Boolean) ?? "";
}

function truncateObjective(value: string): string {
  if (value.length <= ROOM_LOOP_GOAL_MAX_OBJECTIVE_LENGTH) {
    return value;
  }
  const suffix = "\n\n[Loop 内容过长，已截断；仍以退出条件为准。]";
  return `${value.slice(0, ROOM_LOOP_GOAL_MAX_OBJECTIVE_LENGTH - suffix.length).trimEnd()}${suffix}`;
}
