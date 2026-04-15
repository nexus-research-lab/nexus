import { Cpu } from "lucide-react";

import { AgentCostSummary, ConversationCostSummary } from "@/types/cost";
import { formatCost, formatTokens } from "@/lib/utils";

import { ContextSection } from "./context-section";

const METRIC_CARD_CLASS_NAME = "surface-card rounded-[22px] px-3 py-2";

interface UsageSectionProps {
  agent_cost_summary: AgentCostSummary;
  conversation_cost_summary: ConversationCostSummary;
}

export function UsageSection({
  agent_cost_summary,
  conversation_cost_summary,
}: UsageSectionProps) {
  const lastRunDurationMs = conversation_cost_summary.last_run_duration_ms ?? null;

  return (
    <ContextSection
      icon={<Cpu className="h-3.5 w-3.5" />}
      title="Conversation Usage"
    >
      <div className="grid grid-cols-2 gap-2">
        <div className={METRIC_CARD_CLASS_NAME}>
          <p className="text-[11px] uppercase tracking-[0.12em] text-(--text-soft)">当前 room</p>
          <p className="mt-1 text-sm font-semibold text-(--text-strong)">
            {formatCost(conversation_cost_summary.total_cost_usd)}
          </p>
        </div>
        <div className={METRIC_CARD_CLASS_NAME}>
          <p className="text-[11px] uppercase tracking-[0.12em] text-(--text-soft)">成员累计</p>
          <p className="mt-1 text-sm font-semibold text-(--text-strong)">
            {formatCost(agent_cost_summary.total_cost_usd)}
          </p>
        </div>
      </div>
      <div className="mt-3 space-y-1.5">
        <div className="flex justify-between gap-4">
          <span className="text-[11px] text-(--text-soft)">总 Tokens</span>
          <span className="text-[11px] font-medium text-(--text-default)">
            {formatTokens(conversation_cost_summary.total_tokens)}
          </span>
        </div>
        <div className="flex justify-between gap-4">
          <span className="text-[11px] text-(--text-soft)">输入 / 输出</span>
          <span className="text-[11px] font-medium text-(--text-default)">
            {formatTokens(conversation_cost_summary.total_input_tokens)} / {formatTokens(conversation_cost_summary.total_output_tokens)}
          </span>
        </div>
        <div className="flex justify-between gap-4">
          <span className="text-[11px] text-(--text-soft)">缓存</span>
          <span className="text-[11px] font-medium text-(--text-default)">
            {formatTokens(conversation_cost_summary.total_cache_read_input_tokens)} / {formatTokens(conversation_cost_summary.total_cache_creation_input_tokens)}
          </span>
        </div>
        <div className="flex justify-between gap-4">
          <span className="text-[11px] text-(--text-soft)">上次耗时</span>
          <span className="text-[11px] font-medium text-(--text-default)">
            {lastRunDurationMs !== null ? `${(lastRunDurationMs / 1000).toFixed(1)}s` : "-"}
          </span>
        </div>
      </div>
    </ContextSection>
  );
}
