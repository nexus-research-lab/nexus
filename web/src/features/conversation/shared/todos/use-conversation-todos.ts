/**
 * INPUT: 当前会话消息与 session。
 * OUTPUT: 引用稳定的单会话任务列表，以及 Room 按 Agent 隔离的进程集合。
 * POS: Todo 纯投影与 React 会话控制器之间的 memo 边界。
 */
import { useMemo, useRef } from "react";

import type { Message } from "@/types/conversation/message/entity";
import type { TodoItem } from "@/types/conversation/todo";

import {
  areTodoListsEqual,
  areTodoProcessListsEqual,
  projectConversationTodoProcesses,
  projectConversationTodos,
  type ConversationTodoProcess,
} from "./todo-projection-model";

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

export function useConversationTodoProcesses(
  messages: Message[],
  sessionKey: string | null,
): ConversationTodoProcess[] {
  const stableProcessesRef = useRef<ConversationTodoProcess[]>([]);
  const projectedProcesses = useMemo(
    () => projectConversationTodoProcesses(messages, sessionKey),
    [messages, sessionKey],
  );

  if (!areTodoProcessListsEqual(
    stableProcessesRef.current,
    projectedProcesses,
  )) {
    stableProcessesRef.current = projectedProcesses;
  }
  return stableProcessesRef.current;
}
