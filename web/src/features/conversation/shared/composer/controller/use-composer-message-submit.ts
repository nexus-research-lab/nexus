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

interface ComposerMessageSubmission {
  content: string;
  deliver: DeliverMessage;
  policy: AgentConversationDeliveryPolicy;
}

export function useComposerMessageSubmit(
  {
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
  }: UseComposerMessageSubmitOptions,
) {
  return useCallback(
    () => runComposerMessageSubmission({
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
    }),
    [
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
    ],
  );
}

async function runComposerMessageSubmission(
  options: UseComposerMessageSubmitOptions,
): Promise<void> {
  const submission = resolveMessageSubmission(options);
  if (!submission) {
    return;
  }
  const attachments = await options.prepareAttachments();
  if (!attachments) {
    return;
  }
  try {
    await submission.deliver(
      submission.content,
      submission.policy,
      attachments,
    );
    completeMessageSubmission(options, submission.content);
  } catch (error) {
    console.error("发送消息失败:", error);
  }
}

function resolveMessageSubmission(
  options: UseComposerMessageSubmitOptions,
): ComposerMessageSubmission | null {
  const content = options.input.trim();
  if (!canStartMessageSubmission(content, options)) {
    return null;
  }
  const delivery = resolveComposerDelivery(
    [options.isLoading, options.queueItemCount > 0].some(Boolean),
    options.queueWhenSessionBusy,
    options.defaultDeliveryPolicy,
  );
  const handlers = {
    enqueue: options.onEnqueueMessage,
    send: options.onSendMessage,
  };
  const deliver = handlers[delivery.handler];
  if (!deliver) {
    return null;
  }
  return { content, deliver, policy: delivery.policy };
}

function canStartMessageSubmission(
  content: string,
  options: UseComposerMessageSubmitOptions,
): boolean {
  const hasContent = [Boolean(content), options.attachmentCount > 0].some(
    Boolean,
  );
  return [hasContent, !options.isPreparingAttachments].every(Boolean);
}

function completeMessageSubmission(
  options: UseComposerMessageSubmitOptions,
  content: string,
): void {
  options.recordHistory(content);
  options.resetInput();
  options.clearAttachments();
  options.clearAttachmentError();
  options.resetTextareaHeight();
}
