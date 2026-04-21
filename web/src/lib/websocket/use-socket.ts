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

export interface UseWebSocketOptions extends Omit<
  WebSocketConfig,
  "protocols"
> {
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

  public unsubscribe(subscriber_id: number): void {
    this.subscribers.delete(subscriber_id);
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

const shared_channels = new Map<string, SharedWebSocketChannel>();
const shared_channel_cleanup_timers = new Map<string, number>();
let next_subscriber_id = 1;
const SHARED_SOCKET_RELEASE_DELAY_MS = 300;

function build_shared_channel_config(
  options: UseWebSocketOptions,
): WebSocketConfig {
  return {
    url: options.url,
    reconnect: options.reconnect ?? true,
    max_reconnect_attempts: options.max_reconnect_attempts ?? 5,
    reconnect_delay: options.reconnect_delay ?? 1000,
    max_reconnect_delay: options.max_reconnect_delay ?? 30000,
    heartbeat_interval: options.heartbeat_interval ?? 30000,
    heartbeat_timeout: options.heartbeat_timeout ?? 10000,
  };
}

function get_or_create_shared_channel(
  options: UseWebSocketOptions,
): SharedWebSocketChannel {
  const existing_channel = shared_channels.get(options.url);
  if (existing_channel) {
    return existing_channel;
  }

  const next_channel = new SharedWebSocketChannel(
    build_shared_channel_config(options),
  );
  shared_channels.set(options.url, next_channel);
  return next_channel;
}

export function useWebSocket(options: UseWebSocketOptions) {
  const [state, setState] = useState<WebSocketState>(
    () =>
      shared_channels.get(options.url)?.get_snapshot().state ?? "disconnected",
  );
  const [error, setError] = useState<Event | null>(
    () => shared_channels.get(options.url)?.get_snapshot().error ?? null,
  );
  const channel_ref = useRef<SharedWebSocketChannel | null>(null);
  const on_message_ref = useRef(options.on_message);
  const on_error_ref = useRef(options.on_error);
  const on_state_change_ref = useRef(options.on_state_change);

  useEffect(() => {
    on_message_ref.current = options.on_message;
    on_error_ref.current = options.on_error;
    on_state_change_ref.current = options.on_state_change;
  }, [options.on_error, options.on_message, options.on_state_change]);

  // 使用useCallback稳定化回调函数
  const on_message_callback = useCallback((msg: any) => {
    on_message_ref.current?.(msg);
  }, []);

  const on_error_callback = useCallback((err: Event) => {
    on_error_ref.current?.(err);
  }, []);

  const on_state_change_callback = useCallback((new_state: WebSocketState) => {
    on_state_change_ref.current?.(new_state);
  }, []);

  useEffect(() => {
    const cleanup_timer = shared_channel_cleanup_timers.get(options.url);
    if (cleanup_timer) {
      window.clearTimeout(cleanup_timer);
      shared_channel_cleanup_timers.delete(options.url);
    }

    const channel = get_or_create_shared_channel(options);
    const subscriber_id = next_subscriber_id++;

    channel_ref.current = channel;
    channel.subscribe({
      id: subscriber_id,
      on_message: on_message_callback,
      on_error: on_error_callback,
      on_state_change: on_state_change_callback,
      set_error: setError,
      set_state: setState,
    });

    // 已登录应用内的多个页面共享同一条 WebSocket。
    // 这里仅在首次订阅时建立连接，后续页面切换复用现有客户端。
    if (options.auto_connect !== false) {
      channel.connect();
    }

    return () => {
      channel.unsubscribe(subscriber_id);
      if (!channel.has_subscribers()) {
        const next_timer = window.setTimeout(() => {
          if (channel.has_subscribers()) {
            return;
          }
          console.debug("[useWebSocket] Cleaning up shared WebSocket client");
          channel.disconnect();
          if (shared_channels.get(options.url) === channel) {
            shared_channels.delete(options.url);
          }
          shared_channel_cleanup_timers.delete(options.url);
        }, SHARED_SOCKET_RELEASE_DELAY_MS);
        shared_channel_cleanup_timers.set(options.url, next_timer);
      }
      if (channel_ref.current === channel) {
        channel_ref.current = null;
      }
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- 回调已通过 ref 稳定化；共享连接按 url 维度创建，配置由首个订阅者固定。
  }, [options.url]);

  const send = useCallback((data: WebSocketMessage): WebSocketSendResult => {
    if (!channel_ref.current) {
      return { disposition: "dropped" };
    }
    return channel_ref.current.send(data);
  }, []);

  const connect = useCallback(() => {
    channel_ref.current?.connect();
  }, []);

  const disconnect = useCallback(() => {
    channel_ref.current?.disconnect();
  }, []);

  const reconnect = () => {
    channel_ref.current?.reconnect();
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
