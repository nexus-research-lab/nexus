"use client";

import { useCallback, useEffect, useRef } from "react";

import { getSubagentTaskMessagesApi } from "@/lib/api/conversation/subagent-task-api";
import type { SubagentTaskMessagesResponse } from "@/types/conversation/subagent-task";

import {
  isSubagentTaskActive,
  normalizeSubagentTask,
  SUBAGENT_TASK_POLL_INTERVAL_MS,
  subagentTaskErrorMessage,
} from "../subagent-task-model";
import { useScopedResource } from "../use-scoped-resource";
import {
  createSubagentTaskThreadResourceSnapshot,
  type SubagentTaskThreadScope,
} from "./subagent-task-thread-model";

interface SubagentTaskThreadResource {
  detail: SubagentTaskMessagesResponse | null;
  error: string | null;
  isLoading: boolean;
  refresh: (silent?: boolean) => Promise<void>;
}

export function useSubagentTaskThreadResource(
  scope: SubagentTaskThreadScope,
): SubagentTaskThreadResource {
  const scopeRef = useRef(scope);
  scopeRef.current = scope;
  const createSnapshot = useCallback(
    (scopeKey: string) => createSubagentTaskThreadResourceSnapshot(
      scopeKey,
      scope.task.capabilities.transcript,
    ),
    [scope.task.capabilities.transcript],
  );
  const {
    beginRequest,
    commit,
    invalidateRequests,
    isCurrentRequest,
    snapshot,
  } = useScopedResource(scope.key, createSnapshot);

  const refresh = useCallback(async (silent = false) => {
    const currentScope = scopeRef.current;
    if (currentScope.key !== scope.key || !currentScope.task.capabilities.transcript) {
      return;
    }
    const requestId = beginRequest(scope.key);
    if (requestId === null) {
      return;
    }
    if (!silent) {
      commit(scope.key, (current) => ({
        ...current,
        error: null,
        isLoading: true,
      }));
    }

    try {
      const result = await getSubagentTaskMessagesApi(
        currentScope.source,
        currentScope.task.task_id,
      );
      if (!isCurrentRequest(scope.key, requestId)) {
        return;
      }
      const latestScope = scopeRef.current;
      commit(scope.key, (current) => ({
        ...current,
        detail: {
          ...result,
          task: normalizeSubagentTask(
            result.task,
            latestScope.task.runtime_kind,
            latestScope.task.capabilities,
          ),
        },
        error: null,
        isLoading: false,
      }));
    } catch (requestError) {
      if (!isCurrentRequest(scope.key, requestId)) {
        return;
      }
      commit(scope.key, (current) => ({
        ...current,
        error: subagentTaskErrorMessage(requestError),
        isLoading: false,
      }));
    }
  }, [beginRequest, commit, isCurrentRequest, scope.key]);

  useEffect(() => {
    invalidateRequests();
    if (scope.task.capabilities.transcript) {
      void refresh();
    } else {
      commit(scope.key, (current) => ({
        ...current,
        detail: null,
        error: null,
        isLoading: false,
      }));
    }
    return invalidateRequests;
  }, [
    commit,
    invalidateRequests,
    refresh,
    scope.key,
    scope.task.capabilities.transcript,
  ]);

  const effectiveTask = snapshot.detail?.task ?? scope.task;
  const taskIsActive = isSubagentTaskActive(effectiveTask);
  const transcriptAvailable = effectiveTask.capabilities.transcript;
  useEffect(() => {
    if (!transcriptAvailable || !taskIsActive) {
      return undefined;
    }
    const intervalId = window.setInterval(() => {
      void refresh(true);
    }, SUBAGENT_TASK_POLL_INTERVAL_MS);
    return () => window.clearInterval(intervalId);
  }, [refresh, taskIsActive, transcriptAvailable]);

  return {
    detail: snapshot.detail,
    error: snapshot.error,
    isLoading: snapshot.isLoading,
    refresh,
  };
}
