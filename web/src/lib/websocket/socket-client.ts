/**
 * WebSocket 核心客户端类
 */

import {
  WebSocketClientCallbacks,
  WebSocketConfig,
  WebSocketMessage,
  WebSocketSendResult,
  WebSocketState,
} from '@/types/system/websocket';
import { notify_auth_required } from '@/lib/api/http';

export class WebSocketClient {
  private ws: WebSocket | null = null;
  private config: Required<WebSocketConfig>;
  private callbacks: WebSocketClientCallbacks;
  private state: WebSocketState = 'disconnected';
  private isIntentionalDisconnect = false;
  private reconnectAttempts = 0;
  private reconnectTimer: NodeJS.Timeout | null = null;
  private heartbeatTimer: NodeJS.Timeout | null = null;
  private heartbeatTimeoutTimer: NodeJS.Timeout | null = null;
  private messageQueue: WebSocketMessage[] = [];
  private lastServerActivityTime = 0;

  private readonly DEFAULT_CONFIG: Required<WebSocketConfig> = {
    url: '',
    protocols: [],
    reconnect: true,
    max_reconnect_attempts: 5,
    reconnect_delay: 1000,
    max_reconnect_delay: 30000,
    heartbeat_interval: 30000,
    heartbeat_timeout: 10000,
  };

  constructor(config: WebSocketConfig, callbacks: WebSocketClientCallbacks = {}) {
    this.config = {...this.DEFAULT_CONFIG, ...config};
    this.callbacks = callbacks;
  }

  /**
   * 连接WebSocket
   */
  public connect(): void {
    if (this.state === 'connecting' || this.state === 'connected') {
      console.warn('[WebSocketClient] Already connecting or connected');
      return;
    }

    // 重连或重新连接前，清空主动断开标记，避免误伤真实错误。
    this.isIntentionalDisconnect = false;
    this.setState('connecting');
    this.createConnection();
  }

  /**
   * 断开连接
   */
  public disconnect(): void {
    // 标记为主动断开，忽略卸载或手动关闭过程中的 error/1006 噪音。
    this.isIntentionalDisconnect = true;
    this.config.reconnect = false; // 禁止自动重连
    this.cleanup();
    if (this.ws) {
      this.ws.close(1000, 'Client disconnect');
      this.ws = null;
    }
    this.setState('disconnected');
  }

  /**
   * 发送消息
   */
  public send(data: WebSocketMessage): WebSocketSendResult {
    if (!this.should_queue_message(data) && this.isTransportStale()) {
      console.warn('[WebSocketClient] Transport stale, reconnect before sending business message', data.type);
      this.forceReconnect('Transport stale');
      return { disposition: 'dropped' };
    }

    if (this.state === 'connected' && this.ws?.readyState === WebSocket.OPEN) {
      try {
        this.ws.send(JSON.stringify(data));
        return { disposition: 'sent' };
      } catch (error) {
        console.error('[WebSocketClient] Send error:', error);
        if (this.should_queue_message(data)) {
          this.messageQueue.push(data);
          return { disposition: 'queued' };
        }
        return { disposition: 'dropped' };
      }
    }

    if (this.should_queue_message(data)) {
      this.messageQueue.push(data);
      console.warn('[WebSocketClient] Message queued, not connected');
      return { disposition: 'queued' };
    }

    console.warn('[WebSocketClient] Message dropped, transport unavailable', data.type);
    return { disposition: 'dropped' };
  }

  /**
   * 获取当前状态
   */
  public getState(): WebSocketState {
    return this.state;
  }

  /**
   * 强制重建连接，避免继续复用僵尸连接。
   */
  public forceReconnect(reason: string = 'Force reconnect'): void {
    const previousSocket = this.ws;
    this.cleanup();
    this.isIntentionalDisconnect = false;
    this.ws = null;
    this.setState('reconnecting');

    if (previousSocket) {
      previousSocket.onopen = null;
      previousSocket.onmessage = null;
      previousSocket.onerror = null;
      previousSocket.onclose = null;
      try {
        previousSocket.close(4001, reason);
      } catch (error) {
        console.debug('[WebSocketClient] Ignore close error during forceReconnect', error);
      }
    }

    this.createConnection();
  }


  /**
   * 创建WebSocket连接
   */
  private createConnection(): void {
    try {
      this.ws = new WebSocket(this.config.url, this.config.protocols);

      this.ws.onopen = (event) => this.handleOpen(event);
      this.ws.onmessage = (event) => this.handleMessage(event);
      this.ws.onerror = (event) => this.handleError(event);
      this.ws.onclose = (event) => this.handleClose(event);
    } catch (error) {
      console.error('[WebSocketClient] Connection error:', error);
      this.handleConnectionFailure();
    }
  }

  /**
   * 处理连接打开
   */
  private handleOpen(event: Event): void {
    console.debug('[WebSocketClient] Connected');
    this.isIntentionalDisconnect = false;
    this.lastServerActivityTime = Date.now();
    this.setState('connected');
    this.reconnectAttempts = 0;

    // 启动心跳
    this.startHeartbeat();

    // 发送队列中的消息
    this.flushMessageQueue();

    // 回调
    this.callbacks.on_open?.(event);
    if (this.reconnectAttempts > 0) {
      this.callbacks.on_reconnected?.();
    }
  }

  /**
   * 处理消息
   */
  private handleMessage(event: MessageEvent): void {
    try {
      const data = JSON.parse(event.data);
      this.lastServerActivityTime = Date.now();

      // 处理pong响应
      if (data.event_type === 'pong') {
        this.resetHeartbeatTimeout();
        return;
      }

      this.callbacks.on_message?.(data);
    } catch (error) {
      console.error('[WebSocketClient] Message parse error:', error);
    }
  }

  /**
   * 处理连接错误
   */
  private handleError(event: Event): void {
    if (this.isIntentionalDisconnect) {
      console.debug('[WebSocketClient] Ignored WebSocket error during intentional disconnect');
      return;
    }

    console.error('[WebSocketClient] WebSocket error:', event);
    this.callbacks.on_error?.(event);
  }

  /**
   * 处理连接关闭
   */
  private handleClose(event: CloseEvent): void {
    console.debug('[WebSocketClient] Disconnected:', event.code, event.reason);

    this.cleanup();
    this.callbacks.on_close?.(event);

    if (this.isIntentionalDisconnect) {
      this.ws = null;
      this.setState('disconnected');
      return;
    }

    if (event.code === 4401) {
      this.ws = null;
      this.setState('failed');
      notify_auth_required();
      return;
    }

    // 判断是否需要重连
    if (this.config.reconnect && !event.wasClean && event.code !== 1000) {
      this.attemptReconnect();
    } else {
      this.ws = null;
      this.setState('disconnected');
    }
  }


  /**
   * 尝试重连
   */
  private attemptReconnect(): void {
    if (this.reconnectAttempts >= this.config.max_reconnect_attempts) {
      console.error('[WebSocketClient] Max reconnect attempts reached');
      this.setState('failed');
      this.callbacks.on_max_retries_reached?.();
      return;
    }

    this.reconnectAttempts++;
    this.setState('reconnecting');
    this.callbacks.on_reconnecting?.(this.reconnectAttempts);

    // 指数退避
    const delay = Math.min(
      this.config.reconnect_delay * Math.pow(2, this.reconnectAttempts - 1),
      this.config.max_reconnect_delay
    );

    console.debug(`[WebSocketClient] Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts}/${this.config.max_reconnect_attempts})`);

    this.reconnectTimer = setTimeout(() => {
      this.createConnection();
    }, delay);
  }

  /**
   * 连接失败处理
   */
  private handleConnectionFailure(): void {
    if (this.config.reconnect) {
      this.attemptReconnect();
    } else {
      this.setState('failed');
    }
  }

  /**
   * 启动心跳
   */
  private startHeartbeat(): void {
    this.stopHeartbeat();

    // 如果heartbeatInterval为0,则禁用心跳
    if (this.config.heartbeat_interval === 0) {
      return;
    }

    this.heartbeatTimer = setInterval(() => {
      if (this.state === 'connected') {
        this.send({type: 'ping'});

        // 启动心跳超时检测
        this.heartbeatTimeoutTimer = setTimeout(() => {
          console.warn('[WebSocketClient] Heartbeat timeout, reconnecting...');
          this.ws?.close(4000, 'Heartbeat timeout');
        }, this.config.heartbeat_timeout);
      }
    }, this.config.heartbeat_interval);
  }

  /**
   * 停止心跳
   */
  private stopHeartbeat(): void {
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer);
      this.heartbeatTimer = null;
    }
    if (this.heartbeatTimeoutTimer) {
      clearTimeout(this.heartbeatTimeoutTimer);
      this.heartbeatTimeoutTimer = null;
    }
  }

  /**
   * 重置心跳超时
   */
  private resetHeartbeatTimeout(): void {
    if (this.heartbeatTimeoutTimer) {
      clearTimeout(this.heartbeatTimeoutTimer);
      this.heartbeatTimeoutTimer = null;
    }
  }

  /**
   * 发送队列中的消息
   */
  private flushMessageQueue(): void {
    while (this.messageQueue.length > 0) {
      const message = this.messageQueue.shift();
      if (message) {
        this.send(message);
      }
    }
  }

  /**
   * 清理资源
   */
  private cleanup(): void {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    this.stopHeartbeat();
  }

  /**
   * 只有会话绑定/订阅这类幂等控制消息允许离线排队。
   * chat/interrupt/permission_response 必须立刻失败，避免前端误以为后端已受理。
   */
  private should_queue_message(data: WebSocketMessage): boolean {
    switch (data.type) {
      case 'ping':
      case 'bind_session':
      case 'unbind_session':
      case 'subscribe_room':
      case 'unsubscribe_room':
      case 'subscribe_workspace':
      case 'unsubscribe_workspace':
        return true;
      default:
        return false;
    }
  }

  /**
   * OPEN 但长时间收不到任何服务端活动时，视为僵尸连接。
   */
  private isTransportStale(): boolean {
    if (this.state !== 'connected' || this.ws?.readyState !== WebSocket.OPEN) {
      return false;
    }
    if (this.lastServerActivityTime <= 0) {
      return false;
    }

    const maxSilenceMs = this.config.heartbeat_interval + this.config.heartbeat_timeout;
    return Date.now() - this.lastServerActivityTime > maxSilenceMs;
  }

  /**
   * 设置状态
   */
  private setState(newState: WebSocketState): void {
    if (this.state !== newState) {
      console.debug(`[WebSocketClient] State: ${this.state} -> ${newState}`);
      this.state = newState;
      this.callbacks.on_state_change?.(newState);
    }
  }
}
