"use client";

import { useCallback, useEffect, useRef, useState } from "react";

interface ScopedSnapshot {
  scopeKey: string;
}

type SnapshotUpdater<Snapshot extends ScopedSnapshot> = (
  snapshot: Snapshot,
) => Snapshot;

interface ScopedResource<Snapshot extends ScopedSnapshot> {
  beginRequest: (expectedScopeKey: string) => number | null;
  commit: (
    expectedScopeKey: string,
    update: SnapshotUpdater<Snapshot>,
  ) => void;
  invalidateRequests: () => void;
  isCurrentRequest: (expectedScopeKey: string, requestId: number) => boolean;
  snapshot: Snapshot;
}

/**
 * 子智能体列表和线程都依赖同一条异步边界：结果只能写回发起请求时的作用域。
 * 该 Hook 只维护作用域与请求代次，不解释任何任务业务状态。
 */
export function useScopedResource<Snapshot extends ScopedSnapshot>(
  scopeKey: string,
  createSnapshot: (scopeKey: string) => Snapshot,
): ScopedResource<Snapshot> {
  const mountedRef = useRef(true);
  const scopeKeyRef = useRef(scopeKey);
  scopeKeyRef.current = scopeKey;
  const requestSequenceRef = useRef(0);
  const [storedSnapshot, setStoredSnapshot] = useState<Snapshot>(() =>
    createSnapshot(scopeKey),
  );
  const snapshot = storedSnapshot.scopeKey === scopeKey
    ? storedSnapshot
    : createSnapshot(scopeKey);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      requestSequenceRef.current += 1;
    };
  }, []);

  const commit = useCallback((
    expectedScopeKey: string,
    update: SnapshotUpdater<Snapshot>,
  ) => {
    if (!mountedRef.current || scopeKeyRef.current !== expectedScopeKey) {
      return;
    }
    setStoredSnapshot((current) => {
      if (!mountedRef.current || scopeKeyRef.current !== expectedScopeKey) {
        return current;
      }
      const scopedSnapshot = current.scopeKey === expectedScopeKey
        ? current
        : createSnapshot(expectedScopeKey);
      return update(scopedSnapshot);
    });
  }, [createSnapshot]);

  const beginRequest = useCallback((expectedScopeKey: string) => {
    if (!mountedRef.current || scopeKeyRef.current !== expectedScopeKey) {
      return null;
    }
    requestSequenceRef.current += 1;
    return requestSequenceRef.current;
  }, []);

  const invalidateRequests = useCallback(() => {
    requestSequenceRef.current += 1;
  }, []);

  const isCurrentRequest = useCallback((
    expectedScopeKey: string,
    requestId: number,
  ) => mountedRef.current
    && scopeKeyRef.current === expectedScopeKey
    && requestSequenceRef.current === requestId, []);

  return {
    beginRequest,
    commit,
    invalidateRequests,
    isCurrentRequest,
    snapshot,
  };
}
