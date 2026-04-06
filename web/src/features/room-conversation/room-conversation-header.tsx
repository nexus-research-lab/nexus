"use client";

import { memo, useState } from "react";
import {
  FolderTree,
  Hash,
  History,
  MessageSquare,
  Settings,
  UserPlus,
} from "lucide-react";

import {
  WorkspaceSurfaceHeader,
  WorkspaceTaskStrip,
} from "@/shared/ui/workspace/workspace-surface-header";
import { useI18n } from "@/shared/i18n/i18n-context";
import { WorkspaceConversationSwitcher } from "@/shared/ui/workspace/workspace-conversation-switcher";
import { WorkspacePillButton } from "@/shared/ui/workspace/workspace-pill-button";
import { WorkspaceStatusBadge } from "@/shared/ui/workspace/workspace-status-badge";
import { WorkspaceIconFrame } from "@/shared/ui/workspace/workspace-catalog-card";
import { Agent } from "@/types/agent";
import { RoomConversationView } from "@/types/conversation";
import { RoomSurfaceTabKey } from "@/types/room-surface";
import { TodoItem } from "@/types/todo";
import { UpdateRoomParams } from "@/types/room";

import { RoomMemberPickerDialog } from "@/features/room-members/room-member-picker-dialog";
import { RoomSettingsPanel } from "./room-settings-panel";

interface RoomConversationHeaderProps {
  conversation_id: string | null;
  room_id: string | null;
  current_room_title: string | null;
  room_description: string;
  conversations: RoomConversationView[];
  is_loading: boolean;
  room_members: Agent[];
  available_room_agents: Agent[];
  todos: TodoItem[];
  active_tab: RoomSurfaceTabKey;
  on_change_tab: (tab: RoomSurfaceTabKey) => void;
  on_select_conversation: (conversation_id: string) => void;
  on_create_conversation?: (title?: string) => Promise<string | null>;
  on_add_room_member: (agent_id: string) => Promise<void>;
  on_update_room: (room_id: string, params: UpdateRoomParams) => Promise<void>;
  on_delete_room: () => Promise<void>;
}

/** 获取名称首字母缩写 */
function getInitials(name: string | null): string {
  if (!name) return "AG";
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return "AG";
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
  return `${parts[0][0] ?? ""}${parts[1][0] ?? ""}`.toUpperCase();
}

/** 成员头像堆叠组件 */
function MemberAvatarStack({
  room_members,
}: {
  room_members: Agent[];
}) {
  const { t } = useI18n();
  const MAX_VISIBLE = 5;
  const visible_members = room_members.slice(0, MAX_VISIBLE);
  const overflow_count = room_members.length - MAX_VISIBLE;

  return (
    <div
      className="flex items-center rounded-full px-[7px] py-[3px]"
      style={{
        background: "var(--chip-default-background)",
        border: "1px solid var(--chip-default-border)",
      }}
    >
      <div className="ml-1 flex items-center gap-0">
        <WorkspaceIconFrame
          class_name="ml-0 h-[27px] w-[27px] rounded-full text-[8px] font-bold text-slate-900/86"
          shape="round"
          size="sm"
        >
          {t("room.you")}
        </WorkspaceIconFrame>
        {visible_members.map((member) => (
          <WorkspaceIconFrame
            key={member.agent_id}
            class_name="-ml-[6px] h-[27px] w-[27px] rounded-full text-[8px] font-bold"
            shape="round"
            size="sm"
            title={member.name}
          >
            {getInitials(member.name)}
          </WorkspaceIconFrame>
        ))}
        {overflow_count > 0 ? (
          <WorkspaceIconFrame
            class_name="-ml-[6px] h-[27px] w-[27px] rounded-full text-[8px] font-semibold"
            shape="round"
            size="sm"
          >
            +{overflow_count}
          </WorkspaceIconFrame>
        ) : null}
      </div>
    </div>
  );
}

function RoomHeaderActions({
  available_room_agents,
  room_id,
  room_name,
  room_description,
  on_add_room_member,
  on_update_room,
  on_delete_room,
}: {
  available_room_agents: Agent[];
  room_id: string | null;
  room_name: string;
  room_description: string;
  on_add_room_member: (agent_id: string) => Promise<void>;
  on_update_room: (room_id: string, params: UpdateRoomParams) => Promise<void>;
  on_delete_room: () => Promise<void>;
}) {
  const { t } = useI18n();
  const [is_member_picker_open, set_is_member_picker_open] = useState(false);
  const [is_settings_open, set_is_settings_open] = useState(false);

  const handle_update_room = async (next_room_id: string, params: UpdateRoomParams) => {
    await on_update_room(next_room_id, params);
    set_is_settings_open(false);
  };

  const handle_delete_room = async () => {
    await on_delete_room();
    set_is_settings_open(false);
  };

  return (
    <>
      <div className="hidden items-center gap-2 lg:flex">
        <WorkspacePillButton
          aria-label={t("room.add_member")}
          onClick={() => set_is_member_picker_open(true)}
          density="compact"
          size="icon"
          variant="icon"
          title={t("room.add_member")}
        >
          <UserPlus className="h-4 w-4" />
        </WorkspacePillButton>
        <WorkspacePillButton
          aria-label={t("room.settings")}
          onClick={() => set_is_settings_open(true)}
          density="compact"
          size="icon"
          variant="icon"
          title={t("room.settings")}
        >
          <Settings className="h-4 w-4" />
        </WorkspacePillButton>
      </div>

      <RoomMemberPickerDialog
        agents={available_room_agents}
        is_open={is_member_picker_open}
        on_cancel={() => set_is_member_picker_open(false)}
        on_select={(agent_id) => {
          void on_add_room_member(agent_id);
          set_is_member_picker_open(false);
        }}
      />

      <RoomSettingsPanel
        is_open={is_settings_open}
        room_id={room_id}
        room_name={room_name}
        room_description={room_description}
        on_update_room={handle_update_room}
        on_delete_room={handle_delete_room}
        on_close={() => set_is_settings_open(false)}
      />
    </>
  );
}

const RoomConversationHeaderView = memo(({
  conversation_id,
  room_id,
  current_room_title,
  room_description,
  conversations,
  is_loading,
  room_members,
  available_room_agents,
  todos,
  active_tab,
  on_change_tab,
  on_select_conversation,
  on_create_conversation,
  on_add_room_member,
  on_update_room,
  on_delete_room,
}: RoomConversationHeaderProps) => {
  const { t } = useI18n();
  const header_title = current_room_title?.trim() || t("room.untitled_collaboration");
  const room_tabs: { key: RoomSurfaceTabKey; label: string; icon: typeof MessageSquare }[] = [
    { key: "chat", label: t("room.chat"), icon: MessageSquare },
    { key: "history", label: t("room.history"), icon: History },
    { key: "workspace", label: t("room.workspace"), icon: FolderTree },
  ];

  const title_trailing = (
    <WorkspaceConversationSwitcher
      conversations={conversations}
      conversation_id={conversation_id}
      density="compact"
      on_select_conversation={on_select_conversation}
      on_create_conversation={on_create_conversation}
    />
  );

  const trailing = (
    <>
      <div className="hidden lg:flex">
        <MemberAvatarStack room_members={room_members} />
      </div>
      <RoomHeaderActions
        available_room_agents={available_room_agents}
        on_add_room_member={on_add_room_member}
        on_delete_room={on_delete_room}
        on_update_room={on_update_room}
        room_description={room_description}
        room_id={room_id}
        room_name={header_title}
      />
      <WorkspaceStatusBadge
        icon={<span className="text-current">●</span>}
        label={is_loading ? t("status.collaborating") : t("status.online")}
        size="compact"
        tone={is_loading ? "running" : "active"}
      />
    </>
  );

  return (
    <WorkspaceSurfaceHeader
      active_tab={active_tab}
      badge="ROOM"
      density="compact"
      leading={<Hash size={14} className="text-slate-800/72" />}
      on_change_tab={on_change_tab}
      tabs_trailing={<WorkspaceTaskStrip todos={todos} />}
      tabs={room_tabs}
      title={header_title}
      title_trailing={title_trailing}
      trailing={trailing}
    />
  );
});

RoomConversationHeaderView.displayName = "RoomConversationHeaderView";

export function RoomConversationHeader(props: RoomConversationHeaderProps) {
  return <RoomConversationHeaderView {...props} />;
}
