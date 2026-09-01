/**
 * INPUT: WebSocket 配置、React 生命周期与挂载时 auth owner generation。
 * OUTPUT: 共享连接的 owner-scoped 事件/状态投影、发送面和逻辑租约。
 * POS: React 与共享 WebSocket 通道边界；旧 owner 订阅不得收取事件或继续发送。
 */
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  useSyncExternalStore,
} from "react";

import {
  captureAuthOwnerScopeGeneration,
  isAuthOwnerScopeGenerationCurrent,
  subscribeAuthOwnerScopeGeneration,
} from "@/shared/auth/auth-owner-generation";
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
import type { RequestTransportLeaseOptions } from "./request-transport-leases";
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
  const ownerScopeGeneration = useSyncExternalStore(
    subscribeAuthOwnerScopeGeneration,
    captureAuthOwnerScopeGeneration,
    captureAuthOwnerScopeGeneration,
  );
  const initialSnapshot = sharedWebSocketRegistry.getSnapshot(channelKey);
  const [state, setState] = useState<WebSocketState>(initialSnapshot.state);
  const [error, setError] = useState<Event | null>(initialSnapshot.error);
  const channelRef = useRef<SharedWebSocketChannel | null>(null);
  const ownerScopeGenerationRef = useRef(ownerScopeGeneration);
  const onMessageRef = useRef(onMessage);
  const onErrorRef = useRef(onError);
  const onStateChangeRef = useRef(onStateChange);

  useEffect(() => {
    onMessageRef.current = onMessage;
    onErrorRef.current = onError;
    onStateChangeRef.current = onStateChange;
  }, [onError, onMessage, onStateChange]);

  const ownsCurrentScope = useCallback(
    () => isAuthOwnerScopeGenerationCurrent(ownerScopeGenerationRef.current),
    [],
  );

  useEffect(() => {
    const channel = sharedWebSocketRegistry.acquire(channelKey, config);
    // reset 已经从 registry 摘除旧通道；只有绑定到新通道后才能接受当前代次。
    ownerScopeGenerationRef.current = ownerScopeGeneration;
    channelRef.current = channel;
    const ownsChannelScope = () => (
      isAuthOwnerScopeGenerationCurrent(ownerScopeGeneration)
      && channelRef.current === channel
    );
    const subscriberId = channel.subscribe({
      onError: (nextError) => {
        if (ownsChannelScope()) {
          onErrorRef.current?.(nextError);
        }
      },
      onMessage: (message) => {
        if (ownsChannelScope()) {
          onMessageRef.current?.(message);
        }
      },
      onStateChange: (nextState) => {
        if (ownsChannelScope()) {
          onStateChangeRef.current?.(nextState);
        }
      },
      setError: (nextError) => {
        if (ownsChannelScope()) {
          setError(nextError);
        }
      },
      setState: (nextState) => {
        if (ownsChannelScope()) {
          setState(nextState);
        }
      },
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
    ownerScopeGeneration,
  ]);

  useEffect(() => {
    if (typeof window === "undefined" || typeof document === "undefined") {
      return;
    }

    const reconnectWhenRecoverable = (): void => {
      if (!ownsCurrentScope()) {
        return;
      }
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
  }, [channelKey, ownsCurrentScope]);

  const send = useCallback(
    (message: WebSocketMessage): WebSocketSendResult => {
      if (!ownsCurrentScope()) {
        return { disposition: "dropped" };
      }
      return channelRef.current?.send(message) ?? { disposition: "dropped" };
    },
    [ownsCurrentScope],
  );
  const connect = useCallback(() => {
    if (ownsCurrentScope()) {
      channelRef.current?.connect();
    }
  }, [ownsCurrentScope]);
  const disconnect = useCallback(() => {
    if (ownsCurrentScope()) {
      channelRef.current?.disconnect();
    }
  }, [ownsCurrentScope]);
  const reconnect = useCallback(() => {
    if (ownsCurrentScope()) {
      channelRef.current?.reconnect();
    }
  }, [ownsCurrentScope]);
  const acquireSessionBinding = useCallback(
    (lease: object, message: WebSocketMessage): (() => void) => {
      if (!ownsCurrentScope()) {
        return () => {};
      }
      const channel = channelRef.current;
      return channel?.acquireSessionBinding(lease, message) ?? (() => {});
    },
    [ownsCurrentScope],
  );
  const acquireRequestTransportLease = useCallback(
    (options: RequestTransportLeaseOptions): (() => void) => {
      if (!ownsCurrentScope()) {
        return () => {};
      }
      const channel = channelRef.current;
      if (!channel) {
        return () => {};
      }
      const generation = ownerScopeGenerationRef.current;
      const ownsLeaseScope = () => (
        isAuthOwnerScopeGenerationCurrent(generation)
        && channelRef.current === channel
      );
      return channel.acquireRequestTransportLease({
        ...options,
        onAccepted: () => {
          if (ownsLeaseScope()) {
            options.onAccepted();
          }
        },
        onRejected: (reason) => {
          if (ownsLeaseScope()) {
            options.onRejected(reason);
          }
        },
        onTimeout: options.onTimeout
          ? () => {
              if (ownsLeaseScope()) {
                options.onTimeout?.();
              }
            }
          : undefined,
      });
    },
    [ownsCurrentScope],
  );

  return {
    acquireRequestTransportLease,
    acquireSessionBinding,
    channelKey,
    connect,
    disconnect,
    error,
    reconnect,
    send,
    state,
  };
}
