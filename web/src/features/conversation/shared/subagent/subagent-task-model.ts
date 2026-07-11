import type {
  SubagentRuntimeKind,
  SubagentTask,
  SubagentTaskCapabilities,
  SubagentTaskListResponse,
  SubagentTaskSource,
} from "@/types/conversation/subagent-task";

type SubagentTaskViewStatus =
  | "pending"
  | "running"
  | "completed"
  | "stopped"
  | "failed";

export const SUBAGENT_TASK_POLL_INTERVAL_MS = 3_000;

const EMPTY_CAPABILITIES: SubagentTaskCapabilities = {
  observe: false,
  transcript: false,
  stop: false,
  send_message: false,
  resume: false,
};

const SUBAGENT_AVATAR_PALETTE = [
  "#9b87e8",
  "#f5a24f",
  "#f1c93e",
  "#8bcf76",
  "#58c4c9",
  "#63a7db",
  "#70d2b5",
  "#b891ed",
];

const SUBAGENT_STATUS_BY_ALIAS: Readonly<Record<string, SubagentTaskViewStatus>> = {
  queued: "pending",
  created: "pending",
  pending: "pending",
  running: "running",
  started: "running",
  in_progress: "running",
  "in progress": "running",
  completed: "completed",
  complete: "completed",
  success: "completed",
  done: "completed",
  finished: "completed",
  stopped: "stopped",
  deleted: "stopped",
  cancelled: "stopped",
  canceled: "stopped",
  killed: "stopped",
  interrupted: "stopped",
  failed: "failed",
  error: "failed",
};

const SUBAGENT_RUNTIME_BY_ALIAS: Readonly<Record<string, SubagentRuntimeKind>> = {
  nxs: "nxs",
  go: "nxs",
  "go-native": "nxs",
  gonative: "nxs",
  claude: "claude",
  cc: "claude",
  "claude-code": "claude",
  claudecode: "claude",
  mixed: "mixed",
};

function normalizeSubagentTaskStatus(status?: string | null): SubagentTaskViewStatus {
  return SUBAGENT_STATUS_BY_ALIAS[normalizeAlias(status)] ?? "pending";
}

export function isSubagentTaskActive(task: SubagentTask): boolean {
  const status = normalizeSubagentTaskStatus(task.status);
  return status === "pending" || status === "running";
}

export function canSendSubagentTaskMessage(task: SubagentTask): boolean {
  if (!task.capabilities.send_message) {
    return false;
  }
  if (isSubagentTaskActive(task)) {
    return true;
  }
  return task.capabilities.resume;
}

export function canStopSubagentTask(task: SubagentTask): boolean {
  return task.capabilities.stop && isSubagentTaskActive(task);
}

export function subagentTaskTitle(task: SubagentTask): string {
  return task.name?.trim() || task.agent_type?.trim() || "Subagent";
}

export function subagentTaskSourceKey(source: SubagentTaskSource | null): string {
  if (!source) {
    return "";
  }
  if (source.kind === "session") {
    return `session:${source.session_key}`;
  }
  return `room:${source.room_id}:${source.conversation_id}`;
}

export function subagentTaskErrorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

export function normalizeSubagentTaskListResponse(
  response: SubagentTaskListResponse,
): SubagentTaskListResponse {
  const runtimeKind = normalizeSubagentRuntimeKind(response.runtime_kind);
  const capabilities = normalizeSubagentTaskCapabilities(response.capabilities);
  return {
    runtime_kind: runtimeKind,
    capabilities,
    items: (response.items ?? []).map((task) =>
      normalizeSubagentTask(task, runtimeKind, capabilities),
    ),
  };
}

export function normalizeSubagentTask(
  task: SubagentTask,
  fallbackRuntimeKind: SubagentRuntimeKind = "unknown",
  fallbackCapabilities: SubagentTaskCapabilities = EMPTY_CAPABILITIES,
): SubagentTask {
  return {
    ...task,
    runtime_kind: normalizeSubagentRuntimeKind(
      task.runtime_kind ?? fallbackRuntimeKind,
    ),
    capabilities: normalizeSubagentTaskCapabilities(
      task.capabilities,
      fallbackCapabilities,
    ),
  };
}

function normalizeSubagentRuntimeKind(
  value?: string | null,
): SubagentRuntimeKind {
  return SUBAGENT_RUNTIME_BY_ALIAS[normalizeAlias(value)] ?? "unknown";
}

function normalizeSubagentTaskCapabilities(
  value?: Partial<SubagentTaskCapabilities> | null,
  fallback: SubagentTaskCapabilities = EMPTY_CAPABILITIES,
): SubagentTaskCapabilities {
  return {
    observe: value?.observe ?? fallback.observe,
    transcript: value?.transcript ?? fallback.transcript,
    stop: value?.stop ?? fallback.stop,
    send_message: value?.send_message ?? fallback.send_message,
    resume: value?.resume ?? fallback.resume,
  };
}

export function subagentTaskTimestamp(task: SubagentTask): number {
  return normalizeTimestamp(task.updated_at) ?? normalizeTimestamp(task.started_at) ?? 0;
}

export function subagentTaskAvatarColor(taskId: string): string {
  return SUBAGENT_AVATAR_PALETTE[stableHash(taskId) % SUBAGENT_AVATAR_PALETTE.length];
}

/** 给 Thread 消息轨道提供与列表一致的稳定头像。 */
export function subagentTaskAvatarDataUrl(taskId: string): string {
  const color = subagentTaskAvatarColor(taskId);
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><circle cx="16" cy="16" r="16" fill="${color}"/><g fill="none" stroke="white" stroke-opacity=".72" stroke-width="5" stroke-linecap="round"><path d="M16 5v22"/><path d="M5 16h22"/><path d="m8.2 8.2 15.6 15.6"/><path d="m23.8 8.2-15.6 15.6"/></g><circle cx="16" cy="16" r="5" fill="${color}" fill-opacity=".82" stroke="white" stroke-opacity=".7"/></svg>`;
  return `data:image/svg+xml,${encodeURIComponent(svg)}`;
}

function normalizeTimestamp(value?: number): number | null {
  if (!value || value <= 0) {
    return null;
  }
  return value < 1_000_000_000_000 ? value * 1000 : value;
}

function stableHash(value: string): number {
  let hash = 0;
  for (let index = 0; index < value.length; index += 1) {
    hash = (hash * 31 + value.charCodeAt(index)) >>> 0;
  }
  return hash;
}

function normalizeAlias(value?: string | null): string {
  return value?.trim().toLowerCase() ?? "";
}
