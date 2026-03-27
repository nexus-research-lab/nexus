import { Agent } from "@/types/agent";
import { cn } from "@/lib/utils";

interface RoomMemberSummaryCardProps {
  agent: Agent;
  runtime_status: "Running" | "Idle" | "Active";
  localized_runtime_status: string;
  on_edit_agent: (agent_id: string) => void;
}

export function RoomMemberSummaryCard({
  agent,
  runtime_status,
  localized_runtime_status,
  on_edit_agent,
}: RoomMemberSummaryCardProps) {
  return (
    <section className="border-b workspace-divider px-4 py-4">
      <div className="workspace-card rounded-[24px] px-4 py-4">
        <div className="flex items-start justify-between gap-3">
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-slate-700/50">
              当前成员
            </p>
            <p className="mt-1 text-[28px] font-black tracking-[-0.05em] text-slate-950/90">
              {agent.name}
            </p>
            <p className="mt-1 text-[12px] text-slate-700/54">
              当前正在推进这条协作
            </p>
          </div>

          <div className="flex flex-col items-end gap-2">
            <span className={cn(
              "inline-flex items-center gap-2 rounded-full px-3 py-1 text-[11px] font-semibold",
              runtime_status === "Running"
                ? "bg-emerald-100 text-emerald-700"
                : runtime_status === "Idle"
                  ? "bg-slate-200 text-slate-700"
                  : "bg-emerald-100 text-emerald-700",
            )}>
              <span className="h-2 w-2 rounded-full bg-current" />
              {localized_runtime_status}
            </span>
            <button
              aria-label="打开 Agent 设置"
              className="workspace-chip inline-flex h-8 items-center gap-1.5 rounded-full px-3 text-[11px] font-semibold text-slate-900/84 transition-colors hover:text-slate-950 focus-visible:ring-2 focus-visible:ring-primary/40 focus-visible:ring-offset-1"
              onClick={() => on_edit_agent(agent.agent_id)}
              type="button"
            >
              成员设置
            </button>
          </div>
        </div>
      </div>
    </section>
  );
}
