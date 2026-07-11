import type {
  WebSocketMessage,
  WebSocketSendResult,
  WebSocketState,
} from "@/types/system/websocket";

import { WebSocketClient } from "./socket-client";
import type { ResolvedWebSocketConfig } from "./socket-policy";

const SHARED_SOCKET_RELEASE_DELAY_MS = 300;

export interface SharedSocketSubscriber {
  onError?: (error: Event) => void;
  onMessage?: (message: unknown) => void;
  onStateChange?: (state: WebSocketState) => void;
  setError: (error: Event | null) => void;
  setState: (state: WebSocketState) => void;
}

interface SharedSocketSnapshot {
  error: Event | null;
  state: WebSocketState;
}

export class SharedWebSocketChannel {
  private readonly client: WebSocketClient;
  private readonly subscribers = new Map<number, SharedSocketSubscriber>();
  private nextSubscriberId = 1;
  private state: WebSocketState = "disconnected";
  private error: Event | null = null;

  constructor(config: ResolvedWebSocketConfig) {
    this.client = new WebSocketClient(config, {
      onError: (error) => this.publishError(error),
      onMessage: (message) => this.publishMessage(message),
      onStateChange: (state) => this.publishState(state),
    });
  }

  subscribe(subscriber: SharedSocketSubscriber): number {
    const subscriberId = this.nextSubscriberId;
    this.nextSubscriberId += 1;
    this.subscribers.set(subscriberId, subscriber);
    subscriber.setState(this.state);
    subscriber.setError(this.error);
    return subscriberId;
  }

  unsubscribe(subscriberId: number): void {
    this.subscribers.delete(subscriberId);
  }

  hasSubscribers(): boolean {
    return this.subscribers.size > 0;
  }

  connect(): void {
    if (this.state === "disconnected" || this.state === "failed") {
      this.client.connect();
    }
  }

  disconnect(): void {
    this.client.disconnect();
  }

  reconnect(): void {
    this.client.forceReconnect();
  }

  send(message: WebSocketMessage): WebSocketSendResult {
    return this.client.send(message);
  }

  getSnapshot(): SharedSocketSnapshot {
    return { error: this.error, state: this.state };
  }

  private publishMessage(message: unknown): void {
    for (const subscriber of this.subscribers.values()) {
      subscriber.onMessage?.(message);
    }
  }

  private publishError(error: Event): void {
    this.error = error;
    for (const subscriber of this.subscribers.values()) {
      subscriber.setError(error);
      subscriber.onError?.(error);
    }
  }

  private publishState(state: WebSocketState): void {
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
  }
}

class SharedWebSocketRegistry {
  private readonly channels = new Map<string, SharedWebSocketChannel>();
  private readonly cleanupTimers = new Map<string, number>();

  getSnapshot(channelKey: string): SharedSocketSnapshot {
    return (
      this.channels.get(channelKey)?.getSnapshot() ?? {
        error: null,
        state: "disconnected",
      }
    );
  }

  acquire(
    channelKey: string,
    config: ResolvedWebSocketConfig,
  ): SharedWebSocketChannel {
    this.cancelRelease(channelKey);
    const existingChannel = this.channels.get(channelKey);
    if (existingChannel) {
      return existingChannel;
    }
    const channel = new SharedWebSocketChannel(config);
    this.channels.set(channelKey, channel);
    return channel;
  }

  release(channelKey: string, channel: SharedWebSocketChannel): void {
    if (channel.hasSubscribers()) {
      return;
    }
    this.cancelRelease(channelKey);
    const timerId = window.setTimeout(() => {
      this.cleanupTimers.delete(channelKey);
      if (
        channel.hasSubscribers() ||
        this.channels.get(channelKey) !== channel
      ) {
        return;
      }
      console.debug("[useWebSocket] Cleaning up shared WebSocket client");
      channel.disconnect();
      this.channels.delete(channelKey);
    }, SHARED_SOCKET_RELEASE_DELAY_MS);
    this.cleanupTimers.set(channelKey, timerId);
  }

  private cancelRelease(channelKey: string): void {
    const timerId = this.cleanupTimers.get(channelKey);
    if (timerId === undefined) {
      return;
    }
    window.clearTimeout(timerId);
    this.cleanupTimers.delete(channelKey);
  }
}

export const sharedWebSocketRegistry = new SharedWebSocketRegistry();
