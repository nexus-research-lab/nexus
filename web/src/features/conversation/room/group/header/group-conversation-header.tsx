// INPUT: Room 身份、成员、会话导航与目录准备/管理命令。
// OUTPUT: 标准群头像、共享标签与按当前 Room/owner 隔离的成员入口。
// POS: 群聊 Header 装配；成员写事务归页面命令，临时打开流程归本目录 Hook。

"use client";

import { memo } from "react";

import type { RoomDialogSubmission } from "@/features/conversation/room/members/create-room-dialog";
import { RoomMemberManagerDialog } from "@/features/conversation/room/members/room-member-manager-dialog";
import { CONVERSATION_TOUR_ANCHORS } from "@/features/onboarding/tours/conversation-tour";
import { useSidebarStore } from "@/store/sidebar";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiRoomAvatar } from "@/shared/ui/display/avatar";
import { RoomConversationTabs } from "@/features/navigation/conversation-tabs/room-conversation-tabs";
import type { FinalConversationReplacementHandler } from "@/features/navigation/conversation-tabs/final-conversation-replacement";
import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace/surface/workspace-surface-header";
import type { Agent } from "@/types/agent/agent";
import type { RoomConversationView } from "@/types/conversation/conversation";
import type { RoomSurfaceTabKey } from "@/features/conversation/room/surface/header/room-header-tabs";
import { buildRoomHeaderTabs } from "@/features/conversation/room/surface/header/room-header-tabs";
import { RoomHistoryMenu } from "@/features/conversation/room/surface/history/room-history-menu";

import { GroupMemberAvatarStack } from "./group-member-avatar-stack";
import { useRoomMemberManager } from "../../members/use-room-member-manager";

interface GroupConversationHeaderProps {
  activeTab: RoomSurfaceTabKey;
  availableRoomAgents: Agent[];
  conversationId: string | null;
  conversations: RoomConversationView[];
  currentRoomTitle: string | null;
  onChangeTab: (tab: RoomSurfaceTabKey) => void;
  onCloseConversation: (conversationId: string) => Promise<void>;
  onCreateConversation: (title?: string) => Promise<string | null>;
  onReplaceFinalConversation: FinalConversationReplacementHandler;
  onDeleteConversation: (conversationId: string) => Promise<string | null>;
  onManageRoom: (submission: RoomDialogSubmission) => Promise<void>;
  onOpenMemberManager: () => Promise<void>;
  onSelectConversation: (conversationId: string) => void;
  onUpdateConversationTitle?: (conversationId: string, title: string) => Promise<void>;
  roomAvatar?: string | null;
  roomHostAgentId?: string | null;
  roomHostAutoReplyEnabled: boolean;
  roomId: string | null;
  roomMembers: Agent[];
  roomPrivateMessagesEnabled: boolean;
  roomSkillNames: string[];
}

export const GroupConversationHeader = memo(function GroupConversationHeader({
  activeTab,
  availableRoomAgents,
  conversationId,
  conversations,
  currentRoomTitle,
  onChangeTab,
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
}: GroupConversationHeaderProps) {
  const { t } = useI18n();
  const widePanelCollapsed = useSidebarStore((state) => state.wide_panel_collapsed);
  const memberManager = useRoomMemberManager(roomId, onOpenMemberManager);
  const headerTitle = currentRoomTitle?.trim() || t("room.untitled_collaboration");
  const roomTabs = buildRoomHeaderTabs(t);

  return (
    <>
      <WorkspaceSurfaceHeader
        activeTab={activeTab}
        compactTabsLabel={t("room.panels")}
        leading={(
          <UiRoomAvatar
            avatar={roomAvatar}
            maxMembers={4}
            members={roomMembers.map((member) => ({
              avatar: member.avatar,
              id: member.agent_id,
              name: member.name,
            }))}
            roomId={roomId}
            size="md"
            title={headerTitle}
          />
        )}
        leadingVariant="identity"
        onChangeTab={onChangeTab}
        navigationTrailing={(
          <GroupMemberAvatarStack
            disabled={!roomId}
            isLoading={memberManager.isLoading}
            members={roomMembers}
            onClick={memberManager.open}
            tourAnchor={CONVERSATION_TOUR_ANCHORS.member_manage}
          />
        )}
        tabs={roomTabs}
        tabsLeading={(
          <RoomConversationTabs
            conversationId={conversationId}
            conversations={conversations}
            leadingControl={(
              <RoomHistoryMenu
                conversationId={conversationId}
                conversations={conversations}
                onCreateConversation={onCreateConversation}
                onDeleteConversation={onDeleteConversation}
                onSelectConversation={onSelectConversation}
                onUpdateConversationTitle={onUpdateConversationTitle}
                triggerVariant="session"
              />
            )}
            onCloseConversation={onCloseConversation}
            onCreateConversation={onCreateConversation}
            onReplaceFinalConversation={onReplaceFinalConversation}
            onSelectConversation={onSelectConversation}
            tourAnchor={CONVERSATION_TOUR_ANCHORS.session_switcher}
          />
        )}
        title={widePanelCollapsed ? headerTitle : undefined}
      />

      <RoomMemberManagerDialog
        availableRoomAgents={availableRoomAgents}
        initialAvatar={roomAvatar ?? ""}
        initialHostAgentId={roomHostAgentId ?? null}
        initialHostAutoReplyEnabled={roomHostAutoReplyEnabled}
        initialName={headerTitle}
        initialPrivateMessagesEnabled={roomPrivateMessagesEnabled}
        initialRoomSkillNames={roomSkillNames}
        isOpen={memberManager.isOpen}
        onClose={memberManager.close}
        onManageRoom={onManageRoom}
        roomMembers={roomMembers}
      />
    </>
  );
});
