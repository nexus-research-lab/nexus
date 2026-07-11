import { useCallback } from "react";
import type { Dispatch, RefObject, SetStateAction } from "react";

import type { Message, WebSocketState } from "@/types";

import { removeFailedOutboundUserMessage } from "../runtime/model/conversation-runtime-reconciliation";

interface UseChatAckFailureOptions {
  clearOutboundRequest: (clientRequestId: string) => void;
  rejectPendingChatAck: (clientRequestId: string, reason: string) => boolean;
  setError: Dispatch<SetStateAction<string | null>>;
  setMessages: Dispatch<SetStateAction<Message[]>>;
  wsReconnectRef: RefObject<() => void>;
  wsStateRef: RefObject<WebSocketState>;
}

export function useChatAckFailure({
  clearOutboundRequest,
  rejectPendingChatAck,
  setError,
  setMessages,
  wsReconnectRef,
  wsStateRef,
}: UseChatAckFailureOptions) {
  // 超时只拒绝 ACK 等待并触发重连，失败消息由 Promise catch 统一收口。
  const handleChatAckTimeout = useCallback((
    clientRequestId: string,
    message: string,
  ): void => {
    if (!rejectPendingChatAck(clientRequestId, message)) {
      return;
    }
    if (wsStateRef.current === "connected") {
      wsReconnectRef.current();
    }
  }, [rejectPendingChatAck, wsReconnectRef, wsStateRef]);

  const settleChatAckWaitFailure = useCallback((
    clientRequestId: string,
    clientMessageId: string,
    cause: unknown,
  ): void => {
    const message = cause instanceof Error
      ? cause.message
      : "消息未送达后端，请重试";
    clearOutboundRequest(clientRequestId);
    setMessages((currentMessages) => (
      removeFailedOutboundUserMessage(currentMessages, clientMessageId)
    ));
    setError(message);
  }, [clearOutboundRequest, setError, setMessages]);

  return { handleChatAckTimeout, settleChatAckWaitFailure };
}
