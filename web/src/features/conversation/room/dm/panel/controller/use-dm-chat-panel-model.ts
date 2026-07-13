import { useCallback } from "react";

import { useConversationPanelEnvironment } from "@/features/conversation/shared/use-conversation-panel-environment";
import { useI18n } from "@/shared/i18n/i18n-context";

import type { DmChatPanelProps } from "../dm-chat-panel-types";
import type { DmChatPanelViewModel } from "../view/dm-chat-panel-view";
import { buildDmChatPanelViewModel } from "./dm-chat-panel-projection";
import { useDmChatComposerModel } from "./use-dm-chat-composer-model";
import { useDmChatSessionController } from "./use-dm-chat-session-controller";
import { useDmGoalController } from "./use-dm-goal-controller";

export function useDmChatPanelModel({
  currentAgentAvatar,
  currentAgentName,
  currentAgentPermissionMode,
  initialDraft,
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
  const goalScopeLabel = t("dm.goal_scope");
  const composer = useDmChatComposerModel({
    agentId: sessionIdentity?.agent_id ?? null,
    conversation: session.conversation,
    goalScopeLabel,
    initialDraft: initialDraft ?? null,
    onCreateGoal: goal.createGoal,
    onInitialDraftConsumed,
    scrollToBottom: session.scroll.scrollToBottom,
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
    onEditLastUserMessage: handleEditLastUserMessage,
    onOpenAgentContact,
    onOpenWorkspaceFile,
    session,
    workspaceAgentId: sessionIdentity?.agent_id ?? null,
  });
}
