import { notifyAuthRequired } from "@/lib/api/core/http-auth";
import type {
  WebSocketConfig,
  WebSocketMessage,
  WebSocketSendResult,
  WebSocketState,
} from "@/types/system/websocket";

import { SocketHeartbeat } from "./socket-heartbeat";
import {
  getReconnectDelay,
  isWebSocketTransportStale,
  resolveWebSocketConfig,
  shouldQueueWebSocketMessage,
  type ResolvedWebSocketConfig,
} from "./socket-policy";

interface WebSocketClientCallbacks {
  onError?: (event: Event) => void;
  onMessage?: (data: unknown) => void;
  onStateChange?: (state: WebSocketState) => void;
}

export class WebSocketClient {
  private readonly callbacks: WebSocketClientCallbacks;
  private readonly config: ResolvedWebSocketConfig;
  private readonly heartbeat: SocketHeartbeat;
  private socket: WebSocket | null = null;
  private state: WebSocketState = "disconnected";
  private intentionalDisconnect = false;
  private reconnectEnabled: boolean;
  private reconnectAttempts = 0;
  private reconnectTimerId: number | null = null;
  private messageQueue: WebSocketMessage[] = [];
  private lastServerActivityTime = 0;

  constructor(
    config: WebSocketConfig,
    callbacks: WebSocketClientCallbacks = {},
  ) {
    this.config = resolveWebSocketConfig(config);
    this.callbacks = callbacks;
    this.reconnectEnabled = this.config.reconnect;
    this.heartbeat = new SocketHeartbeat({
      intervalMs: this.config.heartbeatInterval,
      timeoutMs: this.config.heartbeatTimeout,
      isConnected: () => this.isSocketOpen(),
      onTimeout: () => {
        console.warn("[WebSocketClient] Heartbeat timeout, reconnecting...");
        this.socket?.close(4000, "Heartbeat timeout");
      },
      sendPing: () => {
        this.send({ type: "ping" });
      },
    });
  }

  connect(): void {
    if (
      this.state === "connecting" ||
      this.state === "connected" ||
      this.state === "reconnecting"
    ) {
      return;
    }
    this.intentionalDisconnect = false;
    this.reconnectEnabled = this.config.reconnect;
    this.reconnectAttempts = 0;
    this.clearReconnectTimer();
    this.setState("connecting");
    this.createConnection();
  }

  disconnect(): void {
    this.intentionalDisconnect = true;
    this.reconnectEnabled = false;
    this.messageQueue = [];
    this.lastServerActivityTime = 0;
    this.cleanupTimers();
    this.closeCurrentSocket(1000, "Client disconnect");
    this.setState("disconnected");
  }

  forceReconnect(reason = "Force reconnect"): void {
    this.intentionalDisconnect = false;
    this.reconnectEnabled = this.config.reconnect;
    this.reconnectAttempts = 0;
    this.cleanupTimers();
    this.closeCurrentSocket(4001, reason);
    this.setState("reconnecting");
    this.createConnection();
  }

  send(message: WebSocketMessage): WebSocketSendResult {
    const canQueue = shouldQueueWebSocketMessage(message);
    if (!canQueue && this.isTransportStale()) {
      console.warn(
        "[WebSocketClient] Transport stale, reconnect before sending business message",
        message.type,
      );
      this.forceReconnect("Transport stale");
      return { disposition: "dropped" };
    }

    if (this.isSocketOpen()) {
      return this.sendOpenMessage(message, canQueue);
    }
    if (canQueue) {
      this.messageQueue.push(message);
      console.warn("[WebSocketClient] Message queued, not connected");
      return { disposition: "queued" };
    }
    console.warn(
      "[WebSocketClient] Message dropped, transport unavailable",
      message.type,
    );
    return { disposition: "dropped" };
  }

  private sendOpenMessage(
    message: WebSocketMessage,
    canQueue: boolean,
  ): WebSocketSendResult {
    try {
      this.socket?.send(JSON.stringify(message));
      return { disposition: "sent" };
    } catch (error) {
      console.error("[WebSocketClient] Send error:", error);
      if (canQueue) {
        this.messageQueue.push(message);
      }
      this.forceReconnect("Send failed");
      return { disposition: canQueue ? "queued" : "dropped" };
    }
  }

  private createConnection(): void {
    try {
      const socket = new WebSocket(this.config.url, this.config.protocols);
      this.socket = socket;
      socket.onopen = () => this.handleOpen(socket);
      socket.onmessage = (event) => this.handleMessage(socket, event);
      socket.onerror = (event) => this.handleError(socket, event);
      socket.onclose = (event) => this.handleClose(socket, event);
    } catch (error) {
      this.socket = null;
      console.error("[WebSocketClient] Connection error:", error);
      this.handleConnectionFailure();
    }
  }

  private handleOpen(socket: WebSocket): void {
    if (this.socket !== socket) {
      return;
    }
    console.debug("[WebSocketClient] Connected");
    this.intentionalDisconnect = false;
    this.lastServerActivityTime = Date.now();
    this.reconnectAttempts = 0;
    this.setState("connected");
    this.heartbeat.start();
    this.flushMessageQueue();
  }

  private handleMessage(socket: WebSocket, event: MessageEvent): void {
    if (this.socket !== socket) {
      return;
    }
    try {
      const data = JSON.parse(event.data) as { event_type?: string };
      this.lastServerActivityTime = Date.now();
      if (data.event_type === "pong") {
        this.heartbeat.acknowledge();
        return;
      }
      this.callbacks.onMessage?.(data);
    } catch (error) {
      console.error("[WebSocketClient] Message parse error:", error);
    }
  }

  private handleError(socket: WebSocket, event: Event): void {
    if (this.socket !== socket || this.intentionalDisconnect) {
      return;
    }
    console.error("[WebSocketClient] WebSocket error:", event);
    this.callbacks.onError?.(event);
  }

  private handleClose(socket: WebSocket, event: CloseEvent): void {
    if (this.socket !== socket) {
      return;
    }
    console.debug("[WebSocketClient] Disconnected:", event.code, event.reason);
    this.socket = null;
    this.cleanupTimers();

    if (this.intentionalDisconnect) {
      this.setState("disconnected");
      return;
    }
    if (event.code === 4401) {
      this.setState("failed");
      notifyAuthRequired();
      return;
    }
    if (this.reconnectEnabled && !event.wasClean && event.code !== 1000) {
      this.attemptReconnect();
      return;
    }
    this.setState("disconnected");
  }

  private handleConnectionFailure(): void {
    if (this.reconnectEnabled) {
      this.attemptReconnect();
      return;
    }
    this.setState("failed");
  }

  private attemptReconnect(): void {
    if (this.reconnectAttempts >= this.config.maxReconnectAttempts) {
      console.error("[WebSocketClient] Max reconnect attempts reached");
      this.setState("failed");
      return;
    }

    this.reconnectAttempts += 1;
    this.setState("reconnecting");
    const delay = getReconnectDelay(this.config, this.reconnectAttempts);
    console.debug(
      `[WebSocketClient] Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts}/${this.config.maxReconnectAttempts})`,
    );
    this.clearReconnectTimer();
    this.reconnectTimerId = window.setTimeout(() => {
      this.reconnectTimerId = null;
      this.createConnection();
    }, delay);
  }

  private flushMessageQueue(): void {
    const queuedMessages = this.messageQueue;
    this.messageQueue = [];
    for (const message of queuedMessages) {
      this.send(message);
    }
  }

  private isSocketOpen(): boolean {
    return (
      this.state === "connected" &&
      this.socket?.readyState === WebSocket.OPEN
    );
  }

  private isTransportStale(): boolean {
    return isWebSocketTransportStale({
      config: this.config,
      isSocketOpen: this.isSocketOpen(),
      lastServerActivityTime: this.lastServerActivityTime,
      now: Date.now(),
      state: this.state,
    });
  }

  private closeCurrentSocket(code: number, reason: string): void {
    const socket = this.socket;
    this.socket = null;
    if (!socket) {
      return;
    }
    socket.onopen = null;
    socket.onmessage = null;
    socket.onerror = null;
    socket.onclose = null;
    try {
      socket.close(code, reason);
    } catch (error) {
      console.debug("[WebSocketClient] Ignore close error", error);
    }
  }

  private cleanupTimers(): void {
    this.clearReconnectTimer();
    this.heartbeat.stop();
  }

  private clearReconnectTimer(): void {
    if (this.reconnectTimerId !== null) {
      window.clearTimeout(this.reconnectTimerId);
      this.reconnectTimerId = null;
    }
  }

  private setState(nextState: WebSocketState): void {
    if (this.state === nextState) {
      return;
    }
    console.debug(`[WebSocketClient] State: ${this.state} -> ${nextState}`);
    this.state = nextState;
    this.callbacks.onStateChange?.(nextState);
  }
}
