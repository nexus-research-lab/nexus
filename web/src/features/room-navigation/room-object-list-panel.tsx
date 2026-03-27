"use client";

import { useMemo, useState } from "react";
import {
  Clock3,
  MessageCircleMore,
  Trash2,
  Waypoints,
} from "lucide-react";

import { cn, formatRelativeTime, truncate } from "@/lib/utils";
import { ConfirmDialog, PromptDialog } from "@/shared/ui/confirm-dialog";
import { WorkspacePillButton } from "@/shared/ui/workspace-pill-button";
import { WorkspaceSidebarItem } from "@/shared/ui/workspace-sidebar-item";
import { WorkspaceSidebarShell } from "@/shared/ui/workspace-sidebar-shell";
import { Agent } from "@/types/agent";
import { Conversation } from "@/types/conversation";
import { RoomAggregate } from "@/types/room";

type RoomObjectSpace = "dm" | "room";

interface RoomObjectListPanelProps {
  active_space: RoomObjectSpace;
  agents: Agent[];
  current_room_id: string | null;
  current_room_title: string;
  conversations: Conversation[];
  rooms: RoomAggregate[];
  on_delete_room: () => Promise<void>;
  on_open_contacts: () => void;
  on_open_room: (room_id: string) => void;
  on_update_room: (params: {name?: string; description?: string; title?: string}) => Promise<void>;
}

interface RoomListItem {
  room_id: string;
  room_name: string;
  room_subtitle: string;
  last_activity_at: number;
  member_count: number;
}

function getDmTitle(room: RoomAggregate, agents: Agent[]) {
  const member_agent_id = room.members.find((member) => member.member_type === "agent")?.member_agent_id;
  const matched_agent = agents.find((agent) => agent.agent_id === member_agent_id);
  return matched_agent?.name || room.room.name?.trim() || "未命名 DM";
}

export function RoomObjectListPanel({
  active_space,
  agents,
  current_room_id,
  current_room_title,
  conversations,
  rooms,
  on_delete_room,
  on_open_contacts,
  on_open_room,
  on_update_room,
}: RoomObjectListPanelProps) {
  const [is_delete_dialog_open, set_is_delete_dialog_open] = useState(false);
  const [is_rename_dialog_open, set_is_rename_dialog_open] = useState(false);

  const last_activity_by_room = useMemo(() => {
    const activity_map = new Map<string, number>();

    conversations.forEach((conversation) => {
      if (!conversation.room_id) {
        return;
      }
      const previous_timestamp = activity_map.get(conversation.room_id) ?? 0;
      if (conversation.last_activity_at > previous_timestamp) {
        activity_map.set(conversation.room_id, conversation.last_activity_at);
      }
    });

    return activity_map;
  }, [conversations]);

  const room_items = useMemo<RoomListItem[]>(() => {
    return rooms
      .filter((room) => (
        active_space === "dm" ? room.room.room_type === "dm" : room.room.room_type !== "dm"
      ))
      .map((room) => {
        const member_count = room.members.filter((member) => member.member_type === "agent").length;
        const room_name = active_space === "dm"
          ? getDmTitle(room, agents)
          : room.room.name?.trim() || "未命名协作";

        return {
          room_id: room.room.id,
          room_name,
          room_subtitle: active_space === "dm" ? "1v1 协作" : `${member_count} 位成员`,
          last_activity_at: last_activity_by_room.get(room.room.id) ?? 0,
          member_count,
        };
      })
      .sort((left, right) => right.last_activity_at - left.last_activity_at);
  }, [active_space, agents, last_activity_by_room, rooms]);

  return (
    <>
      <WorkspaceSidebarShell
        empty_state={!room_items.length ? (
          <div className="workspace-card rounded-[18px] px-4 py-4 text-sm leading-6 text-slate-700/60">
            {active_space === "dm"
              ? "还没有可打开的直接协作。"
              : "还没有可切换的协作空间。"}
          </div>
        ) : null}
        header_action={active_space === "room" ? (
          <WorkspacePillButton
            onClick={() => set_is_delete_dialog_open(true)}
            size="sm"
          >
            <Trash2 className="h-3.5 w-3.5" />
            删除
          </WorkspacePillButton>
        ) : null}
        title={active_space === "dm" ? "Direct Messages" : "Rooms"}
      >
        {room_items.map((room) => {
          const is_active = room.room_id === current_room_id;
          return (
            <WorkspaceSidebarItem
              key={room.room_id}
              icon={active_space === "dm"
                ? <MessageCircleMore className="h-3.5 w-3.5" />
                : <Waypoints className="h-3.5 w-3.5" />}
              is_active={is_active}
              meta={(
                <div className="flex items-center gap-2 text-[10px] text-slate-700/44">
                  <Clock3 className="h-3.5 w-3.5" />
                  <span>
                    {room.last_activity_at > 0 ? formatRelativeTime(room.last_activity_at) : "刚刚创建"}
                  </span>
                </div>
              )}
              on_click={() => on_open_room(room.room_id)}
              subtitle={room.room_subtitle}
              title={truncate(room.room_name, 22)}
            />
          );
        })}
      </WorkspaceSidebarShell>

      <PromptDialog
        default_value={current_room_title}
        is_open={is_rename_dialog_open}
        message="输入新的协作名称"
        on_cancel={() => set_is_rename_dialog_open(false)}
        on_confirm={(name) => {
          const next_name = name.trim();
          if (next_name) {
            void on_update_room({name: next_name});
          }
          set_is_rename_dialog_open(false);
        }}
        placeholder="为这个协作命名"
        title="重命名协作"
      />

      <ConfirmDialog
        cancel_text="取消"
        confirm_text="删除"
        is_open={is_delete_dialog_open}
        message={`确定要删除协作「${current_room_title}」吗？删除后无法恢复。`}
        on_cancel={() => set_is_delete_dialog_open(false)}
        on_confirm={() => {
          void on_delete_room();
          set_is_delete_dialog_open(false);
        }}
        title="删除协作"
        variant="danger"
      />
    </>
  );
}
