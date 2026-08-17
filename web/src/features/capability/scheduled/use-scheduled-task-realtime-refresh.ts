"use client";

import { useCallback, useEffect, useRef } from "react";

import { getDesktopWebsocketProtocols } from "@/config/desktop-runtime";
import { getAgentWsUrl } from "@/config/runtime-endpoints";
import { useAppEventSubscription, useWebSocket } from "@/lib/websocket";
import { parseEventMessage } from "@/lib/websocket/protocol/event-message";

import { notifyCapabilitySummaryMutated } from "../capability-summary-events";

interface ScheduledTaskRealtimeRefreshOptions {
  refreshTasks: (options?: { silent?: boolean }) => Promise<void>;
}

export function useScheduledTaskRealtimeRefresh({
  refreshTasks,
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

  const previousWsStateRef = useRef(wsState);
  useEffect(() => {
    const previousWsState = previousWsStateRef.current;
    previousWsStateRef.current = wsState;
    if (wsState !== "connected" || previousWsState === "connected") {
      return;
    }
    void refreshTasks({ silent: true }).catch((err: unknown) => {
      console.debug("[scheduled-tasks] Connection refresh failed:", err);
    });
  }, [refreshTasks, wsState]);

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
}
