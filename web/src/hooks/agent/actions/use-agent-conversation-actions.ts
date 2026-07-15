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
  type OutboundChatRequest,
} from "./conversation-chat-actions";
import {
  sendSessionPermissionResponse,
  stopSessionGeneration,
} from "./conversation-control-actions";
import {
  deleteInputQueueMessage,
  enqueueInputQueueMessage,
  guideInputQueueMessage,
  reorderInputQueueMessages,
} from "./input-queue-actions";

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
  // 对外命令保持稳定，执行时读取当前会话上下文，避免消息流更新重建整组回调。
  const actionContextRef = useRef(actionContext);
  actionContextRef.current = actionContext;

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
      () => sendSessionMessage(content, actionContextRef.current, options),
    ),
    [sendWithAck],
  );

  const rewriteLastMessage = useCallback(
    (targetRoundId: string, content: string): Promise<void> => sendWithAck(
      () => rewriteLastUserMessage(
        targetRoundId,
        content,
        actionContextRef.current,
      ),
    ),
    [sendWithAck],
  );

  const enqueueQueueMessage = useCallback(
    async (
      content: string,
      deliveryPolicy: AgentConversationDeliveryPolicy = "queue",
      attachments: AgentConversationSendOptions["attachments"] = [],
      targetAgentIDs: string[] = [],
    ): Promise<void> => {
      enqueueInputQueueMessage(
        content,
        actionContextRef.current,
        deliveryPolicy,
        attachments,
        targetAgentIDs,
      );
    },
    [],
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
