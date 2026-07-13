import { useMemo, useRef } from "react";

import type { Message } from "@/types/conversation/message/entity";
import type { TodoItem } from "@/types/conversation/todo";

import { areTodoListsEqual, projectConversationTodos } from "./todo-projection-model";

export function useConversationTodos(
  messages: Message[],
  sessionKey: string | null,
): TodoItem[] {
  const stableTodosRef = useRef<TodoItem[]>([]);
  const projectedTodos = useMemo(
    () => projectConversationTodos(messages, sessionKey),
    [messages, sessionKey],
  );

  if (!areTodoListsEqual(stableTodosRef.current, projectedTodos)) {
    stableTodosRef.current = projectedTodos;
  }
  return stableTodosRef.current;
}
