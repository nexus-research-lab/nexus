"use client";

import { Bot, Settings, Trash2, UserPlus } from "lucide-react";
import { useState } from "react";

import { ConfirmDialog } from "@/shared/ui/dialog/confirm-dialog";
import { WorkspaceInspectorSection } from "@/shared/ui/workspace/workspace-inspector-section";
import { WorkspacePillButton } from "@/shared/ui/workspace/workspace-pill-button";
import { Agent } from "@/types/agent";
import { Conversation } from "@/types/conversation";
import { TodoItem } from "@/types/todo";
import { UpdateRoomParams } from "@/types/room";

import { CollaborationStatusSection } from "./collaboration-status-section";
import { MemberSummaryCard } from "./member-summary-card";
import { RoomMemberPickerDialog } from "@/features/room-members/room-member-picker-dialog";
import { ProgressSection } from "./progress-section";
import { RoomSettingsPanel } from "@/features/room-conversation/room-settings-panel";

interface RoomContextPanelProps {
  agent: Agent;
  available_room_agents: Agent[];
  current_agent_id: string | null;
  current_room_type: string;
  room_id: string | null;
  room_name: string;
  room_description: string;
  room_members: Agent[];
  room_conversations: Conversation[];
  active_conversation: Conversation | null;
  todos: TodoItem[];
  is_conversation_busy: boolean;
  on_add_room_member: (agent_id: string) => Promise<void>;
  on_edit_agent: (agent_id: string) => void;
  on_remove_room_member: (agent_id: string) => Promise<void>;
  on_update_room: (room_id: string, params: UpdateRoomParams) => Promise<void>;
  on_delete_room: () => Promise<void>;
  on_select_agent: (agent_id: string) => void;
}

export function RoomContextPanel({
  agent,
  available_room_agents,
  current_agent_id,
  current_room_type,
  room_id,
  room_name,
  room_description,
  room_members,
  room_conversations,
  active_conversation,
  todos,
  is_conversation_busy,
  on_add_room_member,
  on_edit_agent,
  on_remove_room_member,
  on_update_room,
  on_delete_room,
  on_select_agent,
}: RoomContextPanelProps) {
  const [is_member_picker_open, set_is_member_picker_open] = useState(false);
  const [is_settings_open, set_is_settings_open] = useState(false);
  const [pending_remove_agent_id, set_pending_remove_agent_id] = useState<string | null>(null);
  const runtime_status = is_conversation_busy ? "Running" : active_conversation?.is_active === false ? "Idle" : "Active";
  const localized_runtime_status =
    runtime_status === "Running" ? "协作中" : runtime_status === "Idle" ? "待命" : "在线";
  const pending_remove_agent =
    room_members.find((member) => member.agent_id === pending_remove_agent_id) ?? null;

  const handle_update_room = async (room_id: string, params: UpdateRoomParams) => {
    await on_update_room(room_id, params);
    set_is_settings_open(false);
  };

  const handle_delete_room = async () => {
    await on_delete_room();
    set_is_settings_open(false);
  };

  return (
    <>
      <>
        {current_room_type === "room" ? (
          <WorkspaceInspectorSection
            action={(
              <div className="flex gap-1.5">
                <WorkspacePillButton
                  onClick={() => set_is_settings_open(true)}
                  size="sm"
                  variant="default"
                >
                  <Settings className="h-3.5 w-3.5" />
                </WorkspacePillButton>
                <WorkspacePillButton
                  onClick={() => set_is_member_picker_open(true)}
                  size="sm"
                >
                  <UserPlus className="h-3.5 w-3.5" />
                  添加
                </WorkspacePillButton>
              </div>
            )}
            icon={Bot}
            title="Members"
          >
            <div className="space-y-1.5">
              {room_members.map((member) => {
                const is_active = member.agent_id === current_agent_id;
                return (
                  <div
                    key={member.agent_id}
                    className={`group flex items-center gap-2 rounded-[16px] px-1 py-1 transition-all duration-300 ${
                      is_active ? "bg-white/14" : "hover:bg-white/10"
                    }`}
                  >
                    <button
                      className="flex min-w-0 flex-1 items-center gap-3 rounded-[14px] px-2 py-1 text-left"
                      onClick={() => on_select_agent(member.agent_id)}
                      type="button"
                    >
                      <div className="home-glass-pill flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-slate-900/76">
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
                    </button>
                    <button
                      aria-label={`移除 ${member.name}`}
                      className="shrink-0 rounded-xl p-1.5 text-slate-700/54 opacity-0 transition-all group-hover:opacity-100 hover:bg-white/10 hover:text-destructive"
                      onClick={() => set_pending_remove_agent_id(member.agent_id)}
                      type="button"
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </button>
                  </div>
                );
              })}
            </div>
          </WorkspaceInspectorSection>
        ) : null}

        <MemberSummaryCard
          agent={agent}
          localized_runtime_status={localized_runtime_status}
          on_edit_agent={on_edit_agent}
          runtime_status={runtime_status}
        />

        <CollaborationStatusSection
          active_conversation={active_conversation}
          localized_runtime_status={localized_runtime_status}
          total_member_count={room_members.length}
        />

        <ProgressSection todos={todos} />
      </>

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
