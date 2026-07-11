import { useCallback } from "react";

import type {
  AgentConversationDefaultDeliveryPolicy,
  AgentConversationDeliveryPolicy,
} from "@/types/agent/agent-conversation";
import type { MessageAttachment } from "@/types/conversation/message/attachment";

import { resolveComposerDelivery } from "../composer-model";

type DeliverMessage = (
  content: string,
  deliveryPolicy: AgentConversationDeliveryPolicy,
  attachments?: MessageAttachment[],
) => void | Promise<void>;

interface UseComposerMessageSubmitOptions {
  attachmentCount: number;
  clearAttachmentError: () => void;
  clearAttachments: () => void;
  defaultDeliveryPolicy: AgentConversationDefaultDeliveryPolicy;
  input: string;
  isLoading: boolean;
  isPreparingAttachments: boolean;
  onEnqueueMessage?: DeliverMessage;
  onSendMessage: DeliverMessage;
  prepareAttachments: () => Promise<MessageAttachment[] | null>;
  queueItemCount: number;
  queueWhenSessionBusy: boolean;
  recordHistory: (value: string) => void;
  resetInput: () => void;
  resetTextareaHeight: () => void;
}

export function useComposerMessageSubmit({
  attachmentCount,
  clearAttachmentError,
  clearAttachments,
  defaultDeliveryPolicy,
  input,
  isLoading,
  isPreparingAttachments,
  onEnqueueMessage,
  onSendMessage,
  prepareAttachments,
  queueItemCount,
  queueWhenSessionBusy,
  recordHistory,
  resetInput,
  resetTextareaHeight,
}: UseComposerMessageSubmitOptions) {
  return useCallback(async () => {
    const content = input.trim();
    if ((!content && attachmentCount === 0) || isPreparingAttachments) {
      return;
    }

    const delivery = resolveComposerDelivery(
      isLoading || queueItemCount > 0,
      queueWhenSessionBusy,
      defaultDeliveryPolicy,
    );
    const deliver = delivery.handler === "enqueue"
      ? onEnqueueMessage
      : onSendMessage;
    if (!deliver) {
      return;
    }

    const preparedAttachments = await prepareAttachments();
    if (!preparedAttachments) {
      return;
    }

    try {
      await deliver(content, delivery.policy, preparedAttachments);
      recordHistory(content);
      resetInput();
      clearAttachments();
      clearAttachmentError();
      resetTextareaHeight();
    } catch (error) {
      console.error("发送消息失败:", error);
    }
  }, [
    attachmentCount,
    clearAttachmentError,
    clearAttachments,
    defaultDeliveryPolicy,
    input,
    isLoading,
    isPreparingAttachments,
    onEnqueueMessage,
    onSendMessage,
    prepareAttachments,
    queueItemCount,
    queueWhenSessionBusy,
    recordHistory,
    resetInput,
    resetTextareaHeight,
  ]);
}
