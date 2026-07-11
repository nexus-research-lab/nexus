"use client";

import { useCallback, useEffect, useMemo, useRef } from "react";

import { listSubagentTasksApi } from "@/lib/api/conversation/subagent-task-api";
import type {
  SubagentTask,
  SubagentTaskListResponse,
  SubagentTaskSource,
} from "@/types/conversation/subagent-task";

import {
  isSubagentTaskActive,
  normalizeSubagentTaskListResponse,
  SUBAGENT_TASK_POLL_INTERVAL_MS,
  subagentTaskErrorMessage,
  subagentTaskSourceKey,
} from "./subagent-task-model";
import { useScopedResource } from "./use-scoped-resource";

const EMPTY_TASKS: SubagentTask[] = [];

interface UseSubagentTasksResult {
  data: SubagentTaskListResponse | null;
  error: string | null;
  isLoading: boolean;
  refresh: (silent?: boolean) => Promise<void>;
  tasks: SubagentTask[];
}

interface TaskListSnapshot {
  data: SubagentTaskListResponse | null;
  error: string | null;
  isLoading: boolean;
  scopeKey: string;
}

export function useSubagentTasks(
  source: SubagentTaskSource | null,
  enabled: boolean,
): UseSubagentTasksResult {
  const sourceKey = subagentTaskSourceKey(source);
  const scopeKey = enabled && source ? sourceKey : "";
  const sourceRef = useRef(source);
  sourceRef.current = source;
  const {
    beginRequest,
    commit,
    invalidateRequests,
    isCurrentRequest,
    snapshot,
  } = useScopedResource(scopeKey, createTaskListSnapshot);

  const refresh = useCallback(async (silent = false) => {
    const currentSource = sourceRef.current;
    if (!currentSource || !scopeKey || subagentTaskSourceKey(currentSource) !== scopeKey) {
      return;
    }
    const requestId = beginRequest(scopeKey);
    if (requestId === null) {
      return;
    }
    if (!silent) {
      commit(scopeKey, (current) => ({
        ...current,
        error: null,
        isLoading: true,
      }));
    }

    try {
      const response = await listSubagentTasksApi(currentSource);
      if (!isCurrentRequest(scopeKey, requestId)) {
        return;
      }
      commit(scopeKey, (current) => ({
        ...current,
        data: normalizeSubagentTaskListResponse(response),
        error: null,
        isLoading: false,
      }));
    } catch (requestError) {
      if (!isCurrentRequest(scopeKey, requestId)) {
        return;
      }
      commit(scopeKey, (current) => ({
        ...current,
        error: subagentTaskErrorMessage(requestError),
        isLoading: false,
      }));
    }
  }, [beginRequest, commit, isCurrentRequest, scopeKey]);

  useEffect(() => {
    invalidateRequests();
    if (scopeKey) {
      void refresh();
    }
    return invalidateRequests;
  }, [invalidateRequests, refresh, scopeKey]);

  const tasks = snapshot.data?.items ?? EMPTY_TASKS;
  const hasRunningTasks = useMemo(
    () => tasks.some(isSubagentTaskActive),
    [tasks],
  );

  useEffect(() => {
    if (!scopeKey || !hasRunningTasks) {
      return undefined;
    }
    const intervalId = window.setInterval(() => {
      void refresh(true);
    }, SUBAGENT_TASK_POLL_INTERVAL_MS);
    return () => window.clearInterval(intervalId);
  }, [hasRunningTasks, refresh, scopeKey]);

  return {
    data: snapshot.data,
    error: snapshot.error,
    isLoading: snapshot.isLoading,
    refresh,
    tasks,
  };
}

function createTaskListSnapshot(scopeKey: string): TaskListSnapshot {
  return {
    data: null,
    error: null,
    isLoading: Boolean(scopeKey),
    scopeKey,
  };
}
