import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";

import type {
  WebSocketConfig,
  WebSocketMessage,
  WebSocketSendResult,
  WebSocketState,
} from "@/types/system/websocket";

import {
  sharedWebSocketRegistry,
  type SharedWebSocketChannel,
} from "./shared-socket-channel";
import {
  buildSharedSocketKey,
  resolveWebSocketConfig,
} from "./socket-policy";

export interface UseWebSocketOptions extends WebSocketConfig {
  autoConnect?: boolean;
  onError?: (error: Event) => void;
  onMessage?: (message: unknown) => void;
  onStateChange?: (state: WebSocketState) => void;
}

export function useWebSocket(options: UseWebSocketOptions) {
  const {
    autoConnect,
    heartbeatInterval,
    heartbeatTimeout,
    maxReconnectAttempts,
    maxReconnectDelay,
    onError,
    onMessage,
    onStateChange,
    protocols,
    reconnect: reconnectEnabled,
    reconnectDelay,
    url,
  } = options;
  const protocolsKey = Array.isArray(protocols)
    ? protocols.join("\u001e")
    : (protocols ?? "");
  const config = useMemo(
    () => resolveWebSocketConfig({
      heartbeatInterval,
      heartbeatTimeout,
      maxReconnectAttempts,
      maxReconnectDelay,
      protocols: protocolsKey ? protocolsKey.split("\u001e") : [],
      reconnect: reconnectEnabled,
      reconnectDelay,
      url,
    }),
    [
      heartbeatInterval,
      heartbeatTimeout,
      maxReconnectAttempts,
      maxReconnectDelay,
      protocolsKey,
      reconnectDelay,
      reconnectEnabled,
      url,
    ],
  );
  const channelKey = buildSharedSocketKey(config);
  const initialSnapshot = sharedWebSocketRegistry.getSnapshot(channelKey);
  const [state, setState] = useState<WebSocketState>(initialSnapshot.state);
  const [error, setError] = useState<Event | null>(initialSnapshot.error);
  const channelRef = useRef<SharedWebSocketChannel | null>(null);
  const onMessageRef = useRef(onMessage);
  const onErrorRef = useRef(onError);
  const onStateChangeRef = useRef(onStateChange);

  useEffect(() => {
    onMessageRef.current = onMessage;
    onErrorRef.current = onError;
    onStateChangeRef.current = onStateChange;
  }, [onError, onMessage, onStateChange]);

  const publishMessage = useCallback((message: unknown) => {
    onMessageRef.current?.(message);
  }, []);
  const publishError = useCallback((nextError: Event) => {
    onErrorRef.current?.(nextError);
  }, []);
  const publishState = useCallback((nextState: WebSocketState) => {
    onStateChangeRef.current?.(nextState);
  }, []);

  useEffect(() => {
    const channel = sharedWebSocketRegistry.acquire(channelKey, config);
    channelRef.current = channel;
    const subscriberId = channel.subscribe({
      onError: publishError,
      onMessage: publishMessage,
      onStateChange: publishState,
      setError,
      setState,
    });
    if (autoConnect !== false) {
      channel.connect();
    }

    return () => {
      channel.unsubscribe(subscriberId);
      sharedWebSocketRegistry.release(channelKey, channel);
      if (channelRef.current === channel) {
        channelRef.current = null;
      }
    };
  }, [
    channelKey,
    config,
    autoConnect,
    publishError,
    publishMessage,
    publishState,
  ]);

  useEffect(() => {
    if (typeof window === "undefined" || typeof document === "undefined") {
      return;
    }

    const reconnectWhenRecoverable = (): void => {
      const snapshot = channelRef.current?.getSnapshot();
      if (snapshot?.state === "failed") {
        channelRef.current?.reconnect();
      }
    };
    const handleVisibilityChange = (): void => {
      if (document.visibilityState === "visible") {
        reconnectWhenRecoverable();
      }
    };

    window.addEventListener("online", reconnectWhenRecoverable);
    document.addEventListener("visibilitychange", handleVisibilityChange);
    return () => {
      window.removeEventListener("online", reconnectWhenRecoverable);
      document.removeEventListener("visibilitychange", handleVisibilityChange);
    };
  }, [channelKey]);

  const send = useCallback(
    (message: WebSocketMessage): WebSocketSendResult =>
      channelRef.current?.send(message) ?? { disposition: "dropped" },
    [],
  );
  const connect = useCallback(() => {
    channelRef.current?.connect();
  }, []);
  const disconnect = useCallback(() => {
    channelRef.current?.disconnect();
  }, []);
  const reconnect = useCallback(() => {
    channelRef.current?.reconnect();
  }, []);

  return { connect, disconnect, error, reconnect, send, state };
}
