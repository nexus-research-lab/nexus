/**
 * INPUT: 规范化 Socket 配置、订阅者、Session/请求 lease 与 owner reset。
 * OUTPUT: 共享物理通道、引用计数生命周期和跨 owner 强制摘除/断连。
 * POS: WebSocket 连接注册中心；旧 channel cleanup 永远不能影响同 key 的新 owner 通道。
 */
import type {
  WebSocketMessage,
  WebSocketSendResult,
  WebSocketState,
} from "@/types/system/websocket";

import { WebSocketClient } from "./socket-client";
import type { ResolvedWebSocketConfig } from "./socket-policy";
import {
  RequestTransportLeaseRegistry,
  type RequestTransportLeaseOptions,
} from "./request-transport-leases";
import { SessionBindingLeaseRegistry } from "./session-binding-leases";

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
  private readonly requestTransports: RequestTransportLeaseRegistry;
  private readonly sessionBindings: SessionBindingLeaseRegistry;
  private readonly subscribers = new Map<number, SharedSocketSubscriber>();
  private nextSubscriberId = 1;
  private state: WebSocketState = "disconnected";
  private error: Event | null = null;

  constructor(
    config: ResolvedWebSocketConfig,
    onRequestTransportsIdle: () => void = () => {},
  ) {
    this.client = new WebSocketClient(config, {
      onError: (error) => this.publishError(error),
      onMessage: (message) => this.publishMessage(message),
      onStateChange: (state) => this.publishState(state),
    });
    this.sessionBindings = new SessionBindingLeaseRegistry(
      (message) => this.client.send(message),
      () => this.state === "connected",
    );
    this.requestTransports = new RequestTransportLeaseRegistry(
      (lease, binding) => this.sessionBindings.retain(lease, binding),
      onRequestTransportsIdle,
    );
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

  hasConsumers(): boolean {
    return this.hasSubscribers() || this.requestTransports.hasLeases();
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

  acquireSessionBinding(
    lease: object,
    message: WebSocketMessage,
  ): () => void {
    return this.sessionBindings.acquire(lease, message);
  }

  acquireRequestTransportLease(
    options: RequestTransportLeaseOptions,
  ): () => void {
    return this.requestTransports.acquire(options);
  }

  getSnapshot(): SharedSocketSnapshot {
    return { error: this.error, state: this.state };
  }

  /**
   * Auth owner 变化时永久废弃本通道。先断开网络，再静默清空请求、Session
   * 与订阅租约，避免旧身份的 binding 被新连接重放。
   */
  disposeOwnerScope(): void {
    this.client.disconnect();
    this.requestTransports.resetOwnerScope();
    this.sessionBindings.resetOwnerScope();
    this.subscribers.clear();
  }

  private publishMessage(message: unknown): void {
    this.requestTransports.handleMessage(message);
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
      this.sessionBindings.replay();
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

export class SharedWebSocketRegistry {
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
    let channel: SharedWebSocketChannel;
    channel = new SharedWebSocketChannel(
      config,
      () => this.release(channelKey, channel),
    );
    this.channels.set(channelKey, channel);
    return channel;
  }

  release(channelKey: string, channel: SharedWebSocketChannel): void {
    // owner reset 后旧 Hook cleanup 仍可能迟到；它没有权限触碰同 key 的新通道或其 timer。
    if (this.channels.get(channelKey) !== channel) {
      return;
    }
    if (channel.hasConsumers()) {
      return;
    }
    this.cancelRelease(channelKey);
    const timerId = window.setTimeout(() => {
      this.cleanupTimers.delete(channelKey);
      if (channel.hasConsumers() || this.channels.get(channelKey) !== channel) {
        return;
      }
      console.debug("[useWebSocket] Cleaning up shared WebSocket client");
      channel.disconnect();
      this.channels.delete(channelKey);
    }, SHARED_SOCKET_RELEASE_DELAY_MS);
    this.cleanupTimers.set(channelKey, timerId);
  }

  /** 原子摘除并断开全部旧 owner 通道；后续相同 key 必须创建新握手。 */
  resetOwnerScope(): void {
    for (const timerId of this.cleanupTimers.values()) {
      window.clearTimeout(timerId);
    }
    this.cleanupTimers.clear();
    const channels = Array.from(this.channels.values());
    this.channels.clear();
    for (const channel of channels) {
      channel.disposeOwnerScope();
    }
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

/** Auth owner scope reset 的应用装配入口。 */
export function resetSharedWebSocketsOwnerScope(): void {
  sharedWebSocketRegistry.resetOwnerScope();
}
