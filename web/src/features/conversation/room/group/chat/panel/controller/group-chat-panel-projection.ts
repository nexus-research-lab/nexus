import type { useConversationSession } from "@/features/conversation/shared/session/use-conversation-session";
import type { Agent } from "@/types/agent/agent";

import type {
  GroupChatComposerModel,
  GroupChatPanelViewModel,
} from "../view/group-chat-panel-view";
import type { RoomGoalComposerModel } from "./use-room-goal-composer";

export interface RoomAgentDirectory {
  avatars?: Record<string, string | null>;
  names?: Record<string, string>;
}

interface BuildGroupChatPanelViewModelOptions {
  composer: GroupChatComposerModel;
  currentAgentAvatar: string | null;
  currentAgentName: string | null;
  currentUserAvatar: string | null;
  directory: RoomAgentDirectory;
  goal: RoomGoalComposerModel;
  isMobileLayout: boolean;
  onCreateConversation: (
    title?: string,
  ) => void | Promise<string | null>;
  onOpenAgentContact?: (agentId: string) => void;
  onOpenWorkspaceFile?: (path: string) => void;
  providerWarningVisible: boolean;
  roomHostAgentId: string | null;
  roomHostAutoReplyEnabled: boolean;
  roomMembers: Agent[];
  session: ReturnType<typeof useConversationSession>;
}

export function buildGroupChatPanelViewModel({
  composer,
  currentAgentAvatar,
  currentAgentName,
  currentUserAvatar,
  directory,
  goal,
  isMobileLayout,
  onCreateConversation,
  onOpenAgentContact,
  onOpenWorkspaceFile,
  providerWarningVisible,
  roomHostAgentId,
  roomHostAutoReplyEnabled,
  roomMembers,
  session,
}: BuildGroupChatPanelViewModelOptions): GroupChatPanelViewModel {
  return {
    composer,
    feed: buildFeedModel({
      currentAgentAvatar,
      currentAgentName,
      currentUserAvatar,
      directory,
      isMobileLayout,
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
    isMobileLayout,
    navigator: buildNavigatorModel(session),
    onCreateConversation,
    providerWarningVisible,
    scrollToLatest: buildScrollToLatestModel(session),
    sessionKey: session.sessionKey,
    viewport: buildViewportModel(session),
  };
}

function buildFeedModel({
  currentAgentAvatar,
  currentAgentName,
  currentUserAvatar,
  directory,
  isMobileLayout,
  onOpenAgentContact,
  onOpenWorkspaceFile,
  session,
}: Pick<
  BuildGroupChatPanelViewModelOptions,
  | "currentAgentAvatar"
  | "currentAgentName"
  | "currentUserAvatar"
  | "directory"
  | "isMobileLayout"
  | "onOpenAgentContact"
  | "onOpenWorkspaceFile"
  | "session"
>): GroupChatPanelViewModel["feed"] {
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
      agentAvatarMap: directory.avatars,
      agentNameMap: directory.names,
      currentAgentAvatar,
      currentAgentName,
      currentUserAvatar,
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
    activityKey: `${conversation.messages.length}:${conversation.is_loading ? "loading" : "idle"}:${goal.refreshSequence}`,
    isLoading: conversation.is_loading,
    roomHostAgentId,
    roomHostAutoReplyEnabled,
    roomMembers,
    sessionKey,
  };
}

function buildNavigatorModel(
  session: ReturnType<typeof useConversationSession>,
): GroupChatPanelViewModel["navigator"] {
  const { conversation, roundScrollRef, scroll, sessionKey, timeline } = session;
  return {
    onLoadRoundWindow: conversation.load_round_window,
    onNavigateStart: scroll.pauseFollowLatest,
    roundScrollRef,
    scopeKey: sessionKey,
    scrollRef: scroll.scrollRef,
    timeline,
  };
}

function buildScrollToLatestModel(
  session: ReturnType<typeof useConversationSession>,
): GroupChatPanelViewModel["scrollToLatest"] {
  return {
    isLoading: session.conversation.is_loading,
    onClick: () => session.scroll.scrollToBottom("smooth"),
    visible: session.scroll.showScrollToBottom,
  };
}

function buildViewportModel(
  session: ReturnType<typeof useConversationSession>,
): GroupChatPanelViewModel["viewport"] {
  const { conversation, history, scroll } = session;
  return {
    error: conversation.error,
    isHistoryLoading: conversation.is_history_loading,
    onScroll: history.handleScroll,
    onTouchEnd: scroll.onTouchEnd,
    onTouchMove: scroll.onTouchMove,
    onTouchStart: scroll.onTouchStart,
    onWheel: scroll.onWheel,
    scrollRef: scroll.scrollRef,
  };
}
