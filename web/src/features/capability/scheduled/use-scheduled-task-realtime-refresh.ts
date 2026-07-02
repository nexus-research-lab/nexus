"use client";

import { useCallback, useEffect } from "react";

import { get_desktop_websocket_protocols } from "@/config/desktop-runtime";
import { get_agent_ws_url } from "@/config/options";
import { useAppEventSubscription, useWebSocket } from "@/lib/websocket";
import type { EventMessage } from "@/types/conversation/message";

import { notify_scheduled_tasks_mutated } from "../scheduled-task-events";

const RUNNING_TASK_FALLBACK_POLL_INTERVAL_MS = 30000;
const ENABLED_TASK_FALLBACK_POLL_INTERVAL_MS = 120000;

interface ScheduledTaskRealtimeRefreshOptions {
  enabled_count: number;
  refresh_tasks: (options?: { silent?: boolean }) => Promise<void>;
  running_count: number;
}

export function useScheduledTaskRealtimeRefresh({
  enabled_count: enabledCount,
  refresh_tasks: refreshTasks,
  running_count: runningCount,
}: ScheduledTaskRealtimeRefreshOptions): void {
  const wsUrl = get_agent_ws_url();

  const handleRealtimeMessage = useCallback((rawMessage: unknown) => {
    const event = rawMessage as EventMessage;
    if (event.event_type !== "scheduled_task_changed") {
      return;
    }
    notify_scheduled_tasks_mutated(event.agent_id ?? "");
    if (typeof document !== "undefined" && document.visibilityState !== "visible") {
      return;
    }
    void refreshTasks({ silent: true }).catch((err: unknown) => {
      console.debug("[scheduled-tasks] Realtime refresh failed:", err);
    });
  }, [refreshTasks]);

  const { send: wsSend, state: wsState } = useWebSocket({
    url: wsUrl,
    protocols: get_desktop_websocket_protocols(),
    auto_connect: true,
    reconnect: true,
    heartbeat_interval: 30000,
    on_message: handleRealtimeMessage,
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
