/**
 * INPUT: Goal REST responses and server-derived Goal/Execution binding view.
 * OUTPUT: Consumed DM/Room Goal resource, server-derived continuation lifecycle and clear-gate types; no unused event mirror.
 * POS: Consumed Goal HTTP transport types; creation uses the control command, and metadata is never a WorkGraph binding source.
 */
export type GoalStatus =
  | "active"
  | "paused"
  | "complete"
  | "blocked"
  | "budget_limited"
  | "usage_limited";

export type GoalContinuationState =
  | "inactive"
  | "ready"
  | "recovering"
  | "suspended";

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
  continuation_state: GoalContinuationState;
  version: number;
  created_by?: string;
  created_at: string;
  updated_at: string;
  completed_at?: string | null;
  blocked_at?: string | null;
	blocker?: GoalBlocker | null;
  usage_finalized: boolean;
  usage_finalized_at?: string | null;
  last_error?: string;
  metadata?: Record<string, unknown>;
}

export interface GoalBlocker {
  id: string;
  reason: string;
  needed_input: string;
  since_objective_revision: number;
  blocked_at?: string;
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

export interface UpdateGoalInput {
  objective?: string;
  token_budget?: number | null;
  metadata?: Record<string, unknown> | null;
}

export interface ClearGoalResult {
  cleared: boolean;
}
