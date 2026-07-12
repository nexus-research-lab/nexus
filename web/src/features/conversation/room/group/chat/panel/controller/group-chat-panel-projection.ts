import type { RefObject } from "react";

import {
  buildConversationPanelFrameModel,
  type ConversationPanelEnvironment,
  type ConversationPanelSessionSource,
} from "@/features/conversation/shared/conversation-panel-model";
import { buildGoalActivityKey } from "@/features/conversation/shared/goal/goal-model";
import type { Agent } from "@/types/agent/agent";
import type { UseAgentConversationReturn } from "@/types/agent/agent-conversation";
import type { SessionRoundIndexItem } from "@/types/conversation/room";

import type {
  GroupChatComposerModel,
  GroupChatPanelViewModel,
} from "../view/group-chat-panel-view";
import type { RoomGoalComposerModel } from "./use-room-goal-composer";

export interface RoomAgentDirectory {
  avatars: Record<string, string | null>;
  names: Record<string, string>;
}

type GroupChatSession = Omit<
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
    | "stop_generation"
  >;
  roundIndexItems: SessionRoundIndexItem[];
  scroll: ConversationPanelSessionSource["scroll"] & {
    bottomAnchorRef: RefObject<HTMLDivElement | null>;
    feedRef: RefObject<HTMLDivElement | null>;
  };
};

interface BuildGroupChatPanelViewModelOptions {
  composer: GroupChatComposerModel;
  currentAgentAvatar: string | null;
  currentAgentName: string | null;
  directory: RoomAgentDirectory;
  environment: ConversationPanelEnvironment;
  goal: RoomGoalComposerModel;
  onCreateConversation: (
    title?: string,
  ) => void | Promise<string | null>;
  onOpenAgentContact?: (agentId: string) => void;
  onOpenWorkspaceFile?: (path: string) => void;
  roomHostAgentId: string | null;
  roomHostAutoReplyEnabled: boolean;
  roomMembers: Agent[];
  session: GroupChatSession;
}

export function buildGroupChatPanelViewModel({
  composer,
  currentAgentAvatar,
  currentAgentName,
  directory,
  environment,
  goal,
  onCreateConversation,
  onOpenAgentContact,
  onOpenWorkspaceFile,
  roomHostAgentId,
  roomHostAutoReplyEnabled,
  roomMembers,
  session,
}: BuildGroupChatPanelViewModelOptions): GroupChatPanelViewModel {
  return {
    ...buildConversationPanelFrameModel(session, environment),
    composer,
    feed: buildFeedModel({
      currentAgentAvatar,
      currentAgentName,
      directory,
      environment,
      onOpenAgentContact,
      onOpenWorkspaceFile,
      session,
    }),
    goalLead: buildGoalLeadModel({ goal, roomMembers, session }),
    goalPanel: buildGoalPanelModel({
      goal,
      roomHostAgentId,
      roomHostAutoReplyEnabled,
      roomMembers,
      session,
    }),
    onCreateConversation,
  };
}

function buildFeedModel({
  currentAgentAvatar,
  currentAgentName,
  directory,
  environment,
  onOpenAgentContact,
  onOpenWorkspaceFile,
  session,
}: Pick<
  BuildGroupChatPanelViewModelOptions,
  | "currentAgentAvatar"
  | "currentAgentName"
  | "directory"
  | "environment"
  | "onOpenAgentContact"
  | "onOpenWorkspaceFile"
  | "session"
>): GroupChatPanelViewModel["feed"] {
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
      agentAvatarMap: directory.avatars,
      agentNameMap: directory.names,
      currentAgentAvatar,
      currentAgentName,
      currentUserAvatar: environment.currentUserAvatar,
      isLastRoundPendingPermissions: conversation.pending_permissions,
      onOpenAgentContact,
      onOpenWorkspaceFile,
      onPermissionResponse: conversation.send_permission_response,
      onStopMessage: conversation.stop_generation,
      runtimePhase: conversation.runtime_phase,
    },
    source: {
      liveRoundIds: conversation.live_round_ids,
      messageGroups: timeline.message_groups,
      pendingPermissionGroups: timeline.pending_permission_groups,
      pendingSlotGroups: timeline.pending_slot_groups,
      roundIds: timeline.feed_round_ids,
      roundIndexItems,
    },
  };
}

function buildGoalLeadModel({
  goal,
  roomMembers,
  session,
}: Pick<
  BuildGroupChatPanelViewModelOptions,
  "goal" | "roomMembers" | "session"
>): GroupChatPanelViewModel["goalLead"] {
  return {
    agentId: goal.leadAgentId,
    disabled: session.conversation.is_loading || roomMembers.length === 0,
    onChange: goal.setLeadAgentId,
    roomMembers,
  };
}

function buildGoalPanelModel({
  goal,
  roomHostAgentId,
  roomHostAutoReplyEnabled,
  roomMembers,
  session,
}: Pick<
  BuildGroupChatPanelViewModelOptions,
  | "goal"
  | "roomHostAgentId"
  | "roomHostAutoReplyEnabled"
  | "roomMembers"
  | "session"
>): GroupChatPanelViewModel["goalPanel"] {
  const { conversation, sessionKey } = session;
  return {
    activityKey: buildGoalActivityKey(
      conversation.messages.length,
      conversation.is_loading,
      goal.refreshSequence,
    ),
    isLoading: conversation.is_loading,
    roomHostAgentId,
    roomHostAutoReplyEnabled,
    roomMembers,
    sessionKey,
  };
}
