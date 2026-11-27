/**
 * INPUT: 当前 session_key、WS 失效 revision 与 Execution 只读 API。
 * OUTPUT: 带请求竞态保护、单飞读取、失效合并和最近 managed 图保留的 WorkGraph 骨架资源。
 * POS: DM/Room 共用的 Execution 前端状态入口；节点运行详情由 use-execution-node-detail 按需读取。
 */
"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { getLatestExecutionApi } from "@/lib/api/conversation/execution-api";
import { getErrorMessage, getResourceFailure } from "@/lib/error-message";
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

/**
 * 读取调度状态：同一 session 同时至多一个请求；在途期间的失效只记一次 trailing
 * 读取，页面隐藏期间的失效推迟到重新可见时读一次。失效只是“请重读”的信号，
 * 合并后最终仍读到最新状态，不会让每个工具事件都触发一次整图读取。
 */
interface ExecutionReadSchedule {
  deferred: boolean;
  inFlight: boolean;
  trailing: boolean;
  version: number;
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

function isDocumentHidden(): boolean {
  return typeof document !== "undefined" && document.visibilityState === "hidden";
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
  const scheduleRef = useRef<ExecutionReadSchedule>({
    deferred: false,
    inFlight: false,
    trailing: false,
    version: 0,
  });
  const visibleSnapshot = snapshot.sessionKey === sessionKey
    ? snapshot
    : emptySnapshot(sessionKey);
  const rawExecution = visibleSnapshot.execution;
  const execution = rawExecution?.id === dismissedExecutionId
    ? null
    : rawExecution;

  const read = useCallback(async (showLoading: boolean): Promise<void> => {
    const schedule = scheduleRef.current;
    if (!sessionKey) {
      schedule.version += 1;
      setSnapshot(emptySnapshot(null));
      return;
    }
    if (schedule.inFlight) {
      schedule.trailing = true;
      return;
    }
    if (isDocumentHidden()) {
      schedule.deferred = true;
      return;
    }
    const version = schedule.version;
    schedule.inFlight = true;
    schedule.deferred = false;
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
      if (schedule.version !== version) {
        return;
      }
      setSnapshot({
        error: null,
        execution: latest,
        lastSuccessfulAt: Date.now(),
        loading: false,
        sessionKey,
      });
      if (latest) {
        setDismissedExecutionId((current) => (current === latest.id ? current : null));
      }
    } catch (error) {
      if (schedule.version !== version) {
        return;
      }
      setSnapshot((current) => current.sessionKey === sessionKey
        ? {
            ...current,
            error: getErrorMessage(error, "执行进程读取失败"),
            loading: false,
            ...(getResourceFailure(error, "").access ? { execution: null, lastSuccessfulAt: null } : {}),
          }
        : current);
    } finally {
      // 版本已变（切换 session/卸载）时，新作用域的调度已被重置，不得再改写它。
      if (schedule.version === version) {
        schedule.inFlight = false;
        if (schedule.trailing) {
          schedule.trailing = false;
          void read(false);
        }
      }
    }
  }, [sessionKey]);

  // 作用域切换：使在途请求失效并立刻释放单飞锁，新 session 不等待旧响应。
  useEffect(() => {
    const schedule = scheduleRef.current;
    return () => {
      schedule.version += 1;
      schedule.inFlight = false;
      schedule.trailing = false;
      schedule.deferred = false;
    };
  }, [sessionKey]);

  useEffect(() => {
    const timeout = window.setTimeout(() => {
      void read(true);
    }, EXECUTION_INVALIDATION_DEBOUNCE_MS);
    return () => window.clearTimeout(timeout);
  }, [invalidationKey, read]);

  useEffect(() => {
    const readWhenVisible = () => {
      if (document.visibilityState === "visible") {
        void read(false);
      }
    };
    document.addEventListener("visibilitychange", readWhenVisible);
    return () => document.removeEventListener("visibilitychange", readWhenVisible);
  }, [read]);

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
    refresh: () => void read(false),
    sessionKey,
  };
}
