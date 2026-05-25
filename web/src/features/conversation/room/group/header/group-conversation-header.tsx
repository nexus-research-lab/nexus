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
  conversations: RoomConversationView[];
  room_members: Agent[];
  available_room_agents: Agent[];
  todos: TodoItem[];
  active_tab: RoomSurfaceTabKey;
  on_replay_tour?: () => void;
  on_change_tab: (tab: RoomSurfaceTabKey) => void;
  on_select_conversation: (conversation_id: string) => void;
  on_create_conversation?: (title?: string) => Promise<string | null>;
  on_add_room_member: (agent_id: string) => Promise<void>;
  on_remove_room_member: (agent_id: string) => Promise<void>;
  on_open_member_manager: () => Promise<void>;
  on_update_room: (room_id: string, params: UpdateRoomParams) => Promise<void>;
}

function MemberAvatarStack({
  room_members,
  on_click,
  tour_anchor,
}: {
  room_members: Agent[];
  on_click: () => void;
  tour_anchor?: string;
}) {
  const { t } = useI18n();
  const visible_members = room_members.slice(0, 4);
  const overflow_count = Math.max(0, room_members.length - visible_members.length);

  return (
    <button
      className="flex h-7 items-center gap-1.5 rounded-full border border-(--divider-subtle-color) bg-(--surface-panel-background) px-2 text-[10.5px] font-medium text-(--text-default) transition-[border-color,background,color,transform] duration-(--motion-duration-fast) hover:-translate-y-px hover:border-(--surface-interactive-hover-border) hover:text-(--text-strong)"
      data-tour-anchor={tour_anchor}
      onClick={on_click}
      type="button"
    >
      <div className="flex items-center -space-x-1.5">
        {visible_members.map((member) => (
          <UiAgentAvatar
            avatar={member.avatar}
            class_name="ring-1 ring-(--background)"
            key={member.agent_id}
            name={member.name}
            size="xs"
            title={member.name}
          />
        ))}
        {overflow_count > 0 ? (
          <span className="flex h-5.5 w-5.5 items-center justify-center rounded-full border border-(--surface-avatar-border) bg-(--surface-avatar-background) text-[8px] font-bold text-(--text-strong) shadow-(--surface-avatar-shadow)">
            +{overflow_count}
          </span>
        ) : null}
      </div>
      <span className="hidden sm:inline">{t("room.members")}</span>
    </button>
  );
}

const GroupConversationHeaderView = memo(({
  conversation_id,
  room_id,
  current_room_title,
  room_skill_names,
  room_avatar,
  room_host_agent_id,
  room_host_auto_reply_enabled,
  conversations,
  room_members,
  available_room_agents,
  todos,
  active_tab,
  on_replay_tour,
  on_change_tab,
  on_select_conversation,
  on_create_conversation,
  on_add_room_member,
  on_remove_room_member,
  on_open_member_manager,
  on_update_room,
}: GroupConversationHeaderProps) => {
  const { t } = useI18n();
  const [is_member_list_open, set_is_member_list_open] = useState(false);
  const header_title = current_room_title?.trim() || t("room.untitled_collaboration");
  const room_tabs: {
    key: RoomSurfaceTabKey;
    label: string;
    icon: LucideIcon;
    anchor?: string;
  }[] = [
    { key: "history", label: t("room.history"), icon: History, anchor: CONVERSATION_TOUR_ANCHORS.tab_history },
    { key: "workspace", label: t("room.workspace"), icon: FolderTree, anchor: CONVERSATION_TOUR_ANCHORS.tab_workspace },
    { key: "about", label: t("room.about"), icon: Info, anchor: CONVERSATION_TOUR_ANCHORS.tab_about },
  ];

  const member_agent_ids = room_members.map((member) => member.agent_id);
  const all_room_agents = [
    ...room_members,
    ...available_room_agents.filter(
      (agent) => !room_members.some((member) => member.agent_id === agent.agent_id),
    ),
  ];

  const handle_open_member_list = async () => {
    await on_open_member_manager();
    set_is_member_list_open(true);
  };

  const conversation_tabs = (
    <WorkspaceConversationTabs
      conversations={conversations}
      conversation_id={conversation_id}
      on_create_conversation={on_create_conversation}
      on_select_conversation={on_select_conversation}
      tour_anchor={CONVERSATION_TOUR_ANCHORS.session_switcher}
    />
  );

  const trailing = (
    <div className="flex items-center gap-2">
      <div className="hidden lg:flex">
        <MemberAvatarStack
          on_click={() => {
            void handle_open_member_list();
          }}
          room_members={room_members}
          tour_anchor={CONVERSATION_TOUR_ANCHORS.member_manage}
        />
      </div>
      {on_replay_tour ? (
        <WorkspaceSurfaceToolbarAction onClick={on_replay_tour}>
          <Compass className="h-3.5 w-3.5" />
          {t("common.view_guide")}
        </WorkspaceSurfaceToolbarAction>
      ) : null}
    </div>
  );

  return (
    <>
      <WorkspaceSurfaceHeader
        active_tab={active_tab}
        density="compact"
        leading={(
          <UiRoomAvatar
            avatar={room_avatar}
            class_name="h-full w-full rounded-full border-0 shadow-none"
            max_members={4}
            members={room_members.map((member) => ({
              avatar: member.avatar,
              id: member.agent_id,
              name: member.name,
            }))}
            room_id={room_id}
            title={header_title}
          />
        )}
        on_change_tab={on_change_tab}
        tabs={room_tabs}
        tabs_leading={conversation_tabs}
        tabs_trailing={<WorkspaceTaskStrip todos={todos} />}
        title={header_title}
        trailing={trailing}
      />

      <CreateRoomDialog
        agents={all_room_agents}
        confirm_label={t("common.save")}
        dialog_subtitle={t("room.manage_dialog_subtitle")}
        dialog_title={t("room.manage_dialog_title")}
        initial_avatar={room_avatar ?? ""}
        initial_host_agent_id={room_host_agent_id ?? null}
        initial_host_auto_reply_enabled={room_host_auto_reply_enabled}
        initial_name={header_title}
        initial_selected_agent_ids={member_agent_ids}
        initial_room_skill_names={room_skill_names}
        is_open={is_member_list_open}
        mode="manage"
        on_cancel={() => set_is_member_list_open(false)}
        on_confirm={async (next_agent_ids, name, avatar, skill_names, host_agent_id, host_auto_reply_enabled) => {
          if (!room_id) {
            return;
          }

          const next_agent_id_set = new Set(next_agent_ids);
          const current_agent_id_set = new Set(member_agent_ids);
          const agent_ids_to_add = next_agent_ids.filter((agent_id) => !current_agent_id_set.has(agent_id));
          const agent_ids_to_remove = member_agent_ids.filter((agent_id) => !next_agent_id_set.has(agent_id));

          for (const agent_id of agent_ids_to_add) {
            await on_add_room_member(agent_id);
          }

          await on_update_room(room_id, {
            name,
            avatar,
            skill_names,
            host_agent_id,
            host_auto_reply_enabled,
          });

          for (const agent_id of agent_ids_to_remove) {
            await on_remove_room_member(agent_id);
          }

          set_is_member_list_open(false);
        }}
      />
    </>
  );
});

GroupConversationHeaderView.displayName = "GroupConversationHeaderView";

export function GroupConversationHeader(props: GroupConversationHeaderProps) {
  return <GroupConversationHeaderView {...props} />;
}
