import type { SystemMessage } from "@/types/conversation/message/entity";
import type { TaskProgressContent } from "@/types/conversation/message/content";
import type { TodoItem } from "@/types/conversation/todo";

import { inferSystemTaskStatus, inferTaskProgressStatus } from "./todo-status-model";

const SYSTEM_TASK_SUBTYPES = new Set([
  "task_started",
  "task_notification",
  "task_updated",
]);

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
    ? value as Record<string, unknown>
    : {};
}

function metadataString(value: unknown): string | null {
  return typeof value === "string" && value.trim() ? value.trim() : null;
}

export function upsertSystemRuntimeTask(
  tasksById: Map<string, TodoItem>,
  message: SystemMessage,
): boolean {
  const metadata = message.metadata;
  const subtype = metadata?.subtype;
  if (!metadata || !subtype || !SYSTEM_TASK_SUBTYPES.has(subtype)) {
    return false;
  }

  const patch = metadataRecord(metadata.patch);
  const taskId = metadataString(metadata.task_id)
    ?? metadataString(metadata.tool_use_id)
    ?? message.message_id;
  const existing = tasksById.get(taskId);
  const content = normalizeTaskContent(metadataString(patch.description) ?? undefined)
    || normalizeTaskContent(metadataString(metadata.description) ?? undefined)
    || normalizeTaskContent(message.content)
    || existing?.content;
  if (!content) {
    return false;
  }

  tasksById.set(taskId, {
    content,
    status: inferSystemTaskStatus(
      subtype,
      metadataString(metadata.status) ?? metadataString(patch.status),
      existing?.status,
    ),
    active_form: existing?.active_form,
  });
  return true;
}

export function upsertAssistantRuntimeTask(
  tasksById: Map<string, TodoItem>,
  block: TaskProgressContent,
): boolean {
  const taskId = block.task_id?.trim();
  if (!taskId) {
    return false;
  }
  const existing = tasksById.get(taskId);
  const content = normalizeTaskContent(block.description) || existing?.content;
  if (!content) {
    return false;
  }
  tasksById.set(taskId, {
    content,
    status: inferTaskProgressStatus(block, existing?.status),
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
