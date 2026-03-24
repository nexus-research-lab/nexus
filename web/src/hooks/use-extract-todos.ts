import { useEffect, useRef, useState } from "react";
import { Message, ResultMessage } from "@/types/message";
import { TodoItem } from "@/types/todo";

function isSameSessionMessage(message: Message, externalSessionKey: string): boolean {
  return !message.session_key || message.session_key === externalSessionKey;
}

export const useExtractTodos = (
  messages: Message[],
  externalSessionKey: string | null
) => {
  const [todos, setTodos] = useState<TodoItem[]>([]);
  const prevSessionRef = useRef<string | null>(null);

  // Extract todos from messages (reset on session change)
  useEffect(() => {
    // Session changed - reset todos immediately
    if (prevSessionRef.current !== externalSessionKey) {
      setTodos([]);
      prevSessionRef.current = externalSessionKey;
    }

    // No session - don't extract
    if (!externalSessionKey || messages.length === 0) {
      return;
    }

    let latestTodos: TodoItem[] = [];
    let latestTodoRoundId: string | null = null;
    let latestTodoIndex = -1;
    let found = false;

    // Iterate backwards to find the latest TodoWrite tool use
    // Only consider messages from current session
    for (let i = messages.length - 1; i >= 0; i--) {
      const msg = messages[i];

      // Skip messages that don't belong to current session
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
      }
      if (found) break;
    }

    if (!found || latestTodos.length === 0 || !latestTodoRoundId) {
      setTodos([]);
      return;
    }

    // 终态收敛：该轮异常结束（中断/错误）时直接清空 Todo，避免右上角 Agent Plan 残留
    const roundResult = [...messages]
      .reverse()
      .find((msg): msg is ResultMessage =>
        msg.role === "result"
        && msg.round_id === latestTodoRoundId
        && isSameSessionMessage(msg, externalSessionKey)
      );

    if (roundResult && roundResult.is_error) {
      setTodos([]);
      return;
    }

    // 跨轮兜底：如果已进入新轮次而旧轮无终态，也清空旧 Todo，避免挂住
    const hasLaterRoundMessage = messages.slice(latestTodoIndex + 1).some((msg) =>
      isSameSessionMessage(msg, externalSessionKey)
      && msg.round_id
      && msg.round_id !== latestTodoRoundId
      && msg.role !== "system"
    );

    if (hasLaterRoundMessage && !roundResult) {
      setTodos([]);
      return;
    }

    setTodos(latestTodos);
  }, [messages, externalSessionKey]);

  return todos;
};
