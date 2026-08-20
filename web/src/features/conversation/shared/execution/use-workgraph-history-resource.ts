/**
 * INPUT: 当前 session_key 与历史图 API。
 * OUTPUT: 按需刷新的 exact managed WorkGraph 历史资源。
 * POS: WorkGraph Surface 的历史读取控制器；不读取或修改命名工作图目录。
 */
"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { getExecutionHistoryApi } from "@/lib/api/conversation/execution-api";
import { getErrorMessage } from "@/lib/error-message";
import type { ExecutionView } from "@/types/conversation/execution";

export interface WorkGraphHistoryResource {
  error: string | null;
  history: ExecutionView[];
  isLoading: boolean;
  refresh: () => void;
}

export function useWorkGraphHistoryResource(
  sessionKey: string | null,
  enabled: boolean,
): WorkGraphHistoryResource {
  const [history, setHistory] = useState<ExecutionView[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setLoading] = useState(false);
  const requestRef = useRef(0);

  const refresh = useCallback(async () => {
    if (!sessionKey || !enabled) {
      return;
    }
    const request = requestRef.current + 1;
    requestRef.current = request;
    setLoading(true);
    setError(null);
    try {
      const nextHistory = await getExecutionHistoryApi(sessionKey);
      if (requestRef.current === request) {
        setHistory(nextHistory);
      }
    } catch (reason) {
      if (requestRef.current === request) {
        setError(getErrorMessage(reason, "WorkGraph 历史读取失败"));
      }
    } finally {
      if (requestRef.current === request) {
        setLoading(false);
      }
    }
  }, [enabled, sessionKey]);

  useEffect(() => {
    if (enabled) {
      void refresh();
    }
  }, [enabled, refresh]);

  return {
    error,
    history,
    isLoading,
    refresh: () => void refresh(),
  };
}
