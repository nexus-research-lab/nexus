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
  on_message?: (message: any) => void;
  on_error?: (error: Event) => void;
  on_state_change?: (state: WebSocketState) => void;
  auto_connect?: boolean;
}

interface SharedWebSocketSubscriber {
  id: number;
  on_message?: (message: any) => void;
  on_error?: (error: Event) => void;
  on_state_change?: (state: WebSocketState) => void;
  set_error: (error: Event | null) => void;
  set_state: (state: WebSocketState) => void;
}

class SharedWebSocketChannel {
  private readonly client: WebSocketClient;
  private readonly subscribers = new Map<number, SharedWebSocketSubscriber>();
  private state: WebSocketState = "disconnected";
  private error: Event | null = null;

  constructor(config: WebSocketConfig) {
    this.client = new WebSocketClient(config, {
      on_message: (message) => {
        for (const subscriber of this.subscribers.values()) {
          subscriber.on_message?.(message);
        }
      },
      on_error: (error) => {
        this.error = error;
        for (const subscriber of this.subscribers.values()) {
          subscriber.set_error(error);
          subscriber.on_error?.(error);
        }
      },
      on_state_change: (state) => {
        this.state = state;
        if (state === "connected") {
          this.error = null;
        }
        for (const subscriber of this.subscribers.values()) {
          subscriber.set_state(state);
          if (state === "connected") {
            subscriber.set_error(null);
          }
          subscriber.on_state_change?.(state);
        }
      },
    });
  }

  public subscribe(subscriber: SharedWebSocketSubscriber): void {
    this.subscribers.set(subscriber.id, subscriber);
    subscriber.set_state(this.state);
    subscriber.set_error(this.error);
  }

  public unsubscribe(subscriberId: number): void {
    this.subscribers.delete(subscriberId);
  }

  public has_subscribers(): boolean {
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

  public get_snapshot(): { error: Event | null; state: WebSocketState } {
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
    max_reconnect_attempts: options.max_reconnect_attempts ?? 5,
    reconnect_delay: options.reconnect_delay ?? 1000,
    max_reconnect_delay: options.max_reconnect_delay ?? 30000,
    heartbeat_interval: options.heartbeat_interval ?? 30000,
    heartbeat_timeout: options.heartbeat_timeout ?? 10000,
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
      sharedChannels.get(channelKey)?.get_snapshot().state ?? "disconnected",
  );
  const [error, setError] = useState<Event | null>(
    () => sharedChannels.get(channelKey)?.get_snapshot().error ?? null,
  );
  const channelRef = useRef<SharedWebSocketChannel | null>(null);
  const onMessageRef = useRef(options.on_message);
  const onErrorRef = useRef(options.on_error);
  const onStateChangeRef = useRef(options.on_state_change);

  useEffect(() => {
    onMessageRef.current = options.on_message;
    onErrorRef.current = options.on_error;
    onStateChangeRef.current = options.on_state_change;
  }, [options.on_error, options.on_message, options.on_state_change]);

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
      on_message: onMessageCallback,
      on_error: onErrorCallback,
      on_state_change: onStateChangeCallback,
      set_error: setError,
      set_state: setState,
    });

    // 已登录应用内的多个页面共享同一条 WebSocket。
    // 这里仅在首次订阅时建立连接，后续页面切换复用现有客户端。
    if (options.auto_connect !== false) {
      channel.connect();
    }

    return () => {
      channel.unsubscribe(subscriberId);
      if (!channel.has_subscribers()) {
        const nextTimer = window.setTimeout(() => {
          if (channel.has_subscribers()) {
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
      const snapshot = channelRef.current?.get_snapshot();
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
