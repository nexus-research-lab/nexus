/**
 * INPUT: 订阅消息、事件判定函数与权威读取动作。
 * OUTPUT: 无轮询的实时失效刷新：短窗口合并、单飞加尾随一次、重连补读、页面不可见时延后。
 * POS: 应用事件到 REST 快照重读的唯一协调器；只触发读取，不重放任何 mutation。
 */
"use client";

import { useCallback, useEffect, useRef } from "react";

import { getDesktopWebsocketProtocols } from "@/config/desktop-runtime";
import { getAgentWsUrl } from "@/config/runtime-endpoints";
import type { EventMessage } from "@/types/generated/protocol";
import type { WebSocketMessage } from "@/types/system/websocket";

import { parseEventMessage } from "./protocol/event-message";
import { useWebSocket } from "./use-socket";

const REFRESH_COALESCE_MS = 120;
const APP_EVENTS_SUBSCRIPTION: WebSocketMessage = { type: "subscribe_app_events" };

export interface AppEventRefreshOptions {
  enabled?: boolean;
  /** 返回 true 表示该事件令当前快照失效；可在此处理与可见性无关的副作用。 */
  onEvent: (event: EventMessage) => boolean;
  refresh: () => Promise<unknown>;
  /** 窗口重新获得焦点时补读；用于存在无推送状态（如外部连接就绪度）的资源。 */
  revalidateOnFocus?: boolean;
  /** 订阅消息；退订消息由 `subscribe_` 前缀推导，默认订阅全局应用事件。 */
  subscription?: WebSocketMessage;
}

export function useAppEventRefresh({
  enabled = true,
  onEvent,
  refresh,
  revalidateOnFocus = false,
  subscription = APP_EVENTS_SUBSCRIPTION,
}: AppEventRefreshOptions): void {
  const onEventRef = useRef(onEvent);
  onEventRef.current = onEvent;
  const refreshRef = useRef(refresh);
  refreshRef.current = refresh;
  const timerRef = useRef<number | null>(null);
  const runningRef = useRef(false);
  const trailingRef = useRef(false);
  const deferredRef = useRef(false);
  const enabledRef = useRef(enabled);
  enabledRef.current = enabled;

  const schedule = useCallback((): void => {
    if (!enabledRef.current || typeof window === "undefined") {
      return;
    }
    if (document.visibilityState !== "visible") {
      // 后台页面不读；回到前台时补读一次。
      deferredRef.current = true;
      return;
    }
    if (runningRef.current) {
      trailingRef.current = true;
      return;
    }
    if (timerRef.current !== null) {
      return;
    }
    timerRef.current = window.setTimeout(() => {
      timerRef.current = null;
      if (!enabledRef.current) {
        return;
      }
      runningRef.current = true;
      void refreshRef.current().catch((error: unknown) => {
        console.debug("[app-event-refresh] Coalesced refresh failed:", error);
      }).finally(() => {
        runningRef.current = false;
        if (trailingRef.current) {
          trailingRef.current = false;
          schedule();
        }
      });
    }, REFRESH_COALESCE_MS);
  }, []);

  useEffect(() => () => {
    trailingRef.current = false;
    deferredRef.current = false;
    if (timerRef.current !== null) {
      window.clearTimeout(timerRef.current);
      timerRef.current = null;
    }
  }, []);

  const handleMessage = useCallback((rawMessage: unknown) => {
    const event = parseEventMessage(rawMessage);
    if (event && onEventRef.current(event)) {
      schedule();
    }
  }, [schedule]);

  const { send, state } = useWebSocket({
    url: getAgentWsUrl(),
    protocols: getDesktopWebsocketProtocols(),
    autoConnect: enabled,
    reconnect: true,
    heartbeatInterval: 30000,
    onMessage: handleMessage,
  });

  const subscriptionKey = JSON.stringify(subscription);
  useEffect(() => {
    if (!enabled || state !== "connected") {
      return;
    }
    const message = JSON.parse(subscriptionKey) as WebSocketMessage;
    send(message);
    return () => {
      send({ ...message, type: message.type.replace(/^subscribe_/, "unsubscribe_") });
    };
  }, [enabled, send, state, subscriptionKey]);

  // 断线期间的事件已丢失：重连成功后补读一次，首连沿用调用方的首屏读取。
  const previousStateRef = useRef(state);
  useEffect(() => {
    const previous = previousStateRef.current;
    previousStateRef.current = state;
    if (state === "connected" && previous === "reconnecting") {
      schedule();
    }
  }, [schedule, state]);

  useEffect(() => {
    if (!enabled) {
      return;
    }
    const handleVisible = () => {
      if (document.visibilityState !== "visible") {
        return;
      }
      if (deferredRef.current || revalidateOnFocus) {
        deferredRef.current = false;
        schedule();
      }
    };
    window.addEventListener("focus", handleVisible);
    document.addEventListener("visibilitychange", handleVisible);
    return () => {
      window.removeEventListener("focus", handleVisible);
      document.removeEventListener("visibilitychange", handleVisible);
    };
  }, [enabled, revalidateOnFocus, schedule]);
}
