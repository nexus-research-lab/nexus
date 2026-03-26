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
          room_subtitle: active_space === "dm" ? "1v1 协作" : `${member_count} 位成员`,
          last_activity_at: last_activity_by_room.get(room.room.id) ?? 0,
          member_count,
        };
      })
      .sort((left, right) => right.last_activity_at - left.last_activity_at);
  }, [active_space, agents, last_activity_by_room, rooms]);

  return (
    <>
      <aside className={cn(
        "hidden min-h-0 shrink-0 border-r border-white/10 bg-[linear-gradient(180deg,rgba(57,70,103,0.96),rgba(41,51,81,0.94))] text-white shadow-[inset_-1px_0_0_rgba(255,255,255,0.06)] lg:flex lg:flex-col",
        HOME_WORKSPACE_OBJECT_LIST_WIDTH_CLASS,
      )}>
        <div className="border-b border-white/8 px-3.5 pb-2.5 pt-3">
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0">
              <p className="text-[10px] font-semibold uppercase tracking-[0.22em] text-white/36">
                {active_space === "dm" ? "DMs" : "Rooms"}
              </p>
              <div className="mt-1 flex items-center gap-2">
                <p className="text-[16px] font-black tracking-[-0.04em] text-white/96">
                  {active_space === "dm" ? "Direct Messages" : "Rooms"}
                </p>
                <span className="rounded-full bg-white/10 px-2 py-0.5 text-[10px] font-semibold text-white/52">
                  {room_items.length}
                </span>
              </div>
              <p className="mt-1 text-[11px] text-white/44">
                {room_items.length} 个{active_space === "dm" ? "DM" : "协作空间"}
              </p>
            </div>

            {active_space === "dm" ? (
              <button
                className="inline-flex items-center gap-1.5 rounded-full border border-white/14 bg-white/10 px-3 py-1.5 text-[11px] font-semibold text-white/88 transition hover:bg-white/14"
                onClick={on_open_contacts}
                type="button"
              >
                <Users className="h-3.5 w-3.5" />
                成员网络
              </button>
            ) : (
              <button
                className="inline-flex items-center gap-1.5 rounded-full border border-white/14 bg-white/10 px-3 py-1.5 text-[11px] font-semibold text-white/88 transition hover:bg-white/14"
                onClick={() => set_is_rename_dialog_open(true)}
                type="button"
              >
                <Pencil className="h-3.5 w-3.5" />
                改名
              </button>
            )}
          </div>

          {active_space === "room" ? (
            <div className="mt-2 flex items-center gap-2">
              <button
                className="inline-flex items-center gap-1.5 rounded-full border border-white/14 bg-white/10 px-3 py-1.5 text-[11px] font-semibold text-white/88 transition hover:bg-white/14"
                onClick={() => set_is_delete_dialog_open(true)}
                type="button"
              >
                <Trash2 className="h-3.5 w-3.5" />
                删除
              </button>
            </div>
          ) : null}
        </div>

        <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto px-2 py-2">
          <div className="space-y-1.5">
            {room_items.map((room) => {
              const is_active = room.room_id === current_room_id;
              return (
                <button
                  key={room.room_id}
                  className={cn(
                    "group flex w-full items-start gap-3 rounded-[14px] px-3 py-2 text-left transition-all duration-300",
                    is_active
                      ? "border border-white/18 bg-white/16 shadow-[0_10px_18px_rgba(8,12,24,0.22)]"
                      : "border border-transparent hover:bg-white/10",
                  )}
                  onClick={() => on_open_room(room.room_id)}
                  type="button"
                >
                  <div className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-full border border-white/10 bg-white/8 text-white/84">
                    {active_space === "dm" ? (
                      <MessageCircleMore className="h-3.5 w-3.5" />
                    ) : (
                      <Waypoints className="h-3.5 w-3.5" />
                    )}
                  </div>

                  <div className="min-w-0 flex-1">
                    <p className="truncate text-[13px] font-semibold text-white/94">
                      {truncate(room.room_name, 22)}
                    </p>
                    <p className="mt-0.5 text-[11px] text-white/56">{room.room_subtitle}</p>
                    <div className="mt-1.5 flex items-center gap-2 text-[10px] text-white/42">
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
              <div className="rounded-[16px] border border-white/10 bg-white/8 px-4 py-4 text-sm leading-6 text-white/62">
                {active_space === "dm"
                  ? "还没有可打开的直接协作。"
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
