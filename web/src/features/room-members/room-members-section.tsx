import { useState } from "react";
import { Bot, Trash2, UserPlus } from "lucide-react";

import { Agent } from "@/types/agent";
import { cn } from "@/lib/utils";
import { ConfirmDialog } from "@/shared/ui/confirm-dialog";

import { RoomMemberPickerDialog } from "./room-member-picker-dialog";

interface RoomMembersSectionProps {
  can_manage_members?: boolean;
  current_agent_id: string | null;
  members: Agent[];
  available_agents: Agent[];
  on_add_member: (agent_id: string) => Promise<void>;
  on_remove_member: (agent_id: string) => Promise<void>;
  on_select_agent: (agent_id: string) => void;
}

function getInitials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) {
    return "AG";
  }
  if (parts.length === 1) {
    return parts[0].slice(0, 2).toUpperCase();
  }
  return `${parts[0][0] ?? ""}${parts[1][0] ?? ""}`.toUpperCase();
}

export function RoomMembersSection({
  can_manage_members = true,
  current_agent_id,
  members,
  available_agents,
  on_add_member,
  on_remove_member,
  on_select_agent,
}: RoomMembersSectionProps) {
  const [is_member_picker_open, set_is_member_picker_open] = useState(false);
  const [pending_remove_agent_id, set_pending_remove_agent_id] = useState<string | null>(null);
  const pending_remove_agent = members.find((member) => member.agent_id === pending_remove_agent_id) ?? null;

  return (
    <>
      <section className="px-3 py-4">
        <div className="mb-2 flex items-center justify-between gap-2">
          <div className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.18em] text-slate-700/50">
            <Bot className="h-3.5 w-3.5" />
            Members
          </div>
          {can_manage_members ? (
            <button
              className="workspace-chip inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-[11px] font-semibold text-slate-900/78"
              onClick={() => set_is_member_picker_open(true)}
              type="button"
            >
              <UserPlus className="h-3.5 w-3.5" />
              添加
            </button>
          ) : null}
        </div>

        <div className="space-y-1.5">
          {members.map((member) => {
            const is_current_agent = member.agent_id === current_agent_id;
            return (
              <button
                key={member.agent_id}
                className={cn(
                  "group flex w-full items-center gap-3 rounded-[16px] px-3 py-2 text-left transition-all duration-300",
                  is_current_agent ? "bg-white/20" : "hover:bg-white/12",
                )}
                onClick={() => on_select_agent(member.agent_id)}
                type="button"
              >
                <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full border border-white/26 bg-white/16 text-[11px] font-bold text-slate-900/82">
                  {getInitials(member.name)}
                </div>
                <div className="min-w-0 flex-1">
                  <p className="truncate text-[13px] font-semibold text-slate-900/82">
                    {member.name}
                  </p>
                  <p className="truncate text-[11px] text-slate-700/48">
                    {is_current_agent ? "当前活跃" : "参与协作中"}
                  </p>
                </div>
                <span className={cn(
                  "h-2 w-2 shrink-0 rounded-full",
                  is_current_agent ? "bg-emerald-400" : "bg-slate-300",
                )}/>
                {can_manage_members ? (
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
                ) : null}
              </button>
            );
          })}
        </div>
        <div className="mt-4 border-b workspace-divider" />
      </section>

      <RoomMemberPickerDialog
        agents={available_agents}
        is_open={is_member_picker_open}
        on_cancel={() => set_is_member_picker_open(false)}
        on_select={(agent_id) => {
          void on_add_member(agent_id);
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
            void on_remove_member(pending_remove_agent_id);
          }
          set_pending_remove_agent_id(null);
        }}
        title="移除成员"
        variant="danger"
      />
    </>
  );
}
