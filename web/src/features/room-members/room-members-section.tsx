import { Bot, MessageSquarePlus } from "lucide-react";

import { Agent } from "@/types/agent";
import { cn } from "@/lib/utils";

interface RoomMembersSectionProps {
  current_agent_id: string | null;
  members: Agent[];
  on_create_agent: () => void;
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
  current_agent_id,
  members,
  on_create_agent,
  on_select_agent,
}: RoomMembersSectionProps) {
  return (
    <section className="border-t workspace-divider px-5 py-5">
      <div className="mb-3 flex items-center justify-between gap-2">
        <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.16em] text-slate-700/56">
          <Bot className="h-3.5 w-3.5" />
          Members
        </div>
        <button
          className="inline-flex items-center gap-1.5 rounded-full bg-[linear-gradient(135deg,rgba(174,163,255,0.18),rgba(255,255,255,0.82))] px-3 py-1.5 text-[11px] font-semibold text-slate-900/82 shadow-[0_10px_20px_rgba(133,119,255,0.10)]"
          onClick={on_create_agent}
          type="button"
        >
          <MessageSquarePlus className="h-3.5 w-3.5" />
          添加成员
        </button>
      </div>

      <div className="space-y-2">
        {members.map((member) => {
          const isCurrentAgent = member.agent_id === current_agent_id;
          return (
            <button
              key={member.agent_id}
              className={cn(
                "flex w-full items-center gap-3 rounded-[20px] px-3 py-3 text-left transition-all duration-300",
                isCurrentAgent ? "workspace-card-strong" : "workspace-card hover:-translate-y-0.5",
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
                  {isCurrentAgent ? "当前正在这个 room 中协作" : "可加入当前 room 的成员"}
                </p>
              </div>
              <span className={cn(
                "h-2.5 w-2.5 shrink-0 rounded-full",
                isCurrentAgent ? "bg-emerald-400" : "bg-slate-300",
              )}/>
            </button>
          );
        })}
      </div>
    </section>
  );
}
