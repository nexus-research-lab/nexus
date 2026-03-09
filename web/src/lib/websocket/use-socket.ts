/**
 * useWebSocket Hook
 *
 * 在React组件中使用WebSocket
 */

import { useEffect, useRef, useState, useCallback } from 'react';
import { WebSocketClient } from './socket-client';
import { WebSocketConfig, WebSocketState, WebSocketMessage } from './types';

export interface UseWebSocketOptions extends Omit<WebSocketConfig, 'protocols'> {
  onMessage?: (message: any) => void;
  onError?: (error: Event) => void;
  onStateChange?: (state: WebSocketState) => void;
  autoConnect?: boolean;
}

export function useWebSocket(options: UseWebSocketOptions) {
  const [state, setState] = useState<WebSocketState>('disconnected');
  const [error, setError] = useState<Event | null>(null);
  const clientRef = useRef<WebSocketClient | null>(null);

  // 使用useCallback稳定化回调函数
  const onMessageCallback = useCallback((msg: any) => {
    options.onMessage?.(msg);
  }, [options.onMessage]);

  const onErrorCallback = useCallback((err: Event) => {
    setError(err);
    options.onError?.(err);
  }, [options.onError]);

  const onStateChangeCallback = useCallback((newState: WebSocketState) => {
    setState(newState);
    options.onStateChange?.(newState);
  }, [options.onStateChange]);

  useEffect(() => {
    // 创建WebSocket客户端
    const client = new WebSocketClient({
      url: options.url,
      reconnect: options.reconnect ?? true,
      maxReconnectAttempts: options.maxReconnectAttempts ?? 5,
      reconnectDelay: options.reconnectDelay ?? 1000,
      maxReconnectDelay: options.maxReconnectDelay ?? 30000,
      heartbeatInterval: options.heartbeatInterval ?? 30000, // 支持外部配置心跳间隔
    }, {
      onMessage: onMessageCallback,
      onError: onErrorCallback,
      onStateChange: onStateChangeCallback,
    });

    clientRef.current = client;

    // 自动连接
    if (options.autoConnect !== false) {
      client.connect();
    }

    // 清理
    return () => {
      console.debug('[useWebSocket] Cleaning up WebSocket client');
      client.disconnect();
    };
  }, [options.url]); // 只依赖URL,避免重复创建

  const send = useCallback((data: WebSocketMessage) => {
    clientRef.current?.send(data);
  }, []);

  const connect = useCallback(() => {
    clientRef.current?.connect();
  }, []);

  const disconnect = useCallback(() => {
    clientRef.current?.disconnect();
  }, []);

  return {
    state,
    error,
    send,
    connect,
    disconnect,
  };
}
