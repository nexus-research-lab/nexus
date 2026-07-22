import { useCallback } from "react";

import { prepareWorkspaceAttachments } from "@/features/conversation/shared/composer/attachments/composer-attachments";
import { useConversationComposerHandlers } from "@/features/conversation/shared/composer/use-conversation-composer-handlers";
import { CONVERSATION_TOUR_ANCHORS } from "@/features/onboarding/tours/conversation-tour";
import { useDefaultChatDeliveryPolicy } from "@/hooks/settings/use-default-chat-delivery-policy";
import { useI18n } from "@/shared/i18n/i18n-context";
import type {
  AgentConversationDeliveryPolicy,
  UseAgentConversationReturn,
} from "@/types/agent/agent-conversation";
import type { MessageAttachment } from "@/types/conversation/message/attachment";
import type { AgentRuntimeKind } from "@/types/settings/preferences";

import type { DmChatComposerModel } from "../view/dm-chat-panel-view";

type ComposerConversation = Pick<
  UseAgentConversationReturn,
  | "delete_input_queue_message"
  | "enqueue_input_queue_message"
  | "guide_input_queue_message"
  | "input_queue_items"
  | "is_loading"
  | "reorder_input_queue_messages"
  | "runtime_phase"
  | "send_message"
  | "stop_generation"
>;

interface UseDmChatComposerModelOptions {
  agentId: string | null;
  conversation: ComposerConversation;
  goalScopeLabel: string;
  initialDraft: string | null;
  onCreateGoal: (objective: string) => Promise<void>;
  onInitialDraftConsumed?: () => void;
  operationStageClientId?: string | null;
  scrollToBottom: (behavior?: ScrollBehavior) => void;
  sessionKey: string | null;
  runtimeKind: AgentRuntimeKind;
}

export function useDmChatComposerModel({
  agentId,
  conversation,
  goalScopeLabel,
  initialDraft,
  onCreateGoal,
  onInitialDraftConsumed,
  operationStageClientId,
  scrollToBottom,
  sessionKey,
  runtimeKind,
}: UseDmChatComposerModelOptions): DmChatComposerModel {
  const { t } = useI18n();
  const defaultDeliveryPolicy = useDefaultChatDeliveryPolicy();
  const enqueueInputQueueMessage = conversation.enqueue_input_queue_message;
  const prepareAttachments = useCallback(
    async (files: File[]) => {
      if (!agentId) {
        throw new Error(t("dm.attachment_session_not_ready"));
      }
      return prepareWorkspaceAttachments(agentId, files);
    },
    [agentId, t],
  );
  const handlers = useConversationComposerHandlers({
    initialDraft,
    initialDraftLogLabel: "DM",
    isLoading: conversation.is_loading,
    onInitialDraftConsumed,
    operationStageClientId,
    prepareAttachments,
    scrollToBottom,
    sendMessage: conversation.send_message,
    sessionKey,
  });
  const handleEnqueueMessage = useCallback(
    async (
      content: string,
      deliveryPolicy: AgentConversationDeliveryPolicy,
      attachments: MessageAttachment[] = [],
      targetAgentIDs: string[] = [],
    ) => enqueueInputQueueMessage(
      content,
      deliveryPolicy,
      attachments,
      targetAgentIDs,
      operationStageClientId?.trim() || undefined,
    ),
    [enqueueInputQueueMessage, operationStageClientId],
  );

  return {
    defaultDeliveryPolicy,
    goalScopeLabel,
    inputQueueItems: conversation.input_queue_items,
    isLoading: conversation.is_loading,
    onCreateGoal: sessionKey ? onCreateGoal : undefined,
    onDeleteQueuedMessage: conversation.delete_input_queue_message,
    onEnqueueMessage: handleEnqueueMessage,
    onGuideQueuedMessage: conversation.guide_input_queue_message,
    onPrepareAttachments: handlers.handlePrepareAttachments,
    onReorderQueueMessages: conversation.reorder_input_queue_messages,
    onSendMessage: handlers.handleSendMessage,
    onStop: conversation.stop_generation,
    runtimePhase: conversation.runtime_phase,
    runtimeKind,
    tourAnchor: CONVERSATION_TOUR_ANCHORS.composer,
  };
}
