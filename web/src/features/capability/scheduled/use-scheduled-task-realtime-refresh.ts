"use client";

import { useCallback, useEffect } from "react";

import { getDesktopWebsocketProtocols } from "@/config/desktop-runtime";
import { getAgentWsUrl } from "@/config/runtime-endpoints";
import { useAppEventSubscription, useWebSocket } from "@/lib/websocket";
import { parseEventMessage } from "@/lib/websocket/protocol/event-message";

import { notifyCapabilitySummaryMutated } from "../capability-summary-events";

const RUNNING_TASK_FALLBACK_POLL_INTERVAL_MS = 30000;
const ENABLED_TASK_FALLBACK_POLL_INTERVAL_MS = 120000;

function getFallbackPollInterval(enabledCount: number, runningCount: number): number {
  if (runningCount > 0) {
    return RUNNING_TASK_FALLBACK_POLL_INTERVAL_MS;
  }
  if (enabledCount > 0) {
    return ENABLED_TASK_FALLBACK_POLL_INTERVAL_MS;
  }
  return 0;
}

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
    const pollIntervalMs = getFallbackPollInterval(enabledCount, runningCount);
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
