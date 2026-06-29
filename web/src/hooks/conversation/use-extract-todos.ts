import { useMemo, useRef } from "react";
import { are_equivalent_session_keys } from "@/lib/conversation/session-key";
import { AssistantMessage, Message, SystemMessage, TaskProgressContent } from "@/types/conversation/message";
import { TodoItem } from "@/types/conversation/todo";

function is_same_session_message(message: Message, external_session_key: string): boolean {
  return !message.session_key || are_equivalent_session_keys(message.session_key, external_session_key);
}

function is_same_todo(left: TodoItem, right: TodoItem): boolean {
  return (
    left.content === right.content &&
    left.status === right.status &&
    left.active_form === right.active_form
  );
}

function are_todos_equal(left: TodoItem[], right: TodoItem[]): boolean {
  if (left === right) {
    return true;
  }
  if (left.length !== right.length) {
    return false;
  }
  return left.every((item, index) => {
    const right_item = right[index];
    return Boolean(right_item && is_same_todo(item, right_item));
  });
}

export const useExtractTodos = (
  messages: Message[],
  external_session_key: string | null
) => {
  const stable_todos_ref = useRef<TodoItem[]>([]);

  const computed_todos = useMemo(() => {
    if (!external_session_key || messages.length === 0) {
      return [];
    }

    let latestTodos: TodoItem[] = [];
    let latestTodoRoundId: string | null = null;
    let latestTodoIndex = -1;
    let found = false;

    for (let i = messages.length - 1; i >= 0; i--) {
      const msg = messages[i];
      if (!is_same_session_message(msg, external_session_key)) {
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
          const runtime_task_todos = extract_runtime_task_todos_for_round(messages, msg.round_id, external_session_key);
          if (runtime_task_todos.length > 0) {
            return complete_orphan_tasks_when_round_done(
              runtime_task_todos,
              is_round_completed(messages, msg.round_id, external_session_key),
            );
          }
        }
      }

      if (msg.role === "system" && !found) {
        const runtime_task_todos = extract_runtime_task_todos_for_round(messages, msg.round_id, external_session_key);
        const round_todos = extract_latest_todos_for_round(messages, msg.round_id, external_session_key);
        if (round_todos && round_todos.length > 0) {
          return merge_todos_with_runtime_tasks(round_todos, runtime_task_todos);
        }
        if (runtime_task_todos.length > 0) {
          return complete_orphan_tasks_when_round_done(
            runtime_task_todos,
            is_round_completed(messages, msg.round_id, external_session_key),
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
        && is_same_session_message(msg, external_session_key)
        && Boolean(msg.result_summary)
      );

    if (roundAssistantWithSummary?.result_summary?.is_error) {
      return [];
    }

    const hasLaterRoundMessage = messages.slice(latestTodoIndex + 1).some((msg) =>
      is_same_session_message(msg, external_session_key)
      && msg.round_id
      && msg.round_id !== latestTodoRoundId
      && msg.role !== "system"
    );

    if (hasLaterRoundMessage && !roundAssistantWithSummary?.result_summary) {
      return [];
    }

    const runtime_task_todos = extract_runtime_task_todos_for_round(
      messages,
      latestTodoRoundId,
      external_session_key,
    );
    return merge_todos_with_runtime_tasks(latestTodos, runtime_task_todos);
  }, [external_session_key, messages]);

  if (!are_todos_equal(stable_todos_ref.current, computed_todos)) {
    stable_todos_ref.current = computed_todos;
  }

  return stable_todos_ref.current;
};

function merge_todos_with_runtime_tasks(
  todos: TodoItem[],
  runtime_task_todos: TodoItem[],
): TodoItem[] {
  if (runtime_task_todos.length === 0) {
    return todos;
  }
  const progress_by_content = new Map<string, TodoItem>();
  for (const item of runtime_task_todos) {
    progress_by_content.set(normalize_todo_content(item.content), item);
  }
  const seen_contents = new Set<string>();
  const merged = todos.map((todo) => {
    const normalized_content = normalize_todo_content(todo.content);
    seen_contents.add(normalized_content);
    const progress = progress_by_content.get(normalized_content);
    if (!progress) {
      return todo;
    }
    return {
      ...todo,
      active_form: progress.active_form ?? todo.active_form,
      status: progress.status,
    };
  });
  for (const progress of runtime_task_todos) {
    const normalized_content = normalize_todo_content(progress.content);
    if (!seen_contents.has(normalized_content)) {
      merged.push(progress);
    }
  }
  return merged;
}

function extract_latest_todos_for_round(
  messages: Message[],
  round_id: string | undefined,
  external_session_key: string,
): TodoItem[] | null {
  if (!round_id) {
    return null;
  }
  let latest_todos: TodoItem[] | null = null;
  for (const msg of messages) {
    if (
      msg.role !== "assistant" ||
      msg.round_id !== round_id ||
      !is_same_session_message(msg, external_session_key) ||
      !Array.isArray(msg.content)
    ) {
      continue;
    }
    for (const block of msg.content) {
      if (block?.type === "tool_use" && block.name === "TodoWrite") {
        if (block.input && Array.isArray(block.input.todos)) {
          latest_todos = block.input.todos;
        }
      }
    }
  }
  return latest_todos;
}

function extract_runtime_task_todos_for_round(
  messages: Message[],
  round_id: string | undefined,
  external_session_key: string,
): TodoItem[] {
  if (!round_id) {
    return [];
  }
  const tasks_by_id = new Map<string, TodoItem>();
  for (const msg of messages) {
    if (msg.round_id !== round_id || !is_same_session_message(msg, external_session_key)) {
      continue;
    }
    if (msg.role === "system") {
      upsert_system_task_todo(tasks_by_id, msg);
      continue;
    }
    if (msg.role !== "assistant" || !Array.isArray(msg.content)) {
      continue;
    }
    for (const block of msg.content) {
      if (!block || block.type !== "task_progress") {
        continue;
      }
      upsert_task_progress_todo(tasks_by_id, block);
    }
  }
  return [...tasks_by_id.values()];
}

// 该轮已产出最终回复（非错误）即视为结束——其下子 Agent 必然已跑完。
function is_round_completed(
  messages: Message[],
  round_id: string | undefined,
  external_session_key: string,
): boolean {
  if (!round_id) {
    return false;
  }
  return messages.some((msg) =>
    msg.role === "assistant" &&
    msg.round_id === round_id &&
    is_same_session_message(msg, external_session_key) &&
    Boolean(msg.result_summary && !msg.result_summary.is_error),
  );
}

// 轮次结束后，把独立展示的子 Agent 任务里残留的「运行中/待执行」归为完成，
// 避免任务条永久转圈。仅用于无 TodoWrite 治理的 orphan 场景，不动真实计划项。
function complete_orphan_tasks_when_round_done(
  tasks: TodoItem[],
  round_completed: boolean,
): TodoItem[] {
  if (!round_completed) {
    return tasks;
  }
  return tasks.map((task) =>
    task.status === "completed" ? task : { ...task, status: "completed" },
  );
}

function upsert_system_task_todo(
  tasks_by_id: Map<string, TodoItem>,
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
  const patch = record_from_task_metadata(metadata.patch);
  const task_id =
    string_from_task_metadata(metadata.task_id) ??
    string_from_task_metadata(metadata.tool_use_id) ??
    message.message_id;
  const existing = tasks_by_id.get(task_id);
  const description =
    normalize_task_progress_content(string_from_task_metadata(patch?.description) ?? undefined) ||
    normalize_task_progress_content(string_from_task_metadata(metadata.description) ?? undefined) ||
    normalize_task_progress_content(message.content) ||
    existing?.content;

  if (!description) {
    return;
  }

  tasks_by_id.set(task_id, {
    content: description,
    status: infer_system_task_status(
      subtype,
      string_from_task_metadata(metadata.status) ?? string_from_task_metadata(patch?.status),
      existing?.status,
    ),
    active_form: existing?.active_form,
  });
}

function upsert_task_progress_todo(
  tasks_by_id: Map<string, TodoItem>,
  block: TaskProgressContent,
) {
  const task_id = block.task_id?.trim();
  if (!task_id) {
    return;
  }
  const existing = tasks_by_id.get(task_id);
  const content = normalize_task_progress_content(block.description) || existing?.content;
  if (!content) {
    return;
  }
  tasks_by_id.set(task_id, {
    content,
    status: infer_task_progress_status(block, existing?.status),
    active_form: existing?.active_form,
  });
}

function normalize_task_progress_content(description: string | undefined): string {
  const value = description?.trim() ?? "";
  if (!value) {
    return "";
  }
  const colon_index = value.indexOf(":");
  if (colon_index >= 0 && colon_index < value.length - 1) {
    return value.slice(colon_index + 1).trim();
  }
  return value;
}

function normalize_todo_content(content: string): string {
  return content.replace(/\s+/g, " ").trim().toLowerCase();
}

function record_from_task_metadata(value: unknown): Record<string, unknown> {
  return typeof value === "object" && value !== null ? value as Record<string, unknown> : {};
}

function string_from_task_metadata(value: unknown): string | null {
  return typeof value === "string" && value.trim() ? value.trim() : null;
}

function infer_system_task_status(
  subtype: string,
  status: string | null,
  fallback: TodoItem["status"] | undefined,
): TodoItem["status"] {
  const normalized_status = status?.toLowerCase().trim() ?? "";
  if (
    normalized_status === "completed" ||
    normalized_status === "complete" ||
    normalized_status === "success" ||
    normalized_status === "done" ||
    normalized_status === "stopped" ||
    normalized_status === "cancelled" ||
    normalized_status === "canceled" ||
    normalized_status === "killed" ||
    normalized_status === "interrupted" ||
    normalized_status === "failed" ||
    normalized_status === "error"
  ) {
    return "completed";
  }
  if (
    normalized_status === "pending" ||
    normalized_status === "queued" ||
    normalized_status === "created"
  ) {
    return "pending";
  }
  if (
    normalized_status === "running" ||
    normalized_status === "in_progress" ||
    normalized_status === "in progress" ||
    normalized_status === "started"
  ) {
    return "in_progress";
  }
  // task_notification 表示子任务已回报最终结果 = 终态，须优先于运行中的 fallback。
  if (subtype === "task_notification") {
    return "completed";
  }
  if (fallback) {
    return fallback;
  }
  return "in_progress";
}

function infer_task_progress_status(
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
