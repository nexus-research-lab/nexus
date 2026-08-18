import {
  useCallback,
  useRef,
} from "react";

import type {
  AgentConversationDeliveryPolicy,
  AgentConversationSendOptions,
} from "@/types/agent/agent-conversation";
import type { PermissionDecisionPayload } from "@/types/conversation/interaction/permission";
import { areEquivalentSessionKeys } from "@/lib/conversation/session-key";
import type { RequestTransportLeaseOptions } from "@/lib/websocket/request-transport-leases";
import {
  getGoalRequestAcceptanceTimeoutMs,
  getMessageSendAckTimeoutMs,
} from "@/config/conversation-policy";

import type { AgentConversationActionContext } from "./conversation-action-context";
import {
  rewriteLastUserMessage,
  sendSessionMessage,
} from "./conversation-chat-actions";
import { setSessionGoal } from "./conversation-goal-actions";
import {
  sendSessionPermissionResponse,
  stopSessionGeneration,
} from "./conversation-control-actions";
import {
  createInputQueueDraftFingerprint,
  deleteInputQueueMessage,
  enqueueInputQueueMessage,
  guideInputQueueMessage,
  reorderInputQueueMessages,
  resolveInputQueueClientMessageId,
} from "./input-queue-actions";
import type { OutboundRequestDescriptor } from "./outbound-request";
import { createOutboundRequestDescriptor } from "./outbound-request";
import { buildSessionBindMessage } from "./conversation-command-builders";
import {
  RequestAcceptanceRejectedError,
  RequestAcceptanceUnknownError,
  type RequestAcceptanceCorrelation,
} from "./use-pending-request-acks";
import type { RequestAckTimeoutOptions } from "./use-request-ack-failure";

interface UseAgentConversationActionsParams {
  acquireRequestTransportLease: (
    options: RequestTransportLeaseOptions,
  ) => () => void;
  actionContext: AgentConversationActionContext;
  beginAgentRoundStop: (agentRoundId: string) => boolean;
  clearOutboundRequest: (clientRequestId: string) => void;
  confirmAgentRoundStop: (agentRoundId: string) => void;
  discardPendingRequestAck: (clientRequestId: string) => void;
  handleRequestAckTimeout: (
    clientRequestId: string,
    clientMessageId: string,
    options?: RequestAckTimeoutOptions,
  ) => void;
  readStoppingAgentRoundIds: () => string[];
  rejectPendingRequestAck: (
    clientRequestId: string,
    cause: string | Error,
  ) => boolean;
  resolvePendingRequestAck: (clientRequestId?: string | null) => boolean;
  settleAgentRoundStop: (agentRoundId: string) => void;
  settleChatAckWaitFailure: (
    clientRequestId: string,
    clientMessageId: string,
    error: unknown,
  ) => void;
  settleRequestAckWaitFailure: (
    clientRequestId: string,
    error: unknown,
  ) => void;
  trackPendingRequestAck: (
    clientRequestId: string,
    preserveAcrossSessionTransition?: boolean,
  ) => boolean;
  trackOutboundRequest: (clientRequestId: string) => void;
  waitForRequestAck: (
    clientRequestId: string,
    onTimeout: () => void,
    timeoutMs?: number,
  ) => Promise<void>;
}

type SendOutboundRequest = () =>
  Promise<OutboundRequestDescriptor | null> | OutboundRequestDescriptor | null;
type SendPreparedOutboundRequest = (
  request: OutboundRequestDescriptor,
) => ReturnType<SendOutboundRequest>;
type SettleOutboundRequestFailure = (
  request: OutboundRequestDescriptor,
  error: unknown,
) => void;

interface SendWithAckOptions {
  ackTimeoutMs?: number;
  preparedRequest?: OutboundRequestDescriptor;
  retainSessionUntilSettled?: boolean;
  timeoutMessage?: string;
}

type DurableSubmissionOptions = Pick<
  SendWithAckOptions,
  "ackTimeoutMs" | "timeoutMessage"
>;

const REQUEST_TRANSPORT_TIMEOUT_MARGIN_MS = 5_000;

/**
 * 装配用户命令与 ACK 生命周期。
 * 低层动作只负责协议发送，这里统一保证发送、超时、失败和运行态收口顺序一致。
 */
export function useAgentConversationActions({
  acquireRequestTransportLease,
  actionContext,
  beginAgentRoundStop,
  clearOutboundRequest,
  confirmAgentRoundStop,
  discardPendingRequestAck,
  handleRequestAckTimeout,
  readStoppingAgentRoundIds,
  rejectPendingRequestAck,
  resolvePendingRequestAck,
  settleAgentRoundStop,
  settleChatAckWaitFailure,
  settleRequestAckWaitFailure,
  trackPendingRequestAck,
  trackOutboundRequest,
  waitForRequestAck,
}: UseAgentConversationActionsParams) {
  // 对外命令保持稳定，执行时读取当前会话上下文，避免消息流更新重建整组回调。
  const actionContextRef = useRef(actionContext);
  actionContextRef.current = actionContext;
  const inputQueueClientMessageIDsRef = useRef<Map<string, string>>(new Map());
  const inputQueueScopeRef = useRef<string | null>(null);
  const inputQueueScope = actionContext.sessionKey
    ?? actionContext.activeSessionKeyRef.current;
  if (inputQueueScopeRef.current !== inputQueueScope) {
    inputQueueScopeRef.current = inputQueueScope;
    inputQueueClientMessageIDsRef.current.clear();
  }

  const sendWithAck = useCallback(
    async (
      sendRequest: SendOutboundRequest,
      settleFailure: SettleOutboundRequestFailure,
      options: SendWithAckOptions = {},
    ): Promise<void> => {
      // activeSessionKeyRef 会在视图切换时先于 React state 更新；优先读取它，
      // 避免旧会话的 ACK 在切换窗口内被误判为当前会话结果。
      const requestSessionKey = actionContextRef.current.activeSessionKeyRef.current
        ?? actionContextRef.current.sessionKey;
      const requestIdentity = actionContextRef.current.identity;
      const preparedRequest = options.preparedRequest;
      const ackTimeoutMs = options.ackTimeoutMs ?? getMessageSendAckTimeoutMs();
      const unknownMessage = options.timeoutMessage
        ?? "暂时无法确认消息是否已受理；正在核对会话状态";
      if (options.retainSessionUntilSettled && !preparedRequest) {
        throw new Error("durable submission requires a prepared request identity");
      }
      const acceptanceCorrelation = preparedRequest && requestSessionKey
        ? {
            clientMessageId: preparedRequest.client_message_id,
            clientRequestId: preparedRequest.client_request_id,
            sessionKey: requestSessionKey,
          } satisfies RequestAcceptanceCorrelation
        : null;
      // 用户提交的 raw ACK owner 必须先于命令发送注册；否则极快 ACK 可能在
      // sendRequest 的 Promise continuation 之前到达并被当成 foreign ACK。
      const releasePreparedTransport = options.retainSessionUntilSettled
        && preparedRequest
        && requestSessionKey
        ? acquireRequestTransportLease({
            clientRequestId: preparedRequest.client_request_id,
            onAccepted: () => {
              trackPendingRequestAck(preparedRequest.client_request_id, true);
              resolvePendingRequestAck(preparedRequest.client_request_id);
            },
            onRejected: (reason) => {
              trackPendingRequestAck(preparedRequest.client_request_id, true);
              rejectPendingRequestAck(
                preparedRequest.client_request_id,
                new RequestAcceptanceRejectedError(
                  reason,
                  acceptanceCorrelation,
                ),
              );
            },
            onTimeout: () => {
              trackPendingRequestAck(preparedRequest.client_request_id, true);
              rejectPendingRequestAck(
                preparedRequest.client_request_id,
                new RequestAcceptanceUnknownError(
                  unknownMessage,
                  acceptanceCorrelation,
                ),
              );
            },
            sessionBinding: buildSessionBindMessage({
              session_key: requestSessionKey,
              agent_id: requestIdentity?.agent_id,
              room_id: requestIdentity?.room_id,
              conversation_id: requestIdentity?.conversation_id,
            }),
            timeoutMs: ackTimeoutMs + REQUEST_TRANSPORT_TIMEOUT_MARGIN_MS,
          })
        : () => {};
      let request: OutboundRequestDescriptor | null;
      try {
        request = await sendRequest();
      } catch (error) {
        releasePreparedTransport();
        if (preparedRequest) {
          discardPendingRequestAck(preparedRequest.client_request_id);
        }
        throw error;
      }
      if (!request) {
        releasePreparedTransport();
        if (preparedRequest) {
          discardPendingRequestAck(preparedRequest.client_request_id);
        }
        return;
      }
      if (
        preparedRequest
        && (
          request.client_request_id !== preparedRequest.client_request_id
          || request.client_message_id !== preparedRequest.client_message_id
        )
      ) {
        releasePreparedTransport();
        discardPendingRequestAck(preparedRequest.client_request_id);
        throw new Error("请求传输身份与已发送命令不一致");
      }

      const {
        client_message_id: clientMessageId,
        client_request_id: requestId,
      } = request;
      const requestAcceptanceCorrelation = requestSessionKey
        ? {
            clientMessageId,
            clientRequestId: requestId,
            sessionKey: requestSessionKey,
          } satisfies RequestAcceptanceCorrelation
        : null;
      trackPendingRequestAck(
        requestId,
        options.retainSessionUntilSettled,
      );
      trackOutboundRequest(requestId);

      try {
        await waitForRequestAck(
          requestId,
          () => {
            handleRequestAckTimeout(requestId, clientMessageId, {
              ...(requestSessionKey && options.retainSessionUntilSettled
                ? {
                    recoveryScope: {
                      identity: requestIdentity,
                      sessionKey: requestSessionKey,
                    },
                  }
                : {}),
              ...(options.timeoutMessage
                ? { unknownMessage: options.timeoutMessage }
                : {}),
            });
          },
          ackTimeoutMs,
        );
      } catch (error) {
        const currentSessionKey = actionContextRef.current.activeSessionKeyRef.current
          ?? actionContextRef.current.sessionKey;
        const ownerError = error instanceof RequestAcceptanceUnknownError
          && requestAcceptanceCorrelation
          ? new RequestAcceptanceUnknownError(
              error.message,
              requestAcceptanceCorrelation,
            )
          : error;
        if (
          requestSessionKey
          && currentSessionKey
          && areEquivalentSessionKeys(requestSessionKey, currentSessionKey)
        ) {
          settleFailure(request, ownerError);
        } else {
          // 旧会话请求仍按 request ID 收口，但不得把错误或乐观消息
          // 投影进用户已经切换到的新会话。
          clearOutboundRequest(requestId);
        }
        throw ownerError;
      } finally {
        // ACK、明确拒绝、状态未知超时和显式 Session reset 都会结束
        // 原请求 Promise；请求级 socket/binding 租约必须在同一边界释放。
        releasePreparedTransport();
      }

      clearOutboundRequest(requestId);
    },
    [
      clearOutboundRequest,
      discardPendingRequestAck,
      handleRequestAckTimeout,
      acquireRequestTransportLease,
      rejectPendingRequestAck,
      resolvePendingRequestAck,
      trackPendingRequestAck,
      trackOutboundRequest,
      waitForRequestAck,
    ],
  );

  const sendDurableSubmission = useCallback(
    (
      sendRequest: SendPreparedOutboundRequest,
      settleFailure: SettleOutboundRequestFailure,
      options: DurableSubmissionOptions = {},
      clientMessageId?: string,
    ): Promise<void> => {
      const preparedRequest = createOutboundRequestDescriptor(clientMessageId);
      return sendWithAck(
        () => sendRequest(preparedRequest),
        settleFailure,
        {
          ...options,
          preparedRequest,
          retainSessionUntilSettled: true,
        },
      );
    },
    [sendWithAck],
  );

  const sendMessage = useCallback(
    (
      content: string,
      options: AgentConversationSendOptions = {},
    ): Promise<void> => sendDurableSubmission(
      (request) => sendSessionMessage(
        content,
        actionContextRef.current,
        options,
        request,
      ),
      (request, error) => settleChatAckWaitFailure(
        request.client_request_id,
        request.client_message_id,
        error,
      ),
    ),
    [sendDurableSubmission, settleChatAckWaitFailure],
  );

  const setGoal = useCallback(
    (
      objective: string,
      options: Parameters<typeof setSessionGoal>[2] = {},
    ): Promise<void> => sendDurableSubmission(
      (request) => setSessionGoal(
        objective,
        actionContextRef.current,
        options,
        request,
      ),
      (request, error) => settleChatAckWaitFailure(
        request.client_request_id,
        request.client_message_id,
        error,
      ),
      {
        ackTimeoutMs: getGoalRequestAcceptanceTimeoutMs(),
        timeoutMessage: "暂时无法确认 Goal 是否已受理；正在核对 Goal 状态",
      },
    ),
    [sendDurableSubmission, settleChatAckWaitFailure],
  );

  const rewriteLastMessage = useCallback(
    (targetRoundId: string, content: string): Promise<void> => sendDurableSubmission(
      (request) => rewriteLastUserMessage(
        targetRoundId,
        content,
        actionContextRef.current,
        request,
      ),
      (request, error) => settleChatAckWaitFailure(
        request.client_request_id,
        request.client_message_id,
        error,
      ),
    ),
    [sendDurableSubmission, settleChatAckWaitFailure],
  );

  const enqueueQueueMessage = useCallback(
    async (
      content: string,
      deliveryPolicy: AgentConversationDeliveryPolicy = "queue",
      attachments: AgentConversationSendOptions["attachments"] = [],
      targetAgentIDs: string[] = [],
    ): Promise<void> => {
      const fingerprint = createInputQueueDraftFingerprint(
        content,
        deliveryPolicy,
        attachments,
        targetAgentIDs,
      );
      const scopedFingerprint = [
        actionContextRef.current.sessionKey
          ?? actionContextRef.current.activeSessionKeyRef.current
          ?? "",
        fingerprint,
      ].join("\n");
      const clientMessageId = resolveInputQueueClientMessageId(
        inputQueueClientMessageIDsRef.current,
        scopedFingerprint,
      );
      await sendDurableSubmission(
        (request) => enqueueInputQueueMessage(
          content,
          actionContextRef.current,
          deliveryPolicy,
          attachments,
          targetAgentIDs,
          request,
        ),
        (request, error) => settleRequestAckWaitFailure(
          request.client_request_id,
          error,
        ),
        {},
        clientMessageId,
      );
      inputQueueClientMessageIDsRef.current.delete(scopedFingerprint);
    },
    [sendDurableSubmission, settleRequestAckWaitFailure],
  );

  const deleteQueueMessage = useCallback(
    async (itemId: string): Promise<void> => {
      deleteInputQueueMessage(itemId, actionContextRef.current);
    },
    [],
  );

  const guideQueueMessage = useCallback(
    async (itemId: string): Promise<void> => {
      guideInputQueueMessage(itemId, actionContextRef.current);
    },
    [],
  );

  const reorderQueueMessages = useCallback(
    async (orderedIds: string[]): Promise<void> => {
      reorderInputQueueMessages(orderedIds, actionContextRef.current);
    },
    [],
  );

  const stopGeneration = useCallback(
    (agentRoundId?: string): void => {
      const normalizedAgentRoundId = agentRoundId?.trim() ?? "";
      if (
        normalizedAgentRoundId
        && readStoppingAgentRoundIds().includes(normalizedAgentRoundId)
      ) {
        return;
      }
      const request = stopSessionGeneration(
        actionContextRef.current,
        normalizedAgentRoundId || undefined,
      );
      if (!request) {
        return;
      }
      if (normalizedAgentRoundId) {
        beginAgentRoundStop(normalizedAgentRoundId);
      }
      void sendWithAck(
        () => request,
        (failedRequest, error) => {
          if (normalizedAgentRoundId) {
            settleAgentRoundStop(normalizedAgentRoundId);
          }
          settleRequestAckWaitFailure(failedRequest.client_request_id, error);
        },
        { timeoutMessage: "停止请求未被后端确认，请重试" },
      ).then(() => {
        if (normalizedAgentRoundId) {
          confirmAgentRoundStop(normalizedAgentRoundId);
        }
      }).catch(() => undefined);
    },
    [
      beginAgentRoundStop,
      confirmAgentRoundStop,
      readStoppingAgentRoundIds,
      sendWithAck,
      settleAgentRoundStop,
      settleRequestAckWaitFailure,
    ],
  );

  const sendPermissionResponse = useCallback(
    (payload: PermissionDecisionPayload): boolean => (
      sendSessionPermissionResponse(payload, actionContextRef.current)
    ),
    [],
  );

  return {
    deleteQueueMessage,
    enqueueQueueMessage,
    guideQueueMessage,
    reorderQueueMessages,
    rewriteLastMessage,
    sendMessage,
    sendPermissionResponse,
    setGoal,
    stopGeneration,
  };
}
