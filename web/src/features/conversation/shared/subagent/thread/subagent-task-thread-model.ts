import { groupMessagesByRound } from "@/features/conversation/shared/timeline/timeline-model";
import type { ConversationThreadRound } from "@/features/conversation/shared/thread/conversation-thread-model";
import type { Message } from "@/types/conversation/message";
import type {
  SubagentTask,
  SubagentTaskMessagesResponse,
  SubagentTaskSource,
} from "@/types/conversation/subagent-task";

import {
  canSendSubagentTaskMessage,
  canStopSubagentTask,
  isSubagentTaskActive,
  subagentTaskSourceKey,
} from "../subagent-task-model";

const EMPTY_MESSAGES: Message[] = [];

export type SubagentTaskCommand = "send" | "stop";

export interface SubagentTaskThreadError {
  message: string;
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

export interface SubagentTaskThreadCommandSnapshot {
  command: SubagentTaskCommand | null;
  draft: string;
  error: string | null;
  scopeKey: string;
}

export interface SubagentTaskThreadProjection {
  canSend: boolean;
  canStop: boolean;
  isResume: boolean;
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

export function createSubagentTaskThreadCommandSnapshot(
  scopeKey: string,
): SubagentTaskThreadCommandSnapshot {
  return {
    command: null,
    draft: "",
    error: null,
    scopeKey,
  };
}

export function projectSubagentTaskThread(
  task: SubagentTask,
  detail: SubagentTaskMessagesResponse | null,
): SubagentTaskThreadProjection {
  const effectiveTask = detail?.task ?? task;
  const messages = detail?.messages ?? EMPTY_MESSAGES;
  const canSend = canSendSubagentTaskMessage(effectiveTask);
  return {
    canSend,
    canStop: canStopSubagentTask(effectiveTask),
    isResume: canSend && !isSubagentTaskActive(effectiveTask),
    messages,
    rounds: Array.from(
      groupMessagesByRound(messages),
      ([roundId, roundMessages]) => ({ roundId, messages: roundMessages }),
    ),
    task: effectiveTask,
  };
}

export function resolveSubagentTaskThreadError(
  commandError: string | null,
  resourceError: string | null,
): SubagentTaskThreadError | null {
  if (commandError) {
    return { message: commandError, retryable: false };
  }
  if (resourceError) {
    return { message: resourceError, retryable: true };
  }
  return null;
}
