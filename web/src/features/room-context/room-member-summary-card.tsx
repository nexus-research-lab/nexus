import { Agent } from "@/types/agent";
import { cn } from "@/lib/utils";

interface RoomMemberSummaryCardProps {
  agent: Agent;
  modelName: string;
  runtimeStatus: "Running" | "Idle" | "Active";
  localizedRuntimeStatus: string;
  onEditAgent: (agentId: string) => void;
}

export function RoomMemberSummaryCard({
  agent,
  modelName,
  runtimeStatus,
  localizedRuntimeStatus,
  onEditAgent,
}: RoomMemberSummaryCardProps) {
  return (
    <section className="border-b workspace-divider px-4 py-4">
      <div className="workspace-card-strong rounded-[24px] px-4 py-4">
        <div className="flex items-start justify-between gap-3">
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-slate-700/50">
              Current Member
            </p>
            <p className="mt-1 text-[32px] font-black tracking-[-0.05em] text-slate-950/90">
              {agent.name}
            </p>
            <p className="mt-1 text-[12px] text-slate-700/54">
              当前 room 的执行成员 · {modelName}
            </p>
          </div>

          <div className="flex flex-col items-end gap-2">
            <span className={cn(
              "inline-flex items-center gap-2 rounded-full px-3 py-1 text-[11px] font-semibold",
              runtimeStatus === "Running"
                ? "bg-emerald-100 text-emerald-700"
                : runtimeStatus === "Idle"
                  ? "bg-slate-200 text-slate-700"
                  : "bg-emerald-100 text-emerald-700",
            )}>
              <span className="h-2 w-2 rounded-full bg-current" />
              {localizedRuntimeStatus}
            </span>
            <button
              aria-label="打开 Agent 设置"
              className="workspace-chip inline-flex h-9 items-center gap-1.5 rounded-full px-4 text-[11px] font-semibold text-slate-900/84 transition-colors hover:text-slate-950 focus-visible:ring-2 focus-visible:ring-primary/40 focus-visible:ring-offset-1"
              onClick={() => onEditAgent(agent.agent_id)}
              type="button"
            >
              设置
            </button>
          </div>
        </div>
      </div>
    </section>
  );
}
