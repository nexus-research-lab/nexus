import { useCallback } from "react";

import { prepareWorkspaceAttachments } from "@/features/conversation/shared/composer/attachments/composer-attachments";
import { useConversationComposerHandlers } from "@/features/conversation/shared/composer/use-conversation-composer-handlers";
import { CONVERSATION_TOUR_ANCHORS } from "@/features/onboarding/tours/conversation-tour";
import { useDefaultChatDeliveryPolicy } from "@/hooks/settings/use-default-chat-delivery-policy";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { UseAgentConversationReturn } from "@/types/agent/agent-conversation";

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
  scrollToBottom: (behavior?: ScrollBehavior) => void;
  sessionKey: string | null;
}

export function useDmChatComposerModel({
  agentId,
  conversation,
  goalScopeLabel,
  initialDraft,
  onCreateGoal,
  onInitialDraftConsumed,
  scrollToBottom,
  sessionKey,
}: UseDmChatComposerModelOptions): DmChatComposerModel {
  const { t } = useI18n();
  const defaultDeliveryPolicy = useDefaultChatDeliveryPolicy();
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
    prepareAttachments,
    scrollToBottom,
    sendMessage: conversation.send_message,
    sessionKey,
  });

  return {
    defaultDeliveryPolicy,
    goalScopeLabel,
    inputQueueItems: conversation.input_queue_items,
    isLoading: conversation.is_loading,
    onCreateGoal: sessionKey ? onCreateGoal : undefined,
    onDeleteQueuedMessage: conversation.delete_input_queue_message,
    onEnqueueMessage: conversation.enqueue_input_queue_message,
    onGuideQueuedMessage: conversation.guide_input_queue_message,
    onPrepareAttachments: handlers.handlePrepareAttachments,
    onReorderQueueMessages: conversation.reorder_input_queue_messages,
    onSendMessage: handlers.handleSendMessage,
    onStop: conversation.stop_generation,
    runtimePhase: conversation.runtime_phase,
    tourAnchor: CONVERSATION_TOUR_ANCHORS.composer,
  };
}
