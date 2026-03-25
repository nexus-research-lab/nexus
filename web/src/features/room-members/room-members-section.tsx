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
      <section className="border-t workspace-divider px-2 py-5">
        <div className="mb-3 flex items-center justify-between gap-2">
          <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.16em] text-slate-700/56">
            <Bot className="h-3.5 w-3.5" />
            伙伴
          </div>
          {can_manage_members ? (
            <button
              className="inline-flex items-center gap-1.5 rounded-full bg-[linear-gradient(135deg,rgba(174,163,255,0.18),rgba(255,255,255,0.82))] px-3 py-1.5 text-[11px] font-semibold text-slate-900/82 shadow-[0_10px_20px_rgba(133,119,255,0.10)]"
              onClick={() => set_is_member_picker_open(true)}
              type="button"
            >
              <UserPlus className="h-3.5 w-3.5" />
              添加成员
            </button>
          ) : null}
        </div>

        <div className="space-y-2">
          {members.map((member) => {
            const is_current_agent = member.agent_id === current_agent_id;
            return (
              <button
                key={member.agent_id}
                className={cn(
                  "group flex w-full items-center gap-3 rounded-[20px] px-3 py-3 text-left transition-all duration-300",
                  is_current_agent ? "workspace-card-strong" : "workspace-card hover:-translate-y-0.5",
                )}
                onClick={() => on_select_agent(member.agent_id)}
                type="button"
              >
                <div className="workspace-chip flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-[11px] font-bold text-slate-900/82">
                  {getInitials(member.name)}
                </div>
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-semibold text-slate-900/84">
                    {member.name}
                  </p>
                  <p className="truncate text-[11px] text-slate-700/50">
                    {is_current_agent ? "当前正在这个 room 中协作" : "当前已加入这个 room"}
                  </p>
                </div>
                <span className={cn(
                  "h-2.5 w-2.5 shrink-0 rounded-full",
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
