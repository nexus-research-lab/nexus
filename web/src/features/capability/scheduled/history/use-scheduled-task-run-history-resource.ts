"use client";

import { useCallback, useEffect, useLayoutEffect, useRef } from "react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { listScheduledTaskRunsApi } from "@/lib/api/capability/scheduled-task-api";
import { getResourceFailure, type ResourceFailure } from "@/lib/error-message";
import type { ScheduledTaskRunItem } from "@/types/capability/scheduled-task/run";

interface RunHistoryResourceState {
  failure: ResourceFailure | null;
  hasSnapshot: boolean;
  isLoading: boolean;
  runs: ScheduledTaskRunItem[];
}

export function useScheduledTaskRunHistoryResource(taskJobId: string | null) {
  const [state, setState] = useResettableState<RunHistoryResourceState>({
    failure: null,
    hasSnapshot: false,
    isLoading: taskJobId !== null,
    runs: [],
  }, taskJobId ?? "closed");
  const activeTaskJobIdRef = useRef<string | null>(null);
  const activeRequestRef = useRef<symbol | null>(null);

  const refresh = useCallback(async (): Promise<void> => {
    if (!taskJobId) {
      return;
    }
    const request = Symbol(taskJobId);
    activeRequestRef.current = request;
    setState((current) => ({
      ...current,
      failure: current.failure?.access ? current.failure : null,
      isLoading: true,
    }));
    try {
      const runs = await listScheduledTaskRunsApi(taskJobId);
      if (
        activeTaskJobIdRef.current === taskJobId
        && activeRequestRef.current === request
      ) {
        setState((current) => ({
          ...current,
          failure: null,
          hasSnapshot: true,
          runs,
        }));
      }
    } catch (error) {
      if (
        activeTaskJobIdRef.current === taskJobId
        && activeRequestRef.current === request
      ) {
        setState((current) => ({
          ...current,
          failure: getResourceFailure(error, "加载运行历史失败"),
        }));
      }
      throw error;
    } finally {
      if (
        activeTaskJobIdRef.current === taskJobId
        && activeRequestRef.current === request
      ) {
        setState((current) => ({ ...current, isLoading: false }));
      }
    }
  }, [setState, taskJobId]);

  useLayoutEffect(() => {
    activeTaskJobIdRef.current = taskJobId;
    activeRequestRef.current = null;
    return () => {
      if (activeTaskJobIdRef.current === taskJobId) {
        activeTaskJobIdRef.current = null;
        activeRequestRef.current = null;
      }
    };
  }, [taskJobId]);

  useEffect(() => {
    if (!taskJobId) {
      activeRequestRef.current = null;
      return;
    }
    void refresh().catch(() => undefined);
  }, [refresh, taskJobId]);

  return {
    ...state,
    refresh,
  };
}
