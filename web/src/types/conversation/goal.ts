/**
 * INPUT: Goal REST responses and server-derived Goal/Execution binding view.
 * OUTPUT: DM/Room Goal resource, lifecycle and clear-gate types.
 * POS: Goal HTTP transport types; metadata is never a WorkGraph binding source.
 */
export type GoalStatus =
  | "active"
  | "paused"
  | "complete"
  | "blocked"
  | "budget_limited"
  | "usage_limited";

export interface GoalUsage {
  input_tokens?: number;
  output_tokens?: number;
  cache_creation_input_tokens?: number;
  cache_read_input_tokens?: number;
  reasoning_tokens?: number;
  actual_tokens?: number;
  actual_tokens_estimated?: boolean;
  budget_tokens?: number;
  total_tokens?: number;
  runtime_seconds?: number;
}

export interface Goal {
  id: string;
  session_key: string;
  objective: string;
  status: GoalStatus;
  token_budget?: number | null;
  usage?: GoalUsage;
  time_used_seconds?: number;
  continuation_count: number;
  empty_progress_count: number;
  version: number;
  created_by?: string;
  created_at: string;
  updated_at: string;
  completed_at?: string | null;
  blocked_at?: string | null;
  usage_finalized: boolean;
  usage_finalized_at?: string | null;
  last_error?: string;
  metadata?: Record<string, unknown>;
}

export interface GoalUsageReport {
  goal_id: string;
  session_key: string;
  status: GoalStatus;
  usage: GoalUsage;
  time_used_seconds: number;
  usage_finalized: boolean;
  usage_finalized_at?: string | null;
  goal_updated_at: string;
}

export type GoalExecutionBindingState =
  | "standalone"
  | "reserved"
  | "pending"
  | "confirmed"
  | "conflict";

export interface GoalExecutionBinding {
  state: GoalExecutionBindingState;
  execution_id?: string;
}

export interface GoalEvent {
  id: string;
  goal_id: string;
  session_key: string;
  event_type: string;
  source: "user" | "model" | "system" | "external";
  round_id?: string;
  payload?: Record<string, unknown>;
  created_at: string;
}

export interface CreateGoalInput {
  session_key: string;
  objective: string;
  token_budget?: number | null;
  replace_existing?: boolean;
  room_lead_agent_id?: string | null;
  metadata?: Record<string, unknown> | null;
}

export interface UpdateGoalInput {
  objective?: string;
  token_budget?: number | null;
  metadata?: Record<string, unknown> | null;
}

export interface ClearGoalResult {
  cleared: boolean;
}
