/**
 * useWebSocket Hook
 *
 * 在 React 组件中使用 WebSocket。
 */

import { useEffect, useRef, useState, useCallback } from "react";
import { WebSocketClient } from "./socket-client";
import {
  WebSocketConfig,
  WebSocketState,
  WebSocketMessage,
  WebSocketSendResult,
} from "@/types/system/websocket";

export interface UseWebSocketOptions extends WebSocketConfig {
  onMessage?: (message: any) => void;
  onError?: (error: Event) => void;
  onStateChange?: (state: WebSocketState) => void;
  autoConnect?: boolean;
}

interface SharedWebSocketSubscriber {
  id: number;
  onMessage?: (message: any) => void;
  onError?: (error: Event) => void;
  onStateChange?: (state: WebSocketState) => void;
  setError: (error: Event | null) => void;
  setState: (state: WebSocketState) => void;
}

class SharedWebSocketChannel {
  private readonly client: WebSocketClient;
  private readonly subscribers = new Map<number, SharedWebSocketSubscriber>();
  private state: WebSocketState = "disconnected";
  private error: Event | null = null;

  constructor(config: WebSocketConfig) {
    this.client = new WebSocketClient(config, {
      onMessage: (message) => {
        for (const subscriber of this.subscribers.values()) {
          subscriber.onMessage?.(message);
        }
      },
      onError: (error) => {
          this.error = error;
        for (const subscriber of this.subscribers.values()) {
          subscriber.setError(error);
          subscriber.onError?.(error);
        }
      },
      onStateChange: (state) => {
        this.state = state;
        if (state === "connected") {
          this.error = null;
        }
        for (const subscriber of this.subscribers.values()) {
          subscriber.setState(state);
          if (state === "connected") {
            subscriber.setError(null);
          }
          subscriber.onStateChange?.(state);
        }
      },
    });
  }

  public subscribe(subscriber: SharedWebSocketSubscriber): void {
    this.subscribers.set(subscriber.id, subscriber);
    subscriber.setState(this.state);
    subscriber.setError(this.error);
  }

  public unsubscribe(subscriberId: number): void {
    this.subscribers.delete(subscriberId);
  }

  public hasSubscribers(): boolean {
    return this.subscribers.size > 0;
  }

  public connect(): void {
    this.client.connect();
  }

  public disconnect(): void {
    this.client.disconnect();
  }

  public reconnect(): void {
    this.client.forceReconnect();
  }

  public send(data: WebSocketMessage): WebSocketSendResult {
    return this.client.send(data);
  }

  public getSnapshot(): { error: Event | null; state: WebSocketState } {
    return {
      state: this.state,
      error: this.error,
    };
  }
}

const sharedChannels = new Map<string, SharedWebSocketChannel>();
const sharedChannelCleanupTimers = new Map<string, number>();
let nextSubscriberId = 1;
const SHARED_SOCKET_RELEASE_DELAY_MS = 300;

function buildSharedChannelConfig(
  options: UseWebSocketOptions,
): WebSocketConfig {
  return {
    url: options.url,
    protocols: options.protocols ?? [],
    reconnect: options.reconnect ?? true,
    maxReconnectAttempts: options.maxReconnectAttempts ?? 5,
    reconnectDelay: options.reconnectDelay ?? 1000,
    maxReconnectDelay: options.maxReconnectDelay ?? 30000,
    heartbeatInterval: options.heartbeatInterval ?? 30000,
    heartbeatTimeout: options.heartbeatTimeout ?? 10000,
  };
}

function getOrCreateSharedChannel(
  options: UseWebSocketOptions,
): SharedWebSocketChannel {
  const channelKey = buildSharedChannelKey(options);
  const existingChannel = sharedChannels.get(channelKey);
  if (existingChannel) {
    return existingChannel;
  }

  const nextChannel = new SharedWebSocketChannel(
    buildSharedChannelConfig(options),
  );
  sharedChannels.set(channelKey, nextChannel);
  return nextChannel;
}

function buildSharedChannelKey(options: UseWebSocketOptions): string {
  const protocols = Array.isArray(options.protocols)
    ? options.protocols.join(",")
    : options.protocols ?? "";
  return `${options.url}::${protocols}`;
}

export function useWebSocket(options: UseWebSocketOptions) {
  const channelKey = buildSharedChannelKey(options);
  const [state, setState] = useState<WebSocketState>(
    () =>
      sharedChannels.get(channelKey)?.getSnapshot().state ?? "disconnected",
  );
  const [error, setError] = useState<Event | null>(
    () => sharedChannels.get(channelKey)?.getSnapshot().error ?? null,
  );
  const channelRef = useRef<SharedWebSocketChannel | null>(null);
  const onMessageRef = useRef(options.onMessage);
  const onErrorRef = useRef(options.onError);
  const onStateChangeRef = useRef(options.onStateChange);

  useEffect(() => {
    onMessageRef.current = options.onMessage;
    onErrorRef.current = options.onError;
    onStateChangeRef.current = options.onStateChange;
  }, [options.onError, options.onMessage, options.onStateChange]);

  // 使用useCallback稳定化回调函数
  const onMessageCallback = useCallback((msg: any) => {
    onMessageRef.current?.(msg);
  }, []);

  const onErrorCallback = useCallback((err: Event) => {
    onErrorRef.current?.(err);
  }, []);

  const onStateChangeCallback = useCallback((newState: WebSocketState) => {
    onStateChangeRef.current?.(newState);
  }, []);

  useEffect(() => {
    const cleanupTimer = sharedChannelCleanupTimers.get(channelKey);
    if (cleanupTimer) {
      window.clearTimeout(cleanupTimer);
      sharedChannelCleanupTimers.delete(channelKey);
    }

    const channel = getOrCreateSharedChannel(options);
    const subscriberId = nextSubscriberId++;

    channelRef.current = channel;
    channel.subscribe({
      id: subscriberId,
      onMessage: onMessageCallback,
      onError: onErrorCallback,
      onStateChange: onStateChangeCallback,
      setError: setError,
      setState: setState,
    });

    // 已登录应用内的多个页面共享同一条 WebSocket。
    // 这里仅在首次订阅时建立连接，后续页面切换复用现有客户端。
    if (options.autoConnect !== false) {
      channel.connect();
    }

    return () => {
      channel.unsubscribe(subscriberId);
      if (!channel.hasSubscribers()) {
        const nextTimer = window.setTimeout(() => {
          if (channel.hasSubscribers()) {
            return;
          }
          console.debug("[useWebSocket] Cleaning up shared WebSocket client");
          channel.disconnect();
          if (sharedChannels.get(channelKey) === channel) {
            sharedChannels.delete(channelKey);
          }
          sharedChannelCleanupTimers.delete(channelKey);
        }, SHARED_SOCKET_RELEASE_DELAY_MS);
        sharedChannelCleanupTimers.set(channelKey, nextTimer);
      }
      if (channelRef.current === channel) {
        channelRef.current = null;
      }
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- 回调已通过 ref 稳定化；共享连接按 url 和 protocol 维度创建，配置由首个订阅者固定。
  }, [channelKey, options.url]);

  useEffect(() => {
    if (typeof window === "undefined" || typeof document === "undefined") {
      return;
    }

    const reconnectWhenRecoverable = () => {
      const snapshot = channelRef.current?.getSnapshot();
      if (!snapshot) {
        return;
      }
      if (snapshot.state !== "failed") {
        return;
      }
      channelRef.current?.reconnect();
    };

    const handleVisibilityChange = () => {
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

  const send = useCallback((data: WebSocketMessage): WebSocketSendResult => {
    if (!channelRef.current) {
      return { disposition: "dropped" };
    }
    return channelRef.current.send(data);
  }, []);

  const connect = useCallback(() => {
    channelRef.current?.connect();
  }, []);

  const disconnect = useCallback(() => {
    channelRef.current?.disconnect();
  }, []);

  const reconnect = () => {
    channelRef.current?.reconnect();
  };

  return {
    state,
    error,
    send,
    connect,
    disconnect,
    reconnect,
  };
}
