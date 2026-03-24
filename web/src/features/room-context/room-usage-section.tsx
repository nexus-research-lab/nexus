import { Cpu } from "lucide-react";

import { AgentCostSummary, SessionCostSummary } from "@/types/cost";
import { formatCost, formatTokens } from "@/lib/utils";

interface RoomUsageSectionProps {
  agent_cost_summary: AgentCostSummary;
  session_cost_summary: SessionCostSummary;
}

export function RoomUsageSection({
  agent_cost_summary,
  session_cost_summary,
}: RoomUsageSectionProps) {
  const lastRunDurationMs = session_cost_summary.last_run_duration_ms ?? null;

  return (
    <section className="border-b workspace-divider px-4 py-4">
      <div className="mb-3 flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.16em] text-slate-700/56">
        <Cpu className="h-3.5 w-3.5" />
        Session Usage
      </div>
      <div className="grid grid-cols-2 gap-2">
        <div className="workspace-card rounded-[22px] px-3 py-2">
          <p className="text-[11px] uppercase tracking-[0.12em] text-slate-700/50">当前 room</p>
          <p className="mt-1 text-sm font-semibold text-slate-900/86">
            {formatCost(session_cost_summary.total_cost_usd)}
          </p>
        </div>
        <div className="workspace-card rounded-[22px] px-3 py-2">
          <p className="text-[11px] uppercase tracking-[0.12em] text-slate-700/50">成员累计</p>
          <p className="mt-1 text-sm font-semibold text-slate-900/86">
            {formatCost(agent_cost_summary.total_cost_usd)}
          </p>
        </div>
      </div>
      <div className="mt-3 space-y-1.5">
        <div className="flex justify-between gap-4">
          <span className="text-[11px] text-slate-700/54">总 Tokens</span>
          <span className="text-[11px] font-medium text-slate-900/84">
            {formatTokens(session_cost_summary.total_tokens)}
          </span>
        </div>
        <div className="flex justify-between gap-4">
          <span className="text-[11px] text-slate-700/54">输入 / 输出</span>
          <span className="text-[11px] font-medium text-slate-900/84">
            {formatTokens(session_cost_summary.total_input_tokens)} / {formatTokens(session_cost_summary.total_output_tokens)}
          </span>
        </div>
        <div className="flex justify-between gap-4">
          <span className="text-[11px] text-slate-700/54">缓存</span>
          <span className="text-[11px] font-medium text-slate-900/84">
            {formatTokens(session_cost_summary.total_cache_read_input_tokens)} / {formatTokens(session_cost_summary.total_cache_creation_input_tokens)}
          </span>
        </div>
        <div className="flex justify-between gap-4">
          <span className="text-[11px] text-slate-700/54">上次耗时</span>
          <span className="text-[11px] font-medium text-slate-900/84">
            {lastRunDurationMs !== null ? `${(lastRunDurationMs / 1000).toFixed(1)}s` : "-"}
          </span>
        </div>
      </div>
    </section>
  );
}
