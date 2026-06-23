import {
  Dispatch,
  MutableRefObject,
  SetStateAction,
  useEffect,
  useRef,
} from "react";

import { get_desktop_websocket_protocols } from "@/config/desktop-runtime";
import { useWebSocket } from "@/lib/websocket";
import {
  WebSocketMessage,
  WebSocketSendResult,
  WebSocketState,
} from "@/types/system/websocket";

import {
  build_room_subscription_message,
  build_session_bind_message,
} from "./conversation-actions";

type ConversationSocketSend = (payload: WebSocketMessage) => WebSocketSendResult;

interface UseAgentConversationSocketOptions {
  ws_url: string;
  agent_id: string | null;
  room_id: string | null;
  conversation_id: string | null;
  session_key: string | null;
  session_seq_cursor_ref: MutableRefObject<number>;
  room_seq_cursor_ref: MutableRefObject<number>;
  ws_send_ref: MutableRefObject<ConversationSocketSend>;
  ws_reconnect_ref: MutableRefObject<() => void>;
  ws_state_ref: MutableRefObject<WebSocketState>;
  on_message: (backend_message: unknown) => void;
  on_error?: (error: Error) => void;
  set_error: Dispatch<SetStateAction<string | null>>;
}

export function useAgentConversationSocket({
  ws_url,
  agent_id,
  room_id,
  conversation_id,
  session_key,
  session_seq_cursor_ref,
  room_seq_cursor_ref,
  ws_send_ref,
  ws_reconnect_ref,
  ws_state_ref,
  on_message,
  on_error,
  set_error,
}: UseAgentConversationSocketOptions) {
  const has_connected_ref = useRef(false);

  const {
    state: ws_state,
    send: ws_send,
    reconnect: ws_reconnect,
  } = useWebSocket({
    url: ws_url,
    protocols: get_desktop_websocket_protocols(),
    auto_connect: true,
    reconnect: true,
    heartbeat_interval: 30000,
    on_message,
    on_error: (event) => {
      // 开发环境 StrictMode 会触发一次挂载后立即清理，
      // 这时 connecting 阶段被主动断开会产生一次无意义的 error。
      if (!has_connected_ref.current) {
        console.debug(
          "[useAgentConversation] Ignored transient WebSocket error before first successful connection",
          event,
        );
        return;
      }

      const error_message = "WebSocket error occurred";
      console.error("[useAgentConversation] WebSocket error:", event);
      set_error(error_message);
      on_error?.(new Error(error_message));
    },
  });

  useEffect(() => {
    ws_send_ref.current = ws_send;
  }, [ws_send, ws_send_ref]);

  useEffect(() => {
    ws_reconnect_ref.current = ws_reconnect;
  }, [ws_reconnect, ws_reconnect_ref]);

  useEffect(() => {
    ws_state_ref.current = ws_state;
  }, [ws_state, ws_state_ref]);

  useEffect(() => {
    if (ws_state === "connected") {
      has_connected_ref.current = true;
      set_error(null);
    }
  }, [set_error, ws_state]);

  useEffect(() => {
    if (!agent_id || ws_state !== "connected") {
      return;
    }

    ws_send({
      type: "subscribe_workspace",
      agent_id,
      watch_files: true,
    });

    return () => {
      ws_send({
        type: "unsubscribe_workspace",
        agent_id,
        watch_files: true,
      });
    };
  }, [agent_id, ws_send, ws_state]);

  useEffect(() => {
    if (!session_key || ws_state !== "connected") {
      return;
    }

    // WebSocket 重连后，后端需要重新知道当前连接服务哪个 session，
    // 否则挂起中的权限请求无法重投到新连接。
    ws_send(build_session_bind_message({
      session_key,
      last_seen_session_seq: session_seq_cursor_ref.current,
      agent_id,
      room_id,
      conversation_id,
    }));

    return () => {
      // 共享 WebSocket 常驻于应用路由壳后，
      // 会话组件卸载时必须显式解绑旧 session，避免权限请求和 session 状态继续路由到已离开的页面上下文。
      ws_send({
        type: "unbind_session",
        session_key,
      });
    };
  }, [
    agent_id,
    conversation_id,
    room_id,
    session_key,
    session_seq_cursor_ref,
    ws_send,
    ws_state,
  ]);

  useEffect(() => {
    session_seq_cursor_ref.current = 0;
    room_seq_cursor_ref.current = 0;
  }, [room_id, room_seq_cursor_ref, session_key, session_seq_cursor_ref]);

  useEffect(() => {
    if (!room_id || ws_state !== "connected") {
      return;
    }

    ws_send(build_room_subscription_message({
      type: "subscribe_room",
      room_id,
      conversation_id,
      last_seen_room_seq: room_seq_cursor_ref.current,
    }));

    return () => {
      ws_send(build_room_subscription_message({
        type: "unsubscribe_room",
        room_id,
        conversation_id,
      }));
    };
  }, [conversation_id, room_id, room_seq_cursor_ref, ws_send, ws_state]);

  return { ws_send, ws_state };
}
