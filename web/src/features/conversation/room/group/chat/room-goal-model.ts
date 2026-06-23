import type { Agent } from "@/types/agent/agent";
import type { LoopCatalogItem } from "@/types/capability/loop";
import type { Goal } from "@/types/conversation/goal";

export const ROOM_GOAL_LEAD_AGENT_ID_KEY = "room_goal_lead_agent_id";
export const ROOM_GOAL_LEAD_AGENT_NAME_KEY = "room_goal_lead_agent_name";
export const ROOM_GOAL_COLLABORATION_REQUIRED_KEY =
  "room_goal_collaboration_required";
export const ROOM_GOAL_SCOPE_KEY = "room_goal_scope";
export const ROOM_GOAL_LOOP_SLUG_KEY = "room_goal_loop_slug";
export const ROOM_GOAL_LOOP_TITLE_KEY = "room_goal_loop_title";

const ROOM_LOOP_GOAL_MAX_OBJECTIVE_LENGTH = 3900;

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

export function build_room_loop_goal_metadata(
  room_members: Agent[],
  lead_agent_id: string,
  loop: LoopCatalogItem,
): Record<string, unknown> {
  return {
    ...build_room_goal_metadata(room_members, lead_agent_id),
    [ROOM_GOAL_LOOP_SLUG_KEY]: loop.slug,
    [ROOM_GOAL_LOOP_TITLE_KEY]: loop.title,
  };
}

export function build_room_loop_goal_objective(loop: LoopCatalogItem): string {
  const lines = [
    `按 Loop「${loop.title}」推进这个 Room Goal。`,
    "",
    "目标",
    first_non_empty(loop.kickoff_prompt, loop.description),
    "",
    "步骤",
    ...loop.steps.map((step, index) => {
      const shell_check = step.shell_check?.trim();
      return `${index + 1}. ${step.name}: ${step.prompt}${shell_check ? `\n   验证: ${shell_check}` : ""}`;
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

  return truncate_objective(lines.join("\n"));
}

function first_non_empty(...values: string[]): string {
  return values.map((value) => value.trim()).find(Boolean) ?? "";
}

function truncate_objective(value: string): string {
  if (value.length <= ROOM_LOOP_GOAL_MAX_OBJECTIVE_LENGTH) {
    return value;
  }
  const suffix = "\n\n[Loop 内容过长，已截断；仍以退出条件为准。]";
  return `${value.slice(0, ROOM_LOOP_GOAL_MAX_OBJECTIVE_LENGTH - suffix.length).trimEnd()}${suffix}`;
}
