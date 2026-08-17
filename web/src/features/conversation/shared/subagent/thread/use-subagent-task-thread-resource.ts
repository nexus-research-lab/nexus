"use client";

import { useCallback, useEffect, useRef } from "react";

import { getSubagentTaskMessagesApi } from "@/lib/api/conversation/subagent-task-api";
import type { SubagentTaskMessagesResponse } from "@/types/conversation/subagent-task";

import {
  normalizeSubagentTask,
  subagentTaskErrorMessage,
} from "../subagent-task-model";
import { useScopedResource } from "../use-scoped-resource";
import { useSubagentTaskRealtimeRefresh } from "../use-subagent-task-realtime-refresh";
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

  const refreshFromRealtime = useCallback(() => refresh(true), [refresh]);

  useSubagentTaskRealtimeRefresh({
    enabled: scope.task.capabilities.transcript,
    onChanged: refreshFromRealtime,
    source: scope.source,
    taskId: scope.task.task_id,
  });

  return {
    detail: snapshot.detail,
    error: snapshot.error,
    isLoading: snapshot.isLoading,
    refresh,
  };
}
