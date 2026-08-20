/**
 * INPUT: 当前 session_key 与 Execution 只读 API。
 * OUTPUT: 带请求竞态保护、WS 失效合并和最近 managed 图保留的 WorkGraph 资源。
 * POS: DM/Room 共用的 Execution 前端状态入口。
 */
"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { getLatestExecutionApi } from "@/lib/api/conversation/execution-api";
import { getErrorMessage } from "@/lib/error-message";
import type {
  ExecutionStatus,
  ExecutionView,
} from "@/types/conversation/execution";

const EXECUTION_INVALIDATION_DEBOUNCE_MS = 200;
const ACTIVE_EXECUTION_STATUSES = new Set<ExecutionStatus>([
  "active",
  "waiting",
]);

interface ExecutionResourceSnapshot {
  error: string | null;
  execution: ExecutionView | null;
  lastSuccessfulAt: number | null;
  loading: boolean;
  sessionKey: string | null;
}

export interface ExecutionResource {
  dismiss: () => void;
  error: string | null;
  execution: ExecutionView | null;
  isLoading: boolean;
  isStale: boolean;
  lastSuccessfulAt: number | null;
  refresh: () => void;
  sessionKey: string | null;
}

function emptySnapshot(sessionKey: string | null): ExecutionResourceSnapshot {
  return {
    error: null,
    execution: null,
    lastSuccessfulAt: null,
    loading: Boolean(sessionKey),
    sessionKey,
  };
}

export function useExecutionResource({
  invalidationKey = null,
  sessionKey,
}: {
  invalidationKey?: number | string | null;
  sessionKey: string | null;
}): ExecutionResource {
  const [snapshot, setSnapshot] = useState<ExecutionResourceSnapshot>(() => (
    emptySnapshot(sessionKey)
  ));
  const [dismissedExecutionId, setDismissedExecutionId] = useState<string | null>(
    null,
  );
  const requestVersionRef = useRef(0);
  const visibleSnapshot = snapshot.sessionKey === sessionKey
    ? snapshot
    : emptySnapshot(sessionKey);
  const rawExecution = visibleSnapshot.execution;
  const execution = rawExecution?.id === dismissedExecutionId
    ? null
    : rawExecution;

  const refresh = useCallback(async (showLoading = false) => {
    if (!sessionKey) {
      requestVersionRef.current += 1;
      setSnapshot(emptySnapshot(null));
      return;
    }
    const requestVersion = requestVersionRef.current + 1;
    requestVersionRef.current = requestVersion;
    if (showLoading) {
      setSnapshot((current) => ({
        error: null,
        execution: current.sessionKey === sessionKey ? current.execution : null,
        lastSuccessfulAt: current.sessionKey === sessionKey
          ? current.lastSuccessfulAt
          : null,
        loading: true,
        sessionKey,
      }));
    }
    try {
      const latest = await getLatestExecutionApi(sessionKey);
      if (requestVersionRef.current !== requestVersion) {
        return;
      }
      setSnapshot({
        error: null,
        execution: latest,
        lastSuccessfulAt: Date.now(),
        loading: false,
        sessionKey,
      });
      if (latest && latest.id !== dismissedExecutionId) {
        setDismissedExecutionId(null);
      }
    } catch (error) {
      if (requestVersionRef.current !== requestVersion) {
        return;
      }
      setSnapshot((current) => current.sessionKey === sessionKey
        ? {
            ...current,
            error: getErrorMessage(error, "执行进程读取失败"),
            loading: false,
          }
        : current);
    }
  }, [dismissedExecutionId, sessionKey]);

  useEffect(() => {
    const timeout = window.setTimeout(() => {
      void refresh(true);
    }, EXECUTION_INVALIDATION_DEBOUNCE_MS);
    return () => window.clearTimeout(timeout);
  }, [invalidationKey, refresh, sessionKey]);

  useEffect(() => {
    const refreshWhenVisible = () => {
      if (document.visibilityState === "visible") {
        void refresh(false);
      }
    };
    document.addEventListener("visibilitychange", refreshWhenVisible);
    return () => document.removeEventListener("visibilitychange", refreshWhenVisible);
  }, [refresh]);

  return {
    dismiss: () => {
      if (rawExecution && !ACTIVE_EXECUTION_STATUSES.has(rawExecution.status)) {
        setDismissedExecutionId(rawExecution.id);
      }
    },
    error: visibleSnapshot.error,
    execution,
    isLoading: visibleSnapshot.loading,
    isStale: Boolean(visibleSnapshot.error && execution),
    lastSuccessfulAt: visibleSnapshot.lastSuccessfulAt,
    refresh: () => void refresh(false),
    sessionKey,
  };
}
