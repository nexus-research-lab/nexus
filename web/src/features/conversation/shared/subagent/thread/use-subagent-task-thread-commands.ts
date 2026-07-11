"use client";

import { useCallback, useRef } from "react";

import {
  sendSubagentTaskMessageApi,
  stopSubagentTaskApi,
} from "@/lib/api/conversation/subagent-task-api";

import { subagentTaskErrorMessage } from "../subagent-task-model";
import { useScopedResource } from "../use-scoped-resource";
import {
  createSubagentTaskThreadCommandSnapshot,
  type SubagentTaskCommand,
  type SubagentTaskThreadScope,
} from "./subagent-task-thread-model";

interface CommandToken {
  id: number;
  kind: SubagentTaskCommand;
  scopeKey: string;
}

interface UseSubagentTaskThreadCommandsOptions {
  canSend: boolean;
  canStop: boolean;
  onRefreshTasks: () => void;
  onRefreshThread: (silent?: boolean) => Promise<void>;
  scope: SubagentTaskThreadScope;
}

interface SubagentTaskThreadCommands {
  command: SubagentTaskCommand | null;
  draft: string;
  error: string | null;
  sendMessage: () => Promise<void>;
  setDraft: (value: string) => void;
  stop: () => Promise<void>;
}

export function useSubagentTaskThreadCommands({
  canSend,
  canStop,
  onRefreshTasks,
  onRefreshThread,
  scope,
}: UseSubagentTaskThreadCommandsOptions): SubagentTaskThreadCommands {
  const scopeRef = useRef(scope);
  scopeRef.current = scope;
  const commandSequenceRef = useRef(0);
  const commandRef = useRef<CommandToken | null>(null);
  const { commit, snapshot } = useScopedResource(
    scope.key,
    createSubagentTaskThreadCommandSnapshot,
  );

  const runCommand = useCallback(async (
    kind: SubagentTaskCommand,
    execute: (currentScope: SubagentTaskThreadScope) => Promise<unknown>,
    clearDraft = false,
  ) => {
    const currentScope = scopeRef.current;
    if (currentScope.key !== scope.key || commandRef.current?.scopeKey === scope.key) {
      return;
    }
    const token: CommandToken = {
      id: commandSequenceRef.current + 1,
      kind,
      scopeKey: currentScope.key,
    };
    commandSequenceRef.current = token.id;
    commandRef.current = token;
    commit(scope.key, (current) => ({
      ...current,
      command: kind,
      error: null,
    }));

    try {
      await execute(currentScope);
      if (!isCurrentCommand(scopeRef.current, commandRef.current, token)) {
        return;
      }
      commit(scope.key, (current) => ({
        ...current,
        draft: clearDraft ? "" : current.draft,
      }));
      onRefreshTasks();
      void onRefreshThread(true);
    } catch (commandError) {
      if (!isCurrentCommand(scopeRef.current, commandRef.current, token)) {
        return;
      }
      commit(scope.key, (current) => ({
        ...current,
        error: subagentTaskErrorMessage(commandError),
      }));
    } finally {
      if (isCurrentCommand(scopeRef.current, commandRef.current, token)) {
        commandRef.current = null;
        commit(scope.key, (current) => ({ ...current, command: null }));
      }
    }
  }, [commit, onRefreshTasks, onRefreshThread, scope.key]);

  const sendMessage = useCallback(async () => {
    const message = snapshot.draft.trim();
    if (!canSend || !message) {
      return;
    }
    await runCommand(
      "send",
      (currentScope) => sendSubagentTaskMessageApi(
        currentScope.source,
        currentScope.task.task_id,
        message,
      ),
      true,
    );
  }, [canSend, runCommand, snapshot.draft]);

  const stop = useCallback(async () => {
    if (!canStop) {
      return;
    }
    await runCommand(
      "stop",
      (currentScope) => stopSubagentTaskApi(
        currentScope.source,
        currentScope.task.task_id,
      ),
    );
  }, [canStop, runCommand]);

  const setDraft = useCallback((value: string) => {
    commit(scope.key, (current) => ({ ...current, draft: value }));
  }, [commit, scope.key]);

  return {
    command: snapshot.command,
    draft: snapshot.draft,
    error: snapshot.error,
    sendMessage,
    setDraft,
    stop,
  };
}

function isCurrentCommand(
  currentScope: SubagentTaskThreadScope,
  currentCommand: CommandToken | null,
  expectedCommand: CommandToken,
): boolean {
  return currentScope.key === expectedCommand.scopeKey
    && currentCommand?.id === expectedCommand.id
    && currentCommand.scopeKey === expectedCommand.scopeKey;
}
