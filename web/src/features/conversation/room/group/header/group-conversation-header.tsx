"use client";

import { memo, useState } from "react";

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
  const [memberDialogRoomId, setMemberDialogRoomId] = useState<string | null>(null);
  const headerTitle = currentRoomTitle?.trim() || t("room.untitled_collaboration");
  const roomTabs = buildRoomHeaderTabs(t);
  const handleOpenMemberList = async () => {
    const scopeRoomId = roomId;
    if (!scopeRoomId) {
      return;
    }
    await onOpenMemberManager();
    setMemberDialogRoomId(scopeRoomId);
  };

  return (
    <>
      <WorkspaceSurfaceHeader
        activeTab={activeTab}
        compactTabsLabel={t("room.panels")}
        leading={(
          <UiRoomAvatar
            avatar={roomAvatar}
            className="h-full w-full radius-control-sm border-0 shadow-none"
            maxMembers={4}
            members={roomMembers.map((member) => ({
              avatar: member.avatar,
              id: member.agent_id,
              name: member.name,
            }))}
            roomId={roomId}
            title={headerTitle}
          />
        )}
        leadingClassName="h-10 w-10 rounded-[10px]"
        leadingVariant="identity"
        onChangeTab={onChangeTab}
        navigationTrailing={(
          <GroupMemberAvatarStack
            members={roomMembers}
            onClick={() => void handleOpenMemberList()}
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
        isOpen={roomId !== null && memberDialogRoomId === roomId}
        onClose={() => setMemberDialogRoomId(null)}
        onManageRoom={onManageRoom}
        roomMembers={roomMembers}
      />
    </>
  );
});
