/**
 * INPUT: Selected history entry and its exact session.
 * OUTPUT: Lazily loaded WorkGraph; late responses cannot cross selection or session boundaries.
 * POS: History detail read owner, separate from the lightweight history directory.
 */
import { useCallback, useEffect, useRef } from "react";
import { getExecutionApi } from "@/lib/api/conversation/execution-api";
import { getErrorMessage, getResourceFailure } from "@/lib/error-message";
import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import type { ExecutionView } from "@/types/conversation/execution";

export function useExecutionViewResource(sessionKey: string | null, executionId: string | null) {
  const scope = JSON.stringify([sessionKey, executionId]);
  const [execution, setExecution] = useResettableState<ExecutionView | null>(null, scope);
  const [error, setError] = useResettableState<string | null>(null, scope);
  const [isLoading, setLoading] = useResettableState(false, scope);
  const requestRef = useRef(0);
  const scopeRef = useRef(scope);
  scopeRef.current = scope;
  const refresh = useCallback(async () => {
    if (!sessionKey || !executionId) return;
    const request = ++requestRef.current;
    const current = () => requestRef.current === request && scopeRef.current === scope;
    setLoading(true);
    setError(null);
    try {
      const value = await getExecutionApi(sessionKey, executionId);
      if (current()) setExecution(value);
    } catch (reason) {
      if (current()) {
        if (getResourceFailure(reason, "").access) setExecution(null);
        setError(getErrorMessage(reason, "WorkGraph 历史读取失败"));
      }
    } finally {
      if (current()) setLoading(false);
    }
  }, [sessionKey, executionId, scope, setExecution, setError, setLoading]);
  useEffect(() => {
    void refresh();
    return () => { requestRef.current += 1; };
  }, [refresh]);
  return { execution, error, isLoading, isStale: Boolean(error && execution), refresh: () => void refresh() };
}
