import {
  useCallback,
  useRef,
  type Dispatch,
  type SetStateAction,
} from "react";

import type { RoomPendingAgentSlotState } from "@/types/agent/agent-conversation";
import type {
  AgentConversationDeliveryPolicy,
  AgentConversationSendOptions,
} from "@/types/agent/agent-conversation";
import type { PermissionDecisionPayload } from "@/types/conversation/interaction/permission";

import type { AgentConversationActionContext } from "./conversation-action-context";
import {
  rewriteLastUserMessage,
  sendSessionMessage,
} from "./conversation-chat-actions";
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
  clearOutboundRequest: (clientRequestId: string) => void;
  handleRequestAckTimeout: (
    clientRequestId: string,
    message: string,
  ) => void;
  setPendingAgentSlots: Dispatch<SetStateAction<RoomPendingAgentSlotState[]>>;
  settleChatAckWaitFailure: (
    clientRequestId: string,
    clientMessageId: string,
    error: unknown,
  ) => void;
  settleRequestAckWaitFailure: (
    clientRequestId: string,
    error: unknown,
  ) => void;
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
  clearOutboundRequest,
  handleRequestAckTimeout,
  setPendingAgentSlots,
  settleChatAckWaitFailure,
  settleRequestAckWaitFailure,
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
    ): Promise<void> => {
      const request = await sendRequest();
      if (!request) {
        return;
      }

      const { client_request_id: requestId } = request;
      trackOutboundRequest(requestId);

      try {
        await waitForRequestAck(requestId, () => {
          handleRequestAckTimeout(requestId, "消息未送达后端，请重试");
        });
      } catch (error) {
        settleFailure(request, error);
        throw error;
      }

      clearOutboundRequest(requestId);
    },
    [
      clearOutboundRequest,
      handleRequestAckTimeout,
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
      stopSessionGeneration(actionContextRef.current, agentRoundId);
      if (!agentRoundId) {
        return;
      }
      setPendingAgentSlots((currentSlots) => currentSlots.map((slot) => (
        slot.agent_round_id === agentRoundId
          ? { ...slot, status: "cancelled" }
          : slot
      )));
    },
    [setPendingAgentSlots],
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
    stopGeneration,
  };
}
