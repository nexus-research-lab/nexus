import type { RefObject } from "react";

import {
  buildConversationPanelFrameModel,
  type ConversationPanelEnvironment,
  type ConversationPanelSessionSource,
} from "@/features/conversation/shared/conversation-panel-model";
import { buildGoalActivityKey } from "@/features/conversation/shared/goal/goal-model";
import type { UseAgentConversationReturn } from "@/types/agent/agent-conversation";
import type { SessionRoundIndexItem } from "@/types/conversation/room";

import type {
  DmChatComposerModel,
  DmChatPanelViewModel,
} from "../view/dm-chat-panel-view";
import type { DmGoalControllerModel } from "./use-dm-goal-controller";

type DmChatSession = Omit<
  ConversationPanelSessionSource,
  "conversation" | "scroll"
> & {
  conversation: ConversationPanelSessionSource["conversation"] & Pick<
    UseAgentConversationReturn,
    | "live_round_ids"
    | "messages"
    | "pending_permissions"
    | "runtime_phase"
    | "send_permission_response"
  >;
  roundIndexItems: SessionRoundIndexItem[];
  scroll: ConversationPanelSessionSource["scroll"] & {
    bottomAnchorRef: RefObject<HTMLDivElement | null>;
    feedRef: RefObject<HTMLDivElement | null>;
  };
};
type DmGoalProjection = Pick<
  DmGoalControllerModel,
  "continuationHold" | "refreshSequence"
>;

interface BuildDmChatPanelViewModelOptions {
  composer: DmChatComposerModel;
  currentAgentAvatar: string | null;
  currentAgentName: string | null;
  environment: ConversationPanelEnvironment;
  goal: DmGoalProjection;
  goalScopeLabel: string;
  onEditLastUserMessage: (messageId: string, content: string) => void;
  onOpenAgentContact?: (agentId: string) => void;
  onOpenWorkspaceFile?: (path: string) => void;
  session: DmChatSession;
  workspaceAgentId: string | null;
}

export function buildDmChatPanelViewModel({
  composer,
  currentAgentAvatar,
  currentAgentName,
  environment,
  goal,
  goalScopeLabel,
  onEditLastUserMessage,
  onOpenAgentContact,
  onOpenWorkspaceFile,
  session,
  workspaceAgentId,
}: BuildDmChatPanelViewModelOptions): DmChatPanelViewModel {
  return {
    ...buildConversationPanelFrameModel(session, environment),
    composer,
    feed: buildDmFeedModel({
      currentAgentAvatar,
      currentAgentName,
      environment,
      onEditLastUserMessage,
      onOpenAgentContact,
      onOpenWorkspaceFile,
      session,
      workspaceAgentId,
    }),
    goalPanel: buildDmGoalPanelModel(goal, goalScopeLabel, session),
  };
}

function buildDmFeedModel({
  currentAgentAvatar,
  currentAgentName,
  environment,
  onEditLastUserMessage,
  onOpenAgentContact,
  onOpenWorkspaceFile,
  session,
  workspaceAgentId,
}: Pick<
  BuildDmChatPanelViewModelOptions,
  | "currentAgentAvatar"
  | "currentAgentName"
  | "environment"
  | "onEditLastUserMessage"
  | "onOpenAgentContact"
  | "onOpenWorkspaceFile"
  | "session"
  | "workspaceAgentId"
>): DmChatPanelViewModel["feed"] {
  const { conversation, roundIndexItems, roundScrollRef, scroll, timeline } =
    session;
  return {
    isMobileLayout: environment.isMobileLayout,
    refs: {
      bottomAnchorRef: scroll.bottomAnchorRef,
      feedRef: scroll.feedRef,
      roundScrollRef,
      scrollRef: scroll.scrollRef,
    },
    renderer: {
      currentAgentAvatar,
      currentAgentName,
      currentUserAvatar: environment.currentUserAvatar,
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
