import { groupMessagesByRound } from "@/features/conversation/shared/timeline/timeline-model";
import type { ConversationThreadRound } from "@/features/conversation/shared/thread/conversation-thread-model";
import type { Message } from "@/types/conversation/message/entity";
import type {
  SubagentTask,
  SubagentTaskMessagesResponse,
  SubagentTaskSource,
} from "@/types/conversation/subagent-task";

import {
  preferFreshSubagentTask,
  subagentTaskSourceKey,
} from "../subagent-task-model";

const EMPTY_MESSAGES: Message[] = [];

export interface SubagentTaskThreadError {
  retryable: boolean;
}

export interface SubagentTaskThreadScope {
  key: string;
  source: SubagentTaskSource;
  task: SubagentTask;
}

export interface SubagentTaskThreadResourceSnapshot {
  detail: SubagentTaskMessagesResponse | null;
  error: string | null;
  isLoading: boolean;
  scopeKey: string;
}

export interface SubagentTaskThreadProjection {
  messages: Message[];
  rounds: ConversationThreadRound[];
  task: SubagentTask;
}

export function createSubagentTaskThreadScope(
  source: SubagentTaskSource,
  task: SubagentTask,
): SubagentTaskThreadScope {
  return {
    key: `${subagentTaskSourceKey(source)}:${task.task_id}`,
    source,
    task,
  };
}

export function createSubagentTaskThreadResourceSnapshot(
  scopeKey: string,
  isLoading: boolean,
): SubagentTaskThreadResourceSnapshot {
  return {
    detail: null,
    error: null,
    isLoading,
    scopeKey,
  };
}

export function projectSubagentTaskThread(
  task: SubagentTask,
  detail: SubagentTaskMessagesResponse | null,
): SubagentTaskThreadProjection {
  const effectiveTask = preferFreshSubagentTask(task, detail?.task);
  const messages = detail?.messages ?? EMPTY_MESSAGES;
  return {
    messages,
    rounds: Array.from(
      groupMessagesByRound(messages),
      ([roundId, roundMessages]) => ({ roundId, messages: roundMessages }),
    ),
    task: effectiveTask,
  };
}

export function resolveSubagentTaskThreadError(
  resourceError: string | null,
): SubagentTaskThreadError | null {
  if (resourceError) {
    return { retryable: true };
  }
  return null;
}
