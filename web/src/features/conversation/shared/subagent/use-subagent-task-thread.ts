"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import type { GroupThreadRound } from "@/features/conversation/room/group/thread/group-thread-detail-panel";
import { groupMessagesByRound } from "@/features/conversation/shared/timeline/timeline-model";
import {
  getSubagentTaskMessagesApi,
  sendSubagentTaskMessageApi,
  stopSubagentTaskApi,
} from "@/lib/api/subagent-task-api";
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
  normalizeSubagentTask,
  subagentTaskSourceKey,
} from "./subagent-task-model";

const ACTIVE_TASK_POLL_INTERVAL_MS = 3_000;
const EMPTY_MESSAGES: Message[] = [];

export type SubagentTaskCommand = "send" | "stop";

export interface SubagentTaskThreadError {
  message: string;
  retryable: boolean;
}

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
  rounds: GroupThreadRound[];
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

interface ThreadScope {
  key: string;
  source: SubagentTaskSource;
  task: SubagentTask;
}

interface ThreadState {
  command: SubagentTaskCommand | null;
  commandError: string | null;
  detail: SubagentTaskMessagesResponse | null;
  draft: string;
  isLoading: boolean;
  resourceError: string | null;
  scopeKey: string;
}

interface CommandToken {
  id: number;
  scopeKey: string;
}

type ThreadStateUpdater = (state: ThreadState) => ThreadState;

export function useSubagentTaskThread({
  onRefreshTasks,
  source,
  task,
}: UseSubagentTaskThreadOptions): UseSubagentTaskThreadResult {
  const scopeKey = `${subagentTaskSourceKey(source)}:${task.task_id}`;
  const scopeRef = useRef<ThreadScope>({ key: scopeKey, source, task });
  scopeRef.current = { key: scopeKey, source, task };
  const [storedState, setStoredState] = useState<ThreadState>(() =>
    createThreadState(scopeKey, task.capabilities.transcript),
  );
  const requestSequenceRef = useRef(0);
  const commandSequenceRef = useRef(0);
  const commandRef = useRef<(CommandToken & { kind: SubagentTaskCommand }) | null>(null);
  const state = storedState.scopeKey === scopeKey
    ? storedState
    : createThreadState(scopeKey, task.capabilities.transcript);

  const commit = useCallback((expectedScopeKey: string, update: ThreadStateUpdater) => {
    if (scopeRef.current.key !== expectedScopeKey) {
      return;
    }
    setStoredState((current) => {
      if (scopeRef.current.key !== expectedScopeKey) {
        return current;
      }
      const scopedState = current.scopeKey === expectedScopeKey
        ? current
        : createThreadState(
          expectedScopeKey,
          scopeRef.current.task.capabilities.transcript,
        );
      return update(scopedState);
    });
  }, []);

  const refresh = useCallback(async (silent = false) => {
    const scope = scopeRef.current;
    if (scope.key !== scopeKey || !scope.task.capabilities.transcript) {
      return;
    }
    const requestSequence = requestSequenceRef.current + 1;
    requestSequenceRef.current = requestSequence;
    if (!silent) {
      commit(scope.key, (current) => ({
        ...current,
        isLoading: true,
        resourceError: null,
      }));
    }

    try {
      const result = await getSubagentTaskMessagesApi(scope.source, scope.task.task_id);
      if (!isCurrentRequest(scopeRef.current, scope.key, requestSequenceRef, requestSequence)) {
        return;
      }
      commit(scope.key, (current) => ({
        ...current,
        detail: {
          ...result,
          task: normalizeSubagentTask(
            result.task,
            scope.task.runtime_kind,
            scope.task.capabilities,
          ),
        },
        isLoading: false,
        resourceError: null,
      }));
    } catch (requestError) {
      if (!isCurrentRequest(scopeRef.current, scope.key, requestSequenceRef, requestSequence)) {
        return;
      }
      commit(scope.key, (current) => ({
        ...current,
        isLoading: false,
        resourceError: errorMessage(requestError),
      }));
    }
  }, [commit, scopeKey]);

  useEffect(() => {
    requestSequenceRef.current += 1;
    if (task.capabilities.transcript) {
      void refresh();
    } else {
      commit(scopeKey, (current) => ({
        ...current,
        detail: null,
        isLoading: false,
        resourceError: null,
      }));
    }
    return () => {
      requestSequenceRef.current += 1;
    };
  }, [commit, refresh, scopeKey, task.capabilities.transcript]);

  const effectiveTask = state.detail?.task ?? task;
  const messages = state.detail?.messages ?? EMPTY_MESSAGES;
  const rounds = useMemo<GroupThreadRound[]>(() =>
    Array.from(groupMessagesByRound(messages), ([roundId, roundMessages]) => ({
      roundId,
      messages: roundMessages,
    })), [messages]);
  const canSend = canSendSubagentTaskMessage(effectiveTask);
  const canStop = canStopSubagentTask(effectiveTask);
  const taskIsActive = isSubagentTaskActive(effectiveTask);
  const transcriptAvailable = effectiveTask.capabilities.transcript;

  useEffect(() => {
    if (!transcriptAvailable || !taskIsActive) {
      return undefined;
    }
    const intervalId = window.setInterval(() => {
      void refresh(true);
    }, ACTIVE_TASK_POLL_INTERVAL_MS);
    return () => window.clearInterval(intervalId);
  }, [refresh, taskIsActive, transcriptAvailable]);

  const runCommand = useCallback(async (
    kind: SubagentTaskCommand,
    execute: (scope: ThreadScope) => Promise<unknown>,
    clearDraft = false,
  ) => {
    const scope = scopeRef.current;
    if (scope.key !== scopeKey || commandRef.current?.scopeKey === scope.key) {
      return;
    }
    const token = {
      id: commandSequenceRef.current + 1,
      kind,
      scopeKey: scope.key,
    };
    commandSequenceRef.current = token.id;
    commandRef.current = token;
    commit(scope.key, (current) => ({
      ...current,
      command: kind,
      commandError: null,
    }));

    try {
      await execute(scope);
      if (!isCurrentCommand(scopeRef.current, commandRef.current, token)) {
        return;
      }
      commit(scope.key, (current) => ({
        ...current,
        draft: clearDraft ? "" : current.draft,
      }));
      onRefreshTasks();
      void refresh(true);
    } catch (commandError) {
      if (!isCurrentCommand(scopeRef.current, commandRef.current, token)) {
        return;
      }
      commit(scope.key, (current) => ({
        ...current,
        commandError: errorMessage(commandError),
      }));
    } finally {
      if (isCurrentCommand(scopeRef.current, commandRef.current, token)) {
        commandRef.current = null;
        commit(scope.key, (current) => ({ ...current, command: null }));
      }
    }
  }, [commit, onRefreshTasks, refresh, scopeKey]);

  const sendMessage = useCallback(async () => {
    const message = state.draft.trim();
    if (!canSend || !message) {
      return;
    }
    await runCommand(
      "send",
      (scope) => sendSubagentTaskMessageApi(scope.source, scope.task.task_id, message),
      true,
    );
  }, [canSend, runCommand, state.draft]);

  const stop = useCallback(async () => {
    if (!canStop) {
      return;
    }
    await runCommand(
      "stop",
      (scope) => stopSubagentTaskApi(scope.source, scope.task.task_id),
    );
  }, [canStop, runCommand]);

  const setDraft = useCallback((value: string) => {
    commit(scopeKey, (current) => ({ ...current, draft: value }));
  }, [commit, scopeKey]);

  const error = state.commandError
    ? { message: state.commandError, retryable: false }
    : state.resourceError
      ? { message: state.resourceError, retryable: true }
      : null;

  return {
    canSend,
    canStop,
    command: state.command,
    detail: state.detail,
    draft: state.draft,
    error,
    isLoading: state.isLoading,
    isResume: canSend && !isSubagentTaskActive(effectiveTask),
    messages,
    refresh,
    rounds,
    sendMessage,
    sessionKey: scopeKey,
    setDraft,
    stop,
    task: effectiveTask,
  };
}

function createThreadState(scopeKey: string, isLoading: boolean): ThreadState {
  return {
    command: null,
    commandError: null,
    detail: null,
    draft: "",
    isLoading,
    resourceError: null,
    scopeKey,
  };
}

function isCurrentRequest(
  currentScope: ThreadScope,
  expectedScopeKey: string,
  requestSequenceRef: { current: number },
  requestSequence: number,
): boolean {
  return currentScope.key === expectedScopeKey
    && requestSequenceRef.current === requestSequence;
}

function isCurrentCommand(
  currentScope: ThreadScope,
  currentCommand: (CommandToken & { kind: SubagentTaskCommand }) | null,
  expectedCommand: CommandToken,
): boolean {
  return currentScope.key === expectedCommand.scopeKey
    && currentCommand !== null
    && currentCommand.id === expectedCommand.id
    && currentCommand.scopeKey === expectedCommand.scopeKey;
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
