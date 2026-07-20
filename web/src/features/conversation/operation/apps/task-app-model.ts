/**
 * INPUT: Task-related Operation Stage events from Claude Code or nxs.
 * OUTPUT: A truthful Tasks app session with plan items, task records, and real details.
 * POS: Pure Tasks app projection; presentation and window lifecycle stay outside.
 */
import type { NexusOperationEvent, OperationPhase } from "../operation-types";

export type TaskAppSection = "plan" | "tasks";
export type TaskAppState =
  | "pending"
  | "running"
  | "waiting"
  | "paused"
  | "completed"
  | "failed"
  | "stopped"
  | "observed";

export interface TaskAppUsageItem {
  label: string;
  value: string;
}

export interface TaskAppItem {
  active_label: string | null;
  event: NexusOperationEvent;
  events: NexusOperationEvent[];
  id: string;
  kind: "plan" | "task";
  last_tool_name: string | null;
  output: string | null;
  prompt: string | null;
  state: TaskAppState;
  state_label: string;
  task_id: string | null;
  title: string;
  tool_name: string | null;
  updated_at: number;
  usage: TaskAppUsageItem[];
}

export interface TaskAppSession {
  active_section: TaskAppSection;
  mode_label: string | null;
  plan_items: TaskAppItem[];
  selected_item_id: string | null;
  task_items: TaskAppItem[];
}

const PLAN_TOOL_NAMES = new Set(["TodoWrite", "todo.write", "todo.read", "plan.status"]);
const PLAN_MODE_TOOL_NAMES = new Set(["EnterPlanMode", "ExitPlanMode", "plan.enter", "plan.exit"]);
const CREATE_TOOL_NAMES = new Set(["TaskCreate", "task.create"]);
const READ_TOOL_NAMES = new Set([
  "TaskGet",
  "TaskList",
  "TaskOutput",
  "TaskBackgrounds",
  "AgentOutputTool",
  "task.get",
  "task.list",
  "task.output",
  "task.backgrounds",
]);
const UPDATE_TOOL_NAMES = new Set(["TaskUpdate", "task.update"]);
const STOP_TOOL_NAMES = new Set(["TaskStop", "task.stop"]);
const BACKGROUND_TOOL_NAMES = new Set(["task.background"]);

interface TaskRecord {
  active_label: string | null;
  prompt: string | null;
  state: TaskAppState | null;
  task_id: string | null;
  title: string | null;
  usage: TaskAppUsageItem[];
}

export function buildTaskAppSession(
  event: NexusOperationEvent,
  related_events: NexusOperationEvent[],
): TaskAppSession {
  const events = collect_task_events(event, related_events);
  const latest_plan_event = [...events].reverse().find((item) => PLAN_TOOL_NAMES.has(item.tool_name ?? ""));
  const plan_items = latest_plan_event ? plan_items_from_event(latest_plan_event) : [];
  const task_items = task_items_from_events(events);
  const active_section = resolve_active_section(event, plan_items, task_items);
  const active_items = active_section === "plan" ? plan_items : task_items;
  const selected_item = active_items.find((item) => item.events.some((item_event) => item_event.id === event.id))
    ?? active_items.find((item) => item.state === "running" || item.state === "waiting")
    ?? active_items.at(-1)
    ?? null;

  return {
    active_section,
    mode_label: resolve_plan_mode_label(events),
    plan_items,
    selected_item_id: selected_item?.id ?? null,
    task_items,
  };
}

export function taskAppStateLabel(state: TaskAppState): string {
  const labels: Record<TaskAppState, string> = {
    completed: "已完成",
    failed: "失败",
    observed: "已读取",
    paused: "已暂停",
    pending: "待处理",
    running: "进行中",
    stopped: "已停止",
    waiting: "等待确认",
  };
  return labels[state];
}

function collect_task_events(
  event: NexusOperationEvent,
  related_events: NexusOperationEvent[],
): NexusOperationEvent[] {
  const events = related_events
    .filter((item) => item.surface === "task")
    .filter((item) => item.round_id === event.round_id || item.id === event.id);
  if (!events.some((item) => item.id === event.id)) {
    events.push(event);
  }
  return events.sort((left, right) => left.updated_at - right.updated_at);
}

function plan_items_from_event(event: NexusOperationEvent): TaskAppItem[] {
  const todos = todo_values_from_event(event);
  return todos.flatMap((value, index) => {
    const todo = record_value(value);
    const title = first_string(todo, ["content", "title", "subject"]);
    if (!title) {
      return [];
    }
    const state = task_state(first_string(todo, ["status"])) ?? phase_state(event.phase);
    return [{
      active_label: first_string(todo, ["activeForm", "active_form"]),
      event,
      events: [event],
      id: `plan:${index}:${title}`,
      kind: "plan" as const,
      last_tool_name: null,
      output: null,
      prompt: null,
      state,
      state_label: taskAppStateLabel(state),
      task_id: null,
      title,
      tool_name: event.tool_name ?? null,
      updated_at: event.updated_at,
      usage: [],
    }];
  });
}

function todo_values_from_event(event: NexusOperationEvent): unknown[] {
  if (Array.isArray(event.input_preview?.todos)) return event.input_preview.todos;
  if (Array.isArray(event.input_preview?.items)) return event.input_preview.items;
  const result = record_value(event.result_preview);
  const structured = record_value(result?.structured_output);
  for (const record of [structured, result]) {
    if (!record) continue;
    for (const key of ["todos", "newTodos", "activeTodos", "items"]) {
      if (Array.isArray(record[key])) return record[key];
    }
    const content = record_value(record.content);
    if (Array.isArray(content?.todos)) return content.todos;
  }
  return [];
}

function task_items_from_events(events: NexusOperationEvent[]): TaskAppItem[] {
  const items = new Map<string, TaskAppItem>();
  for (const event of events) {
    if (PLAN_TOOL_NAMES.has(event.tool_name ?? "") || PLAN_MODE_TOOL_NAMES.has(event.tool_name ?? "")) {
      continue;
    }
    const records = task_records_from_event(event);
    if (records.length > 0) {
      for (const record of records) {
        upsert_task_item(items, event, record);
      }
      continue;
    }
    upsert_task_item(items, event, event_task_record(event));
  }
  return [...items.values()].sort((left, right) => left.updated_at - right.updated_at);
}

function upsert_task_item(
  items: Map<string, TaskAppItem>,
  event: NexusOperationEvent,
  record: TaskRecord,
): void {
  const task_id = record.task_id ?? event_task_id(event);
  const matching_key = task_id ? `task:${task_id}` : null;
  const previous = matching_key ? items.get(matching_key) : find_related_task(items, event);
  const key = matching_key ?? previous?.id ?? `event:${event.tool_use_id ?? event.id}`;
  const title = (record.task_id ? record.title : null)
    ?? previous?.title
    ?? record.title
    ?? fallback_task_title(event, task_id);
  const state = record.state ?? fallback_task_state(event, previous?.state);
  const output = task_output(event) ?? previous?.output ?? null;
  const may_update_task_definition = event.kind === "task_delegate"
    || CREATE_TOOL_NAMES.has(event.tool_name ?? "")
    || UPDATE_TOOL_NAMES.has(event.tool_name ?? "");
  const prompt = (may_update_task_definition ? record.prompt : null)
    ?? previous?.prompt
    ?? record.prompt
    ?? task_prompt(event);
  const usage = record.usage.length > 0 ? record.usage : previous?.usage ?? [];

  items.set(key, {
    active_label: record.active_label ?? previous?.active_label ?? null,
    event,
    events: [...(previous?.events ?? []), event],
    id: key,
    kind: "task",
    last_tool_name: task_last_tool_name(event) ?? previous?.last_tool_name ?? null,
    output,
    prompt,
    state,
    state_label: taskAppStateLabel(state),
    task_id: task_id ?? previous?.task_id ?? null,
    title,
    tool_name: event.tool_name ?? previous?.tool_name ?? null,
    updated_at: event.updated_at,
    usage,
  });
}

function find_related_task(
  items: Map<string, TaskAppItem>,
  event: NexusOperationEvent,
): TaskAppItem | undefined {
  if (!event.tool_use_id) {
    return undefined;
  }
  return [...items.values()].find((item) => item.events.some((candidate) => (
    candidate.tool_use_id === event.tool_use_id
  )));
}

function task_records_from_event(event: NexusOperationEvent): TaskRecord[] {
  const root = record_value(event.result_preview);
  const structured = record_value(root?.structured_output);
  const candidates = [structured, root].filter((value): value is Record<string, unknown> => Boolean(value));
  for (const candidate of candidates) {
    if (Array.isArray(candidate.tasks)) {
      return candidate.tasks.flatMap((value) => {
        const record = task_record(value);
        return record ? [record] : [];
      });
    }
    const nested_task = task_record(candidate.task);
    if (nested_task) {
      return [nested_task];
    }
    const direct = task_record(candidate);
    if (direct) {
      return [direct];
    }
  }
  return task_records_from_text(text_value(event.result_preview));
}

function task_records_from_text(value: string | null): TaskRecord[] {
  if (!value) {
    return [];
  }
  const list_items = value.split("\n").flatMap((line) => {
    const match = line.trim().match(/^#(\S+)\s+\[(pending|in_progress|completed)]\s+(.+)$/i);
    const state = task_state(match?.[2] ?? null);
    return match?.[1] && match[3] && state
      ? [{ active_label: null, prompt: null, state, task_id: match[1], title: match[3].trim(), usage: [] }]
      : [];
  });
  if (list_items.length > 0) {
    return list_items;
  }
  const created = value.match(/^Task #(\S+) created successfully(?::\s*(.*))?/im);
  return created?.[1]
    ? [{ active_label: null, prompt: null, state: "pending", task_id: created[1], title: created[2]?.trim() || null, usage: [] }]
    : [];
}

function task_record(value: unknown): TaskRecord | null {
  const record = record_value(value);
  if (!record) {
    return null;
  }
  const task_id = first_string(record, ["task_id", "taskId", "id"]);
  const title = first_string(record, ["subject", "title", "description", "summary"]);
  if (!task_id && !title) {
    return null;
  }
  return {
    active_label: first_string(record, ["activeForm", "active_form"]),
    prompt: first_string(record, ["prompt", "input", "description"]),
    state: task_state(first_string(record, ["status", "state"])),
    task_id,
    title,
    usage: task_usage(record.usage),
  };
}

function event_task_record(event: NexusOperationEvent): TaskRecord {
  return {
    active_label: first_string(event.input_preview, ["activeForm", "active_form"]),
    prompt: task_prompt(event),
    state: task_state(first_string(event.input_preview, ["status", "state"])),
    task_id: event_task_id(event),
    title: first_string(event.input_preview, ["subject", "title", "description", "prompt", "input", "task"]),
    usage: task_usage(event.input_preview?.usage),
  };
}

function event_task_id(event: NexusOperationEvent): string | null {
  return first_string(event.input_preview, ["task_id", "taskId"])
    ?? task_records_from_text(text_value(event.result_preview))[0]?.task_id
    ?? null;
}

function fallback_task_title(event: NexusOperationEvent, task_id: string | null): string {
  const candidate = event.target?.trim();
  if (candidate && candidate !== event.tool_name && candidate !== task_id) {
    return candidate;
  }
  return task_id ? `任务 ${short_task_id(task_id)}` : "子任务";
}

function fallback_task_state(event: NexusOperationEvent, previous?: TaskAppState): TaskAppState {
  if (event.phase === "error") return "failed";
  if (event.phase === "cancelled") return "stopped";
  if (event.phase === "waiting") return "waiting";
  if (event.phase === "running") return "running";
  const tool_name = event.tool_name ?? "";
  if (STOP_TOOL_NAMES.has(tool_name)) return "stopped";
  if (BACKGROUND_TOOL_NAMES.has(tool_name)) return previous ?? "running";
  if (CREATE_TOOL_NAMES.has(tool_name)) return previous ?? "pending";
  if (READ_TOOL_NAMES.has(tool_name)) return previous ?? "observed";
  if (UPDATE_TOOL_NAMES.has(tool_name)) return previous ?? "observed";
  return previous ?? "completed";
}

function phase_state(phase: OperationPhase): TaskAppState {
  if (phase === "done") return "completed";
  if (phase === "error") return "failed";
  if (phase === "cancelled") return "stopped";
  if (phase === "waiting") return "waiting";
  if (phase === "running") return "running";
  return "pending";
}

function task_state(value: string | null): TaskAppState | null {
  const normalized = value?.trim().toLowerCase();
  if (!normalized) return null;
  if (["created", "pending", "queued"].includes(normalized)) return "pending";
  if (["running", "in_progress", "in progress", "started"].includes(normalized)) return "running";
  if (["awaiting_approval", "waiting"].includes(normalized)) return "waiting";
  if (normalized === "paused") return "paused";
  if (["completed", "finished", "succeeded", "success", "done"].includes(normalized)) return "completed";
  if (["failed", "error", "timed_out", "timeout"].includes(normalized)) return "failed";
  if (["stopped", "canceled", "cancelled", "killed", "deleted"].includes(normalized)) return "stopped";
  return null;
}

function task_prompt(event: NexusOperationEvent): string | null {
  return first_string(event.input_preview, ["prompt", "input", "description", "task"]);
}

function task_last_tool_name(event: NexusOperationEvent): string | null {
  return first_string(event.input_preview, ["last_tool_name", "lastToolName"]);
}

function task_output(event: NexusOperationEvent): string | null {
  if (event.phase === "running" && event.result_preview == null) {
    return null;
  }
  return text_value(event.result_preview);
}

function task_usage(value: unknown): TaskAppUsageItem[] {
  const usage = record_value(value);
  if (!usage) return [];
  const items: TaskAppUsageItem[] = [];
  const total_tokens = finite_number(usage.total_tokens);
  const tool_uses = finite_number(usage.tool_uses);
  const duration_ms = finite_number(usage.duration_ms);
  if (total_tokens !== null) items.push({ label: "Tokens", value: total_tokens.toLocaleString() });
  if (tool_uses !== null) items.push({ label: "工具调用", value: String(tool_uses) });
  if (duration_ms !== null) items.push({ label: "耗时", value: format_duration(duration_ms) });
  return items;
}

function text_value(value: unknown): string | null {
  if (typeof value === "string") return value.trim() || null;
  if (Array.isArray(value)) {
    const lines = value.map(text_value).filter((item): item is string => Boolean(item));
    return lines.length ? lines.join("\n").slice(0, 6000) : null;
  }
  const record = record_value(value);
  if (!record) return null;
  for (const key of ["content", "output", "summary", "message", "error"]) {
    const text = text_value(record[key]);
    if (text) return text.slice(0, 6000);
  }
  return null;
}

function resolve_active_section(
  event: NexusOperationEvent,
  plan_items: TaskAppItem[],
  task_items: TaskAppItem[],
): TaskAppSection {
  if (PLAN_TOOL_NAMES.has(event.tool_name ?? "") || PLAN_MODE_TOOL_NAMES.has(event.tool_name ?? "")) return "plan";
  if (task_items.length > 0) return "tasks";
  return plan_items.length > 0 ? "plan" : "tasks";
}

function resolve_plan_mode_label(events: NexusOperationEvent[]): string | null {
  const mode_event = [...events].reverse().find((event) => PLAN_MODE_TOOL_NAMES.has(event.tool_name ?? ""));
  if (!mode_event) return null;
  return mode_event.tool_name === "ExitPlanMode" || mode_event.tool_name === "plan.exit"
    ? "已进入执行模式"
    : "正在规划";
}

function first_string(record: Record<string, unknown> | null | undefined, keys: string[]): string | null {
  if (!record) return null;
  for (const key of keys) {
    const value = record[key];
    if (typeof value === "string" && value.trim()) return value.trim();
  }
  return null;
}

function record_value(value: unknown): Record<string, unknown> | null {
  return value && typeof value === "object" && !Array.isArray(value)
    ? value as Record<string, unknown>
    : null;
}

function finite_number(value: unknown): number | null {
  if (typeof value === "number" && Number.isFinite(value)) return value;
  if (typeof value === "string" && /^\d+(?:\.\d+)?$/.test(value.trim())) return Number(value);
  return null;
}

function format_duration(duration_ms: number): string {
  if (duration_ms < 1000) return `${Math.round(duration_ms)}ms`;
  if (duration_ms < 60_000) return `${(duration_ms / 1000).toFixed(duration_ms < 10_000 ? 1 : 0)}s`;
  const minutes = Math.floor(duration_ms / 60_000);
  const seconds = Math.floor((duration_ms % 60_000) / 1000);
  return seconds ? `${minutes}m ${seconds}s` : `${minutes}m`;
}

function short_task_id(task_id: string): string {
  return task_id.length > 12 ? `${task_id.slice(0, 8)}…` : task_id;
}
