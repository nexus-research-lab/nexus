import { useMemo } from "react";

import { useProviderAvailability } from "@/hooks/capability/use-provider-availability";
import { buildRoomSharedSessionKey } from "@/lib/conversation/session-key";
import { useAuth } from "@/shared/auth/auth-context";
import type { Agent } from "@/types/agent/agent";

import { useRoomThreadSource } from "../../../thread/live/use-room-thread-source";
import type { GroupChatPanelProps } from "../group-chat-panel-types";
import type { GroupChatPanelViewModel } from "../view/group-chat-panel-view";
import {
  buildGroupChatPanelViewModel,
  type RoomAgentDirectory,
} from "./group-chat-panel-projection";
import { useGroupChatComposerModel } from "./use-group-chat-composer-model";
import { useGroupChatSessionController } from "./use-group-chat-session-controller";
import { useRoomGoalComposer } from "./use-room-goal-composer";

export function useGroupChatPanelModel({
  agentId,
  conversationId,
  currentAgentAvatar = null,
  currentAgentName = null,
  initialDraft = null,
  layout = "desktop",
  onConversationSnapshotChange,
  onCreateConversation = () => {},
  onInitialDraftConsumed,
  onOpenAgentContact,
  onOpenWorkspaceFile,
  onRoomEvent,
  onTodosChange,
  roomHostAgentId = null,
  roomHostAutoReplyEnabled = false,
  roomId = null,
  roomMembers,
}: GroupChatPanelProps): GroupChatPanelViewModel {
  const sessionKey = conversationId
    ? buildRoomSharedSessionKey(conversationId)
    : null;
  const goal = useRoomGoalComposer({
    roomHostAgentId,
    roomMembers,
    sessionKey,
  });
  const session = useGroupChatSessionController({
    agentId,
    conversationId,
    onConversationSnapshotChange,
    onGoalEvent: goal.refresh,
    onRoomEvent,
    onTodosChange,
    roomId,
    sessionKey,
  });
  const directory = useRoomAgentDirectory(roomMembers);
  const { status: authStatus } = useAuth();
  const currentUserAvatar = authStatus?.avatar ?? null;
  const { hasAvailableProvider, isReady: providerReady } =
    useProviderAvailability();
  const isMobileLayout = layout === "mobile";
  const composer = useGroupChatComposerModel({
    conversation: session.conversation,
    conversationId,
    goal,
    initialDraft,
    onInitialDraftConsumed,
    roomId,
    roomMembers,
    scrollToBottom: session.scroll.scrollToBottom,
    sessionKey: session.sessionKey,
  });

  useRoomThreadSource({
    agentAvatarMap: directory.avatars,
    agentNameMap: directory.names,
    conversationId,
    currentUserAvatar,
    messageGroups: session.timeline.message_groups,
    onOpenWorkspaceFile,
    onStopMessage: session.conversation.stop_generation,
    pendingPermissionGroups: session.timeline.pending_permission_groups,
    pendingSlotGroups: session.timeline.pending_slot_groups,
    sendPermissionResponse: session.conversation.send_permission_response,
  });

  return buildGroupChatPanelViewModel({
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
    providerWarningVisible: providerReady && !hasAvailableProvider,
    roomHostAgentId,
    roomHostAutoReplyEnabled,
    roomMembers,
    session,
  });
}

function useRoomAgentDirectory(roomMembers: Agent[]): RoomAgentDirectory {
  return useMemo(() => buildRoomAgentDirectory(roomMembers), [roomMembers]);
}

function buildRoomAgentDirectory(roomMembers: Agent[]): RoomAgentDirectory {
  if (roomMembers.length === 0) {
    return {};
  }
  const avatars: Record<string, string | null> = {};
  const names: Record<string, string> = {};
  for (const member of roomMembers) {
    avatars[member.agent_id] = member.avatar ?? null;
    names[member.agent_id] = member.name;
  }
  return { avatars, names };
}
