import { useCallback } from "react";
import type { Dispatch, RefObject, SetStateAction } from "react";

import { notifyDesktopDiagnostic } from "@/config/desktop-runtime";
import type { InputQueueItem } from "@/types/agent/agent-conversation";
import type { AgentConversationIdentity } from "@/types/agent/agent-conversation";
import type { Message } from "@/types/conversation/message/entity";
import type { WebSocketState } from "@/types/system/websocket";

import { removeFailedOutboundUserMessage } from "../runtime/model/conversation-runtime-reconciliation";
import type { ConversationReliabilityController } from "../reliability/use-conversation-reliability";
import {
  RequestAcceptanceRejectedError,
  RequestAcceptanceUnknownError,
} from "./use-pending-request-acks";

export { RequestAcceptanceUnknownError } from "./use-pending-request-acks";

interface UseRequestAckFailureOptions {
  activeSessionKeyRef: RefObject<string | null>;
  clearOutboundRequest: (clientRequestId: string) => void;
  getInputQueueItems: () => readonly InputQueueItem[];
  hasPendingRequestAck: (clientRequestId: string) => boolean;
  rejectPendingRequestAck: (
    clientRequestId: string,
    cause: string | Error,
  ) => boolean;
  readSessionMessages: (
    identity: AgentConversationIdentity | null,
    sessionKey: string,
  ) => Promise<Message[]>;
  reloadCurrentSession: () => Promise<Message[] | null>;
  resolvePendingRequestAck: (clientRequestId?: string | null) => boolean;
  reliability: ConversationReliabilityController;
  setMessages: Dispatch<SetStateAction<Message[]>>;
  wsReconnectRef: RefObject<() => void>;
  wsStateRef: RefObject<WebSocketState>;
}

export interface RequestAckRecoveryScope {
  identity: AgentConversationIdentity | null;
  sessionKey: string;
}

export interface RequestAckTimeoutOptions {
  recoveryScope?: RequestAckRecoveryScope;
  unknownMessage?: string;
}

export function buildRequestAckRecoveryReader(
  recoveryScope: RequestAckRecoveryScope | undefined,
  readSessionMessages: UseRequestAckFailureOptions["readSessionMessages"],
  reloadCurrentSession: UseRequestAckFailureOptions["reloadCurrentSession"],
): () => Promise<Message[] | null> {
  return recoveryScope
    ? () => readSessionMessages(
        recoveryScope.identity,
        recoveryScope.sessionKey,
      )
    : reloadCurrentSession;
}

export function hasAcceptedClientMessage(
  messages: Message[],
  clientMessageId: string,
  inputQueueItems: readonly InputQueueItem[] = [],
): boolean {
  return messages.some((item) => (
    item.role === "user" && item.client_message_id === clientMessageId
  )) || inputQueueItems.some((item) => (
    item.source === "user" && item.client_message_id === clientMessageId
  ));
}

export type RequestAckRecoveryOutcome = "accepted" | "unknown";

interface RecoverRequestAckTimeoutOptions {
  clientMessageId: string;
  inputQueueItems: () => readonly InputQueueItem[];
  reconnect: () => void;
  reload: () => Promise<Message[] | null>;
  websocketState: () => WebSocketState;
}

function restoreLiveSubscription(
  reconnect: () => void,
  state: WebSocketState,
): void {
  // ACK 慢不等于连接失活：强制重连会取消后端仍在受理的请求。
  // 连通性由心跳检测；这里只恢复断开或已耗尽重试的连接。
  if (state === "disconnected" || state === "failed") {
    reconnect();
  }
}

/** ACK 超时后寻找 durable 正向证据，仅恢复已失效的连接。 */
export async function recoverRequestAckTimeout({
  clientMessageId,
  inputQueueItems,
  reconnect,
  reload,
  websocketState,
}: RecoverRequestAckTimeoutOptions): Promise<RequestAckRecoveryOutcome> {
  try {
    restoreLiveSubscription(reconnect, websocketState());
  } catch (error) {
    console.warn("[RequestAck] 重连失败，继续核对受理证据", {
      client_message_id: clientMessageId,
      error,
    });
    notifyDesktopDiagnostic("request_ack.reconnect_failed", {
      client_message_id: clientMessageId,
    }, error);
  }
  let messages: Message[] = [];
  try {
    messages = await reload() ?? [];
  } catch (error) {
    // 历史读取失败不应遮蔽已经到达的 durable 队列快照。
    console.warn("[RequestAck] 受理核对历史读取失败", {
      client_message_id: clientMessageId,
      error,
    });
    notifyDesktopDiagnostic("request_ack.history_failed", {
      client_message_id: clientMessageId,
    }, error);
  }
  if (hasAcceptedClientMessage(
    messages,
    clientMessageId,
    inputQueueItems(),
  )) {
    return "accepted";
  }
  // 超时、历史缺失与当前队列缺失都不是拒绝证据：消息可能已经入队但
  // 尚未收到重连快照，也可能刚出队而 round marker 仍在写入。
  return "unknown";
}

export function useRequestAckFailure({
  activeSessionKeyRef,
  clearOutboundRequest,
  getInputQueueItems,
  hasPendingRequestAck,
  readSessionMessages,
  rejectPendingRequestAck,
  reloadCurrentSession,
  resolvePendingRequestAck,
  reliability,
  setMessages,
  wsReconnectRef,
  wsStateRef,
}: UseRequestAckFailureOptions) {
  // ACK 丢失不等于后端未受理；超时只读对账，保留仍连通的请求和 optimistic 消息。
  const handleRequestAckTimeout = useCallback((
    clientRequestId: string,
    clientMessageId: string,
    options: RequestAckTimeoutOptions = {},
  ): void => {
    const recoveryScope = options.recoveryScope;
    const startedAt = Date.now();
    const diagnostic = {
      client_request_id: clientRequestId,
      client_message_id: clientMessageId,
      session_key: recoveryScope?.sessionKey ?? activeSessionKeyRef.current,
      websocket_state: wsStateRef.current,
    };
    console.warn("[RequestAck] ACK 超时，开始核对受理状态", diagnostic);
    notifyDesktopDiagnostic("request_ack.timeout", diagnostic);
    void recoverRequestAckTimeout({
      clientMessageId,
      inputQueueItems: getInputQueueItems,
      reconnect: () => wsReconnectRef.current(),
      reload: buildRequestAckRecoveryReader(
        recoveryScope,
        readSessionMessages,
        reloadCurrentSession,
      ),
      websocketState: () => wsStateRef.current,
    }).catch((error): RequestAckRecoveryOutcome => {
      // 恢复自身失败也只能得到未知状态，必须收口原 waiter。
      console.error("[RequestAck] 受理核对失败", { ...diagnostic, error });
      notifyDesktopDiagnostic("request_ack.recovery_failed", diagnostic, error);
      return "unknown";
    }).then((outcome) => {
      const resultDiagnostic = {
        ...diagnostic,
        duration_ms: Date.now() - startedAt,
        outcome,
        pending: hasPendingRequestAck(clientRequestId),
      };
      console.info("[RequestAck] 受理核对完成", resultDiagnostic);
      notifyDesktopDiagnostic("request_ack.recovered", resultDiagnostic);
      // reload 期间真实 ACK 可能已经到达；此时不能再次 settle，否则会在
      // ACK registry 留下永远无人消费的 early-ACK 状态。
      if (!hasPendingRequestAck(clientRequestId)) {
        return;
      }
      if (outcome === "accepted") {
        resolvePendingRequestAck(clientRequestId);
        return;
      }
      rejectPendingRequestAck(
        clientRequestId,
        new RequestAcceptanceUnknownError(
          options.unknownMessage
          ?? "等待回执超时，暂时无法确认消息是否已受理；消息已保留",
        ),
      );
    });
  }, [
    activeSessionKeyRef,
    getInputQueueItems,
    hasPendingRequestAck,
    readSessionMessages,
    rejectPendingRequestAck,
    reloadCurrentSession,
    resolvePendingRequestAck,
    wsReconnectRef,
    wsStateRef,
  ]);

  const settleRequestAckWaitFailure = useCallback((
    clientRequestId: string,
    cause: unknown,
  ): void => {
    const message = cause instanceof Error
      ? cause.message
      : "消息未送达后端，请重试";
    clearOutboundRequest(clientRequestId);
    const correlation = cause instanceof RequestAcceptanceUnknownError
        || cause instanceof RequestAcceptanceRejectedError
      ? cause.correlation
      : null;
    const sessionKey = correlation?.sessionKey ?? activeSessionKeyRef.current;
    notifyDesktopDiagnostic("request_ack.failed", {
      client_request_id: clientRequestId,
      client_message_id: correlation?.clientMessageId,
      session_key: sessionKey,
    }, cause);
    console.error("[useAgentConversation] 请求受理失败", {
      client_request_id: clientRequestId,
      client_message_id: correlation?.clientMessageId,
      session_key: sessionKey,
      error: cause,
      message,
    });
    if (sessionKey) {
      reliability.reportFailure({
        client_request_id: clientRequestId,
        code: cause instanceof RequestAcceptanceRejectedError
          ? "request_rejected"
          : "delivery_unknown",
        session_key: sessionKey,
      });
    }
  }, [activeSessionKeyRef, clearOutboundRequest, reliability]);

  const settleChatAckWaitFailure = useCallback((
    clientRequestId: string,
    clientMessageId: string,
    cause: unknown,
  ): void => {
    settleRequestAckWaitFailure(clientRequestId, cause);
    if (cause instanceof RequestAcceptanceUnknownError) {
      return;
    }
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
