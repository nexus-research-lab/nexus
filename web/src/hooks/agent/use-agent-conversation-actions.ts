import {
  useCallback,
  type Dispatch,
  type SetStateAction,
} from "react";

import type { RoomPendingAgentSlotState } from "@/types";
import type {
  AgentConversationDeliveryPolicy,
  AgentConversationSendOptions,
} from "@/types/agent/agent-conversation";
import type { PermissionDecisionPayload } from "@/types/conversation/permission";

import {
  deleteInputQueueMessage,
  enqueueInputQueueMessage,
  guideInputQueueMessage,
  reorderInputQueueMessages,
  rewriteLastUserMessage,
  sendSessionMessage,
  sendSessionPermissionResponse,
  stopSessionGeneration,
  type AgentConversationActionContext,
  type OutboundChatRequest,
} from "./conversation-actions";

interface UseAgentConversationActionsParams {
  actionContext: AgentConversationActionContext;
  clearOutboundRequest: (clientRequestId: string) => void;
  handleChatAckTimeout: (
    clientRequestId: string,
    message: string,
  ) => void;
  setPendingAgentSlots: Dispatch<SetStateAction<RoomPendingAgentSlotState[]>>;
  settleChatAckWaitFailure: (
    clientRequestId: string,
    clientMessageId: string,
    error: unknown,
  ) => void;
  trackOutboundRequest: (clientRequestId: string) => void;
  waitForChatAck: (
    clientRequestId: string,
    onTimeout: () => void,
  ) => Promise<void>;
}

type SendOutboundRequest = () => Promise<OutboundChatRequest | null>;

/**
 * 装配用户命令与 ACK 生命周期。
 * 低层动作只负责协议发送，这里统一保证发送、超时、失败和运行态收口顺序一致。
 */
export function useAgentConversationActions({
  actionContext,
  clearOutboundRequest,
  handleChatAckTimeout,
  setPendingAgentSlots,
  settleChatAckWaitFailure,
  trackOutboundRequest,
  waitForChatAck,
}: UseAgentConversationActionsParams) {
  const sendWithAck = useCallback(
    async (sendRequest: SendOutboundRequest): Promise<void> => {
      const request = await sendRequest();
      if (!request) {
        return;
      }

      const { client_message_id: messageId, client_request_id: requestId } = request;
      trackOutboundRequest(requestId);

      try {
        await waitForChatAck(requestId, () => {
          handleChatAckTimeout(requestId, "消息未送达后端，请重试");
        });
      } catch (error) {
        settleChatAckWaitFailure(requestId, messageId, error);
        return;
      }

      clearOutboundRequest(requestId);
    },
    [
      clearOutboundRequest,
      handleChatAckTimeout,
      settleChatAckWaitFailure,
      trackOutboundRequest,
      waitForChatAck,
    ],
  );

  const sendMessage = useCallback(
    (
      content: string,
      options: AgentConversationSendOptions = {},
    ): Promise<void> => sendWithAck(
      () => sendSessionMessage(content, actionContext, options),
    ),
    [actionContext, sendWithAck],
  );

  const rewriteLastMessage = useCallback(
    (targetRoundId: string, content: string): Promise<void> => sendWithAck(
      () => rewriteLastUserMessage(targetRoundId, content, actionContext),
    ),
    [actionContext, sendWithAck],
  );

  const enqueueQueueMessage = useCallback(
    async (
      content: string,
      deliveryPolicy: AgentConversationDeliveryPolicy = "queue",
      attachments: AgentConversationSendOptions["attachments"] = [],
    ): Promise<void> => {
      enqueueInputQueueMessage(
        content,
        actionContext,
        deliveryPolicy,
        attachments,
      );
    },
    [actionContext],
  );

  const deleteQueueMessage = useCallback(
    async (itemId: string): Promise<void> => {
      deleteInputQueueMessage(itemId, actionContext);
    },
    [actionContext],
  );

  const guideQueueMessage = useCallback(
    async (itemId: string): Promise<void> => {
      guideInputQueueMessage(itemId, actionContext);
    },
    [actionContext],
  );

  const reorderQueueMessages = useCallback(
    async (orderedIds: string[]): Promise<void> => {
      reorderInputQueueMessages(orderedIds, actionContext);
    },
    [actionContext],
  );

  const stopGeneration = useCallback(
    (agentRoundId?: string): void => {
      stopSessionGeneration(actionContext, agentRoundId);
      if (!agentRoundId) {
        return;
      }
      setPendingAgentSlots((currentSlots) => currentSlots.map((slot) => (
        slot.agent_round_id === agentRoundId
          ? { ...slot, status: "cancelled" }
          : slot
      )));
    },
    [actionContext, setPendingAgentSlots],
  );

  const sendPermissionResponse = useCallback(
    (payload: PermissionDecisionPayload): boolean => (
      sendSessionPermissionResponse(payload, actionContext)
    ),
    [actionContext],
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
