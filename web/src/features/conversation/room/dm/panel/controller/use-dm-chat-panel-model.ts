import { useCallback } from "react";

import { useProviderAvailability } from "@/hooks/capability/use-provider-availability";
import { useAuth } from "@/shared/auth/auth-context";
import { useI18n } from "@/shared/i18n/i18n-context";

import type { DmChatPanelProps } from "../dm-chat-panel-types";
import type { DmChatPanelViewModel } from "../view/dm-chat-panel-view";
import { buildDmChatPanelViewModel } from "./dm-chat-panel-projection";
import { useDmChatComposerModel } from "./use-dm-chat-composer-model";
import { useDmChatSessionController } from "./use-dm-chat-session-controller";
import { useDmGoalController } from "./use-dm-goal-controller";

export function useDmChatPanelModel({
  currentAgentAvatar = null,
  currentAgentName = null,
  currentAgentPermissionMode = null,
  initialDraft = null,
  layout = "desktop",
  onConversationSnapshotChange,
  onInitialDraftConsumed,
  onOpenAgentContact,
  onOpenWorkspaceFile,
  onRoomEvent,
  onTodosChange,
  sessionIdentity,
}: DmChatPanelProps): DmChatPanelViewModel {
  const { t } = useI18n();
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
    initialDraft,
    onCreateGoal: goal.createGoal,
    onInitialDraftConsumed,
    scrollToBottom: session.scroll.scrollToBottom,
    sessionKey,
  });
  const rewriteLastUserMessage = session.conversation.rewrite_last_user_message;
  const handleEditLastUserMessage = useCallback(
    (messageId: string, content: string): void => {
      void rewriteLastUserMessage(messageId, content);
    },
    [rewriteLastUserMessage],
  );
  const { status: authStatus } = useAuth();
  const { hasAvailableProvider, isReady: providerReady } =
    useProviderAvailability();

  return buildDmChatPanelViewModel({
    composer,
    currentAgentAvatar,
    currentAgentName,
    currentUserAvatar: authStatus?.avatar ?? null,
    goal,
    goalScopeLabel,
    isMobileLayout: layout === "mobile",
    onEditLastUserMessage: handleEditLastUserMessage,
    onOpenAgentContact,
    onOpenWorkspaceFile,
    providerWarningVisible: providerReady && !hasAvailableProvider,
    session,
    workspaceAgentId: sessionIdentity?.agent_id ?? null,
  });
}
