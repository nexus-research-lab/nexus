"use client";

import { Bot, Trash2, UserPlus } from "lucide-react";
import { useState } from "react";

import { HOME_AGENT_INSPECTOR_WIDTH_CLASS } from "@/lib/home-layout";
import { ConfirmDialog } from "@/shared/ui/confirm-dialog";
import { Agent } from "@/types/agent";
import { Conversation } from "@/types/conversation";
import { TodoItem } from "@/types/todo";

import { RoomCollaborationStatusSection } from "./room-collaboration-status-section";
import { RoomMemberSummaryCard } from "./room-member-summary-card";
import { RoomMemberPickerDialog } from "../room-members/room-member-picker-dialog";
import { RoomProgressSection } from "./room-progress-section";

interface RoomContextPanelProps {
  agent: Agent;
  available_room_agents: Agent[];
  current_agent_id: string | null;
  current_room_type: string;
  room_name: string;
  room_members: Agent[];
  room_conversations: Conversation[];
  active_conversation: Conversation | null;
  todos: TodoItem[];
  is_conversation_busy: boolean;
  on_add_room_member: (agent_id: string) => Promise<void>;
  on_edit_agent: (agent_id: string) => void;
  on_remove_room_member: (agent_id: string) => Promise<void>;
  on_select_agent: (agent_id: string) => void;
}

export function RoomContextPanel({
  agent,
  available_room_agents,
  current_agent_id,
  current_room_type,
  room_name,
  room_members,
  room_conversations,
  active_conversation,
  todos,
  is_conversation_busy,
  on_add_room_member,
  on_edit_agent,
  on_remove_room_member,
  on_select_agent,
}: RoomContextPanelProps) {
  const [is_member_picker_open, set_is_member_picker_open] = useState(false);
  const [pending_remove_agent_id, set_pending_remove_agent_id] = useState<string | null>(null);
  const runtime_status = is_conversation_busy ? "Running" : active_conversation?.is_active === false ? "Idle" : "Active";
  const localized_runtime_status =
    runtime_status === "Running" ? "协作中" : runtime_status === "Idle" ? "待命" : "在线";
  const pending_remove_agent =
    room_members.find((member) => member.agent_id === pending_remove_agent_id) ?? null;

  return (
    <>
      <aside className={`flex min-h-0 flex-col bg-transparent ${HOME_AGENT_INSPECTOR_WIDTH_CLASS}`}>
        <div className="flex-1 overflow-y-auto scrollbar-hide">
          {current_room_type === "room" ? (
            <section className="border-b workspace-divider px-4 py-4">
              <div className="mb-3 flex items-center justify-between gap-2 text-xs font-semibold uppercase tracking-[0.16em] text-slate-700/56">
                <div className="flex items-center gap-2">
                  <Bot className="h-3.5 w-3.5" />
                  Members
                </div>
                <button
                  className="workspace-chip inline-flex items-center gap-1.5 rounded-full px-3 py-1.5 text-[11px] font-semibold text-slate-900/78"
                  onClick={() => set_is_member_picker_open(true)}
                  type="button"
                >
                  <UserPlus className="h-3.5 w-3.5" />
                  添加
                </button>
              </div>

              <div className="space-y-1.5">
                {room_members.map((member) => {
                  const is_active = member.agent_id === current_agent_id;
                  return (
                    <button
                      key={member.agent_id}
                      className={`group flex w-full items-center gap-3 rounded-[16px] px-3 py-2 text-left transition-all duration-300 ${
                        is_active ? "bg-white/20" : "hover:bg-white/12"
                      }`}
                      onClick={() => on_select_agent(member.agent_id)}
                      type="button"
                    >
                      <div className="workspace-chip flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-slate-900/76">
                        {member.name.slice(0, 2).toUpperCase()}
                      </div>
                      <div className="min-w-0 flex-1">
                        <p className="truncate text-[13px] font-semibold text-slate-900/82">
                          {member.name}
                        </p>
                        <p className="truncate text-[11px] text-slate-700/48">
                          {is_active ? "当前活跃" : "参与协作中"}
                        </p>
                      </div>
                      <span className={`h-2 w-2 shrink-0 rounded-full ${is_active ? "bg-emerald-400" : "bg-slate-300"}`} />
                      <button
                        aria-label={`移除 ${member.name}`}
                        className="workspace-chip rounded-xl p-1.5 text-slate-700/54 opacity-0 transition-all group-hover:opacity-100 hover:text-destructive"
                        onClick={(event) => {
                          event.stopPropagation();
                          set_pending_remove_agent_id(member.agent_id);
                        }}
                        type="button"
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </button>
                    </button>
                  );
                })}
              </div>
            </section>
          ) : null}

        <RoomMemberSummaryCard
          agent={agent}
          localized_runtime_status={localized_runtime_status}
          on_edit_agent={on_edit_agent}
          runtime_status={runtime_status}
        />

        <RoomCollaborationStatusSection
          active_conversation={active_conversation}
          localized_runtime_status={localized_runtime_status}
          total_member_count={room_members.length}
        />

        <RoomProgressSection todos={todos} />
        </div>
      </aside>

      <RoomMemberPickerDialog
        agents={available_room_agents}
        is_open={is_member_picker_open}
        on_cancel={() => set_is_member_picker_open(false)}
        on_select={(agent_id) => {
          void on_add_room_member(agent_id);
          set_is_member_picker_open(false);
        }}
      />

      <ConfirmDialog
        cancel_text="取消"
        confirm_text="移除"
        is_open={Boolean(pending_remove_agent)}
        message={`确定要把 ${pending_remove_agent?.name ?? ""} 移出当前 room 吗？`}
        on_cancel={() => set_pending_remove_agent_id(null)}
        on_confirm={() => {
          if (pending_remove_agent_id) {
            void on_remove_room_member(pending_remove_agent_id);
          }
          set_pending_remove_agent_id(null);
        }}
        title="移除成员"
        variant="danger"
      />
    </>
  );
}
