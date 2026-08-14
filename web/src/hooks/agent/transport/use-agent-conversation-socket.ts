import {
  Dispatch,
  MutableRefObject,
  SetStateAction,
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

type ConversationSocketSend = (payload: WebSocketMessage) => WebSocketSendResult;

const WEBSOCKET_ERROR_MESSAGE = "WebSocket error occurred";

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
  setError: Dispatch<SetStateAction<string | null>>;
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
  setError,
}: UseAgentConversationSocketOptions) {
  const hasConnectedRef = useRef(false);
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

      const errorMessage = WEBSOCKET_ERROR_MESSAGE;
      console.error("[useAgentConversation] WebSocket error:", event);
      setError(errorMessage);
      onError?.(new Error(errorMessage));
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
    if (wsState === "connected") {
      hasConnectedRef.current = true;
      // 重连只清理本 hook 产生的连接错误，不能覆盖已持久化的终态错误。
      setError((current) => (
        current === WEBSOCKET_ERROR_MESSAGE ? null : current
      ));
    }
  }, [setError, wsState]);

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
