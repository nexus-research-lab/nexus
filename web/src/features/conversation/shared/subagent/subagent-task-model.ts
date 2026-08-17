import type {
  SubagentRuntimeKind,
  SubagentTask,
  SubagentTaskCapabilities,
  SubagentTaskListResponse,
  SubagentTaskSource,
} from "@/types/conversation/subagent-task";
import type { EventMessage } from "@/types/generated/protocol";
import { isStringArray } from "@/lib/unknown-value";

type SubagentTaskViewStatus =
  | "pending"
  | "running"
  | "completed"
  | "stopped"
  | "failed";

const EMPTY_CAPABILITIES: SubagentTaskCapabilities = {
  observe: false,
  transcript: false,
  stop: false,
  send_message: false,
  resume: false,
};
const SUBAGENT_CAPABILITY_KEYS = [
  "observe",
  "transcript",
  "stop",
  "send_message",
  "resume",
] as const satisfies readonly (keyof SubagentTaskCapabilities)[];

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
  return task.status.trim().toLowerCase() !== "deleted"
    && task.capabilities.send_message
    && task.capabilities.resume;
}

export function subagentTaskTitle(task: SubagentTask): string {
  return (
    task.name?.trim() ||
    task.description?.trim() ||
    task.agent_type?.trim() ||
    "Subagent"
  );
}

/** 子智能体头像优先沿用拉起工具身份，旧记录回退任务身份。 */
export function subagentTaskAvatarSeed(
  task: Pick<SubagentTask, "task_id" | "tool_use_id">,
): string {
  return task.tool_use_id?.trim() || task.task_id;
}

export function findSubagentTaskByToolUseId(
  tasks: readonly SubagentTask[],
  toolUseId?: string | null,
): SubagentTask | null {
  const normalizedToolUseId = toolUseId?.trim() ?? "";
  if (!normalizedToolUseId) {
    return null;
  }
  return tasks.find((task) => (
    task.tool_use_id?.trim() === normalizedToolUseId
    || task.task_id === normalizedToolUseId
  )) ?? null;
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

export function isSubagentTaskChangeFor(
  event: EventMessage,
  source: SubagentTaskSource,
  taskId?: string | null,
  hostAgentId?: string | null,
): boolean {
  if (event.event_type !== "subagent_task_changed") {
    return false;
  }
  if (source.kind === "session") {
    if (event.session_key !== source.session_key) {
      return false;
    }
  } else if (
    event.room_id !== source.room_id
    || event.conversation_id !== source.conversation_id
  ) {
    return false;
  }
  const normalizedHostAgentId = hostAgentId?.trim() ?? "";
  if (normalizedHostAgentId && event.agent_id?.trim() !== normalizedHostAgentId) {
    return false;
  }
  const taskIDs = event.data.task_ids;
  if (!isStringArray(taskIDs) || taskIDs.length === 0) {
    return false;
  }
  const normalizedTaskId = taskId?.trim() ?? "";
  return !normalizedTaskId || taskIDs.includes(normalizedTaskId);
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
  const capabilities = { ...fallback };
  for (const key of SUBAGENT_CAPABILITY_KEYS) {
    capabilities[key] = value?.[key] ?? fallback[key];
  }
  return capabilities;
}

export function subagentTaskTimestamp(task: SubagentTask): number {
  return normalizeTimestamp(task.updated_at) ?? normalizeTimestamp(task.started_at) ?? 0;
}

export function preferFreshSubagentTask(
  sourceTask: SubagentTask,
  detailTask?: SubagentTask | null,
): SubagentTask {
  if (!detailTask) {
    return sourceTask;
  }
  const sourceTimestamp = subagentTaskTimestamp(sourceTask);
  const detailTimestamp = subagentTaskTimestamp(detailTask);
  if (detailTimestamp > sourceTimestamp) {
    return detailTask;
  }
  if (detailTimestamp < sourceTimestamp) {
    return sourceTask;
  }
  // Equal-timestamp status edges occur in historical runtime projections.
  // Prefer terminal over active so a stale read cannot revive completed work.
  if (isSubagentTaskActive(detailTask) && !isSubagentTaskActive(sourceTask)) {
    return sourceTask;
  }
  return detailTask;
}

function normalizeTimestamp(value?: number): number | null {
  if (!value || value <= 0) {
    return null;
  }
  return value < 1_000_000_000_000 ? value * 1000 : value;
}

function normalizeAlias(value?: string | null): string {
  return value?.trim().toLowerCase() ?? "";
}
