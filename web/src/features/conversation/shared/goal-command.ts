import {
  clear_goal_api,
  complete_goal_api,
  create_goal_api,
  get_current_goal_api,
  pause_goal_api,
  resume_goal_api,
} from "@/lib/api/goal-api";
import { ApiRequestError } from "@/lib/api/http";
import type { Goal } from "@/types/conversation/goal";

export type GoalCommand =
  | { kind: "show" }
  | { kind: "pause" }
  | { kind: "resume" }
  | { kind: "clear" }
  | { kind: "complete" }
  | { kind: "create"; objective: string; token_budget: number | null };

export type GoalCreateCommand = Extract<GoalCommand, { kind: "create" }>;

interface RunGoalCommandOptions {
  replace_existing?: boolean;
}

export function parse_goal_command(content: string): GoalCommand | null {
  const trimmed = content.trim();
  if (trimmed === "/goal") {
    return { kind: "show" };
  }
  const body = goal_command_body(trimmed);
  if (body === null) {
    return null;
  }
  const normalized = body.toLowerCase();
  if (normalized === "pause") return { kind: "pause" };
  if (normalized === "resume" || normalized === "start") return { kind: "resume" };
  if (normalized === "clear") return { kind: "clear" };
  if (normalized === "complete" || normalized === "done") return { kind: "complete" };
  const parsed = parse_goal_create_arguments(body);
  if (!parsed.objective) {
    return { kind: "show" };
  }
  return { kind: "create", ...parsed };
}

export async function run_goal_command(
  session_key: string,
  command: GoalCommand,
  options: RunGoalCommandOptions = {},
) {
  if (command.kind === "show") {
    return;
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
    return;
  }
  const current = await get_current_goal_api(session_key);
  if (command.kind === "pause") await pause_goal_api(current.id);
  if (command.kind === "resume") await resume_goal_api(current.id);
  if (command.kind === "clear") await clear_goal_api(current.id);
  if (command.kind === "complete") await complete_goal_api(current.id);
}

export async function goal_replacement_candidate(
  session_key: string,
  command: GoalCommand,
): Promise<Goal | null> {
  if (command.kind !== "create") {
    return null;
  }
  return current_goal_or_null(session_key);
}

async function clear_current_goal_if_present(session_key: string) {
  const current = await current_goal_or_null(session_key);
  if (current !== null) {
    await clear_goal_api(current.id);
  }
}

async function current_goal_or_null(session_key: string): Promise<Goal | null> {
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
  if (trimmed.startsWith("/goal ")) {
    return trimmed.slice("/goal ".length).trim();
  }
  if (trimmed.startsWith("/goal\n")) {
    return trimmed.slice("/goal\n".length).trim();
  }
  return null;
}

function parse_goal_create_arguments(body: string): {
  objective: string;
  token_budget: number | null;
} {
  const parts = body.split(/\s+/).filter(Boolean);
  const objective_parts: string[] = [];
  let token_budget: number | null = null;
  for (let index = 0; index < parts.length; index++) {
    const part = parts[index];
    if (part === "--tokens" || part === "--token-budget" || part === "--budget" || part === "-b") {
      token_budget = parse_token_budget(parts[index + 1] ?? "");
      index++;
      continue;
    }
    const match = part.match(/^--(?:tokens|token-budget|budget)=(.+)$/);
    if (match) {
      token_budget = parse_token_budget(match[1]);
      continue;
    }
    objective_parts.push(part);
  }
  return {
    objective: objective_parts.join(" ").trim(),
    token_budget,
  };
}

function parse_token_budget(value: string): number | null {
  const normalized = value.trim().replace(/[, _]/g, "");
  const match = normalized.match(/^(\d+(?:\.\d+)?)([kKmM])?$/);
  if (!match) return null;
  const multiplier = match[2]?.toLowerCase() === "m" ? 1_000_000 : match[2] ? 1_000 : 1;
  const parsed = Math.round(Number.parseFloat(match[1]) * multiplier);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : null;
}
