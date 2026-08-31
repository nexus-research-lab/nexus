// INPUT: exact Conversation/Agent scope 与子智能体任务读取 API。
// OUTPUT: scope-fenced 任务快照、加载状态和不暴露底层异常的读取失败标记。
// POS: 子智能体任务目录资源控制器；读取失败不清空同 scope 已有快照。
"use client";

import { useCallback, useEffect, useRef } from "react";

import { listSubagentTasksApi } from "@/lib/api/conversation/subagent-task-api";
import type {
  SubagentTask,
  SubagentTaskListResponse,
  SubagentTaskSource,
} from "@/types/conversation/subagent-task";

import {
  normalizeSubagentTaskListResponse,
  subagentTaskSourceKey,
} from "./subagent-task-model";
import { useScopedResource } from "./use-scoped-resource";
import { useSubagentTaskRealtimeRefresh } from "./use-subagent-task-realtime-refresh";

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
  source: SubagentTaskSource,
  observeChanges: boolean,
  hostAgentId?: string | null,
): UseSubagentTasksResult {
  const scopeKey = subagentTaskSourceKey(source);
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
    if (!scopeKey || subagentTaskSourceKey(currentSource) !== scopeKey) {
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
    } catch {
      if (!isCurrentRequest(scopeKey, requestId)) {
        return;
      }
      commit(scopeKey, (current) => ({
        ...current,
        error: "subagent_task_list_unavailable",
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

  const refreshFromRealtime = useCallback(() => refresh(true), [refresh]);

  useSubagentTaskRealtimeRefresh({
    enabled: observeChanges,
    hostAgentId,
    onChanged: refreshFromRealtime,
    source,
  });

  const tasks = snapshot.data?.items ?? EMPTY_TASKS;

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
