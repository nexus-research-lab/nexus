import { useEffect } from "react";

import type {
  WebSocketMessage,
  WebSocketSendResult,
  WebSocketState,
} from "@/types/system/websocket";

type WebSocketSend = (data: WebSocketMessage) => WebSocketSendResult;

export function useAppEventSubscription(
  send: WebSocketSend,
  state: WebSocketState,
): void {
  useEffect(() => {
    if (state !== "connected") {
      return;
    }
    send({ type: "subscribe_app_events" });
    return () => {
      send({ type: "unsubscribe_app_events" });
    };
  }, [send, state]);
}
