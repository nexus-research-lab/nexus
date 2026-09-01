/**
 * INPUT: exact 子智能体来源、任务身份与 transcript 读取信号。
 * OUTPUT: 作用域隔离的最后成功 transcript、加载态和普通用户可读的读取失败。
 * POS: 只读线程资源；失败不清除已保存记录，也不触发任何任务控制命令。
 */
"use client";

import { useCallback, useEffect, useRef } from "react";

import { getSubagentTaskMessagesApi } from "@/lib/api/conversation/subagent-task-api";
import { getErrorMessage } from "@/lib/error-message";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { SubagentTaskMessagesResponse } from "@/types/conversation/subagent-task";

import { normalizeSubagentTask } from "../subagent-task-model";
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
  const { t } = useI18n();
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
        error: getErrorMessage(
          requestError,
          t("subagents.transcript_load_failed_detail"),
        ),
        isLoading: false,
      }));
    }
  }, [beginRequest, commit, isCurrentRequest, scope.key, t]);

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
