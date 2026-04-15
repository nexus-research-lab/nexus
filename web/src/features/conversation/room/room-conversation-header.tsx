"use client";

import { memo, useState } from "react";
import {
  FolderTree,
  Hash,
  History,
  Info,
  MessageSquare,
} from "lucide-react";

import { get_icon_avatar_src, get_initials, get_room_avatar_icon_id } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import { WorkspaceSurfaceHeader, WorkspaceTaskStrip } from "@/shared/ui/workspace/workspace-surface-header";
import { WorkspaceConversationSwitcher } from "@/shared/ui/workspace/workspace-conversation-switcher";
import { Agent } from "@/types/agent";
import { RoomConversationView } from "@/types/conversation";
import { UpdateRoomParams } from "@/types/room";
import { RoomSurfaceTabKey } from "@/types/room-surface";
import { TodoItem } from "@/types/todo";

import { CreateRoomDialog } from "@/features/conversation/room-members/create-room-dialog";

interface RoomConversationHeaderProps {
  conversation_id: string | null;
  room_id: string | null;
  current_room_title: string | null;
  room_avatar?: string | null;
  conversations: RoomConversationView[];
  room_members: Agent[];
  available_room_agents: Agent[];
  todos: TodoItem[];
  active_tab: RoomSurfaceTabKey;
  on_change_tab: (tab: RoomSurfaceTabKey) => void;
  on_select_conversation: (conversation_id: string) => void;
  on_create_conversation?: (title?: string) => Promise<string | null>;
  on_add_room_member: (agent_id: string) => Promise<void>;
  on_remove_room_member: (agent_id: string) => Promise<void>;
  on_update_room: (room_id: string, params: UpdateRoomParams) => Promise<void>;
}

function MemberAvatarStack({
  room_members,
  on_click,
}: {
  room_members: Agent[];
  on_click: () => void;
}) {
  const { t } = useI18n();
  const visible_members = room_members.slice(0, 4);
  const overflow_count = Math.max(0, room_members.length - visible_members.length);

  return (
    <button
      className="flex h-7 items-center gap-1.5 rounded-full border border-(--divider-subtle-color) bg-(--surface-panel-background) px-2 text-[10.5px] font-medium text-(--text-default) transition-[border-color,background,color,transform] duration-(--motion-duration-fast) hover:-translate-y-[1px] hover:border-(--surface-interactive-hover-border) hover:text-(--text-strong)"
      onClick={on_click}
      type="button"
    >
      <div className="flex items-center -space-x-1.5">
        {visible_members.map((member) => {
          const avatar_src = get_icon_avatar_src(member.avatar);
          return (
            <span
              className="flex h-5.5 w-5.5 items-center justify-center overflow-hidden rounded-full border border-(--surface-avatar-border) bg-(--surface-avatar-background) text-[8px] font-bold text-(--text-strong) shadow-(--surface-avatar-shadow)"
              key={member.agent_id}
              title={member.name}
            >
              {avatar_src ? (
                <img
                  alt={member.name}
                  className="h-full w-full object-cover"
                  src={avatar_src}
                />
              ) : (
                get_initials(member.name)
              )}
            </span>
          );
        })}
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

const RoomConversationHeaderView = memo(({
  conversation_id,
  room_id,
  current_room_title,
  room_avatar,
  conversations,
  room_members,
  available_room_agents,
  todos,
  active_tab,
  on_change_tab,
  on_select_conversation,
  on_create_conversation,
  on_add_room_member,
  on_remove_room_member,
  on_update_room,
}: RoomConversationHeaderProps) => {
  const { t } = useI18n();
  const [is_member_list_open, set_is_member_list_open] = useState(false);
  const header_title = current_room_title?.trim() || t("room.untitled_collaboration");
  const room_tabs: { key: RoomSurfaceTabKey; label: string; icon: typeof MessageSquare }[] = [
    { key: "chat", label: t("room.chat"), icon: MessageSquare },
    { key: "history", label: t("room.history"), icon: History },
    { key: "workspace", label: t("room.workspace"), icon: FolderTree },
    { key: "about", label: t("room.about"), icon: Info },
  ];

  const resolved_room_avatar_id = get_room_avatar_icon_id(room_id, header_title, room_avatar);
  const room_avatar_src = get_icon_avatar_src(resolved_room_avatar_id);

  const member_agent_ids = room_members.map((member) => member.agent_id);
  const all_room_agents = [
    ...room_members,
    ...available_room_agents.filter(
      (agent) => !room_members.some((member) => member.agent_id === agent.agent_id),
    ),
  ];

  const title_trailing = (
    <WorkspaceConversationSwitcher
      conversations={conversations}
      conversation_id={conversation_id}
      density="compact"
      on_create_conversation={on_create_conversation}
      on_select_conversation={on_select_conversation}
      on_view_history={() => on_change_tab("history")}
    />
  );

  const trailing = (
    <>
      <div className="hidden lg:flex">
        <MemberAvatarStack
          on_click={() => set_is_member_list_open(true)}
          room_members={room_members}
        />
      </div>
    </>
  );

  return (
    <>
      <WorkspaceSurfaceHeader
        active_tab={active_tab}
        density="compact"
        leading={room_avatar_src ? (
          <img
            alt={header_title}
            className="h-5 w-5 rounded-[6px] object-contain"
            src={room_avatar_src}
          />
        ) : (
          <Hash size={14} className="text-(--icon-default)" />
        )}
        on_change_tab={on_change_tab}
        tabs={room_tabs}
        tabs_trailing={<WorkspaceTaskStrip todos={todos} />}
        title={header_title}
        title_trailing={title_trailing}
        trailing={trailing}
      />

      <CreateRoomDialog
        agents={all_room_agents}
        confirm_label={t("common.save")}
        dialog_subtitle={t("room.manage_dialog_subtitle")}
        dialog_title={t("room.manage_dialog_title")}
        initial_avatar={room_avatar ?? ""}
        initial_name={header_title}
        initial_selected_agent_ids={member_agent_ids}
        is_open={is_member_list_open}
        mode="manage"
        on_cancel={() => set_is_member_list_open(false)}
        on_confirm={async (next_agent_ids, name, avatar) => {
          if (!room_id) {
            return;
          }

          const next_agent_id_set = new Set(next_agent_ids);
          const current_agent_id_set = new Set(member_agent_ids);
          const agent_ids_to_add = next_agent_ids.filter((agent_id) => !current_agent_id_set.has(agent_id));
          const agent_ids_to_remove = member_agent_ids.filter((agent_id) => !next_agent_id_set.has(agent_id));

          await on_update_room(room_id, {
            name,
            avatar,
          });

          for (const agent_id of agent_ids_to_add) {
            await on_add_room_member(agent_id);
          }

          for (const agent_id of agent_ids_to_remove) {
            await on_remove_room_member(agent_id);
          }

          set_is_member_list_open(false);
        }}
      />
    </>
  );
});

RoomConversationHeaderView.displayName = "RoomConversationHeaderView";

export function RoomConversationHeader(props: RoomConversationHeaderProps) {
  return <RoomConversationHeaderView {...props} />;
}
