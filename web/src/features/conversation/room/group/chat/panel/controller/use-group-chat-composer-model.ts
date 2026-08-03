import { useCallback } from "react";

import { prepareRoomConversationAttachments } from "@/features/conversation/shared/composer/attachments/composer-attachments";
import { useConversationComposerHandlers } from "@/features/conversation/shared/composer/use-conversation-composer-handlers";
import { ROOM_GOAL_SCOPE_LABEL } from "@/features/conversation/shared/goal/goal-continuation-hold";
import { CONVERSATION_TOUR_ANCHORS } from "@/features/onboarding/tours/conversation-tour";
import { useDefaultChatDeliveryPolicy } from "@/hooks/settings/use-default-chat-delivery-policy";
import {
  buildComposerDraftScopeKey,
  buildComposerHistoryScopeKey,
} from "@/features/conversation/shared/composer/composer-draft-scope";
import { useI18n } from "@/shared/i18n/i18n-context";
import { buildRoomAgentSessionKey } from "@/lib/conversation/session-key";
import type { Agent } from "@/types/agent/agent";
import type { UseAgentConversationReturn } from "@/types/agent/agent-conversation";
import type { AgentRuntimeKind } from "@/types/settings/preferences";

import type { GroupChatComposerModel } from "../view/group-chat-panel-view";
import { projectRoomPendingInputQueueItems } from "./group-chat-panel-projection";
import type { RoomGoalComposerModel } from "./use-room-goal-composer";

type ComposerConversation = Pick<
  UseAgentConversationReturn,
  | "delete_input_queue_message"
  | "command_catalog"
  | "context_usage"
  | "context_usage_by_agent"
  | "enqueue_input_queue_message"
  | "guide_input_queue_message"
  | "input_queue_items"
  | "is_loading"
  | "reorder_input_queue_messages"
  | "runtime_phase"
  | "send_message"
>;

interface UseGroupChatComposerModelOptions {
  agentId: string | null;
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
  agentId,
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
  const draftScopeKey = buildComposerDraftScopeKey({ roomId, sessionKey });
  const historyScopeKey = buildComposerHistoryScopeKey({ roomId });
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
    commandCatalog: conversation.command_catalog,
    contextUsage: conversation.context_usage,
    contextUsageItems: roomMembers.map((member) => ({
      agentId: member.agent_id,
      avatar: member.avatar,
      name: member.name,
      usage: conversation.context_usage_by_agent[member.agent_id] ?? null,
    })),
    defaultDeliveryPolicy,
    draftScopeKey,
    enableLoops: true,
    goalCreateDisabledReason: goal.createDisabledReason,
    goalScopeLabel: ROOM_GOAL_SCOPE_LABEL,
    historyScopeKey,
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
    queueWhenSessionBusy: true,
    roomMembers,
    runtimePhase: conversation.runtime_phase,
    runtimeKind,
    sessionSettings: conversationId && roomMembers.length > 0
      ? {
          initialTargetId: agentId ?? roomMembers[0].agent_id,
          runtimeKind,
          targets: roomMembers.map((member) => ({
            agentId: member.agent_id,
            avatar: member.avatar,
            defaultModel: member.options.model,
            defaultPermissionMode: member.options.permission_mode,
            defaultProvider: member.options.provider,
            name: member.name,
            sessionKey: buildRoomAgentSessionKey(
              conversationId,
              member.agent_id,
            ),
          })),
        }
      : undefined,
    tourAnchor: CONVERSATION_TOUR_ANCHORS.composer,
  };
}
