"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { listSubagentTasksApi } from "@/lib/api/subagent-task-api";
import type {
  SubagentTask,
  SubagentTaskListResponse,
  SubagentTaskSource,
} from "@/types/conversation/subagent-task";

import {
  isSubagentTaskActive,
  normalizeSubagentTaskListResponse,
  subagentTaskSourceKey,
} from "./subagent-task-model";

const RUNNING_TASK_POLL_INTERVAL_MS = 3_000;

interface UseSubagentTasksResult {
  data: SubagentTaskListResponse | null;
  error: string | null;
  isLoading: boolean;
  refresh: (silent?: boolean) => Promise<void>;
  tasks: SubagentTask[];
}

export function useSubagentTasks(
  source: SubagentTaskSource | null,
  enabled: boolean,
): UseSubagentTasksResult {
  const [data, setData] = useState<SubagentTaskListResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const requestSequenceRef = useRef(0);
  const sourceKey = subagentTaskSourceKey(source);
  const sourceRef = useRef(source);
  sourceRef.current = source;

  const refresh = useCallback(async (silent = false) => {
    const currentSource = sourceRef.current;
    if (
      !currentSource
      || !enabled
      || subagentTaskSourceKey(currentSource) !== sourceKey
    ) {
      return;
    }
    const requestSequence = requestSequenceRef.current + 1;
    requestSequenceRef.current = requestSequence;
    if (!silent) {
      setIsLoading(true);
    }
    try {
      const response = await listSubagentTasksApi(currentSource);
      if (requestSequenceRef.current !== requestSequence) {
        return;
      }
      setData(normalizeSubagentTaskListResponse(response));
      setError(null);
    } catch (requestError) {
      if (requestSequenceRef.current !== requestSequence) {
        return;
      }
      setError(
        requestError instanceof Error
          ? requestError.message
          : String(requestError),
      );
    } finally {
      if (!silent && requestSequenceRef.current === requestSequence) {
        setIsLoading(false);
      }
    }
  }, [enabled, sourceKey]);

  useEffect(() => {
    requestSequenceRef.current += 1;
    setData(null);
    setError(null);
    setIsLoading(false);
    if (enabled && sourceRef.current) {
      void refresh();
    }
  }, [enabled, refresh, sourceKey]);

  const tasks = useMemo(() => data?.items ?? [], [data?.items]);
  const hasRunningTasks = useMemo(
    () => tasks.some(isSubagentTaskActive),
    [tasks],
  );

  useEffect(() => {
    if (!enabled || !sourceKey || !hasRunningTasks) {
      return undefined;
    }
    const intervalId = window.setInterval(() => {
      void refresh(true);
    }, RUNNING_TASK_POLL_INTERVAL_MS);
    return () => window.clearInterval(intervalId);
  }, [enabled, hasRunningTasks, refresh, sourceKey]);

  return { data, error, isLoading, refresh, tasks };
}
