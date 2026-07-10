import { DmConversationHeader } from "@/features/conversation/room/dm/dm-conversation-header";
import type { Agent } from "@/types/agent/agent";
import type { RoomConversationView } from "@/types/conversation/conversation";
import type { UpdateRoomParams } from "@/types/conversation/room";
import type { RoomSurfaceTabKey } from "@/types/conversation/room-surface";

import { GroupConversationHeader } from "../../group/header/group-conversation-header";
import { CONVERSATION_TOUR_ANCHORS } from "../../room-tour";

interface RoomSurfaceHeaderProps {
  activeSurfaceTab: RoomSurfaceTabKey;
  availableRoomAgents: Agent[];
  conversationId: string | null;
  conversations: RoomConversationView[];
  currentAgent: Agent;
  currentRoomTitle: string;
  isDm: boolean;
  onAddRoomMember: (agentId: string) => Promise<void>;
  onChangeSurfaceTab: (tab: RoomSurfaceTabKey) => void;
  onCloseAuxiliaryPanel: () => void;
  onCloseConversation: (conversationId: string) => Promise<void>;
  onCreateConversation: (title?: string) => Promise<string | null>;
  onOpenMemberManager: () => Promise<void>;
  onRemoveRoomMember: (agentId: string) => Promise<void>;
  onReplayTour?: () => void;
  onSelectConversation: (conversationId: string) => void;
  onUpdateRoom: (roomId: string, params: UpdateRoomParams) => Promise<void>;
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
  onAddRoomMember,
  onChangeSurfaceTab,
  onCloseAuxiliaryPanel,
  onCloseConversation,
  onCreateConversation,
  onOpenMemberManager,
  onRemoveRoomMember,
  onReplayTour,
  onSelectConversation,
  onUpdateRoom,
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
      activeTab={activeSurfaceTab}
      availableRoomAgents={availableRoomAgents}
      conversationId={conversationId}
      conversations={conversations}
      currentRoomTitle={currentRoomTitle}
      onAddRoomMember={onAddRoomMember}
      onChangeTab={onChangeSurfaceTab}
      onCloseActiveTab={onCloseAuxiliaryPanel}
      onCloseConversation={onCloseConversation}
      onCreateConversation={onCreateConversation}
      onOpenMemberManager={onOpenMemberManager}
      onRemoveRoomMember={onRemoveRoomMember}
      onReplayTour={onReplayTour}
      onSelectConversation={onSelectConversation}
      onUpdateRoom={onUpdateRoom}
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
