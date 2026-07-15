import { useCallback } from "react";

import { prepareRoomConversationAttachments } from "@/features/conversation/shared/composer/attachments/composer-attachments";
import { useConversationComposerHandlers } from "@/features/conversation/shared/composer/use-conversation-composer-handlers";
import { ROOM_GOAL_SCOPE_LABEL } from "@/features/conversation/shared/goal/goal-continuation-hold";
import { CONVERSATION_TOUR_ANCHORS } from "@/features/onboarding/tours/conversation-tour";
import { useDefaultChatDeliveryPolicy } from "@/hooks/settings/use-default-chat-delivery-policy";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { Agent } from "@/types/agent/agent";
import type { UseAgentConversationReturn } from "@/types/agent/agent-conversation";
import type { AgentRuntimeKind } from "@/types/settings/preferences";

import type { GroupChatComposerModel } from "../view/group-chat-panel-view";
import { projectRoomPendingInputQueueItems } from "./group-chat-panel-projection";
import type { RoomGoalComposerModel } from "./use-room-goal-composer";

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

interface UseGroupChatComposerModelOptions {
  conversation: ComposerConversation;
  conversationId: string | null;
  goal: RoomGoalComposerModel;
  initialDraft: string | null;
  onInitialDraftConsumed?: () => void;
  roomId: string | null;
  roomMembers: Agent[];
  scrollToBottom: (behavior?: ScrollBehavior) => void;
  sessionKey: string | null;
  runtimeKind: AgentRuntimeKind;
}

export function useGroupChatComposerModel({
  conversation,
  conversationId,
  goal,
  initialDraft,
  onInitialDraftConsumed,
  roomId,
  roomMembers,
  scrollToBottom,
  sessionKey,
  runtimeKind,
}: UseGroupChatComposerModelOptions): GroupChatComposerModel {
  const { t } = useI18n();
  const defaultDeliveryPolicy = useDefaultChatDeliveryPolicy();
  const prepareAttachments = useCallback(
    async (files: File[]) => {
      if (!roomId || !conversationId) {
        throw new Error(t("room.attachment_session_not_ready"));
      }
      return prepareRoomConversationAttachments(roomId, conversationId, files);
    },
    [conversationId, roomId, t],
  );
  const handlers = useConversationComposerHandlers({
    canSendInitialDraft: true,
    initialDraft,
    initialDraftLogLabel: "room",
    isLoading: conversation.is_loading,
    onInitialDraftConsumed,
    prepareAttachments,
    scrollToBottom,
    sendMessage: conversation.send_message,
    sessionKey,
  });

  return {
    defaultDeliveryPolicy,
    enableLoops: true,
    goalCreateDisabledReason: goal.createDisabledReason,
    goalScopeLabel: ROOM_GOAL_SCOPE_LABEL,
    inputQueueItems: projectRoomPendingInputQueueItems(
      conversation.input_queue_items,
    ),
    isLoading: conversation.is_loading,
    onCreateGoal: sessionKey ? goal.onCreateGoal : undefined,
    onCreateLoopGoal: sessionKey ? goal.onCreateLoopGoal : undefined,
    onDeleteQueuedMessage: conversation.delete_input_queue_message,
    onEnqueueMessage: conversation.enqueue_input_queue_message,
    onGuideQueuedMessage: conversation.guide_input_queue_message,
    onPrepareAttachments: handlers.handlePrepareAttachments,
    onReorderQueueMessages: conversation.reorder_input_queue_messages,
    onSendMessage: handlers.handleSendMessage,
    onStop: conversation.stop_generation,
    queueWhenSessionBusy: true,
    roomMembers,
    runtimePhase: conversation.runtime_phase,
    runtimeKind,
    tourAnchor: CONVERSATION_TOUR_ANCHORS.composer,
  };
}
