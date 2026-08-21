/**
 * INPUT: 共享 WebSocket、Session/Room 订阅身份、reliability controller 与重连对账回调。
 * OUTPUT: 自动重连的发送面、binding/subscription replay 和连接恢复后的 durable Session 对账触发。
 * POS: Agent Conversation transport 生命周期边界；不重发业务命令。
 */
import {
  MutableRefObject,
  useEffect,
  useRef,
} from "react";

import { getDesktopWebsocketProtocols } from "@/config/desktop-runtime";
import { useWebSocket } from "@/lib/websocket";
import type { RequestTransportLeaseOptions } from "@/lib/websocket/request-transport-leases";
import {
  WebSocketMessage,
  WebSocketSendResult,
  WebSocketState,
} from "@/types/system/websocket";

import {
  buildRoomSubscriptionMessage,
  buildSessionBindMessage,
} from "../actions/conversation-command-builders";
import type { ConversationReliabilityController } from "../reliability/use-conversation-reliability";

type ConversationSocketSend = (payload: WebSocketMessage) => WebSocketSendResult;

interface UseAgentConversationSocketOptions {
  wsUrl: string;
  agentId: string | null;
  roomId: string | null;
  conversationId: string | null;
  sessionKey: string | null;
  sessionSeqCursorRef: MutableRefObject<number>;
  roomSeqCursorRef: MutableRefObject<number>;
  wsSendRef: MutableRefObject<ConversationSocketSend>;
  wsReconnectRef: MutableRefObject<() => void>;
  wsStateRef: MutableRefObject<WebSocketState>;
  onMessage: (backendMessage: unknown) => void;
  onError?: (error: Error) => void;
  onReconnected: () => Promise<unknown>;
  reliability: ConversationReliabilityController;
}

export function shouldReconcileConversationAfterReconnect(
  hasConnected: boolean,
  previousState: WebSocketState,
  currentState: WebSocketState,
): boolean {
  return hasConnected
    && previousState !== "connected"
    && currentState === "connected";
}

export function useAgentConversationSocket({
  wsUrl,
  agentId,
  roomId,
  conversationId,
  sessionKey,
  sessionSeqCursorRef,
  roomSeqCursorRef,
  wsSendRef,
  wsReconnectRef,
  wsStateRef,
  onMessage,
  onError,
  onReconnected,
  reliability,
}: UseAgentConversationSocketOptions) {
  const hasConnectedRef = useRef(false);
  const previousStateRef = useRef<WebSocketState>("disconnected");
  const sessionBindingLeaseRef = useRef<object>({});

  const {
    acquireRequestTransportLease,
    acquireSessionBinding,
    channelKey,
    state: wsState,
    send: wsSend,
    reconnect: wsReconnect,
  } = useWebSocket({
    url: wsUrl,
    protocols: getDesktopWebsocketProtocols(),
    autoConnect: true,
    reconnect: true,
    heartbeatInterval: 30000,
    onMessage: onMessage,
    onError: (event) => {
      // 开发环境 StrictMode 会触发一次挂载后立即清理，
      // 这时 connecting 阶段被主动断开会产生一次无意义的 error。
      if (!hasConnectedRef.current) {
        console.debug(
          "[useAgentConversation] Ignored transient WebSocket error before first successful connection",
          event,
        );
        return;
      }

      console.error("[useAgentConversation] WebSocket error:", event);
      onError?.(new Error("Conversation realtime transport error"));
    },
  });

  useEffect(() => {
    wsSendRef.current = wsSend;
  }, [wsSend, wsSendRef]);

  useEffect(() => {
    wsReconnectRef.current = wsReconnect;
  }, [wsReconnect, wsReconnectRef]);

  useEffect(() => {
    wsStateRef.current = wsState;
  }, [wsState, wsStateRef]);

  useEffect(() => {
    reliability.observeTransport(wsState);
    if (shouldReconcileConversationAfterReconnect(
      hasConnectedRef.current,
      previousStateRef.current,
      wsState,
    )) {
      // 共享通道会先重放 Session binding，再通知订阅者 connected。
      // 此处重拉 durable 快照，与随后到达的实时事件按消息身份合并，闭合 DM gap；
      // Room 还会额外使用 room_seq replay 和 subscribe snapshot 对账。
      void onReconnected();
    }
    if (wsState === "connected") {
      hasConnectedRef.current = true;
    }
    previousStateRef.current = wsState;
  }, [onReconnected, reliability, wsState]);

  useEffect(() => {
    if (!agentId || wsState !== "connected") {
      return;
    }

    wsSend({
      type: "subscribe_workspace",
      agent_id: agentId,
    });

    return () => {
      wsSend({
        type: "unsubscribe_workspace",
        agent_id: agentId,
      });
    };
  }, [agentId, wsSend, wsState]);

  useEffect(() => {
    if (!sessionKey) {
      return;
    }

    // 共享物理连接按逻辑消费者持有 session 租约：任一旧面板 cleanup
    // 只能释放自己的租约，最后一个消费者离开才会真正 unbind；
    // 通道重连时会统一重放仍有效的绑定与挂起权限请求。
    return acquireSessionBinding(sessionBindingLeaseRef.current, buildSessionBindMessage({
      session_key: sessionKey,
      last_seen_session_seq: sessionSeqCursorRef.current,
      agent_id: agentId,
      room_id: roomId,
      conversation_id: conversationId,
    }));
  }, [
    acquireSessionBinding,
    agentId,
    channelKey,
    conversationId,
    roomId,
    sessionKey,
    sessionSeqCursorRef,
  ]);

  useEffect(() => {
    sessionSeqCursorRef.current = 0;
    roomSeqCursorRef.current = 0;
  }, [roomId, roomSeqCursorRef, sessionKey, sessionSeqCursorRef]);

  useEffect(() => {
    if (!roomId || wsState !== "connected") {
      return;
    }

    wsSend(buildRoomSubscriptionMessage({
      type: "subscribe_room",
      room_id: roomId,
      conversation_id: conversationId,
      last_seen_room_seq: roomSeqCursorRef.current,
    }));

    return () => {
      wsSend(buildRoomSubscriptionMessage({
        type: "unsubscribe_room",
        room_id: roomId,
        conversation_id: conversationId,
      }));
    };
  }, [conversationId, roomId, roomSeqCursorRef, wsSend, wsState]);

  return {
    acquireRequestTransportLease: (
      options: RequestTransportLeaseOptions,
    ) => acquireRequestTransportLease(options),
    wsSend,
    wsState,
  };
}
