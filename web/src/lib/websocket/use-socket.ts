/**
 * useWebSocket Hook
 *
 * 在React组件中使用WebSocket
 */

import { useEffect, useRef, useState, useCallback } from 'react';
import { WebSocketClient } from './socket-client';
import { WebSocketConfig, WebSocketState, WebSocketMessage } from '@/types/websocket';

export interface UseWebSocketOptions extends Omit<WebSocketConfig, 'protocols'> {
  on_message?: (message: any) => void;
  on_error?: (error: Event) => void;
  on_state_change?: (state: WebSocketState) => void;
  auto_connect?: boolean;
}

export function useWebSocket(options: UseWebSocketOptions) {
  const [state, setState] = useState<WebSocketState>('disconnected');
  const [error, setError] = useState<Event | null>(null);
  const clientRef = useRef<WebSocketClient | null>(null);
  const on_message_ref = useRef(options.on_message);
  const on_error_ref = useRef(options.on_error);
  const on_state_change_ref = useRef(options.on_state_change);

  useEffect(() => {
    on_message_ref.current = options.on_message;
  }, [options.on_message]);

  useEffect(() => {
    on_error_ref.current = options.on_error;
  }, [options.on_error]);

  useEffect(() => {
    on_state_change_ref.current = options.on_state_change;
  }, [options.on_state_change]);

  // 使用useCallback稳定化回调函数
  const on_message_callback = useCallback((msg: any) => {
    on_message_ref.current?.(msg);
  }, []);

  const on_error_callback = useCallback((err: Event) => {
    setError(err);
    on_error_ref.current?.(err);
  }, []);

  const on_state_change_callback = useCallback((new_state: WebSocketState) => {
    setState(new_state);
    on_state_change_ref.current?.(new_state);
  }, []);

  useEffect(() => {
    // 创建WebSocket客户端
    const client = new WebSocketClient({
      url: options.url,
      reconnect: options.reconnect ?? true,
      max_reconnect_attempts: options.max_reconnect_attempts ?? 5,
      reconnect_delay: options.reconnect_delay ?? 1000,
      max_reconnect_delay: options.max_reconnect_delay ?? 30000,
      heartbeat_interval: options.heartbeat_interval ?? 30000, // 支持外部配置心跳间隔
    }, {
      on_message: on_message_callback,
      on_error: on_error_callback,
      on_state_change: on_state_change_callback,
    });

    clientRef.current = client;

    // 自动连接
    if (options.auto_connect !== false) {
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
