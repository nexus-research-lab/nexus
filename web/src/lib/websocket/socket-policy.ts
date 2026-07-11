import type {
  WebSocketConfig,
  WebSocketMessage,
  WebSocketState,
} from "@/types/system/websocket";

export type ResolvedWebSocketConfig = Required<WebSocketConfig>;

const DEFAULT_CONFIG: ResolvedWebSocketConfig = {
  heartbeatInterval: 30_000,
  heartbeatTimeout: 10_000,
  maxReconnectAttempts: 5,
  maxReconnectDelay: 30_000,
  protocols: [],
  reconnect: true,
  reconnectDelay: 1_000,
  url: "",
};

const OFFLINE_QUEUE_MESSAGE_TYPES = new Set([
  "bind_session",
  "ping",
  "subscribe_app_events",
  "subscribe_room",
  "subscribe_workspace",
  "unbind_session",
  "unsubscribe_app_events",
  "unsubscribe_room",
  "unsubscribe_workspace",
]);

export function resolveWebSocketConfig(
  config: WebSocketConfig,
): ResolvedWebSocketConfig {
  return {
    heartbeatInterval:
      config.heartbeatInterval ?? DEFAULT_CONFIG.heartbeatInterval,
    heartbeatTimeout:
      config.heartbeatTimeout ?? DEFAULT_CONFIG.heartbeatTimeout,
    maxReconnectAttempts:
      config.maxReconnectAttempts ?? DEFAULT_CONFIG.maxReconnectAttempts,
    maxReconnectDelay:
      config.maxReconnectDelay ?? DEFAULT_CONFIG.maxReconnectDelay,
    protocols: config.protocols ?? DEFAULT_CONFIG.protocols,
    reconnect: config.reconnect ?? DEFAULT_CONFIG.reconnect,
    reconnectDelay: config.reconnectDelay ?? DEFAULT_CONFIG.reconnectDelay,
    url: config.url,
  };
}

export function buildSharedSocketKey(
  config: ResolvedWebSocketConfig,
): string {
  const protocols = Array.isArray(config.protocols)
    ? config.protocols.join("\u001e")
    : config.protocols;
  return [
    config.url,
    protocols,
    config.reconnect,
    config.maxReconnectAttempts,
    config.reconnectDelay,
    config.maxReconnectDelay,
    config.heartbeatInterval,
    config.heartbeatTimeout,
  ].join("\u001f");
}

export function shouldQueueWebSocketMessage(
  message: WebSocketMessage,
): boolean {
  return OFFLINE_QUEUE_MESSAGE_TYPES.has(message.type);
}

export function getReconnectDelay(
  config: ResolvedWebSocketConfig,
  attempt: number,
): number {
  return Math.min(
    config.reconnectDelay * 2 ** Math.max(0, attempt - 1),
    config.maxReconnectDelay,
  );
}

interface TransportStaleInput {
  config: ResolvedWebSocketConfig;
  isSocketOpen: boolean;
  lastServerActivityTime: number;
  now: number;
  state: WebSocketState;
}

export function isWebSocketTransportStale({
  config,
  isSocketOpen,
  lastServerActivityTime,
  now,
  state,
}: TransportStaleInput): boolean {
  if (
    state !== "connected" ||
    !isSocketOpen ||
    config.heartbeatInterval <= 0 ||
    lastServerActivityTime <= 0
  ) {
    return false;
  }
  const maxSilenceMs = config.heartbeatInterval + config.heartbeatTimeout;
  return now - lastServerActivityTime > maxSilenceMs;
}
