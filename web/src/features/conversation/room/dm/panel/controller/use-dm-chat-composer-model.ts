import { useCallback } from "react";

import { prepareWorkspaceAttachments } from "@/features/conversation/shared/composer/attachments/composer-attachments";
import { useConversationComposerHandlers } from "@/features/conversation/shared/composer/use-conversation-composer-handlers";
import { CONVERSATION_TOUR_ANCHORS } from "@/features/onboarding/tours/conversation-tour";
import { useDefaultChatDeliveryPolicy } from "@/hooks/settings/use-default-chat-delivery-policy";
import {
  buildComposerDraftScopeKey,
  buildComposerHistoryScopeKey,
} from "@/features/conversation/shared/composer/composer-draft-scope";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { Agent } from "@/types/agent/agent";
import type { UseAgentConversationReturn } from "@/types/agent/agent-conversation";
import type { AgentRuntimeKind } from "@/types/settings/preferences";

import type { DmChatComposerModel } from "../view/dm-chat-panel-view";

type ComposerConversation = Pick<
  UseAgentConversationReturn,
  | "delete_input_queue_message"
  | "command_catalog"
  | "context_usage"
  | "enqueue_input_queue_message"
  | "guide_input_queue_message"
  | "input_queue_items"
  | "is_loading"
  | "reorder_input_queue_messages"
  | "runtime_phase"
  | "send_message"
  | "set_goal"
  | "stop_generation"
>;

interface UseDmChatComposerModelOptions {
  agent: Agent;
  conversation: ComposerConversation;
  goalScopeLabel: string;
  initialDraft: string | null;
  onInitialDraftConsumed?: () => void;
  scrollToBottom: (behavior?: ScrollBehavior) => void;
  sessionKey: string | null;
  runtimeKind: AgentRuntimeKind;
}

export function useDmChatComposerModel({
  agent,
  conversation,
  goalScopeLabel,
  initialDraft,
  onInitialDraftConsumed,
  scrollToBottom,
  sessionKey,
  runtimeKind,
}: UseDmChatComposerModelOptions): DmChatComposerModel {
  const { t } = useI18n();
  const agentId = agent.agent_id;
  const defaultDeliveryPolicy = useDefaultChatDeliveryPolicy();
  const draftScopeKey = buildComposerDraftScopeKey({ agentId, sessionKey });
  const historyScopeKey = buildComposerHistoryScopeKey({ agentId });
  const setGoal = conversation.set_goal;
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
  const createGoal = useCallback(async (objective: string) => {
    if (!sessionKey) {
      throw new Error(t("dm.goal_session_not_ready"));
    }
    await setGoal(objective, {
      replace_existing: true,
      token_budget: null,
    });
  }, [sessionKey, setGoal, t]);

  return {
    commandCatalog: conversation.command_catalog,
    contextUsage: conversation.context_usage,
    defaultDeliveryPolicy,
    draftScopeKey,
    goalScopeLabel,
    historyScopeKey,
    inputQueueItems: conversation.input_queue_items,
    isLoading: conversation.is_loading,
    onCreateGoal: sessionKey ? createGoal : undefined,
    onDeleteQueuedMessage: conversation.delete_input_queue_message,
    onEnqueueMessage: conversation.enqueue_input_queue_message,
    onGuideQueuedMessage: conversation.guide_input_queue_message,
    onPrepareAttachments: handlers.handlePrepareAttachments,
    onReorderQueueMessages: conversation.reorder_input_queue_messages,
    onSendMessage: handlers.handleSendMessage,
    onStop: conversation.stop_generation,
    runtimePhase: conversation.runtime_phase,
    runtimeKind,
    sessionSettings: sessionKey
      ? {
          initialTargetId: agent.agent_id,
          runtimeKind,
          targets: [{
            agentId: agent.agent_id,
            avatar: agent.avatar,
            defaultModel: agent.options.model,
            defaultPermissionMode: agent.options.permission_mode,
            defaultProvider: agent.options.provider,
            name: agent.name,
            sessionKey,
          }],
        }
      : undefined,
    tourAnchor: CONVERSATION_TOUR_ANCHORS.composer,
  };
}
