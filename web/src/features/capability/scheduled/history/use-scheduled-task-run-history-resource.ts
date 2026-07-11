"use client";

import { useCallback, useEffect, useLayoutEffect, useRef } from "react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { listScheduledTaskRunsApi } from "@/lib/api/capability/scheduled-task-api";
import type { ScheduledTaskRunItem } from "@/types/capability/scheduled-task";

interface RunHistoryResourceState {
  errorMessage: string | null;
  isLoading: boolean;
  runs: ScheduledTaskRunItem[];
}

export function useScheduledTaskRunHistoryResource(taskJobId: string | null) {
  const [state, setState] = useResettableState<RunHistoryResourceState>({
    errorMessage: null,
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
      errorMessage: null,
      isLoading: true,
    }));
    try {
      const runs = await listScheduledTaskRunsApi(taskJobId);
      if (
        activeTaskJobIdRef.current === taskJobId
        && activeRequestRef.current === request
      ) {
        setState((current) => ({ ...current, runs }));
      }
    } catch (error) {
      if (
        activeTaskJobIdRef.current === taskJobId
        && activeRequestRef.current === request
      ) {
        setState((current) => ({
          ...current,
          errorMessage: error instanceof Error ? error.message : "加载运行历史失败",
          runs: [],
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
