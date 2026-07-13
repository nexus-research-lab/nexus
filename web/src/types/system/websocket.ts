/**
 * WebSocket 类型定义
 */

export type WebSocketState =
  | 'disconnected'
  | 'connecting'
  | 'connected'
  | 'reconnecting'
  | 'failed';

export interface WebSocketMessage {
  type: string;
  [key: string]: unknown;
}

export type WebSocketSendDisposition = "sent" | "queued" | "dropped";

export interface WebSocketSendResult {
  disposition: WebSocketSendDisposition;
}

export interface WebSocketConfig {
  url: string;
  protocols?: string | string[];
  reconnect?: boolean;
  maxReconnectAttempts?: number;
  reconnectDelay?: number;
  maxReconnectDelay?: number;
  heartbeatInterval?: number;
  heartbeatTimeout?: number;
}
