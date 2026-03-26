"use client";

import { useMemo, useState } from "react";
import {
  Clock3,
  MessageCircleMore,
  Pencil,
  Trash2,
  Users,
  Waypoints,
} from "lucide-react";

import { HOME_WORKSPACE_OBJECT_LIST_WIDTH_CLASS } from "@/lib/home-layout";
import { cn, formatRelativeTime, truncate } from "@/lib/utils";
import { ConfirmDialog, PromptDialog } from "@/shared/ui/confirm-dialog";
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
          room_subtitle: active_space === "dm" ? "直接协作" : `${member_count} 位成员`,
          last_activity_at: last_activity_by_room.get(room.room.id) ?? 0,
          member_count,
        };
      })
      .sort((left, right) => right.last_activity_at - left.last_activity_at);
  }, [active_space, agents, last_activity_by_room, rooms]);

  return (
    <>
      <aside className={cn(
        "hidden min-h-0 shrink-0 border-r border-white/18 bg-white/8 lg:flex lg:flex-col",
        HOME_WORKSPACE_OBJECT_LIST_WIDTH_CLASS,
      )}>
        <div className="px-4 pb-4 pt-4">
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0">
              <p className="text-[11px] font-semibold uppercase tracking-[0.22em] text-slate-700/44">
                {active_space === "dm" ? "DMs" : "Rooms"}
              </p>
              <p className="mt-1 text-[20px] font-black tracking-[-0.04em] text-slate-950/90">
                {active_space === "dm" ? "直接协作" : "协作空间"}
              </p>
              <p className="mt-1 text-[12px] text-slate-700/54">
                {room_items.length} 个{active_space === "dm" ? "对象" : "空间"}
              </p>
            </div>

            {active_space === "dm" ? (
              <button
                className="workspace-chip inline-flex items-center gap-1.5 rounded-full px-3 py-1.5 text-[11px] font-semibold text-slate-900/78"
                onClick={on_open_contacts}
                type="button"
              >
                <Users className="h-3.5 w-3.5" />
                成员
              </button>
            ) : (
              <button
                className="workspace-chip inline-flex items-center gap-1.5 rounded-full px-3 py-1.5 text-[11px] font-semibold text-slate-900/78"
                onClick={() => set_is_rename_dialog_open(true)}
                type="button"
              >
                <Pencil className="h-3.5 w-3.5" />
                改名
              </button>
            )}
          </div>

          {active_space === "room" ? (
            <div className="mt-3 flex items-center gap-2">
              <button
                className="workspace-chip inline-flex items-center gap-1.5 rounded-full px-3 py-1.5 text-[11px] font-semibold text-slate-900/78"
                onClick={() => set_is_delete_dialog_open(true)}
                type="button"
              >
                <Trash2 className="h-3.5 w-3.5" />
                删除
              </button>
            </div>
          ) : null}
        </div>

        <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto px-2 pb-4">
          <div className="space-y-1.5">
            {room_items.map((room) => {
              const is_active = room.room_id === current_room_id;
              return (
                <button
                  key={room.room_id}
                  className={cn(
                    "group flex w-full items-start gap-3 rounded-[18px] px-3 py-3 text-left transition-all duration-300",
                    is_active
                      ? "border border-white/28 bg-white/20 shadow-[0_14px_24px_rgba(111,126,162,0.08)]"
                      : "border border-transparent hover:bg-white/12",
                  )}
                  onClick={() => on_open_room(room.room_id)}
                  type="button"
                >
                  <div className="workspace-chip mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-slate-900/76">
                    {active_space === "dm" ? (
                      <MessageCircleMore className="h-4 w-4" />
                    ) : (
                      <Waypoints className="h-4 w-4" />
                    )}
                  </div>

                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-semibold text-slate-950/86">
                      {truncate(room.room_name, 22)}
                    </p>
                    <p className="mt-1 text-[12px] text-slate-700/56">{room.room_subtitle}</p>
                    <div className="mt-2 flex items-center gap-2 text-[11px] text-slate-700/48">
                      <Clock3 className="h-3.5 w-3.5" />
                      <span>
                        {room.last_activity_at > 0 ? formatRelativeTime(room.last_activity_at) : "刚刚创建"}
                      </span>
                    </div>
                  </div>
                </button>
              );
            })}

            {!room_items.length ? (
              <div className="workspace-card rounded-[22px] px-4 py-4 text-sm leading-6 text-slate-700/60">
                {active_space === "dm"
                  ? "还没有可直接协作的成员对话。"
                  : "还没有可切换的协作空间。"}
              </div>
            ) : null}
          </div>
        </div>
      </aside>

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
