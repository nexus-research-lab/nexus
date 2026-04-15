import { useMemo, useRef } from "react";
import { are_equivalent_session_keys } from "@/lib/session-key";
import { Message, ResultMessage } from "@/types/message";
import { TodoItem } from "@/types/todo";

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
      }

      if (found) {
        break;
      }
    }

    if (!found || latestTodos.length === 0 || !latestTodoRoundId) {
      return [];
    }

    const roundResult = [...messages]
      .reverse()
      .find((msg): msg is ResultMessage =>
        msg.role === "result"
        && msg.round_id === latestTodoRoundId
        && is_same_session_message(msg, external_session_key)
      );

    if (roundResult && roundResult.is_error) {
      return [];
    }

    const hasLaterRoundMessage = messages.slice(latestTodoIndex + 1).some((msg) =>
      is_same_session_message(msg, external_session_key)
      && msg.round_id
      && msg.round_id !== latestTodoRoundId
      && msg.role !== "system"
    );

    if (hasLaterRoundMessage && !roundResult) {
      return [];
    }

    return latestTodos;
  }, [external_session_key, messages]);

  if (!are_todos_equal(stable_todos_ref.current, computed_todos)) {
    stable_todos_ref.current = computed_todos;
  }

  return stable_todos_ref.current;
};
