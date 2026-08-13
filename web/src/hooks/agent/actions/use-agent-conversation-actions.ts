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

interface UseAgentConversationActionsParams {
  actionContext: AgentConversationActionContext;
  beginAgentRoundStop: (agentRoundId: string) => boolean;
  clearOutboundRequest: (clientRequestId: string) => void;
  confirmAgentRoundStop: (agentRoundId: string) => void;
  handleRequestAckTimeout: (
    clientRequestId: string,
    clientMessageId: string,
    unknownMessage?: string,
  ) => void;
  readStoppingAgentRoundIds: () => string[];
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
  trackPendingRequestAck: (clientRequestId: string) => boolean;
  trackOutboundRequest: (clientRequestId: string) => void;
  waitForRequestAck: (
    clientRequestId: string,
    onTimeout: () => void,
  ) => Promise<void>;
}

type SendOutboundRequest = () =>
  Promise<OutboundRequestDescriptor | null> | OutboundRequestDescriptor | null;
type SettleOutboundRequestFailure = (
  request: OutboundRequestDescriptor,
  error: unknown,
) => void;

/**
 * 装配用户命令与 ACK 生命周期。
 * 低层动作只负责协议发送，这里统一保证发送、超时、失败和运行态收口顺序一致。
 */
export function useAgentConversationActions({
  actionContext,
  beginAgentRoundStop,
  clearOutboundRequest,
  confirmAgentRoundStop,
  handleRequestAckTimeout,
  readStoppingAgentRoundIds,
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
      timeoutMessage?: string,
    ): Promise<void> => {
      // activeSessionKeyRef 会在视图切换时先于 React state 更新；优先读取它，
      // 避免旧会话的 ACK 在切换窗口内被误判为当前会话结果。
      const requestSessionKey = actionContextRef.current.activeSessionKeyRef.current
        ?? actionContextRef.current.sessionKey;
      const request = await sendRequest();
      if (!request) {
        return;
      }

      const {
        client_message_id: clientMessageId,
        client_request_id: requestId,
      } = request;
      trackPendingRequestAck(requestId);
      trackOutboundRequest(requestId);

      try {
        await waitForRequestAck(requestId, () => {
          handleRequestAckTimeout(
            requestId,
            clientMessageId,
            timeoutMessage,
          );
        });
      } catch (error) {
        const currentSessionKey = actionContextRef.current.activeSessionKeyRef.current
          ?? actionContextRef.current.sessionKey;
        if (
          !requestSessionKey
          || !currentSessionKey
          || areEquivalentSessionKeys(requestSessionKey, currentSessionKey)
        ) {
          settleFailure(request, error);
        } else {
          // 旧会话请求仍按 request ID 收口，但不得把错误或乐观消息
          // 投影进用户已经切换到的新会话。
          clearOutboundRequest(requestId);
        }
        throw error;
      }

      clearOutboundRequest(requestId);
    },
    [
      clearOutboundRequest,
      handleRequestAckTimeout,
      trackPendingRequestAck,
      trackOutboundRequest,
      waitForRequestAck,
    ],
  );

  const sendMessage = useCallback(
    (
      content: string,
      options: AgentConversationSendOptions = {},
    ): Promise<void> => sendWithAck(
      () => sendSessionMessage(content, actionContextRef.current, options),
      (request, error) => settleChatAckWaitFailure(
        request.client_request_id,
        request.client_message_id,
        error,
      ),
    ),
    [sendWithAck, settleChatAckWaitFailure],
  );

  const setGoal = useCallback(
    (
      objective: string,
      options: Parameters<typeof setSessionGoal>[2] = {},
    ): Promise<void> => sendWithAck(
      () => setSessionGoal(objective, actionContextRef.current, options),
      (request, error) => settleChatAckWaitFailure(
        request.client_request_id,
        request.client_message_id,
        error,
      ),
    ),
    [sendWithAck, settleChatAckWaitFailure],
  );

  const rewriteLastMessage = useCallback(
    (targetRoundId: string, content: string): Promise<void> => sendWithAck(
      () => rewriteLastUserMessage(
        targetRoundId,
        content,
        actionContextRef.current,
      ),
      (request, error) => settleChatAckWaitFailure(
        request.client_request_id,
        request.client_message_id,
        error,
      ),
    ),
    [sendWithAck, settleChatAckWaitFailure],
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
      await sendWithAck(
        () => enqueueInputQueueMessage(
          content,
          actionContextRef.current,
          deliveryPolicy,
          attachments,
          targetAgentIDs,
          clientMessageId,
        ),
        (request, error) => settleRequestAckWaitFailure(
          request.client_request_id,
          error,
        ),
      );
      inputQueueClientMessageIDsRef.current.delete(scopedFingerprint);
    },
    [sendWithAck, settleRequestAckWaitFailure],
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
        "停止请求未被后端确认，请重试",
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
