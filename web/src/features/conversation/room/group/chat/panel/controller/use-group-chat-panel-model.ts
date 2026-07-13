import { useMemo } from "react";

import { useConversationPanelEnvironment } from "@/features/conversation/shared/use-conversation-panel-environment";
import { buildRoomSharedSessionKey } from "@/lib/conversation/session-key";
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
  currentAgentAvatar,
  currentAgentName,
  initialDraft,
  layout,
  onConversationSnapshotChange,
  onCreateConversation,
  onInitialDraftConsumed,
  onOpenAgentContact,
  onOpenWorkspaceFile,
  onRoomEvent,
  onTodosChange,
  roomHostAgentId,
  roomHostAutoReplyEnabled,
  roomId,
  roomMembers,
  runtimeKind,
}: GroupChatPanelProps): GroupChatPanelViewModel {
  const environment = useConversationPanelEnvironment(layout);
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
  const composer = useGroupChatComposerModel({
    conversation: session.conversation,
    conversationId,
    goal,
    initialDraft: initialDraft ?? null,
    onInitialDraftConsumed,
    roomId,
    roomMembers,
    scrollToBottom: session.scroll.scrollToBottom,
    sessionKey: session.sessionKey,
    runtimeKind,
  });

  useRoomThreadSource({
    agentAvatarMap: directory.avatars,
    agentNameMap: directory.names,
    conversationId,
    currentUserAvatar: environment.currentUserAvatar,
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
  });
}

function useRoomAgentDirectory(roomMembers: Agent[]): RoomAgentDirectory {
  return useMemo(() => buildRoomAgentDirectory(roomMembers), [roomMembers]);
}

function buildRoomAgentDirectory(roomMembers: Agent[]): RoomAgentDirectory {
  const avatars: Record<string, string | null> = {};
  const names: Record<string, string> = {};
  for (const member of roomMembers) {
    avatars[member.agent_id] = member.avatar ?? null;
    names[member.agent_id] = member.name;
  }
  return { avatars, names };
}
