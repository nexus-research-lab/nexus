// INPUT: Scheduled 变更事件、WebSocket 连接与页面可见性恢复信号。
// OUTPUT: 无轮询的 single-flight 静默刷新；同批事件只保留一个 trailing refresh。
// POS: Scheduled 实时失效协调器；只触发权威读取，不重放任何 mutation。

"use client";

import { useCallback, useEffect, useRef } from "react";

import { getDesktopWebsocketProtocols } from "@/config/desktop-runtime";
import { getAgentWsUrl } from "@/config/runtime-endpoints";
import { useAppEventSubscription, useWebSocket } from "@/lib/websocket";
import { parseEventMessage } from "@/lib/websocket/protocol/event-message";

import { notifyCapabilitySummaryMutated } from "../capability-summary-events";

interface ScheduledTaskRealtimeRefreshOptions {
  refreshTasks: (options?: { silent?: boolean }) => Promise<unknown>;
}

const REFRESH_COALESCE_MS = 120;

export function useScheduledTaskRealtimeRefresh({
  refreshTasks,
}: ScheduledTaskRealtimeRefreshOptions): void {
  const wsUrl = getAgentWsUrl();
  const refreshTasksRef = useRef(refreshTasks);
  refreshTasksRef.current = refreshTasks;
  const refreshTimerRef = useRef<number | null>(null);
  const refreshRunningRef = useRef(false);
  const trailingRefreshRef = useRef(false);
  const activeRef = useRef(typeof window !== "undefined");
  const flushRefreshRef = useRef<() => void>(() => undefined);

  const scheduleRefresh = useCallback((): void => {
    if (!activeRef.current || typeof window === "undefined") {
      return;
    }
    if (refreshRunningRef.current) {
      trailingRefreshRef.current = true;
      return;
    }
    if (refreshTimerRef.current !== null) {
      return;
    }
    refreshTimerRef.current = window.setTimeout(() => {
      flushRefreshRef.current();
    }, REFRESH_COALESCE_MS);
  }, []);

  useEffect(() => {
    activeRef.current = true;
    flushRefreshRef.current = () => {
      refreshTimerRef.current = null;
      if (!activeRef.current || refreshRunningRef.current) {
        if (refreshRunningRef.current) {
          trailingRefreshRef.current = true;
        }
        return;
      }
      refreshRunningRef.current = true;
      void refreshTasksRef.current({ silent: true }).catch((error: unknown) => {
        console.debug("[scheduled-tasks] Coalesced refresh failed:", error);
      }).finally(() => {
        refreshRunningRef.current = false;
        if (activeRef.current && trailingRefreshRef.current) {
          trailingRefreshRef.current = false;
          scheduleRefresh();
        }
      });
    };
    return () => {
      activeRef.current = false;
      trailingRefreshRef.current = false;
      flushRefreshRef.current = () => undefined;
      if (refreshTimerRef.current !== null) {
        window.clearTimeout(refreshTimerRef.current);
        refreshTimerRef.current = null;
      }
    };
  }, [scheduleRefresh]);

  const handleRealtimeMessage = useCallback((rawMessage: unknown) => {
    const event = parseEventMessage(rawMessage);
    if (!event || event.event_type !== "scheduled_task_changed") {
      return;
    }
    notifyCapabilitySummaryMutated({
      agent_id: event.agent_id ?? "",
      source: "scheduled_tasks",
    });
    if (typeof document !== "undefined" && document.visibilityState !== "visible") {
      return;
    }
    scheduleRefresh();
  }, [scheduleRefresh]);

  const { send: wsSend, state: wsState } = useWebSocket({
    url: wsUrl,
    protocols: getDesktopWebsocketProtocols(),
    autoConnect: true,
    reconnect: true,
    heartbeatInterval: 30000,
    onMessage: handleRealtimeMessage,
  });

  useAppEventSubscription(wsSend, wsState);

  const previousWsStateRef = useRef(wsState);
  useEffect(() => {
    const previousWsState = previousWsStateRef.current;
    previousWsStateRef.current = wsState;
    if (wsState !== "connected" || previousWsState === "connected") {
      return;
    }
    scheduleRefresh();
  }, [scheduleRefresh, wsState]);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }

    const handlePageRevalidate = () => {
      if (document.visibilityState !== "visible") {
        return;
      }
      scheduleRefresh();
    };

    window.addEventListener("focus", handlePageRevalidate);
    document.addEventListener("visibilitychange", handlePageRevalidate);

    return () => {
      window.removeEventListener("focus", handlePageRevalidate);
      document.removeEventListener("visibilitychange", handlePageRevalidate);
    };
  }, [scheduleRefresh]);
}
