/**
 * 成本相关类型定义
 *
 * [INPUT]: 无外部依赖
 * [OUTPUT]: 对外提供 ConversationCostSummary, AgentCostSummary
 * [POS]: types 模块的成本类型，被 agent-inspector 和 page.tsx 消费
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

export interface ConversationCostSummary {
  agent_id: string;
  session_key: string;
  session_id: string;
  total_input_tokens: number;
  total_output_tokens: number;
  total_tokens: number;
  total_cache_creation_input_tokens: number;
  total_cache_read_input_tokens: number;
  total_cost_usd: number;
  completed_rounds: number;
  error_rounds: number;
  last_round_id?: string | null;
  last_run_duration_ms?: number | null;
  last_run_cost_usd?: number | null;
  updated_at?: string;
}

export interface AgentCostSummary {
  agent_id: string;
  total_input_tokens: number;
  total_output_tokens: number;
  total_tokens: number;
  total_cache_creation_input_tokens: number;
  total_cache_read_input_tokens: number;
  total_cost_usd: number;
  completed_rounds: number;
  error_rounds: number;
  cost_sessions: number;
  updated_at?: string;
}
