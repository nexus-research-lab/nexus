import { Agent } from "@/types/agent";
import { cn } from "@/lib/utils";

interface MemberSummaryCardProps {
  agent: Agent;
  runtime_status: "Running" | "Idle" | "Active";
  localized_runtime_status: string;
  on_edit_agent: (agent_id: string) => void;
}

export function MemberSummaryCard({
  agent,
  runtime_status,
  localized_runtime_status,
  on_edit_agent,
}: MemberSummaryCardProps) {
  return (
    <section className="min-w-[240px] border-b divider-subtle px-4 py-4">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-(--text-soft)">
            当前成员
          </p>
          <p className="mt-1 text-[28px] font-black tracking-[-0.05em] text-(--text-strong)">
            {agent.name}
          </p>
        </div>

        <div className="flex flex-col items-end gap-2">
          <span className={cn(
            "status-badge",
            runtime_status === "Running" || runtime_status === "Active"
              ? "data-[tone=running]"
              : "data-[tone=idle]",
          )}
            data-tone={runtime_status === "Running" || runtime_status === "Active" ? "running" : "idle"}
          >
            <span className="h-2 w-2 rounded-full bg-current" />
            {localized_runtime_status}
          </span>
          <button
            aria-label="打开 Agent 设置"
            className="text-[11px] font-semibold text-(--text-default) transition-colors hover:text-(--text-strong) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/40 focus-visible:ring-offset-1"
            onClick={() => on_edit_agent(agent.agent_id)}
            type="button"
          >
            成员设置
          </button>
        </div>
      </div>
    </section>
  );
}
