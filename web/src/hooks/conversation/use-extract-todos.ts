import { useMemo, useRef } from "react";
import { areEquivalentSessionKeys } from "@/lib/conversation/session-key";
import { AssistantMessage, Message, SystemMessage, TaskProgressContent } from "@/types/conversation/message";
import { TodoItem } from "@/types/conversation/todo";

function isSameSessionMessage(message: Message, externalSessionKey: string): boolean {
  return !message.session_key || areEquivalentSessionKeys(message.session_key, externalSessionKey);
}

function isSameTodo(left: TodoItem, right: TodoItem): boolean {
  return (
    left.content === right.content &&
    left.status === right.status &&
    left.active_form === right.active_form
  );
}

function areTodosEqual(left: TodoItem[], right: TodoItem[]): boolean {
  if (left === right) {
    return true;
  }
  if (left.length !== right.length) {
    return false;
  }
  return left.every((item, index) => {
    const rightItem = right[index];
    return Boolean(rightItem && isSameTodo(item, rightItem));
  });
}

export const useExtractTodos = (
  messages: Message[],
  externalSessionKey: string | null
) => {
  const stableTodosRef = useRef<TodoItem[]>([]);

  const computedTodos = useMemo(() => {
    if (!externalSessionKey || messages.length === 0) {
      return [];
    }

    let latestTodos: TodoItem[] = [];
    let latestTodoRoundId: string | null = null;
    let latestTodoIndex = -1;
    let found = false;

    for (let i = messages.length - 1; i >= 0; i--) {
      const msg = messages[i];
      if (!isSameSessionMessage(msg, externalSessionKey)) {
        continue;
      }

      if (msg.role === "assistant" && Array.isArray(msg.content)) {
        for (const block of msg.content) {
          if (!block) {
            continue;
          }
          if (block.type === "tool_use" && block.name === "TodoWrite") {
            if (block.input && Array.isArray(block.input.todos)) {
              latestTodos = block.input.todos;
              latestTodoRoundId = msg.round_id;
              latestTodoIndex = i;
              found = true;
            }
          }
        }
        if (!found) {
          const runtimeTaskTodos = extractRuntimeTaskTodosForRound(messages, msg.round_id, externalSessionKey);
          if (runtimeTaskTodos.length > 0) {
            return completeOrphanTasksWhenRoundDone(
              runtimeTaskTodos,
              isRoundCompleted(messages, msg.round_id, externalSessionKey),
            );
          }
        }
      }

      if (msg.role === "system" && !found) {
        const runtimeTaskTodos = extractRuntimeTaskTodosForRound(messages, msg.round_id, externalSessionKey);
        const roundTodos = extractLatestTodosForRound(messages, msg.round_id, externalSessionKey);
        if (roundTodos && roundTodos.length > 0) {
          return mergeTodosWithRuntimeTasks(roundTodos, runtimeTaskTodos);
        }
        if (runtimeTaskTodos.length > 0) {
          return completeOrphanTasksWhenRoundDone(
            runtimeTaskTodos,
            isRoundCompleted(messages, msg.round_id, externalSessionKey),
          );
        }
      }

      if (found) {
        break;
      }
    }

    if (!found || latestTodos.length === 0 || !latestTodoRoundId) {
      return [];
    }

    const roundAssistantWithSummary = [...messages]
      .reverse()
      .find((msg): msg is AssistantMessage =>
        msg.role === "assistant"
        && msg.round_id === latestTodoRoundId
        && isSameSessionMessage(msg, externalSessionKey)
        && Boolean(msg.result_summary)
      );

    if (roundAssistantWithSummary?.result_summary?.is_error) {
      return [];
    }

    const hasLaterRoundMessage = messages.slice(latestTodoIndex + 1).some((msg) =>
      isSameSessionMessage(msg, externalSessionKey)
      && msg.round_id
      && msg.round_id !== latestTodoRoundId
      && msg.role !== "system"
    );

    if (hasLaterRoundMessage && !roundAssistantWithSummary?.result_summary) {
      return [];
    }

    const runtimeTaskTodos = extractRuntimeTaskTodosForRound(
      messages,
      latestTodoRoundId,
      externalSessionKey,
    );
    return mergeTodosWithRuntimeTasks(latestTodos, runtimeTaskTodos);
  }, [externalSessionKey, messages]);

  if (!areTodosEqual(stableTodosRef.current, computedTodos)) {
    stableTodosRef.current = computedTodos;
  }

  return stableTodosRef.current;
};

function mergeTodosWithRuntimeTasks(
  todos: TodoItem[],
  runtimeTaskTodos: TodoItem[],
): TodoItem[] {
  if (runtimeTaskTodos.length === 0) {
    return todos;
  }
  const progressByContent = new Map<string, TodoItem>();
  for (const item of runtimeTaskTodos) {
    progressByContent.set(normalizeTodoContent(item.content), item);
  }
  const seenContents = new Set<string>();
  const merged = todos.map((todo) => {
    const normalizedContent = normalizeTodoContent(todo.content);
    seenContents.add(normalizedContent);
    const progress = progressByContent.get(normalizedContent);
    if (!progress) {
      return todo;
    }
    return {
      ...todo,
      active_form: progress.active_form ?? todo.active_form,
      status: progress.status,
    };
  });
  for (const progress of runtimeTaskTodos) {
    const normalizedContent = normalizeTodoContent(progress.content);
    if (!seenContents.has(normalizedContent)) {
      merged.push(progress);
    }
  }
  return merged;
}

function extractLatestTodosForRound(
  messages: Message[],
  roundId: string | undefined,
  externalSessionKey: string,
): TodoItem[] | null {
  if (!roundId) {
    return null;
  }
  let latestTodos: TodoItem[] | null = null;
  for (const msg of messages) {
    if (
      msg.role !== "assistant" ||
      msg.round_id !== roundId ||
      !isSameSessionMessage(msg, externalSessionKey) ||
      !Array.isArray(msg.content)
    ) {
      continue;
    }
    for (const block of msg.content) {
      if (block?.type === "tool_use" && block.name === "TodoWrite") {
        if (block.input && Array.isArray(block.input.todos)) {
          latestTodos = block.input.todos;
        }
      }
    }
  }
  return latestTodos;
}

function extractRuntimeTaskTodosForRound(
  messages: Message[],
  roundId: string | undefined,
  externalSessionKey: string,
): TodoItem[] {
  if (!roundId) {
    return [];
  }
  const tasksById = new Map<string, TodoItem>();
  for (const msg of messages) {
    if (msg.round_id !== roundId || !isSameSessionMessage(msg, externalSessionKey)) {
      continue;
    }
    if (msg.role === "system") {
      upsertSystemTaskTodo(tasksById, msg);
      continue;
    }
    if (msg.role !== "assistant" || !Array.isArray(msg.content)) {
      continue;
    }
    for (const block of msg.content) {
      if (!block || block.type !== "task_progress") {
        continue;
      }
      upsertTaskProgressTodo(tasksById, block);
    }
  }
  return [...tasksById.values()];
}

// 该轮已产出最终回复（非错误）即视为结束——其下子 Agent 必然已跑完。
function isRoundCompleted(
  messages: Message[],
  roundId: string | undefined,
  externalSessionKey: string,
): boolean {
  if (!roundId) {
    return false;
  }
  return messages.some((msg) =>
    msg.role === "assistant" &&
    msg.round_id === roundId &&
    isSameSessionMessage(msg, externalSessionKey) &&
    Boolean(msg.result_summary && !msg.result_summary.is_error),
  );
}

// 轮次结束后，把独立展示的子 Agent 任务里残留的「运行中/待执行」归为完成，
// 避免任务条永久转圈。仅用于无 TodoWrite 治理的 orphan 场景，不动真实计划项。
function completeOrphanTasksWhenRoundDone(
  tasks: TodoItem[],
  roundCompleted: boolean,
): TodoItem[] {
  if (!roundCompleted) {
    return tasks;
  }
  return tasks.map((task) =>
    task.status === "completed" ? task : { ...task, status: "completed" },
  );
}

function upsertSystemTaskTodo(
  tasksById: Map<string, TodoItem>,
  message: SystemMessage,
) {
  const metadata = message.metadata;
  if (!metadata) {
    return;
  }
  const subtype = metadata.subtype;
  if (
    subtype !== "task_started" &&
    subtype !== "task_notification" &&
    subtype !== "task_updated"
  ) {
    return;
  }
  const patch = recordFromTaskMetadata(metadata.patch);
  const taskId =
    stringFromTaskMetadata(metadata.task_id) ??
    stringFromTaskMetadata(metadata.tool_use_id) ??
    message.message_id;
  const existing = tasksById.get(taskId);
  const description =
    normalizeTaskProgressContent(stringFromTaskMetadata(patch?.description) ?? undefined) ||
    normalizeTaskProgressContent(stringFromTaskMetadata(metadata.description) ?? undefined) ||
    normalizeTaskProgressContent(message.content) ||
    existing?.content;

  if (!description) {
    return;
  }

  tasksById.set(taskId, {
    content: description,
    status: inferSystemTaskStatus(
      subtype,
      stringFromTaskMetadata(metadata.status) ?? stringFromTaskMetadata(patch?.status),
      existing?.status,
    ),
    active_form: existing?.active_form,
  });
}

function upsertTaskProgressTodo(
  tasksById: Map<string, TodoItem>,
  block: TaskProgressContent,
) {
  const taskId = block.task_id?.trim();
  if (!taskId) {
    return;
  }
  const existing = tasksById.get(taskId);
  const content = normalizeTaskProgressContent(block.description) || existing?.content;
  if (!content) {
    return;
  }
  tasksById.set(taskId, {
    content,
    status: inferTaskProgressStatus(block, existing?.status),
    active_form: existing?.active_form,
  });
}

function normalizeTaskProgressContent(description: string | undefined): string {
  const value = description?.trim() ?? "";
  if (!value) {
    return "";
  }
  const colonIndex = value.indexOf(":");
  if (colonIndex >= 0 && colonIndex < value.length - 1) {
    return value.slice(colonIndex + 1).trim();
  }
  return value;
}

function normalizeTodoContent(content: string): string {
  return content.replace(/\s+/g, " ").trim().toLowerCase();
}

function recordFromTaskMetadata(value: unknown): Record<string, unknown> {
  return typeof value === "object" && value !== null ? value as Record<string, unknown> : {};
}

function stringFromTaskMetadata(value: unknown): string | null {
  return typeof value === "string" && value.trim() ? value.trim() : null;
}

function inferSystemTaskStatus(
  subtype: string,
  status: string | null,
  fallback: TodoItem["status"] | undefined,
): TodoItem["status"] {
  const normalizedStatus = status?.toLowerCase().trim() ?? "";
  if (
    normalizedStatus === "completed" ||
    normalizedStatus === "complete" ||
    normalizedStatus === "success" ||
    normalizedStatus === "done" ||
    normalizedStatus === "stopped" ||
    normalizedStatus === "cancelled" ||
    normalizedStatus === "canceled" ||
    normalizedStatus === "killed" ||
    normalizedStatus === "interrupted" ||
    normalizedStatus === "failed" ||
    normalizedStatus === "error"
  ) {
    return "completed";
  }
  if (
    normalizedStatus === "pending" ||
    normalizedStatus === "queued" ||
    normalizedStatus === "created"
  ) {
    return "pending";
  }
  if (
    normalizedStatus === "running" ||
    normalizedStatus === "in_progress" ||
    normalizedStatus === "in progress" ||
    normalizedStatus === "started"
  ) {
    return "in_progress";
  }
  // taskNotification 表示子任务已回报最终结果 = 终态，须优先于运行中的 fallback。
  if (subtype === "task_notification") {
    return "completed";
  }
  if (fallback) {
    return fallback;
  }
  return "in_progress";
}

function inferTaskProgressStatus(
  block: TaskProgressContent,
  fallback: TodoItem["status"] | undefined,
): TodoItem["status"] {
  const text = `${block.last_tool_name ?? ""} ${block.description ?? ""}`.toLowerCase();
  if (
    text.includes("completed") ||
    text.includes("complete") ||
    text.includes("finished") ||
    text.includes("done") ||
    text.includes("已完成") ||
    text.includes("完成")
  ) {
    return "completed";
  }
  if (
    text.includes("in_progress") ||
    text.includes("in progress") ||
    text.includes("running") ||
    text.includes("正在") ||
    text.includes("处理中")
  ) {
    return "in_progress";
  }
  if (fallback) {
    return fallback;
  }
  return block.last_tool_name === "TaskCreate" ? "pending" : "in_progress";
}
