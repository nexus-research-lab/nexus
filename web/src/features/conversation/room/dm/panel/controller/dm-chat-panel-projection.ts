import {
  buildConversationNavigatorModel,
  buildConversationScrollToLatestModel,
  buildConversationViewportModel,
} from "@/features/conversation/shared/conversation-panel-model";
import { buildGoalActivityKey } from "@/features/conversation/shared/goal/goal-model";
import type { useConversationSession } from "@/features/conversation/shared/session/use-conversation-session";

import type {
  DmChatComposerModel,
  DmChatPanelViewModel,
} from "../view/dm-chat-panel-view";
import type { DmGoalControllerModel } from "./use-dm-goal-controller";

type DmChatSession = ReturnType<typeof useConversationSession>;
type DmGoalProjection = Pick<
  DmGoalControllerModel,
  "continuationHold" | "refreshSequence"
>;

interface BuildDmChatPanelViewModelOptions {
  composer: DmChatComposerModel;
  currentAgentAvatar: string | null;
  currentAgentName: string | null;
  currentUserAvatar: string | null;
  goal: DmGoalProjection;
  goalScopeLabel: string;
  isMobileLayout: boolean;
  onEditLastUserMessage: (messageId: string, content: string) => void;
  onOpenAgentContact?: (agentId: string) => void;
  onOpenWorkspaceFile?: (path: string) => void;
  providerWarningVisible: boolean;
  session: DmChatSession;
  workspaceAgentId: string | null;
}

export function buildDmChatPanelViewModel({
  composer,
  currentAgentAvatar,
  currentAgentName,
  currentUserAvatar,
  goal,
  goalScopeLabel,
  isMobileLayout,
  onEditLastUserMessage,
  onOpenAgentContact,
  onOpenWorkspaceFile,
  providerWarningVisible,
  session,
  workspaceAgentId,
}: BuildDmChatPanelViewModelOptions): DmChatPanelViewModel {
  return {
    composer,
    feed: buildDmFeedModel({
      currentAgentAvatar,
      currentAgentName,
      currentUserAvatar,
      isMobileLayout,
      onEditLastUserMessage,
      onOpenAgentContact,
      onOpenWorkspaceFile,
      session,
      workspaceAgentId,
    }),
    goalPanel: buildDmGoalPanelModel(goal, goalScopeLabel, session),
    isMobileLayout,
    navigator: buildConversationNavigatorModel(session),
    providerWarningVisible,
    scrollToLatest: buildConversationScrollToLatestModel(session),
    sessionKey: session.sessionKey,
    viewport: buildConversationViewportModel(session),
  };
}

function buildDmFeedModel({
  currentAgentAvatar,
  currentAgentName,
  currentUserAvatar,
  isMobileLayout,
  onEditLastUserMessage,
  onOpenAgentContact,
  onOpenWorkspaceFile,
  session,
  workspaceAgentId,
}: Pick<
  BuildDmChatPanelViewModelOptions,
  | "currentAgentAvatar"
  | "currentAgentName"
  | "currentUserAvatar"
  | "isMobileLayout"
  | "onEditLastUserMessage"
  | "onOpenAgentContact"
  | "onOpenWorkspaceFile"
  | "session"
  | "workspaceAgentId"
>): DmChatPanelViewModel["feed"] {
  const { conversation, roundIndexItems, roundScrollRef, scroll, timeline } =
    session;
  return {
    isMobileLayout,
    refs: {
      bottomAnchorRef: scroll.bottomAnchorRef,
      feedRef: scroll.feedRef,
      roundScrollRef,
      scrollRef: scroll.scrollRef,
    },
    renderer: {
      currentAgentAvatar,
      currentAgentName,
      currentUserAvatar,
      onEditLastUserMessage,
      onOpenAgentContact,
      onOpenWorkspaceFile,
      onPermissionResponse: conversation.send_permission_response,
      workspaceAgentId,
    },
    source: {
      liveRoundIds: conversation.live_round_ids,
      messageGroups: timeline.message_groups,
      pendingPermissions: conversation.pending_permissions,
      roundIds: timeline.feed_round_ids,
      roundIndexItems,
      runtimePhase: conversation.runtime_phase,
    },
  };
}

function buildDmGoalPanelModel(
  goal: DmGoalProjection,
  scopeLabel: string,
  session: DmChatSession,
): DmChatPanelViewModel["goalPanel"] {
  const { conversation, sessionKey } = session;
  return {
    activityKey: buildGoalActivityKey(
      conversation.messages.length,
      conversation.is_loading,
      goal.refreshSequence,
    ),
    continuationHold: goal.continuationHold,
    isGenerating: conversation.is_loading,
    scopeLabel,
    sessionKey,
  };
}
