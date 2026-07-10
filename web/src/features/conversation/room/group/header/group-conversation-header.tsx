"use client";

import { memo, useRef, useState } from "react";
import {
  Bot,
  Compass,
  FolderTree,
  History,
  Info,
  type LucideIcon,
  MoreHorizontal,
} from "lucide-react";

import { UiActionMenu } from "@/shared/ui/action-menu";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiAgentAvatar, UiRoomAvatar } from "@/shared/ui/avatar";
import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace/surface/workspace-surface-header";
import { WorkspaceConversationTabs } from "@/shared/ui/workspace/controls/workspace-conversation-tabs";
import { Agent } from "@/types/agent/agent";
import { RoomConversationView } from "@/types/conversation/conversation";
import { UpdateRoomParams } from "@/types/conversation/room";
import { RoomSurfaceTabKey } from "@/types/conversation/room-surface";

import { CreateRoomDialog } from "@/features/conversation/room/members/create-room-dialog";
import { CONVERSATION_TOUR_ANCHORS } from "../../room-tour";

interface GroupConversationHeaderProps {
  conversationId: string | null;
  roomId: string | null;
  currentRoomTitle: string | null;
  roomSkillNames: string[];
  roomAvatar?: string | null;
  roomHostAgentId?: string | null;
  roomHostAutoReplyEnabled: boolean;
  roomPrivateMessagesEnabled: boolean;
  conversations: RoomConversationView[];
  roomMembers: Agent[];
  availableRoomAgents: Agent[];
  activeTab: RoomSurfaceTabKey;
  onReplayTour?: () => void;
  onChangeTab: (tab: RoomSurfaceTabKey) => void;
  onCloseActiveTab: () => void;
  onSelectConversation: (conversationId: string) => void;
  onCloseConversation: (conversationId: string) => Promise<void>;
  onCreateConversation?: (title?: string) => Promise<string | null>;
  onAddRoomMember: (agentId: string) => Promise<void>;
  onRemoveRoomMember: (agentId: string) => Promise<void>;
  onOpenMemberManager: () => Promise<void>;
  onUpdateRoom: (roomId: string, params: UpdateRoomParams) => Promise<void>;
}

function MemberAvatarStack({
  roomMembers: roomMembers,
  onClick: onClick,
  tourAnchor: tourAnchor,
}: {
  roomMembers: Agent[];
  onClick: () => void;
  tourAnchor?: string;
}) {
  const { t } = useI18n();
  const visibleMembers = roomMembers.slice(0, 4);
  const overflowCount = Math.max(0, roomMembers.length - visibleMembers.length);

  return (
    <button
      className="flex h-7 items-center gap-1.5 rounded-full border border-(--divider-subtle-color) bg-(--surface-panel-background) px-2 text-[10.5px] font-medium text-(--text-default) transition-[border-color,background,color,transform] duration-(--motion-duration-fast) hover:-translate-y-px hover:border-(--surface-interactive-hover-border) hover:text-(--text-strong)"
      data-tour-anchor={tourAnchor}
      onClick={onClick}
      type="button"
    >
      <div className="flex items-center -space-x-1.5">
        {visibleMembers.map((member) => (
          <UiAgentAvatar
            avatar={member.avatar}
            className="ring-1 ring-(--background)"
            key={member.agent_id}
            name={member.name}
            size="xs"
            title={member.name}
          />
        ))}
        {overflowCount > 0 ? (
          <span className="flex h-5.5 w-5.5 items-center justify-center rounded-full border border-(--surface-avatar-border) bg-(--surface-avatar-background) text-[8px] font-bold text-(--text-strong) shadow-(--surface-avatar-shadow)">
            +{overflowCount}
          </span>
        ) : null}
      </div>
      <span className="hidden sm:inline">{t("room.members")}</span>
    </button>
  );
}

const GroupConversationHeaderView = memo(({
  conversationId: conversationId,
  roomId: roomId,
  currentRoomTitle: currentRoomTitle,
  roomSkillNames: roomSkillNames,
  roomAvatar: roomAvatar,
  roomHostAgentId: roomHostAgentId,
  roomHostAutoReplyEnabled: roomHostAutoReplyEnabled,
  roomPrivateMessagesEnabled: roomPrivateMessagesEnabled,
  conversations,
  roomMembers: roomMembers,
  availableRoomAgents: availableRoomAgents,
  activeTab: activeTab,
  onReplayTour: onReplayTour,
  onChangeTab: onChangeTab,
  onCloseActiveTab: onCloseActiveTab,
  onSelectConversation: onSelectConversation,
  onCloseConversation: onCloseConversation,
  onCreateConversation: onCreateConversation,
  onAddRoomMember: onAddRoomMember,
  onRemoveRoomMember: onRemoveRoomMember,
  onOpenMemberManager: onOpenMemberManager,
  onUpdateRoom: onUpdateRoom,
}: GroupConversationHeaderProps) => {
  const { t } = useI18n();
  const [isMemberListOpen, setIsMemberListOpen] = useState(false);
  const moreButtonRef = useRef<HTMLButtonElement>(null);
  const [isMoreOpen, setIsMoreOpen] = useState(false);
  const headerTitle = currentRoomTitle?.trim() || t("room.untitled_collaboration");
  const roomTabs: {
    key: RoomSurfaceTabKey;
    label: string;
    icon: LucideIcon;
    anchor?: string;
  }[] = [
    { key: "history", label: t("room.history"), icon: History, anchor: CONVERSATION_TOUR_ANCHORS.tab_history },
    { key: "workspace", label: t("room.workspace"), icon: FolderTree, anchor: CONVERSATION_TOUR_ANCHORS.tab_workspace },
    { key: "subagents", label: t("subagents.label"), icon: Bot },
    { key: "about", label: t("room.about"), icon: Info, anchor: CONVERSATION_TOUR_ANCHORS.tab_about },
  ];

  const memberAgentIds = roomMembers.map((member) => member.agent_id);
  const allRoomAgents = [
    ...roomMembers,
    ...availableRoomAgents.filter(
      (agent) => !roomMembers.some((member) => member.agent_id === agent.agent_id),
    ),
  ];

  const handleOpenMemberList = async () => {
    await onOpenMemberManager();
    setIsMemberListOpen(true);
  };

  const conversationTabs = (
    <WorkspaceConversationTabs
      conversations={conversations}
      conversationId={conversationId}
      onCloseConversation={onCloseConversation}
      onCreateConversation={onCreateConversation}
      onSelectConversation={onSelectConversation}
      tourAnchor={CONVERSATION_TOUR_ANCHORS.session_switcher}
    />
  );

  const trailing = (
    <div className="flex items-center gap-2">
      <div className="hidden lg:flex">
        <MemberAvatarStack
          onClick={() => {
            void handleOpenMemberList();
          }}
          roomMembers={roomMembers}
          tourAnchor={CONVERSATION_TOUR_ANCHORS.member_manage}
        />
      </div>
      {onReplayTour ? (
        <>
          <button
            ref={moreButtonRef}
            aria-haspopup="menu"
            aria-label={t("common.more_actions")}
            className="inline-flex h-7 w-7 items-center justify-center rounded-full text-(--icon-default) transition-[background,color] hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
            onClick={() => setIsMoreOpen((prev) => !prev)}
            title={t("common.more_actions")}
            type="button"
          >
            <MoreHorizontal className="h-4 w-4" />
          </button>
          <UiActionMenu
            anchorRef={moreButtonRef}
            ariaLabel={t("common.more_actions")}
            isOpen={isMoreOpen}
            items={[
              {
                value: "guide",
                label: t("common.view_guide"),
                icon: <Compass className="h-4 w-4 text-(--icon-muted)" />,
              },
            ]}
            onClose={() => setIsMoreOpen(false)}
            onSelect={(value) => {
              if (value === "guide") {
                onReplayTour?.();
              }
            }}
          />
        </>
      ) : null}
    </div>
  );

  return (
    <>
      <WorkspaceSurfaceHeader
        activeTab={activeTab}
        density="compact"
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
        dismissActiveTabLabel={t("common.close")}
        tabs={roomTabs}
        tabsLeading={conversationTabs}
        title={headerTitle}
        trailing={trailing}
      />

      <CreateRoomDialog
        agents={allRoomAgents}
        confirmLabel={t("common.save")}
        dialogSubtitle={t("room.manage_dialog_subtitle")}
        dialogTitle={t("room.manage_dialog_title")}
        initialAvatar={roomAvatar ?? ""}
        initialHostAgentId={roomHostAgentId ?? null}
        initialHostAutoReplyEnabled={roomHostAutoReplyEnabled}
        initialPrivateMessagesEnabled={roomPrivateMessagesEnabled}
        initialName={headerTitle}
        initialSelectedAgentIds={memberAgentIds}
        initialRoomSkillNames={roomSkillNames}
        isOpen={isMemberListOpen}
        mode="manage"
        onCancel={() => setIsMemberListOpen(false)}
        onConfirm={async (nextAgentIds, name, avatar, skillNames, hostAgentId, hostAutoReplyEnabled, privateMessagesEnabled) => {
          if (!roomId) {
            return;
          }

          const nextAgentIdSet = new Set(nextAgentIds);
          const currentAgentIdSet = new Set(memberAgentIds);
          const agentIdsToAdd = nextAgentIds.filter((agentId) => !currentAgentIdSet.has(agentId));
          const agentIdsToRemove = memberAgentIds.filter((agentId) => !nextAgentIdSet.has(agentId));

          for (const agentId of agentIdsToAdd) {
            await onAddRoomMember(agentId);
          }

          await onUpdateRoom(roomId, {
            name,
            avatar,
            skill_names: skillNames,
            host_agent_id: hostAgentId,
            host_auto_reply_enabled: hostAutoReplyEnabled,
            private_messages_enabled: privateMessagesEnabled,
          });

          for (const agentId of agentIdsToRemove) {
            await onRemoveRoomMember(agentId);
          }

          setIsMemberListOpen(false);
        }}
      />
    </>
  );
});

GroupConversationHeaderView.displayName = "GroupConversationHeaderView";

export function GroupConversationHeader(props: GroupConversationHeaderProps) {
  return <GroupConversationHeaderView {...props} />;
}
