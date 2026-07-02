"use client";

import { memo, useState } from "react";
import {
  Compass,
  FolderTree,
  History,
  Info,
  type LucideIcon,
} from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiAgentAvatar, UiRoomAvatar } from "@/shared/ui/avatar";
import {
  WorkspaceSurfaceHeader,
  WorkspaceSurfaceToolbarAction,
  WorkspaceTaskStrip,
} from "@/shared/ui/workspace/surface/workspace-surface-header";
import { WorkspaceConversationTabs } from "@/shared/ui/workspace/controls/workspace-conversation-tabs";
import { Agent } from "@/types/agent/agent";
import { RoomConversationView } from "@/types/conversation/conversation";
import { UpdateRoomParams } from "@/types/conversation/room";
import { RoomSurfaceTabKey } from "@/types/conversation/room-surface";
import { TodoItem } from "@/types/conversation/todo";

import { CreateRoomDialog } from "@/features/conversation/room/members/create-room-dialog";
import { CONVERSATION_TOUR_ANCHORS } from "../../room-tour";

interface GroupConversationHeaderProps {
  conversation_id: string | null;
  room_id: string | null;
  current_room_title: string | null;
  room_skill_names: string[];
  room_avatar?: string | null;
  room_host_agent_id?: string | null;
  room_host_auto_reply_enabled: boolean;
  room_private_messages_enabled: boolean;
  conversations: RoomConversationView[];
  room_members: Agent[];
  available_room_agents: Agent[];
  todos: TodoItem[];
  active_tab: RoomSurfaceTabKey;
  on_replay_tour?: () => void;
  on_change_tab: (tab: RoomSurfaceTabKey) => void;
  on_select_conversation: (conversationId: string) => void;
  on_close_conversation: (conversationId: string) => Promise<void>;
  on_create_conversation?: (title?: string) => Promise<string | null>;
  on_add_room_member: (agentId: string) => Promise<void>;
  on_remove_room_member: (agentId: string) => Promise<void>;
  on_open_member_manager: () => Promise<void>;
  on_update_room: (roomId: string, params: UpdateRoomParams) => Promise<void>;
}

function MemberAvatarStack({
  room_members: roomMembers,
  on_click: onClick,
  tour_anchor: tourAnchor,
}: {
  room_members: Agent[];
  on_click: () => void;
  tour_anchor?: string;
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
            class_name="ring-1 ring-(--background)"
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
  conversation_id: conversationId,
  room_id: roomId,
  current_room_title: currentRoomTitle,
  room_skill_names: roomSkillNames,
  room_avatar: roomAvatar,
  room_host_agent_id: roomHostAgentId,
  room_host_auto_reply_enabled: roomHostAutoReplyEnabled,
  room_private_messages_enabled: roomPrivateMessagesEnabled,
  conversations,
  room_members: roomMembers,
  available_room_agents: availableRoomAgents,
  todos,
  active_tab: activeTab,
  on_replay_tour: onReplayTour,
  on_change_tab: onChangeTab,
  on_select_conversation: onSelectConversation,
  on_close_conversation: onCloseConversation,
  on_create_conversation: onCreateConversation,
  on_add_room_member: onAddRoomMember,
  on_remove_room_member: onRemoveRoomMember,
  on_open_member_manager: onOpenMemberManager,
  on_update_room: onUpdateRoom,
}: GroupConversationHeaderProps) => {
  const { t } = useI18n();
  const [isMemberListOpen, setIsMemberListOpen] = useState(false);
  const headerTitle = currentRoomTitle?.trim() || t("room.untitled_collaboration");
  const roomTabs: {
    key: RoomSurfaceTabKey;
    label: string;
    icon: LucideIcon;
    anchor?: string;
  }[] = [
    { key: "history", label: t("room.history"), icon: History, anchor: CONVERSATION_TOUR_ANCHORS.tab_history },
    { key: "workspace", label: t("room.workspace"), icon: FolderTree, anchor: CONVERSATION_TOUR_ANCHORS.tab_workspace },
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
      conversation_id={conversationId}
      on_close_conversation={onCloseConversation}
      on_create_conversation={onCreateConversation}
      on_select_conversation={onSelectConversation}
      tour_anchor={CONVERSATION_TOUR_ANCHORS.session_switcher}
    />
  );

  const trailing = (
    <div className="flex items-center gap-2">
      <div className="hidden lg:flex">
        <MemberAvatarStack
          on_click={() => {
            void handleOpenMemberList();
          }}
          room_members={roomMembers}
          tour_anchor={CONVERSATION_TOUR_ANCHORS.member_manage}
        />
      </div>
      {onReplayTour ? (
        <WorkspaceSurfaceToolbarAction onClick={onReplayTour}>
          <Compass className="h-3.5 w-3.5" />
          {t("common.view_guide")}
        </WorkspaceSurfaceToolbarAction>
      ) : null}
    </div>
  );

  return (
    <>
      <WorkspaceSurfaceHeader
        active_tab={activeTab}
        density="compact"
        leading={(
          <UiRoomAvatar
            avatar={roomAvatar}
            class_name="h-full w-full rounded-full border-0 shadow-none"
            max_members={4}
            members={roomMembers.map((member) => ({
              avatar: member.avatar,
              id: member.agent_id,
              name: member.name,
            }))}
            room_id={roomId}
            title={headerTitle}
          />
        )}
        on_change_tab={onChangeTab}
        tabs={roomTabs}
        tabs_leading={conversationTabs}
        tabs_trailing={<WorkspaceTaskStrip todos={todos} />}
        title={headerTitle}
        trailing={trailing}
      />

      <CreateRoomDialog
        agents={allRoomAgents}
        confirm_label={t("common.save")}
        dialog_subtitle={t("room.manage_dialog_subtitle")}
        dialog_title={t("room.manage_dialog_title")}
        initial_avatar={roomAvatar ?? ""}
        initial_host_agent_id={roomHostAgentId ?? null}
        initial_host_auto_reply_enabled={roomHostAutoReplyEnabled}
        initial_private_messages_enabled={roomPrivateMessagesEnabled}
        initial_name={headerTitle}
        initial_selected_agent_ids={memberAgentIds}
        initial_room_skill_names={roomSkillNames}
        is_open={isMemberListOpen}
        mode="manage"
        on_cancel={() => setIsMemberListOpen(false)}
        on_confirm={async (nextAgentIds, name, avatar, skillNames, hostAgentId, hostAutoReplyEnabled, privateMessagesEnabled) => {
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
