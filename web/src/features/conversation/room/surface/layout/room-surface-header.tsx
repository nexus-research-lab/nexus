import { DmConversationHeader } from "@/features/conversation/room/dm/dm-conversation-header";
import type { Agent } from "@/types/agent/agent";
import type { RoomDialogSubmission } from "@/features/conversation/room/members/create-room-dialog";
import type { RoomConversationView } from "@/types/conversation/conversation";
import type { RoomSurfaceTabKey } from "@/features/conversation/room/surface/header/room-header-tabs";

import { GroupConversationHeader } from "../../group/header/group-conversation-header";
import { CONVERSATION_TOUR_ANCHORS } from "@/features/onboarding/tours/conversation-tour";

interface RoomSurfaceHeaderProps {
  activeSurfaceTab: RoomSurfaceTabKey;
  availableRoomAgents: Agent[];
  conversationId: string | null;
  conversations: RoomConversationView[];
  currentAgent: Agent;
  currentRoomTitle: string;
  isDm: boolean;
  onChangeSurfaceTab: (tab: RoomSurfaceTabKey) => void;
  onCloseAuxiliaryPanel: () => void;
  onCloseConversation: (conversationId: string) => Promise<void>;
  onCreateConversation: (title?: string) => Promise<string | null>;
  onManageRoom: (submission: RoomDialogSubmission) => Promise<void>;
  onOpenMemberManager: () => Promise<void>;
  onReplayTour?: () => void;
  onSelectConversation: (conversationId: string) => void;
  roomAvatar?: string | null;
  roomHostAgentId?: string | null;
  roomHostAutoReplyEnabled: boolean;
  roomId: string | null;
  roomMembers: Agent[];
  roomPrivateMessagesEnabled: boolean;
  roomSkillNames: string[];
}

export function RoomSurfaceHeader({
  activeSurfaceTab,
  availableRoomAgents,
  conversationId,
  conversations,
  currentAgent,
  currentRoomTitle,
  isDm,
  onChangeSurfaceTab,
  onCloseAuxiliaryPanel,
  onCloseConversation,
  onCreateConversation,
  onManageRoom,
  onOpenMemberManager,
  onReplayTour,
  onSelectConversation,
  roomAvatar,
  roomHostAgentId,
  roomHostAutoReplyEnabled,
  roomId,
  roomMembers,
  roomPrivateMessagesEnabled,
  roomSkillNames,
}: RoomSurfaceHeaderProps) {
  const header = isDm ? (
    <DmConversationHeader
      activeTab={activeSurfaceTab}
      conversationId={conversationId}
      conversations={conversations}
      currentAgentName={currentAgent.name}
      currentAgentAvatar={currentAgent.avatar ?? null}
      onChangeTab={onChangeSurfaceTab}
      onCloseActiveTab={onCloseAuxiliaryPanel}
      onCloseConversation={onCloseConversation}
      onCreateConversation={onCreateConversation}
      onReplayTour={onReplayTour}
      onSelectConversation={onSelectConversation}
    />
  ) : (
    <GroupConversationHeader
      key={roomId ?? "room-header"}
      activeTab={activeSurfaceTab}
      availableRoomAgents={availableRoomAgents}
      conversationId={conversationId}
      conversations={conversations}
      currentRoomTitle={currentRoomTitle}
      onChangeTab={onChangeSurfaceTab}
      onCloseActiveTab={onCloseAuxiliaryPanel}
      onCloseConversation={onCloseConversation}
      onCreateConversation={onCreateConversation}
      onManageRoom={onManageRoom}
      onOpenMemberManager={onOpenMemberManager}
      onReplayTour={onReplayTour}
      onSelectConversation={onSelectConversation}
      roomAvatar={roomAvatar}
      roomHostAgentId={roomHostAgentId}
      roomHostAutoReplyEnabled={roomHostAutoReplyEnabled}
      roomId={roomId}
      roomMembers={roomMembers}
      roomPrivateMessagesEnabled={roomPrivateMessagesEnabled}
      roomSkillNames={roomSkillNames}
    />
  );

  return (
    <div data-tour-anchor={CONVERSATION_TOUR_ANCHORS.header}>
      {header}
    </div>
  );
}
