/**
 * INPUT: Room/DM Header 身份、会话标签、面板动作与成员管理命令。
 * OUTPUT: 带非交互下缘渐隐的桌面 Room Header。
 * POS: 桌面 Room Surface 顶部视觉边界；保持共享桌面 Header 几何，不参与消息滚动。
 */
import { DmConversationHeader } from "@/features/conversation/room/dm/dm-conversation-header";
import type { Agent } from "@/types/agent/agent";
import type { RoomDialogSubmission } from "@/features/conversation/room/members/create-room-dialog";
import type { RoomConversationView } from "@/types/conversation/conversation";
import type { RoomSurfaceTabKey } from "@/features/conversation/room/surface/header/room-header-tabs";

import { GroupConversationHeader } from "../../group/header/group-conversation-header";
import { CONVERSATION_TOUR_ANCHORS } from "@/features/onboarding/tours/conversation-tour";

import "../room-conversation-header-edge.css";

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
  onReplaceFinalConversation: (
    conversation: RoomConversationView,
    commitConversation: (conversationId: string) => boolean,
  ) => Promise<string | null>;
  onDeleteConversation: (conversationId: string) => Promise<string | null>;
  onManageRoom: (submission: RoomDialogSubmission) => Promise<void>;
  onOpenMemberManager: () => Promise<void>;
  onSelectConversation: (conversationId: string) => void;
  onUpdateConversationTitle?: (conversationId: string, title: string) => Promise<void>;
  roomAvatar?: string | null;
  roomHostAgentId: string | null;
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
  onReplaceFinalConversation,
  onDeleteConversation,
  onManageRoom,
  onOpenMemberManager,
  onSelectConversation,
  onUpdateConversationTitle,
  roomAvatar,
  roomHostAgentId,
  roomHostAutoReplyEnabled,
  roomId,
  roomMembers,
  roomPrivateMessagesEnabled,
  roomSkillNames,
}: RoomSurfaceHeaderProps) {
  const handleToggleSurfaceTab = (tab: RoomSurfaceTabKey) => {
    if (tab === activeSurfaceTab) {
      onCloseAuxiliaryPanel();
      return;
    }
    onChangeSurfaceTab(tab);
  };
  const header = isDm ? (
    <DmConversationHeader
      key={roomId ?? "dm-header"}
      activeTab={activeSurfaceTab}
      conversationId={conversationId}
      conversations={conversations}
      currentAgentName={currentAgent.name}
      currentAgentAvatar={currentAgent.avatar ?? null}
      onChangeTab={handleToggleSurfaceTab}
      onCloseConversation={onCloseConversation}
      onCreateConversation={onCreateConversation}
      onReplaceFinalConversation={onReplaceFinalConversation}
      onDeleteConversation={onDeleteConversation}
      onSelectConversation={onSelectConversation}
      onUpdateConversationTitle={onUpdateConversationTitle}
    />
  ) : (
    <GroupConversationHeader
      key={roomId ?? "room-header"}
      activeTab={activeSurfaceTab}
      availableRoomAgents={availableRoomAgents}
      conversationId={conversationId}
      conversations={conversations}
      currentRoomTitle={currentRoomTitle}
      onChangeTab={handleToggleSurfaceTab}
      onCloseConversation={onCloseConversation}
      onCreateConversation={onCreateConversation}
      onReplaceFinalConversation={onReplaceFinalConversation}
      onDeleteConversation={onDeleteConversation}
      onManageRoom={onManageRoom}
      onOpenMemberManager={onOpenMemberManager}
      onSelectConversation={onSelectConversation}
      onUpdateConversationTitle={onUpdateConversationTitle}
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
    <div
      className="nexus-room-conversation-header-edge"
      data-room-conversation-header-edge="true"
      data-tour-anchor={CONVERSATION_TOUR_ANCHORS.header}
    >
      {header}
    </div>
  );
}
