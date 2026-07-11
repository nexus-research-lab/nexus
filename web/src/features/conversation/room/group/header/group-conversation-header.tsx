"use client";

import { memo, useState } from "react";

import { CreateRoomDialog } from "@/features/conversation/room/members/create-room-dialog";
import type { RoomDialogSubmission } from "@/features/conversation/room/members/create-room-dialog";
import { CONVERSATION_TOUR_ANCHORS } from "@/features/onboarding/tours/conversation-tour";
import { useSidebarStore } from "@/store/sidebar";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiRoomAvatar } from "@/shared/ui/display/avatar";
import { WorkspaceConversationTabs } from "@/shared/ui/workspace/controls/workspace-conversation-tabs";
import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace/surface/workspace-surface-header";
import type { Agent } from "@/types/agent/agent";
import type { RoomConversationView } from "@/types/conversation/conversation";
import type { RoomSurfaceTabKey } from "@/types/conversation/room-surface";
import { RoomHeaderGuideMenu } from "@/features/conversation/room/surface/header/room-header-guide-menu";
import { buildRoomHeaderTabs } from "@/features/conversation/room/surface/header/room-header-tabs";

import { GroupMemberAvatarStack } from "./group-member-avatar-stack";

interface GroupConversationHeaderProps {
  activeTab: RoomSurfaceTabKey;
  availableRoomAgents: Agent[];
  conversationId: string | null;
  conversations: RoomConversationView[];
  currentRoomTitle: string | null;
  onChangeTab: (tab: RoomSurfaceTabKey) => void;
  onCloseActiveTab: () => void;
  onCloseConversation: (conversationId: string) => Promise<void>;
  onCreateConversation?: (title?: string) => Promise<string | null>;
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
}: GroupConversationHeaderProps) {
  const { t } = useI18n();
  const widePanelCollapsed = useSidebarStore((state) => state.wide_panel_collapsed);
  const [memberDialogRoomId, setMemberDialogRoomId] = useState<string | null>(null);
  const headerTitle = currentRoomTitle?.trim() || t("room.untitled_collaboration");
  const roomTabs = buildRoomHeaderTabs(t);
  const memberAgentIds = roomMembers.map((member) => member.agent_id);
  const allRoomAgents = buildRoomAgentCatalog(roomMembers, availableRoomAgents);

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
        density="compact"
        dismissActiveTabLabel={t("common.close")}
        leading={(
          <UiRoomAvatar
            avatar={roomAvatar}
            className="h-full w-full rounded-full border-0 shadow-none"
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
        onChangeTab={onChangeTab}
        onDismissActiveTab={onCloseActiveTab}
        tabs={roomTabs}
        tabsLeading={(
          <WorkspaceConversationTabs
            conversationId={conversationId}
            conversations={conversations}
            onCloseConversation={onCloseConversation}
            onCreateConversation={onCreateConversation}
            onSelectConversation={onSelectConversation}
            tourAnchor={CONVERSATION_TOUR_ANCHORS.session_switcher}
          />
        )}
        title={widePanelCollapsed ? headerTitle : undefined}
        trailing={(
          <div className="flex items-center gap-2">
            <div className="hidden lg:flex">
              <GroupMemberAvatarStack
                members={roomMembers}
                onClick={() => void handleOpenMemberList()}
                tourAnchor={CONVERSATION_TOUR_ANCHORS.member_manage}
              />
            </div>
            {onReplayTour ? (
              <RoomHeaderGuideMenu onReplayTour={onReplayTour} />
            ) : null}
          </div>
        )}
      />

      <CreateRoomDialog
        agents={allRoomAgents}
        initialAvatar={roomAvatar ?? ""}
        initialHostAgentId={roomHostAgentId ?? null}
        initialHostAutoReplyEnabled={roomHostAutoReplyEnabled}
        initialName={headerTitle}
        initialPrivateMessagesEnabled={roomPrivateMessagesEnabled}
        initialRoomSkillNames={roomSkillNames}
        initialSelectedAgentIds={memberAgentIds}
        isOpen={roomId !== null && memberDialogRoomId === roomId}
        mode="manage"
        onCancel={() => setMemberDialogRoomId(null)}
        onConfirm={async (submission) => {
          await onManageRoom(submission);
          setMemberDialogRoomId(null);
        }}
      />
    </>
  );
});

function buildRoomAgentCatalog(
  members: Agent[],
  availableAgents: Agent[],
): Agent[] {
  const memberAgentIds = new Set(members.map((member) => member.agent_id));
  return [
    ...members,
    ...availableAgents.filter((agent) => !memberAgentIds.has(agent.agent_id)),
  ];
}
