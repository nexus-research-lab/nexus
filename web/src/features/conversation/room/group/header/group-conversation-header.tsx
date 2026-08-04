"use client";

import { memo, useState } from "react";

import type { RoomDialogSubmission } from "@/features/conversation/room/members/create-room-dialog";
import { RoomMemberManagerDialog } from "@/features/conversation/room/members/room-member-manager-dialog";
import { CONVERSATION_TOUR_ANCHORS } from "@/features/onboarding/tours/conversation-tour";
import { useMediaQuery } from "@/hooks/ui/use-media-query";
import { useSidebarStore } from "@/store/sidebar";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiRoomAvatar } from "@/shared/ui/display/avatar";
import { WorkspaceConversationTabs } from "@/shared/ui/workspace/controls/workspace-conversation-tabs";
import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace/surface/workspace-surface-header";
import type { Agent } from "@/types/agent/agent";
import type { RoomConversationView } from "@/types/conversation/conversation";
import type { RoomSurfaceTabKey } from "@/features/conversation/room/surface/header/room-header-tabs";
import { RoomHeaderGuideMenu } from "@/features/conversation/room/surface/header/room-header-guide-menu";
import { buildRoomHeaderTabs } from "@/features/conversation/room/surface/header/room-header-tabs";
import { useRoomHeaderOverflowTabs } from "@/features/conversation/room/surface/header/use-room-header-overflow-tabs";
import { RoomHistoryMenu } from "@/features/conversation/room/surface/history/room-history-menu";

import { GroupMemberAvatarStack } from "./group-member-avatar-stack";

const GROUP_MEMBER_STACK_COLLAPSE_MEDIA_QUERY = "(max-width: 1119px)";

interface GroupConversationHeaderProps {
  activeTab: RoomSurfaceTabKey;
  availableRoomAgents: Agent[];
  conversationId: string | null;
  conversations: RoomConversationView[];
  currentRoomTitle: string | null;
  onChangeTab: (tab: RoomSurfaceTabKey) => void;
  onCloseActiveTab: () => void;
  onCloseConversation: (conversationId: string) => Promise<void>;
  onCreateConversation: (title?: string) => Promise<string | null>;
  onDeleteConversation: (conversationId: string) => Promise<string | null>;
  onManageRoom: (submission: RoomDialogSubmission) => Promise<void>;
  onOpenMemberManager: () => Promise<void>;
  onReplayTour?: () => void;
  onSelectConversation: (conversationId: string) => void;
  onUpdateConversationTitle?: (conversationId: string, title: string) => Promise<void>;
  roomAvatar?: string | null;
  roomHostAgentId?: string | null;
  roomHostAutoReplyEnabled: boolean;
  roomId: string | null;
  roomMembers: Agent[];
  roomPrivateMessagesEnabled: boolean;
  roomSkillNames: string[];
  workgraphAvailable: boolean;
}

export const GroupConversationHeader = memo(function GroupConversationHeader({
  activeTab,
  availableRoomAgents,
  conversationId,
  conversations,
  currentRoomTitle,
  onChangeTab,
  onCloseActiveTab,
  onCloseConversation,
  onCreateConversation,
  onDeleteConversation,
  onManageRoom,
  onOpenMemberManager,
  onReplayTour,
  onSelectConversation,
  onUpdateConversationTitle,
  roomAvatar,
  roomHostAgentId,
  roomHostAutoReplyEnabled,
  roomId,
  roomMembers,
  roomPrivateMessagesEnabled,
  roomSkillNames,
  workgraphAvailable,
}: GroupConversationHeaderProps) {
  const { t } = useI18n();
  const showMembersInGuideMenu = useMediaQuery(
    GROUP_MEMBER_STACK_COLLAPSE_MEDIA_QUERY,
  );
  const widePanelCollapsed = useSidebarStore((state) => state.wide_panel_collapsed);
  const [memberDialogRoomId, setMemberDialogRoomId] = useState<string | null>(null);
  const headerTitle = currentRoomTitle?.trim() || t("room.untitled_collaboration");
  const roomTabs = buildRoomHeaderTabs(t, { workgraphAvailable });
  const collapsedRoomTabs = useRoomHeaderOverflowTabs(roomTabs);
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
        dismissActiveTabLabel={t("common.close")}
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
        onDismissActiveTab={onCloseActiveTab}
        navigationTrailing={(
          <>
            <div className="hidden h-full items-center lg:flex">
              <GroupMemberAvatarStack
                members={roomMembers}
                onClick={() => void handleOpenMemberList()}
                tourAnchor={CONVERSATION_TOUR_ANCHORS.member_manage}
              />
            </div>
            {onReplayTour || roomId || collapsedRoomTabs.length > 0 ? (
              <RoomHeaderGuideMenu
                activeTab={activeTab}
                collapsedTabs={collapsedRoomTabs}
                onChangeTab={onChangeTab}
                onCloseActiveTab={onCloseActiveTab}
                onManageMembers={showMembersInGuideMenu
                  ? () => void handleOpenMemberList()
                  : undefined}
                onReplayTour={onReplayTour}
              />
            ) : null}
          </>
        )}
        tabs={roomTabs}
        tabsLeading={(
          <WorkspaceConversationTabs
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
