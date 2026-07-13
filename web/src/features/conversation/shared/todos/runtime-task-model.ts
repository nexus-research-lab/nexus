import type { SystemMessage } from "@/types/conversation/message/entity";
import type { TaskProgressContent } from "@/types/conversation/message/content";
import type { TodoItem } from "@/types/conversation/todo";

import { inferSystemTaskStatus, inferTaskProgressStatus } from "./todo-status-model";

const SYSTEM_TASK_SUBTYPES = new Set([
  "task_started",
  "task_notification",
  "task_updated",
]);

interface RuntimeTaskCandidate {
  contentCandidates: Array<string | null | undefined>;
  resolveStatus: (fallback?: TodoItem["status"]) => TodoItem["status"];
  taskId: string | null;
}

function normalizeTaskContent(description?: string): string {
  const value = description?.trim() ?? "";
  const separatorIndex = value.indexOf(":");
  if (separatorIndex >= 0 && separatorIndex < value.length - 1) {
    return value.slice(separatorIndex + 1).trim();
  }
  return value;
}

function metadataRecord(value: unknown): Record<string, unknown> {
  return typeof value === "object" && value !== null
    ? (value as Record<string, unknown>)
    : {};
}

function metadataString(value: unknown): string | null {
  return typeof value === "string" && value.trim() ? value.trim() : null;
}

function firstMetadataString(values: unknown[]): string | null {
  return values
    .map(metadataString)
    .find((value): value is string => value !== null) ?? null;
}

function firstTaskContent(
  candidates: Array<string | null | undefined>,
): string | null {
  for (const candidate of candidates) {
    const content = normalizeTaskContent(candidate ?? undefined);
    if (content) {
      return content;
    }
  }
  return null;
}

export function upsertSystemRuntimeTask(
  tasksById: Map<string, TodoItem>,
  message: SystemMessage,
): boolean {
  return upsertRuntimeTask(tasksById, systemRuntimeTaskCandidate(message));
}

export function upsertAssistantRuntimeTask(
  tasksById: Map<string, TodoItem>,
  block: TaskProgressContent,
): boolean {
  return upsertRuntimeTask(tasksById, assistantRuntimeTaskCandidate(block));
}

function systemRuntimeTaskCandidate(
  message: SystemMessage,
): RuntimeTaskCandidate | null {
  const metadata = message.metadata;
  const subtype = metadata?.subtype;
  if (!metadata || !subtype || !SYSTEM_TASK_SUBTYPES.has(subtype)) {
    return null;
  }

  const patch = metadataRecord(metadata.patch);
  const status = firstMetadataString([metadata.status, patch.status]);
  return {
    contentCandidates: [
      metadataString(patch.description),
      metadataString(metadata.description),
      message.content,
    ],
    resolveStatus: (fallback) =>
      inferSystemTaskStatus(subtype, status, fallback),
    taskId:
      firstMetadataString([metadata.task_id, metadata.tool_use_id]) ??
      message.message_id,
  };
}

function assistantRuntimeTaskCandidate(
  block: TaskProgressContent,
): RuntimeTaskCandidate {
  return {
    contentCandidates: [block.description],
    resolveStatus: (fallback) => inferTaskProgressStatus(block, fallback),
    taskId: metadataString(block.task_id),
  };
}

function upsertRuntimeTask(
  tasksById: Map<string, TodoItem>,
  candidate: RuntimeTaskCandidate | null,
): boolean {
  if (!candidate?.taskId) {
    return false;
  }
  const existing = tasksById.get(candidate.taskId);
  const content =
    firstTaskContent(candidate.contentCandidates) ?? existing?.content;
  if (!content) {
    return false;
  }
  tasksById.set(candidate.taskId, {
    content,
    status: candidate.resolveStatus(existing?.status),
    active_form: existing?.active_form,
  });
  return true;
}

export function mergeTodoPlanWithRuntimeTasks(
  plan: TodoItem[],
  runtimeTasks: TodoItem[],
): TodoItem[] {
  if (runtimeTasks.length === 0) {
    return plan;
  }

  const runtimeByContent = new Map(
    runtimeTasks.map((task) => [normalizeTodoContent(task.content), task]),
  );
  const mergedContent = new Set<string>();
  const mergedPlan = plan.map((todo) => {
    const contentKey = normalizeTodoContent(todo.content);
    mergedContent.add(contentKey);
    const runtimeTask = runtimeByContent.get(contentKey);
    return runtimeTask
      ? {
          ...todo,
          active_form: runtimeTask.active_form ?? todo.active_form,
          status: runtimeTask.status,
        }
      : todo;
  });

  for (const runtimeTask of runtimeTasks) {
    if (!mergedContent.has(normalizeTodoContent(runtimeTask.content))) {
      mergedPlan.push(runtimeTask);
    }
  }
  return mergedPlan;
}

export function completeOrphanRuntimeTasks(
  tasks: TodoItem[],
  isRoundComplete: boolean,
): TodoItem[] {
  if (!isRoundComplete) {
    return tasks;
  }
  return tasks.map((task) => task.status === "completed"
    ? task
    : {...task, status: "completed"});
}

function normalizeTodoContent(content: string): string {
  return content.replace(/\s+/g, " ").trim().toLowerCase();
}
