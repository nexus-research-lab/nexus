/**
 * WebSocket 类型定义
 */

export type WebSocketState = 'disconnected' | 'connecting' | 'connected' | 'reconnecting' | 'failed';

export interface WebSocketMessage {
    type: string;
    [key: string]: any;
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

export interface WebSocketClientCallbacks {
    onOpen?: (event: Event) => void;
    onMessage?: (data: any) => void;
    onClose?: (event: CloseEvent) => void;
    onError?: (event: Event) => void;
    onReconnecting?: (attempt: number) => void;
    onReconnected?: () => void;
    onMaxRetriesReached?: () => void;
    onStateChange?: (state: WebSocketState) => void;
}
