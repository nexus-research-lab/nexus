import type { NexusOperationPhase } from "./operation-types";

interface OperationInputRow {
  key: string;
  label: string;
  value: string;
}

interface OperationToolProfile {
  title: string;
  action_label: string;
  target_keys: string[];
}

export const PHASE_LABELS: Record<NexusOperationPhase, string> = {
  waiting: "Waiting",
  running: "Running",
  succeeded: "Succeeded",
  failed: "Failed",
  cancelled: "Cancelled",
  completed: "Completed",
};

export function resolve_operation_tool_profile(
  tool_name?: string | null,
  kind?: string | null,
  surface?: string | null,
): OperationToolProfile {
  const normalized_tool = normalize_label(tool_name) || "operation";
  const normalized_kind = normalize_label(kind);
  const normalized_surface = normalize_label(surface);
  return {
    title: title_case(normalized_tool),
    action_label: title_case(normalized_kind || normalized_surface || "tool"),
    target_keys: ["path", "file", "url", "command", "query", "target", "name", "job_id", "run_id"],
  };
}

export function build_operation_input_rows(
  input: unknown,
  target_keys: string[],
  limit: number,
): OperationInputRow[] {
  const rows = flatten_input(input);
  rows.sort((left, right) => {
    const left_priority = target_keys.includes(left.key) ? 0 : 1;
    const right_priority = target_keys.includes(right.key) ? 0 : 1;
    if (left_priority !== right_priority) {
      return left_priority - right_priority;
    }
    return left.key.localeCompare(right.key);
  });
  return rows.slice(0, Math.max(0, limit));
}

function flatten_input(input: unknown): OperationInputRow[] {
  if (!is_record(input)) {
    if (input == null || input === "") {
      return [];
    }
    return [{ key: "input", label: "Input", value: stringify_value(input) }];
  }
  return Object.entries(input)
    .filter(([, value]) => value != null && value !== "")
    .map(([key, value]) => ({
      key,
      label: title_case(key),
      value: stringify_value(value),
    }));
}

function stringify_value(value: unknown): string {
  if (typeof value === "string") {
    return value;
  }
  if (typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  try {
    return JSON.stringify(value);
  } catch {
    return String(value);
  }
}

function is_record(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value != null && !Array.isArray(value);
}

function normalize_label(value?: string | null): string {
  return (value ?? "").trim().replace(/[_-]+/g, " ");
}

function title_case(value: string): string {
  return value
    .split(/\s+/)
    .filter(Boolean)
    .map((item) => item.slice(0, 1).toUpperCase() + item.slice(1))
    .join(" ");
}
