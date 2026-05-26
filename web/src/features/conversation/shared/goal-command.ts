import {
  clear_goal_api,
  create_goal_api,
  get_current_goal_api,
  pause_goal_api,
  resume_goal_api,
} from "@/lib/api/goal-api";
import { ApiRequestError } from "@/lib/api/http";
import type { Goal } from "@/types/conversation/goal";

export type GoalCommand =
  | { kind: "show" }
  | { kind: "edit" }
  | { kind: "pause" }
  | { kind: "resume" }
  | { kind: "clear" }
  | { kind: "invalid"; message: string }
  | { kind: "create"; objective: string; token_budget: number | null };

export type GoalCreateCommand = Extract<GoalCommand, { kind: "create" }>;
export type GoalActionCommandKind = Extract<
  GoalCommand,
  { kind: "pause" | "resume" | "clear" }
>["kind"];

export type GoalCommandRunResult =
  | { kind: "ok" }
  | { kind: "missing_goal"; command: GoalActionCommandKind };

export type GoalCreateDecision =
  | { kind: "create" }
  | { kind: "replace" }
  | { kind: "confirm"; current: Goal };

interface RunGoalCommandOptions {
  replace_existing?: boolean;
}

const MAX_GOAL_OBJECTIVE_CHARS = 4_000;
const GOAL_TOO_LONG_FILE_HINT =
  "请把更长的说明放到文件中，再在 Goal 里引用该文件，例如：/goal follow the instructions in docs/goal.md。";

export function parse_goal_command(content: string): GoalCommand | null {
  const trimmed = content.trim();
  const body = goal_command_body(trimmed);
  if (body === null) {
    return null;
  }
  if (body === "") {
    return { kind: "show" };
  }
  const normalized = body.toLowerCase();
  if (normalized === "edit") return { kind: "edit" };
  if (normalized === "pause") return { kind: "pause" };
  if (normalized === "resume") return { kind: "resume" };
  if (normalized === "clear") return { kind: "clear" };
  const objective = body.trim();
  const objective_chars = Array.from(objective).length;
  if (objective_chars > MAX_GOAL_OBJECTIVE_CHARS) {
    return {
      kind: "invalid",
      message: `Goal objective is too long: ${format_count(objective_chars)} characters. Limit: ${format_count(MAX_GOAL_OBJECTIVE_CHARS)} characters. ${GOAL_TOO_LONG_FILE_HINT}`,
    };
  }
  return { kind: "create", objective, token_budget: null };
}

export async function run_goal_command(
  session_key: string,
  command: GoalCommand,
  options: RunGoalCommandOptions = {},
): Promise<GoalCommandRunResult> {
  if (command.kind === "show") {
    return { kind: "ok" };
  }
  if (command.kind === "invalid") {
    return { kind: "ok" };
  }
  if (command.kind === "edit") {
    return { kind: "ok" };
  }
  if (command.kind === "create") {
    if (options.replace_existing) {
      await clear_current_goal_if_present(session_key);
    }
    await create_goal_api({
      session_key,
      objective: command.objective,
      token_budget: command.token_budget,
    });
    return { kind: "ok" };
  }
  const current = await current_goal_or_null(session_key);
  if (current === null) {
    return { kind: "missing_goal", command: command.kind };
  }
  if (command.kind === "pause") await pause_goal_api(current.id);
  if (command.kind === "resume") await resume_goal_api(current.id);
  if (command.kind === "clear") await clear_goal_api(current.id);
  return { kind: "ok" };
}

export async function goal_create_decision(
  session_key: string,
  command: GoalCommand,
): Promise<GoalCreateDecision> {
  if (command.kind !== "create") {
    return { kind: "create" };
  }
  const current = await current_goal_or_null(session_key);
  if (current === null) {
    return { kind: "create" };
  }
  if (current.status === "complete") {
    return { kind: "replace" };
  }
  return { kind: "confirm", current };
}

async function clear_current_goal_if_present(session_key: string) {
  const current = await current_goal_or_null(session_key);
  if (current !== null) {
    await clear_goal_api(current.id);
  }
}

export async function current_goal_or_null(session_key: string): Promise<Goal | null> {
  try {
    return await get_current_goal_api(session_key);
  } catch (error) {
    if (error instanceof ApiRequestError && error.status === 404) {
      return null;
    }
    throw error;
  }
}

function goal_command_body(trimmed: string): string | null {
  const match = trimmed.match(/^\/goal(?:\s+([\s\S]*))?$/);
  return match ? (match[1] ?? "").trim() : null;
}

function format_count(value: number): string {
  return new Intl.NumberFormat("en-US").format(value);
}
