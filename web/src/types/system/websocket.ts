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
  [key: string]: any;
}

export interface WebSocketConfig {
  url: string;
  protocols?: string | string[];
  reconnect?: boolean;
  max_reconnect_attempts?: number;
  reconnect_delay?: number;
  max_reconnect_delay?: number;
  heartbeat_interval?: number;
  heartbeat_timeout?: number;
}

export interface WebSocketClientCallbacks {
  on_open?: (event: Event) => void;
  on_message?: (data: any) => void;
  on_close?: (event: CloseEvent) => void;
  on_error?: (event: Event) => void;
  on_reconnecting?: (attempt: number) => void;
  on_reconnected?: () => void;
  on_max_retries_reached?: () => void;
  on_state_change?: (state: WebSocketState) => void;
}
