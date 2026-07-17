import { useCallback } from "react";
import type { Dispatch, RefObject, SetStateAction } from "react";

import type { Message } from "@/types/conversation/message/entity";
import type { WebSocketState } from "@/types/system/websocket";

import { removeFailedOutboundUserMessage } from "../runtime/model/conversation-runtime-reconciliation";

interface UseRequestAckFailureOptions {
  clearOutboundRequest: (clientRequestId: string) => void;
  rejectPendingRequestAck: (
    clientRequestId: string,
    reason: string,
  ) => boolean;
  setError: Dispatch<SetStateAction<string | null>>;
  setMessages: Dispatch<SetStateAction<Message[]>>;
  wsReconnectRef: RefObject<() => void>;
  wsStateRef: RefObject<WebSocketState>;
}

export function useRequestAckFailure({
  clearOutboundRequest,
  rejectPendingRequestAck,
  setError,
  setMessages,
  wsReconnectRef,
  wsStateRef,
}: UseRequestAckFailureOptions) {
  // 超时只拒绝 ACK 等待并触发重连，失败消息由 Promise catch 统一收口。
  const handleRequestAckTimeout = useCallback((
    clientRequestId: string,
    message: string,
  ): void => {
    if (!rejectPendingRequestAck(clientRequestId, message)) {
      return;
    }
    if (wsStateRef.current === "connected") {
      wsReconnectRef.current();
    }
  }, [rejectPendingRequestAck, wsReconnectRef, wsStateRef]);

  const settleRequestAckWaitFailure = useCallback((
    clientRequestId: string,
    cause: unknown,
  ): void => {
    const message = cause instanceof Error
      ? cause.message
      : "消息未送达后端，请重试";
    clearOutboundRequest(clientRequestId);
    setError(message);
  }, [clearOutboundRequest, setError]);

  const settleChatAckWaitFailure = useCallback((
    clientRequestId: string,
    clientMessageId: string,
    cause: unknown,
  ): void => {
    settleRequestAckWaitFailure(clientRequestId, cause);
    setMessages((currentMessages) => (
      removeFailedOutboundUserMessage(currentMessages, clientMessageId)
    ));
  }, [setMessages, settleRequestAckWaitFailure]);

  return {
    handleRequestAckTimeout,
    settleChatAckWaitFailure,
    settleRequestAckWaitFailure,
  };
}
