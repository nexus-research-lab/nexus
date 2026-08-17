"use client";

import { useCallback, useEffect, useRef } from "react";

import { getDesktopWebsocketProtocols } from "@/config/desktop-runtime";
import { getAgentWsUrl } from "@/config/runtime-endpoints";
import { useWebSocket } from "@/lib/websocket";
import { parseEventMessage } from "@/lib/websocket/protocol/event-message";
import type { SubagentTaskSource } from "@/types/conversation/subagent-task";

import { isSubagentTaskChangeFor } from "./subagent-task-model";

const SUBAGENT_TASK_INVALIDATION_DEBOUNCE_MS = 200;

interface SubagentTaskRealtimeRefreshOptions {
  enabled: boolean;
  hostAgentId?: string | null;
  onChanged: () => Promise<void>;
  source: SubagentTaskSource;
  taskId?: string | null;
}

export function useSubagentTaskRealtimeRefresh({
  enabled,
  hostAgentId,
  onChanged,
  source,
  taskId,
}: SubagentTaskRealtimeRefreshOptions): void {
  const pendingRefreshRef = useRef(false);
  const refreshTimeoutRef = useRef<number | null>(null);
  const scheduleRefresh = useCallback(() => {
    if (refreshTimeoutRef.current !== null) {
      window.clearTimeout(refreshTimeoutRef.current);
    }
    refreshTimeoutRef.current = window.setTimeout(() => {
      refreshTimeoutRef.current = null;
      void onChanged();
    }, SUBAGENT_TASK_INVALIDATION_DEBOUNCE_MS);
  }, [onChanged]);
  const handleRealtimeMessage = useCallback((rawMessage: unknown) => {
    const event = parseEventMessage(rawMessage);
    if (!enabled || !event || !isSubagentTaskChangeFor(
      event,
      source,
      taskId,
      hostAgentId,
    )) {
      return;
    }
    if (document.visibilityState !== "visible") {
      pendingRefreshRef.current = true;
      return;
    }
    scheduleRefresh();
  }, [enabled, hostAgentId, scheduleRefresh, source, taskId]);

  const { state } = useWebSocket({
    url: getAgentWsUrl(),
    protocols: getDesktopWebsocketProtocols(),
    autoConnect: enabled,
    reconnect: true,
    heartbeatInterval: 30000,
    onMessage: handleRealtimeMessage,
  });

  const previousStateRef = useRef(state);
  useEffect(() => {
    const previousState = previousStateRef.current;
    previousStateRef.current = state;
    if (!enabled || state !== "connected" || previousState !== "reconnecting") {
      return;
    }
    scheduleRefresh();
  }, [enabled, scheduleRefresh, state]);

  useEffect(() => {
    if (!enabled) {
      pendingRefreshRef.current = false;
      if (refreshTimeoutRef.current !== null) {
        window.clearTimeout(refreshTimeoutRef.current);
        refreshTimeoutRef.current = null;
      }
      return;
    }
    const refreshPendingChanges = () => {
      if (document.visibilityState !== "visible" || !pendingRefreshRef.current) {
        return;
      }
      pendingRefreshRef.current = false;
      scheduleRefresh();
    };
    document.addEventListener("visibilitychange", refreshPendingChanges);
    return () => document.removeEventListener("visibilitychange", refreshPendingChanges);
  }, [enabled, scheduleRefresh]);

  useEffect(() => () => {
    if (refreshTimeoutRef.current !== null) {
      window.clearTimeout(refreshTimeoutRef.current);
    }
  }, []);
}
