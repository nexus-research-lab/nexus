import {
  Dispatch,
  MutableRefObject,
  SetStateAction,
  useEffect,
  useRef,
} from "react";

import { getDesktopWebsocketProtocols } from "@/config/desktop-runtime";
import { useWebSocket } from "@/lib/websocket";
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

  const {
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

      const errorMessage = "WebSocket error occurred";
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
      setError(null);
    }
  }, [setError, wsState]);

  useEffect(() => {
    if (!agentId || wsState !== "connected") {
      return;
    }

    wsSend({
      type: "subscribe_workspace",
      agent_id: agentId,
      watch_files: true,
    });

    return () => {
      wsSend({
        type: "unsubscribe_workspace",
        agent_id: agentId,
        watch_files: true,
      });
    };
  }, [agentId, wsSend, wsState]);

  useEffect(() => {
    if (!sessionKey || wsState !== "connected") {
      return;
    }

    // WebSocket 重连后，后端需要重新知道当前连接服务哪个 session，
    // 否则挂起中的权限请求无法重投到新连接。
    wsSend(buildSessionBindMessage({
      session_key: sessionKey,
      last_seen_session_seq: sessionSeqCursorRef.current,
      agent_id: agentId,
      room_id: roomId,
      conversation_id: conversationId,
    }));

    return () => {
      // 共享 WebSocket 常驻于应用路由壳后，
      // 会话组件卸载时必须显式解绑旧 session，避免权限请求和 session 状态继续路由到已离开的页面上下文。
      wsSend({
        type: "unbind_session",
        session_key: sessionKey,
      });
    };
  }, [
    agentId,
    conversationId,
    roomId,
    sessionKey,
    sessionSeqCursorRef,
    wsSend,
    wsState,
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

  return { wsSend, wsState };
}
