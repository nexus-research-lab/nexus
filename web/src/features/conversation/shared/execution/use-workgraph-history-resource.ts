/**
 * INPUT: 当前 session_key 与历史图 API。
 * OUTPUT: 按需刷新的 exact managed WorkGraph 历史资源。
 * POS: WorkGraph Surface 的历史读取控制器；不读取或修改命名工作图目录。
 */
"use client";

import { useCallback, useEffect, useRef } from "react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { getExecutionHistoryApi } from "@/lib/api/conversation/execution-api";
import { getErrorMessage } from "@/lib/error-message";
import type { ExecutionView } from "@/types/conversation/execution";

export interface WorkGraphHistoryResource {
  error: string | null;
  history: ExecutionView[];
  isLoading: boolean;
  isStale: boolean;
  refresh: () => void;
}

export function useWorkGraphHistoryResource(
  sessionKey: string | null,
  enabled: boolean,
): WorkGraphHistoryResource {
  const [history, setHistory] = useResettableState<ExecutionView[]>([], sessionKey);
  const [error, setError] = useResettableState<string | null>(null, sessionKey);
  const [isLoading, setLoading] = useResettableState(false, sessionKey);
  const requestRef = useRef(0);
  const activeSessionRef = useRef(sessionKey);
  activeSessionRef.current = sessionKey;

  const refresh = useCallback(async () => {
    if (!sessionKey || !enabled) {
      return;
    }
    const request = requestRef.current + 1;
    const requestSessionKey = sessionKey;
    requestRef.current = request;
    setLoading(true);
    setError(null);
    try {
      const nextHistory = await getExecutionHistoryApi(sessionKey);
      if (
        requestRef.current === request
        && activeSessionRef.current === requestSessionKey
      ) {
        setHistory(nextHistory);
      }
    } catch (reason) {
      if (
        requestRef.current === request
        && activeSessionRef.current === requestSessionKey
      ) {
        setError(getErrorMessage(reason, "WorkGraph 历史读取失败"));
      }
    } finally {
      if (
        requestRef.current === request
        && activeSessionRef.current === requestSessionKey
      ) {
        setLoading(false);
      }
    }
  }, [enabled, sessionKey, setError, setHistory, setLoading]);

  useEffect(() => {
    if (enabled) {
      void refresh();
    }
  }, [enabled, refresh]);

  return {
    error,
    history,
    isLoading,
    isStale: Boolean(error && history.length > 0),
    refresh: () => void refresh(),
  };
}
