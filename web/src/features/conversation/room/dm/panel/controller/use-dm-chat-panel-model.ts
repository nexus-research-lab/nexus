import { useCallback } from "react";

import { useConversationPanelEnvironment } from "@/features/conversation/shared/use-conversation-panel-environment";
import { useI18n } from "@/shared/i18n/i18n-context";

import type { DmChatPanelProps } from "../dm-chat-panel-types";
import type { DmChatPanelViewModel } from "../view/dm-chat-panel-view";
import { buildDmChatPanelViewModel } from "./dm-chat-panel-projection";
import { useDmChatComposerModel } from "./use-dm-chat-composer-model";
import { useDmChatSessionController } from "./use-dm-chat-session-controller";
import { useDmGoalController } from "./use-dm-goal-controller";
import { useDmOnboardingController } from "./use-dm-onboarding-controller";

export function useDmChatPanelModel({
  currentAgentAvatar,
  currentAgentName,
  currentAgentPermissionMode,
  initialDraft,
  onboarding = false,
  layout,
  onConversationSnapshotChange,
  onInitialDraftConsumed,
  onOpenAgentContact,
  onOpenWorkspaceFile,
  onRoomEvent,
  onTodosChange,
  runtimeKind,
  sessionIdentity,
}: DmChatPanelProps): DmChatPanelViewModel {
  const { t } = useI18n();
  const environment = useConversationPanelEnvironment(layout);
  const sessionKey = sessionIdentity?.session_key ?? null;
  const goal = useDmGoalController({
    agentName: currentAgentName,
    permissionMode: currentAgentPermissionMode,
    sessionKey,
  });
  const session = useDmChatSessionController({
    identity: sessionIdentity,
    onConversationSnapshotChange,
    onGoalEvent: goal.refresh,
    onRoomEvent,
    onTodosChange,
  });
  const onboardingController = useDmOnboardingController({
    agentId: sessionIdentity?.agent_id ?? null,
    agentName: currentAgentName,
    conversationId: sessionIdentity?.conversation_id ?? null,
    enabled: onboarding,
    roomId: sessionIdentity?.room_id ?? null,
    sendMessage: session.conversation.send_message,
    sessionKey,
  });
  const goalScopeLabel = t("dm.goal_scope");
  const composer = useDmChatComposerModel({
    agentId: sessionIdentity?.agent_id ?? null,
    conversation: session.conversation,
    goalScopeLabel,
    initialDraft: initialDraft ?? null,
    canSendInitialDraft: !onboardingController.isActive,
    onCreateGoal: goal.createGoal,
    onInitialDraftConsumed,
    scrollToBottom: session.scroll.scrollToBottom,
    sendMessage: onboardingController.sendMessage,
    sessionKey,
    runtimeKind,
  });
  const rewriteLastUserMessage = session.conversation.rewrite_last_user_message;
  const handleEditLastUserMessage = useCallback(
    (messageId: string, content: string): void => {
      void rewriteLastUserMessage(messageId, content);
    },
    [rewriteLastUserMessage],
  );
  return buildDmChatPanelViewModel({
    composer,
    currentAgentAvatar,
    currentAgentName,
    environment,
    goal,
    goalScopeLabel,
    onboarding,
    onboardingAgentConfirmationCardVisible:
      onboardingController.showAgentConfirmationCard,
    onboardingAgentIdentityCardVisible:
      onboardingController.showAgentIdentityCard,
    onboardingAgentIsCreating:
      onboardingController.isCreatingAgent,
    onboardingAgentStyleCardVisible:
      onboardingController.showAgentStyleCard,
    onboardingAgentStyleChoices:
      onboardingController.agentStyleChoices,
    onboardingAgentTaskDraft:
      onboardingController.agentTaskDraft,
    onboardingDefaultModelCardVisible:
      onboardingController.showDefaultModelCard,
    onboardingDefaultModelChoices:
      onboardingController.defaultModelChoices,
    onboardingDefaultModelSelection:
      onboardingController.defaultModelSelection,
    onboardingProviderChoices: onboardingController.providerChoices,
    onboardingProviderChoicesError:
      onboardingController.providerChoicesError,
    onboardingProviderChoicesLoading:
      onboardingController.providerChoicesLoading,
    onboardingProviderConfigCardVisible:
      onboardingController.showProviderConfigCard,
    onboardingProviderSelectionCardVisible:
      onboardingController.showProviderSelectionCard,
    onboardingRoleCardVisible: onboardingController.showRoleCard,
    onboardingRoomCompletionCardVisible:
      onboardingController.showRoomCompletionCard,
    onboardingRoomIsCreating: onboardingController.isCreatingRoom,
    onboardingRoomLaunchCardVisible:
      onboardingController.showRoomLaunchCard,
    onboardingRoomPlanCardVisible: onboardingController.showRoomPlanCard,
    onboardingRoomStartCardVisible: onboardingController.showRoomStartCard,
    onboardingRoomTaskDraft: onboardingController.roomTaskDraft,
    onConfirmAgentCreation: onboardingController.confirmAgentCreation,
    onConfirmDefaultModel: onboardingController.confirmDefaultModel,
    onConfirmRoomPlan: onboardingController.confirmRoomPlan,
    onEditLastUserMessage: handleEditLastUserMessage,
    onFinishOnboarding: onboardingController.finishOnboarding,
    onLaunchRoomCollaboration:
      onboardingController.launchRoomCollaboration,
    onOpenAgentContact,
    onOpenWorkspaceFile,
    onRetryProviderChoices: onboardingController.retryProviderChoices,
    onRestartAgentDraft: onboardingController.restartAgentDraft,
    onRestartRoomIdea: onboardingController.restartRoomIdea,
    onSelectAgentStyle: onboardingController.selectAgentStyle,
    onSelectDefaultModel: onboardingController.selectDefaultModel,
    onSelectProvider: onboardingController.selectProvider,
    onSelectRole: onboardingController.selectRole,
    onStartRoomTask: onboardingController.startRoomTask,
    session,
    onboardingMessages: onboardingController.messages,
    workspaceAgentId: sessionIdentity?.agent_id ?? null,
  });
}
