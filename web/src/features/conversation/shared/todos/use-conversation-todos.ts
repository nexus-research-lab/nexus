/**
 * INPUT: 当前会话消息与 session。
 * OUTPUT: 引用稳定的单会话任务列表、Room 按 Agent 隔离的进程，以及按 Agent round 隔离的节点局部 Task run。
 * POS: Todo 纯投影与 React 会话控制器之间的 memo 边界；分组投影缓存按 sessionKey 自行失效，跨流式帧复用未变化的 Agent / Agent round。
 */
import { useMemo, useRef, useState } from "react";

import type { Message } from "@/types/conversation/message/entity";
import type { TodoItem } from "@/types/conversation/todo";

import {
  areTaskRunListsEqual,
  areTodoListsEqual,
  createTodoGroupProjectionCache,
  areTodoProcessListsEqual,
  projectConversationTaskRuns,
  projectConversationTodoProcesses,
  projectConversationTodos,
  type ConversationTaskRun,
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
  const [cache] = useState(createTodoGroupProjectionCache);
  const projectedProcesses = useMemo(
    () => projectConversationTodoProcesses(messages, sessionKey, cache),
    [cache, messages, sessionKey],
  );

  if (!areTodoProcessListsEqual(
    stableProcessesRef.current,
    projectedProcesses,
  )) {
    stableProcessesRef.current = projectedProcesses;
  }
  return stableProcessesRef.current;
}

export function useConversationTaskRuns(
  messages: Message[],
  sessionKey: string | null,
): ConversationTaskRun[] {
  const stableRunsRef = useRef<ConversationTaskRun[]>([]);
  const [cache] = useState(createTodoGroupProjectionCache);
  const projectedRuns = useMemo(
    () => projectConversationTaskRuns(messages, sessionKey, cache),
    [cache, messages, sessionKey],
  );

  if (!areTaskRunListsEqual(stableRunsRef.current, projectedRuns)) {
    stableRunsRef.current = projectedRuns;
  }
  return stableRunsRef.current;
}
