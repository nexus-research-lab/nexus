/**
 * INPUT: 当前 owner scope 与任务 Job identity。
 * OUTPUT: 只允许同一 owner+Job 请求代次返回的运行历史快照。
 * POS: Scheduled 历史读取边界；旧 scope 响应必须失败关闭，不能进入命令对账。
 */
"use client";

import { useCallback, useEffect, useLayoutEffect, useRef } from "react";

import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import { listScheduledTaskRunsApi } from "@/lib/api/capability/scheduled-task-api";
import { getResourceFailure, type ResourceFailure } from "@/lib/error-message";
import type { ScheduledTaskRunItem } from "@/types/capability/scheduled-task/run";

interface RunHistoryResourceState {
  failure: ResourceFailure | null;
  hasSnapshot: boolean;
  isLoading: boolean;
  runs: ScheduledTaskRunItem[];
}

class ScheduledTaskRunHistoryRefreshSupersededError extends Error {
  constructor() {
    super("运行历史已由新的任务或账号接管");
    this.name = "ScheduledTaskRunHistoryRefreshSupersededError";
  }
}

function runHistoryResourceKey(
  scopeKey: string | null,
  taskJobId: string | null,
): string | null {
  return scopeKey && taskJobId ? `${scopeKey}\u0000${taskJobId}` : null;
}

export function useScheduledTaskRunHistoryResource(
  taskJobId: string | null,
  scopeKey: string | null,
) {
  const resourceKey = runHistoryResourceKey(scopeKey, taskJobId);
  const [state, setState] = useResettableState<RunHistoryResourceState>({
    failure: null,
    hasSnapshot: false,
    isLoading: resourceKey !== null,
    runs: [],
  }, resourceKey ?? "closed");
  const activeResourceKeyRef = useRef<string | null>(null);
  const activeRequestRef = useRef<symbol | null>(null);

  const refresh = useCallback(async (): Promise<ScheduledTaskRunItem[]> => {
    if (!taskJobId || !resourceKey) {
      return [];
    }
    const request = Symbol(resourceKey);
    activeRequestRef.current = request;
    setState((current) => ({
      ...current,
      failure: current.failure?.access ? current.failure : null,
      isLoading: true,
    }));
    try {
      const runs = await listScheduledTaskRunsApi(taskJobId);
      if (
        activeResourceKeyRef.current !== resourceKey
        || activeRequestRef.current !== request
      ) {
        // 不能只阻止 setState 后仍把旧 owner 数据返回给对账调用方。
        throw new ScheduledTaskRunHistoryRefreshSupersededError();
      }
      setState((current) => ({
        ...current,
        failure: null,
        hasSnapshot: true,
        runs,
      }));
      return runs;
    } catch (error) {
      if (error instanceof ScheduledTaskRunHistoryRefreshSupersededError) {
        throw error;
      }
      if (
        activeResourceKeyRef.current === resourceKey
        && activeRequestRef.current === request
      ) {
        const failure = getResourceFailure(error, "加载运行历史失败");
        setState((current) => ({
          ...current,
          failure,
          hasSnapshot: failure.access ? false : current.hasSnapshot,
          runs: failure.access ? [] : current.runs,
        }));
      }
      throw error;
    } finally {
      if (
        activeResourceKeyRef.current === resourceKey
        && activeRequestRef.current === request
      ) {
        setState((current) => ({ ...current, isLoading: false }));
      }
    }
  }, [resourceKey, setState, taskJobId]);

  useLayoutEffect(() => {
    activeResourceKeyRef.current = resourceKey;
    activeRequestRef.current = null;
    return () => {
      if (activeResourceKeyRef.current === resourceKey) {
        activeResourceKeyRef.current = null;
        activeRequestRef.current = null;
      }
    };
  }, [resourceKey]);

  useEffect(() => {
    if (!resourceKey) {
      activeRequestRef.current = null;
      return;
    }
    void refresh().catch(() => undefined);
  }, [refresh, resourceKey]);

  return {
    ...state,
    refresh,
  };
}
