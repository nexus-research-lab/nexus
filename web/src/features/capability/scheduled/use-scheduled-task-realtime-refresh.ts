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
  enabled_count,
  refresh_tasks,
  running_count,
}: ScheduledTaskRealtimeRefreshOptions): void {
  const ws_url = get_agent_ws_url();

  const handle_realtime_message = useCallback((raw_message: unknown) => {
    const event = raw_message as EventMessage;
    if (event.event_type !== "scheduled_task_changed") {
      return;
    }
    notify_scheduled_tasks_mutated(event.agent_id ?? "");
    if (typeof document !== "undefined" && document.visibilityState !== "visible") {
      return;
    }
    void refresh_tasks({ silent: true }).catch((err: unknown) => {
      console.debug("[scheduled-tasks] Realtime refresh failed:", err);
    });
  }, [refresh_tasks]);

  const { send: ws_send, state: ws_state } = useWebSocket({
    url: ws_url,
    protocols: get_desktop_websocket_protocols(),
    auto_connect: true,
    reconnect: true,
    heartbeat_interval: 30000,
    on_message: handle_realtime_message,
  });

  useAppEventSubscription(ws_send, ws_state);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }

    const handle_page_revalidate = () => {
      if (document.visibilityState !== "visible") {
        return;
      }
      void refresh_tasks({ silent: true }).catch((err: unknown) => {
        console.debug("[scheduled-tasks] Background refresh failed:", err);
      });
    };

    window.addEventListener("focus", handle_page_revalidate);
    document.addEventListener("visibilitychange", handle_page_revalidate);

    return () => {
      window.removeEventListener("focus", handle_page_revalidate);
      document.removeEventListener("visibilitychange", handle_page_revalidate);
    };
  }, [refresh_tasks]);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }
    if (ws_state === "connected") {
      return;
    }
    const poll_interval_ms = running_count > 0
      ? RUNNING_TASK_FALLBACK_POLL_INTERVAL_MS
      : enabled_count > 0 ? ENABLED_TASK_FALLBACK_POLL_INTERVAL_MS : 0;
    if (!poll_interval_ms) {
      return;
    }

    const interval_id = window.setInterval(() => {
      if (document.visibilityState !== "visible") {
        return;
      }
      void refresh_tasks({ silent: true }).catch((err: unknown) => {
        console.debug("[scheduled-tasks] Background refresh failed:", err);
      });
    }, poll_interval_ms);

    return () => window.clearInterval(interval_id);
  }, [enabled_count, refresh_tasks, running_count, ws_state]);
}
