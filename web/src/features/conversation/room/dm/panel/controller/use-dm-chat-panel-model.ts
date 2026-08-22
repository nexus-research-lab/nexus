import { useCallback, useEffect } from "react";

import { useConversationPanelEnvironment } from "@/features/conversation/shared/use-conversation-panel-environment";
import { useComposerGoalSubmissionReconciliation } from "@/features/conversation/shared/composer/composer-goal-submission-reconciliation";
import { useI18n } from "@/shared/i18n/i18n-context";

import type { DmChatPanelProps } from "../dm-chat-panel-types";
import type { DmChatPanelViewModel } from "../view/dm-chat-panel-view";
import { buildDmChatPanelViewModel } from "./dm-chat-panel-projection";
import { useDmChatComposerModel } from "./use-dm-chat-composer-model";
import { useDmChatSessionController } from "./use-dm-chat-session-controller";
import { useDmGoalController } from "./use-dm-goal-controller";

export function useDmChatPanelModel({
  currentAgent,
  embeddedEditor,
  executionResource,
  initialDraft,
  layout,
  onConversationSnapshotChange,
  onExecutionTaskRunsChange,
  onBusyChange,
  onForkConversation,
  onInitialDraftConsumed,
  onOpenAgentContact,
  onOpenSubagentTask,
  onOpenWorkGraph,
  onOpenWorkspaceFile,
  onRoomEvent,
  onTodosChange,
  runtimeKind,
  sessionIdentity,
  todos,
}: DmChatPanelProps): DmChatPanelViewModel {
  const { t } = useI18n();
  const environment = useConversationPanelEnvironment(layout);
  const sessionKey = sessionIdentity?.session_key ?? null;
  const goal = useDmGoalController({
    agentName: currentAgent.name,
    permissionMode: currentAgent.options.permission_mode ?? null,
  });
  const session = useDmChatSessionController({
    identity: sessionIdentity,
    initialScrollAnchor: embeddedEditor ? "top" : "bottom",
    liveContentAlignment: embeddedEditor ? "start" : "end",
    onConversationSnapshotChange,
    onGoalEvent: goal.refresh,
    onRoomEvent,
    onTodosChange,
    visibleAfterUnixMilli: embeddedEditor?.visibleAfterUnixMilli,
  });
  useEffect(() => {
    onBusyChange?.(session.conversation.is_loading);
  }, [onBusyChange, session.conversation.is_loading]);
  useEffect(() => {
    onExecutionTaskRunsChange?.(session.taskRuns);
  }, [onExecutionTaskRunsChange, session.taskRuns]);
  const goalScopeLabel = t("dm.goal_scope");
  const composer = useDmChatComposerModel({
    agent: currentAgent,
    conversation: session.conversation,
    goalScopeLabel,
    initialDraft: initialDraft ?? null,
    onInitialDraftConsumed,
    scrollToBottom: session.scroll.scrollToBottom,
    sessionKey,
    runtimeKind,
    embeddedPlaceholder: embeddedEditor?.placeholder,
  });
  const reconcileGoalSubmission = useComposerGoalSubmissionReconciliation(
    composer.draftScopeKey,
    session.conversation.messages,
  );
  const rewriteLastUserMessage = session.conversation.rewrite_last_user_message;
  const handleEditLastUserMessage = useCallback(
    (messageId: string, content: string): void => {
      void rewriteLastUserMessage(messageId, content);
    },
    [rewriteLastUserMessage],
  );
  const model = buildDmChatPanelViewModel({
    composer,
    currentAgentAvatar: currentAgent.avatar ?? null,
    currentAgentName: currentAgent.name,
    environment,
    execution: executionResource,
    goal,
    goalScopeLabel,
    historyDividerLabel: t("message.history_above"),
    onGoalChange: reconcileGoalSubmission,
    onEditLastUserMessage: handleEditLastUserMessage,
    onForkConversation,
    onOpenAgentContact,
    onOpenSubagentTask,
    onOpenWorkGraph,
    onOpenWorkspaceFile,
    session,
    todos,
    workspaceAgentId: sessionIdentity?.agent_id ?? null,
  });
  model.embedded = Boolean(embeddedEditor);
  model.embeddedIntroduction = embeddedEditor ? {
    ...embeddedEditor.introduction,
    agentAvatar: currentAgent.avatar ?? null,
    agentName: currentAgent.name,
  } : null;
  if (embeddedEditor) {
    model.executionPanel = null;
    model.todos = [];
  }
  return model;
}
