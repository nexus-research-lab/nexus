"use client";

import { useMemo } from "react";

import type { ConversationThreadRound } from "@/features/conversation/shared/thread/conversation-thread-model";
import type { Message } from "@/types/conversation/message";
import type {
  SubagentTask,
  SubagentTaskMessagesResponse,
  SubagentTaskSource,
} from "@/types/conversation/subagent-task";

import {
  createSubagentTaskThreadScope,
  type SubagentTaskCommand,
  type SubagentTaskThreadError,
  projectSubagentTaskThread,
  resolveSubagentTaskThreadError,
} from "./subagent-task-thread-model";
import { useSubagentTaskThreadCommands } from "./use-subagent-task-thread-commands";
import { useSubagentTaskThreadResource } from "./use-subagent-task-thread-resource";

export interface UseSubagentTaskThreadResult {
  canSend: boolean;
  canStop: boolean;
  command: SubagentTaskCommand | null;
  detail: SubagentTaskMessagesResponse | null;
  draft: string;
  error: SubagentTaskThreadError | null;
  isLoading: boolean;
  isResume: boolean;
  messages: Message[];
  refresh: (silent?: boolean) => Promise<void>;
  rounds: ConversationThreadRound[];
  sendMessage: () => Promise<void>;
  sessionKey: string;
  setDraft: (value: string) => void;
  stop: () => Promise<void>;
  task: SubagentTask;
}

interface UseSubagentTaskThreadOptions {
  onRefreshTasks: () => void;
  source: SubagentTaskSource;
  task: SubagentTask;
}

export function useSubagentTaskThread({
  onRefreshTasks,
  source,
  task,
}: UseSubagentTaskThreadOptions): UseSubagentTaskThreadResult {
  const scope = createSubagentTaskThreadScope(source, task);
  const resource = useSubagentTaskThreadResource(scope);
  const projection = useMemo(
    () => projectSubagentTaskThread(task, resource.detail),
    [resource.detail, task],
  );
  const commands = useSubagentTaskThreadCommands({
    canSend: projection.canSend,
    canStop: projection.canStop,
    onRefreshTasks,
    onRefreshThread: resource.refresh,
    scope,
  });

  return {
    canSend: projection.canSend,
    canStop: projection.canStop,
    command: commands.command,
    detail: resource.detail,
    draft: commands.draft,
    error: resolveSubagentTaskThreadError(commands.error, resource.error),
    isLoading: resource.isLoading,
    isResume: projection.isResume,
    messages: projection.messages,
    refresh: resource.refresh,
    rounds: projection.rounds,
    sendMessage: commands.sendMessage,
    sessionKey: scope.key,
    setDraft: commands.setDraft,
    stop: commands.stop,
    task: projection.task,
  };
}
