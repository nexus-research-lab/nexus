import { useMemo, useRef } from "react";
import { are_equivalent_session_keys } from "@/lib/conversation/session-key";
import { AssistantMessage, Message, TaskProgressContent } from "@/types/conversation/message";
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
  return left.every((item, index) => is_same_todo(item, right[index]));
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
          const task_progress_todos = extract_task_progress_todos_for_round(messages, msg.round_id, external_session_key);
          if (task_progress_todos.length > 0) {
            return task_progress_todos;
          }
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

    return latestTodos;
  }, [external_session_key, messages]);

  if (!are_todos_equal(stable_todos_ref.current, computed_todos)) {
    stable_todos_ref.current = computed_todos;
  }

  return stable_todos_ref.current;
};

function extract_task_progress_todos_for_round(
  messages: Message[],
  round_id: string | undefined,
  external_session_key: string,
): TodoItem[] {
  if (!round_id) {
    return [];
  }
  const tasks_by_id = new Map<string, TodoItem>();
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
      if (!block || block.type !== "task_progress") {
        continue;
      }
      upsert_task_progress_todo(tasks_by_id, block);
    }
  }
  return [...tasks_by_id.values()];
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
