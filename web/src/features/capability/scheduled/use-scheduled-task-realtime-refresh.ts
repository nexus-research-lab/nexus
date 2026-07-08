"use client";

import { useCallback, useEffect } from "react";

import { getDesktopWebsocketProtocols } from "@/config/desktop-runtime";
import { getAgentWsUrl } from "@/config/options";
import { useAppEventSubscription, useWebSocket } from "@/lib/websocket";
import type { EventMessage } from "@/types/conversation/message";

import { notifyScheduledTasksMutated } from "../scheduled-task-events";

const RUNNING_TASK_FALLBACK_POLL_INTERVAL_MS = 30000;
const ENABLED_TASK_FALLBACK_POLL_INTERVAL_MS = 120000;

interface ScheduledTaskRealtimeRefreshOptions {
  enabledCount: number;
  refreshTasks: (options?: { silent?: boolean }) => Promise<void>;
  runningCount: number;
}

export function useScheduledTaskRealtimeRefresh({
  enabledCount: enabledCount,
  refreshTasks: refreshTasks,
  runningCount: runningCount,
}: ScheduledTaskRealtimeRefreshOptions): void {
  const wsUrl = getAgentWsUrl();

  const handleRealtimeMessage = useCallback((rawMessage: unknown) => {
    const event = rawMessage as EventMessage;
    if (event.event_type !== "scheduled_task_changed") {
      return;
    }
    notifyScheduledTasksMutated(event.agent_id ?? "");
    if (typeof document !== "undefined" && document.visibilityState !== "visible") {
      return;
    }
    void refreshTasks({ silent: true }).catch((err: unknown) => {
      console.debug("[scheduled-tasks] Realtime refresh failed:", err);
    });
  }, [refreshTasks]);

  const { send: wsSend, state: wsState } = useWebSocket({
    url: wsUrl,
    protocols: getDesktopWebsocketProtocols(),
    autoConnect: true,
    reconnect: true,
    heartbeatInterval: 30000,
    onMessage: handleRealtimeMessage,
  });

  useAppEventSubscription(wsSend, wsState);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }

    const handlePageRevalidate = () => {
      if (document.visibilityState !== "visible") {
        return;
      }
      void refreshTasks({ silent: true }).catch((err: unknown) => {
        console.debug("[scheduled-tasks] Background refresh failed:", err);
      });
    };

    window.addEventListener("focus", handlePageRevalidate);
    document.addEventListener("visibilitychange", handlePageRevalidate);

    return () => {
      window.removeEventListener("focus", handlePageRevalidate);
      document.removeEventListener("visibilitychange", handlePageRevalidate);
    };
  }, [refreshTasks]);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }
    if (wsState === "connected") {
      return;
    }
    const pollIntervalMs = runningCount > 0
      ? RUNNING_TASK_FALLBACK_POLL_INTERVAL_MS
      : enabledCount > 0 ? ENABLED_TASK_FALLBACK_POLL_INTERVAL_MS : 0;
    if (!pollIntervalMs) {
      return;
    }

    const intervalId = window.setInterval(() => {
      if (document.visibilityState !== "visible") {
        return;
      }
      void refreshTasks({ silent: true }).catch((err: unknown) => {
        console.debug("[scheduled-tasks] Background refresh failed:", err);
      });
    }, pollIntervalMs);

    return () => window.clearInterval(intervalId);
  }, [enabledCount, refreshTasks, runningCount, wsState]);
}
