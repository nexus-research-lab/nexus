"use client";

import { useMemo } from "react";

import type { ConversationThreadRound } from "@/features/conversation/shared/thread/conversation-thread-model";
import type { Message } from "@/types/conversation/message/entity";
import type {
  SubagentTask,
  SubagentTaskMessagesResponse,
  SubagentTaskSource,
} from "@/types/conversation/subagent-task";

import {
  createSubagentTaskThreadScope,
  type SubagentTaskThreadError,
  projectSubagentTaskThread,
  resolveSubagentTaskThreadError,
} from "./subagent-task-thread-model";
import { useSubagentTaskThreadResource } from "./use-subagent-task-thread-resource";
import {
  type SubagentTaskActions,
  useSubagentTaskActions,
} from "./use-subagent-task-actions";

export interface UseSubagentTaskThreadResult {
	actions: SubagentTaskActions;
  detail: SubagentTaskMessagesResponse | null;
  error: SubagentTaskThreadError | null;
  isLoading: boolean;
  messages: Message[];
  refresh: (silent?: boolean) => Promise<void>;
  rounds: ConversationThreadRound[];
  sessionKey: string;
  task: SubagentTask;
}

interface UseSubagentTaskThreadOptions {
  source: SubagentTaskSource;
  task: SubagentTask;
}

export function useSubagentTaskThread({
  source,
  task,
}: UseSubagentTaskThreadOptions): UseSubagentTaskThreadResult {
  const scope = createSubagentTaskThreadScope(source, task);
  const resource = useSubagentTaskThreadResource(scope);
  const projection = useMemo(
    () => projectSubagentTaskThread(task, resource.detail),
    [resource.detail, task],
  );
	const actions = useSubagentTaskActions({
		refresh: resource.refresh,
		source,
		task: projection.task,
	});

  return {
		actions,
    detail: resource.detail,
    error: resolveSubagentTaskThreadError(resource.error),
    isLoading: resource.isLoading,
    messages: projection.messages,
    refresh: resource.refresh,
    rounds: projection.rounds,
    sessionKey: scope.key,
    task: projection.task,
  };
}
