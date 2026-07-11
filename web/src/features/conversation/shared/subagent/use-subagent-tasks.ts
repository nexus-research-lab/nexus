"use client";

import {
  type Dispatch,
  type SetStateAction,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";

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
  const scopeKeyRef = useRef(scopeKey);
  scopeKeyRef.current = scopeKey;
  const [storedSnapshot, setStoredSnapshot] = useState<TaskListSnapshot>(() =>
    createTaskListSnapshot(scopeKey),
  );
  const requestSequenceRef = useRef(0);
  const snapshot = storedSnapshot.scopeKey === scopeKey
    ? storedSnapshot
    : createTaskListSnapshot(scopeKey);

  const refresh = useCallback(async (silent = false) => {
    const currentSource = sourceRef.current;
    if (!currentSource || !scopeKey || subagentTaskSourceKey(currentSource) !== scopeKey) {
      return;
    }
    const requestSequence = requestSequenceRef.current + 1;
    requestSequenceRef.current = requestSequence;
    if (!silent) {
      commitSnapshot(setStoredSnapshot, scopeKeyRef, scopeKey, (current) => ({
        ...current,
        error: null,
        isLoading: true,
      }));
    }

    try {
      const response = await listSubagentTasksApi(currentSource);
      if (!isCurrentRequest(scopeKeyRef, scopeKey, requestSequenceRef, requestSequence)) {
        return;
      }
      commitSnapshot(setStoredSnapshot, scopeKeyRef, scopeKey, (current) => ({
        ...current,
        data: normalizeSubagentTaskListResponse(response),
        error: null,
        isLoading: false,
      }));
    } catch (requestError) {
      if (!isCurrentRequest(scopeKeyRef, scopeKey, requestSequenceRef, requestSequence)) {
        return;
      }
      commitSnapshot(setStoredSnapshot, scopeKeyRef, scopeKey, (current) => ({
        ...current,
        error: requestError instanceof Error
          ? requestError.message
          : String(requestError),
        isLoading: false,
      }));
    }
  }, [scopeKey]);

  useEffect(() => {
    requestSequenceRef.current += 1;
    if (scopeKey) {
      void refresh();
    }
    return () => {
      requestSequenceRef.current += 1;
    };
  }, [refresh, scopeKey]);

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
    }, RUNNING_TASK_POLL_INTERVAL_MS);
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

function commitSnapshot(
  setSnapshot: Dispatch<SetStateAction<TaskListSnapshot>>,
  currentScopeKey: { current: string },
  expectedScopeKey: string,
  update: (snapshot: TaskListSnapshot) => TaskListSnapshot,
) {
  if (currentScopeKey.current !== expectedScopeKey) {
    return;
  }
  setSnapshot((current) => {
    if (currentScopeKey.current !== expectedScopeKey) {
      return current;
    }
    const scopedSnapshot = current.scopeKey === expectedScopeKey
      ? current
      : createTaskListSnapshot(expectedScopeKey);
    return update(scopedSnapshot);
  });
}

function isCurrentRequest(
  currentScopeKey: { current: string },
  expectedScopeKey: string,
  requestSequenceRef: { current: number },
  requestSequence: number,
): boolean {
  return currentScopeKey.current === expectedScopeKey
    && requestSequenceRef.current === requestSequence;
}
