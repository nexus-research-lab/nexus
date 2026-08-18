import { useCallback } from "react";

import type {
  AgentConversationDefaultDeliveryPolicy,
  AgentConversationDeliveryPolicy,
  AgentConversationRuntimePhase,
} from "@/types/agent/agent-conversation";
import type { MessageAttachment } from "@/types/conversation/message/attachment";

import type { ComposerDraftSnapshot } from "../composer-draft-store";
import { resolveComposerDelivery } from "../composer-model";

type DeliverMessage = (
  content: string,
  deliveryPolicy: AgentConversationDeliveryPolicy,
  attachments?: MessageAttachment[],
  targetAgentIDs?: string[],
) => void | Promise<void>;

interface UseComposerMessageSubmitOptions {
  attachmentCount: number;
  claimDraftSubmission: () => ComposerDraftSnapshot | null;
  clearAttachmentError: () => void;
  defaultDeliveryPolicy: AgentConversationDefaultDeliveryPolicy;
  input: string;
  isLoading: boolean;
  isPreparingAttachments: boolean;
  onEnqueueMessage: DeliverMessage;
  onSendMessage: DeliverMessage;
  prepareAttachments: () => Promise<MessageAttachment[] | null>;
  queueItemCount: number;
  queueWhenSessionBusy: boolean;
  recordHistory: (value: string) => void;
  resetTextareaHeight: () => void;
  restoreFailedDraftSubmission: (
    submittedDraft: ComposerDraftSnapshot,
  ) => boolean;
  runtimePhase: AgentConversationRuntimePhase | null;
  targetAgentIDs: string[];
}

interface ComposerMessageSubmission {
  content: string;
  deliver: DeliverMessage;
  policy: AgentConversationDeliveryPolicy;
}

/** 受理状态未知时禁止回填草稿，避免把可能已落盘的消息再次发送。 */
export function shouldRestoreMessageDraftAfterFailure(error: unknown): boolean {
  return !(error instanceof Error
    && error.name === "RequestAcceptanceUnknownError");
}

export function useComposerMessageSubmit(
  {
    attachmentCount,
    claimDraftSubmission,
    clearAttachmentError,
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
    resetTextareaHeight,
    restoreFailedDraftSubmission,
    runtimePhase,
    targetAgentIDs,
  }: UseComposerMessageSubmitOptions,
) {
  return useCallback(
    () => runComposerMessageSubmission({
      attachmentCount,
      claimDraftSubmission,
      clearAttachmentError,
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
      resetTextareaHeight,
      restoreFailedDraftSubmission,
      runtimePhase,
      targetAgentIDs,
    }),
    [
      attachmentCount,
      claimDraftSubmission,
      clearAttachmentError,
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
      resetTextareaHeight,
      restoreFailedDraftSubmission,
      runtimePhase,
      targetAgentIDs,
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
  let submittedDraft: ComposerDraftSnapshot | null = null;
  try {
    const delivery = submission.deliver(
      submission.content,
      submission.policy,
      attachments,
      options.targetAgentIDs,
    );
    submittedDraft = options.claimDraftSubmission();
    if (submittedDraft) {
      options.clearAttachmentError();
      options.resetTextareaHeight();
    }
    await delivery;
    options.recordHistory(submission.content);
  } catch (error) {
    if (submittedDraft && shouldRestoreMessageDraftAfterFailure(error)) {
      options.restoreFailedDraftSubmission(submittedDraft);
    }
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
  return { content, deliver, policy: delivery.policy };
}

function canStartMessageSubmission(
  content: string,
  options: UseComposerMessageSubmitOptions,
): boolean {
  const hasContent = [Boolean(content), options.attachmentCount > 0].some(
    Boolean,
  );
  return [
    hasContent,
    !options.isPreparingAttachments,
    options.runtimePhase !== "awaiting_permission",
  ].every(Boolean);
}
