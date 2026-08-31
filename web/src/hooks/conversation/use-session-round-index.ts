/**
 * INPUT: 当前 session_key 与只读 round index API。
 * OUTPUT: 按 exact session 隔离、失败时保留最后成功索引并提供显式重试的资源状态。
 * POS: Conversation 历史模式选择前的可靠性边界；不自动重试，也不把读取失败投影成空索引。
 */
import { useCallback, useEffect, useRef, useState } from "react";

import { getSessionRoundIndexApi } from "@/lib/api/conversation/session-api";
import {
  getResourceFailure,
  type ResourceAccessFailure,
} from "@/lib/error-message";
import type { SessionRoundIndexItem } from "@/types/conversation/history";

interface SessionRoundIndexSnapshot {
  access: ResourceAccessFailure | null;
  error: string | null;
  hasSuccessfulSnapshot: boolean;
  items: SessionRoundIndexItem[];
  loading: boolean;
  scopeKey: string;
}

export interface SessionRoundIndexResource {
  access: ResourceAccessFailure | null;
  error: string | null;
  hasSuccessfulSnapshot: boolean;
  isLoading: boolean;
  isStale: boolean;
  items: SessionRoundIndexItem[];
  retry: () => void;
  scopeKey: string | null;
}

function createSnapshot(scopeKey: string): SessionRoundIndexSnapshot {
  return {
    access: null,
    error: null,
    hasSuccessfulSnapshot: false,
    items: [],
    loading: Boolean(scopeKey),
    scopeKey,
  };
}

export function useSessionRoundIndex(
  sessionKey: string | null,
): SessionRoundIndexResource {
  const scopeKey = sessionKey?.trim() ?? "";
  const [snapshot, setSnapshot] = useState<SessionRoundIndexSnapshot>(() => (
    createSnapshot(scopeKey)
  ));
  const [refreshVersion, setRefreshVersion] = useState(0);
  const requestIdRef = useRef(0);
  const visibleSnapshot = snapshot.scopeKey === scopeKey
    ? snapshot
    : createSnapshot(scopeKey);

  const retry = useCallback(() => {
    if (scopeKey) {
      setRefreshVersion((current) => current + 1);
    }
  }, [scopeKey]);

  useEffect(() => {
    requestIdRef.current += 1;
    const requestId = requestIdRef.current;
    if (!scopeKey) {
      setSnapshot(createSnapshot(""));
      return;
    }
    const controller = new AbortController();
    setSnapshot((current) => current.scopeKey === scopeKey
      ? { ...current, loading: true }
      : createSnapshot(scopeKey));

    void getSessionRoundIndexApi(scopeKey, controller.signal)
      .then((items) => {
        if (requestIdRef.current !== requestId) {
          return;
        }
        setSnapshot({
          access: null,
          error: null,
          hasSuccessfulSnapshot: true,
          items,
          loading: false,
          scopeKey,
        });
      })
      .catch((error) => {
        if (requestIdRef.current !== requestId) {
          return;
        }
        if (error instanceof DOMException && error.name === "AbortError") {
          return;
        }
        console.error("[useSessionRoundIndex] 加载 session round 索引失败:", error);
        const failure = getResourceFailure(error, "更早消息索引暂时无法更新");
        setSnapshot((current) => {
          if (current.scopeKey !== scopeKey) {
            return current;
          }
          const accessLost = failure.access !== null;
          return {
            ...current,
            access: failure.access,
            error: failure.message,
            hasSuccessfulSnapshot: accessLost
              ? false
              : current.hasSuccessfulSnapshot,
            items: accessLost ? [] : current.items,
            loading: false,
          };
        });
      });
    return () => controller.abort();
  }, [refreshVersion, scopeKey]);

  return {
    access: visibleSnapshot.access,
    error: visibleSnapshot.error,
    hasSuccessfulSnapshot: visibleSnapshot.hasSuccessfulSnapshot,
    isLoading: visibleSnapshot.loading,
    isStale: Boolean(
      visibleSnapshot.error && visibleSnapshot.hasSuccessfulSnapshot,
    ),
    items: visibleSnapshot.items,
    retry,
    scopeKey: scopeKey || null,
  };
}
