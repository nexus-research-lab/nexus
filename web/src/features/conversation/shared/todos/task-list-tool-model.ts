import { areEquivalentSessionKeys } from "@/lib/conversation/session-key";
import type {
  ToolResultContent,
  ToolUseContent,
} from "@/types/conversation/message/content";
import type { Message } from "@/types/conversation/message/entity";
import type { TodoItem } from "@/types/conversation/todo";

import {
  isTaskListToolName,
  type TaskListToolName,
} from "./task-tool-names";

interface TaskListItem extends TodoItem {
  id: string;
}

interface TaskToolCall {
  input: Record<string, unknown>;
  name: TaskListToolName;
}

export interface TaskListToolProjection {
  observed: boolean;
  todos: TodoItem[];
}

interface TaskListSnapshot {
  recognized: boolean;
  tasks: TaskListItem[];
}

const TASK_CREATE_RESULT_PATTERN = /^Task #(\S+) created successfully(?::\s*(.*))?$/i;
const TASK_LIST_LINE_PATTERN = /^#(\S+)\s+\[(pending|in_progress|completed)]\s+(.+)$/i;

export function projectTaskListToolTodos(
  messages: Message[],
  sessionKey: string,
): TaskListToolProjection {
  const tasksById = new Map<string, TaskListItem>();
  const toolCallsById = new Map<string, TaskToolCall>();
  const runtimeSessionId = latestRuntimeSessionId(messages, sessionKey);
  let observed = false;

  for (const message of messages) {
    if (
      message.role !== "assistant"
      || !isSameSessionMessage(message, sessionKey)
      || (runtimeSessionId !== null && message.session_id !== runtimeSessionId)
      || !Array.isArray(message.content)
    ) {
      continue;
    }
    for (const block of message.content) {
      if (block.type === "tool_use" && isTaskListToolName(block.name)) {
        observed = true;
        indexTaskToolUse(tasksById, toolCallsById, block);
        continue;
      }
      if (block.type !== "tool_result") {
        continue;
      }
      const toolCall = toolCallsById.get(block.tool_use_id);
      if (!toolCall) {
        continue;
      }
      observed = true;
      applyTaskToolResult(tasksById, block, toolCall, block.tool_use_id);
    }
  }

  return {
    observed,
    todos: [...tasksById.values()].map(({id: _id, ...todo}) => todo),
  };
}

function latestRuntimeSessionId(
  messages: Message[],
  sessionKey: string,
): string | null {
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const message = messages[index];
    if (
      message?.role === "assistant"
      && isSameSessionMessage(message, sessionKey)
      && message.session_id
    ) {
      return message.session_id;
    }
  }
  return null;
}

function isSameSessionMessage(message: Message, sessionKey: string): boolean {
  return !message.session_key
    || areEquivalentSessionKeys(message.session_key, sessionKey);
}

function indexTaskToolUse(
  tasksById: Map<string, TaskListItem>,
  toolCallsById: Map<string, TaskToolCall>,
  block: ToolUseContent,
): void {
  if (!isTaskListToolName(block.name)) {
    return;
  }
  toolCallsById.set(block.id, {input: block.input, name: block.name});
  if (block.name !== "TaskCreate") {
    return;
  }
  const subject = firstString(block.input, ["subject", "title"]);
  if (!subject) {
    return;
  }
  tasksById.set(optimisticTaskId(block.id), {
    active_form: firstString(block.input, ["activeForm", "active_form"])
      ?? undefined,
    content: subject,
    id: optimisticTaskId(block.id),
    status: "pending",
  });
}

function applyTaskToolResult(
  tasksById: Map<string, TaskListItem>,
  block: ToolResultContent,
  call: TaskToolCall,
  toolUseId: string,
): void {
  const handlers: Record<TaskListToolName, () => void> = {
    TaskCreate: () => applyTaskCreateResult(tasksById, block, call.input, toolUseId),
    TaskList: () => applyTaskListResult(tasksById, block),
    TaskUpdate: () => applyTaskUpdateResult(tasksById, block, call.input),
  };
  handlers[call.name]();
}

function applyTaskCreateResult(
  tasksById: Map<string, TaskListItem>,
  block: ToolResultContent,
  input: Record<string, unknown>,
  toolUseId: string,
): void {
  const temporaryId = optimisticTaskId(toolUseId);
  if (block.is_error) {
    tasksById.delete(temporaryId);
    return;
  }

  const output = recordValue(block.structured_output);
  const task = recordValue(output?.task);
  const textMatch = toolResultText(block.content).match(TASK_CREATE_RESULT_PATTERN);
  const taskId = firstString(task, ["id"]) ?? textMatch?.[1]?.trim();
  if (!taskId) {
    return;
  }
  const current = tasksById.get(temporaryId);
  const content = firstString(task, ["subject"])
    ?? firstString(input, ["subject", "title"])
    ?? textMatch?.[2]?.trim()
    ?? current?.content;
  if (!content) {
    return;
  }

  tasksById.delete(temporaryId);
  const existing = tasksById.get(taskId);
  tasksById.set(taskId, {
    active_form: firstString(input, ["activeForm", "active_form"])
      ?? current?.active_form
      ?? existing?.active_form,
    content,
    id: taskId,
    status: existing?.status ?? "pending",
  });
}

function applyTaskListResult(
  tasksById: Map<string, TaskListItem>,
  block: ToolResultContent,
): void {
  if (block.is_error) {
    return;
  }
  const snapshot = taskListSnapshot(block);
  if (!snapshot.recognized) {
    return;
  }

  const nextTasks = new Map<string, TaskListItem>();
  for (const task of snapshot.tasks) {
    nextTasks.set(task.id, {
      ...task,
      active_form: task.active_form ?? tasksById.get(task.id)?.active_form,
    });
  }
  tasksById.clear();
  nextTasks.forEach((task, taskId) => tasksById.set(taskId, task));
}

function applyTaskUpdateResult(
  tasksById: Map<string, TaskListItem>,
  block: ToolResultContent,
  input: Record<string, unknown>,
): void {
  const output = recordValue(block.structured_output);
  const text = toolResultText(block.content);
  if (block.is_error || output?.success === false || /task not found/i.test(text)) {
    return;
  }

  const taskId = firstString(output, ["taskId", "task_id"])
    ?? firstString(input, ["taskId", "task_id", "id"]);
  if (!taskId) {
    return;
  }
  const status = taskStatus(firstString(input, ["status"]));
  if (firstString(input, ["status"]) === "deleted") {
    tasksById.delete(taskId);
    return;
  }

  const current = tasksById.get(taskId);
  const content = firstString(input, ["subject", "title"])
    ?? current?.content;
  if (!content) {
    return;
  }
  tasksById.set(taskId, {
    active_form: firstString(input, ["activeForm", "active_form"])
      ?? current?.active_form,
    content,
    id: taskId,
    status: status ?? current?.status ?? "pending",
  });
}

function taskListSnapshot(block: ToolResultContent): TaskListSnapshot {
  const output = recordValue(block.structured_output);
  if (Array.isArray(output?.tasks)) {
    return {
      recognized: true,
      tasks: output.tasks.flatMap((value) => {
        const task = taskListItem(value);
        return task ? [task] : [];
      }),
    };
  }

  const text = toolResultText(block.content).trim();
  if (/^No tasks found$/i.test(text)) {
    return {recognized: true, tasks: []};
  }
  const tasks = text.split("\n").flatMap((line) => {
    const match = line.trim().match(TASK_LIST_LINE_PATTERN);
    const status = taskStatus(match?.[2]);
    const content = match?.[3]?.trim();
    return match?.[1] && status && content
      ? [{content, id: match[1], status}]
      : [];
  });
  return {recognized: tasks.length > 0, tasks};
}

function taskListItem(value: unknown): TaskListItem | null {
  const task = recordValue(value);
  const id = firstString(task, ["id"]);
  const content = firstString(task, ["subject", "title"]);
  const status = taskStatus(firstString(task, ["status"]));
  if (!id || !content || !status) {
    return null;
  }
  return {
    active_form: firstString(task, ["activeForm", "active_form"])
      ?? undefined,
    content,
    id,
    status,
  };
}

function taskStatus(value: string | null | undefined): TodoItem["status"] | null {
  switch (value?.trim().toLowerCase()) {
    case "completed":
      return "completed";
    case "in_progress":
      return "in_progress";
    case "pending":
      return "pending";
    default:
      return null;
  }
}

function firstString(
  record: Record<string, unknown> | null,
  keys: string[],
): string | null {
  if (!record) {
    return null;
  }
  for (const key of keys) {
    const value = record[key];
    if (typeof value === "string" && value.trim()) {
      return value.trim();
    }
  }
  return null;
}

function recordValue(value: unknown): Record<string, unknown> | null {
  return typeof value === "object" && value !== null && !Array.isArray(value)
    ? value as Record<string, unknown>
    : null;
}

function toolResultText(content: ToolResultContent["content"]): string {
  if (typeof content === "string") {
    return content;
  }
  return content.flatMap((value) => {
    if (typeof value === "string") {
      return [value];
    }
    const record = recordValue(value);
    const text = firstString(record, ["text"]);
    return text ? [text] : [];
  }).join("\n");
}

function optimisticTaskId(toolUseId: string): string {
  return `tool:${toolUseId}`;
}
