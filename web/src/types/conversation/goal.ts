export type GoalStatus = "active" | "paused" | "complete" | "blocked" | "cleared";

export interface GoalUsage {
  input_tokens?: number;
  output_tokens?: number;
  cache_creation_input_tokens?: number;
  cache_read_input_tokens?: number;
  reasoning_tokens?: number;
  total_tokens?: number;
}

export interface Goal {
  id: string;
  session_key: string;
  objective: string;
  status: GoalStatus;
  token_budget?: number | null;
  usage?: GoalUsage;
  continuation_count: number;
  empty_progress_count: number;
  version: number;
  created_by?: string;
  created_at: string;
  updated_at: string;
  completed_at?: string | null;
  blocked_at?: string | null;
  cleared_at?: string | null;
  last_error?: string;
  metadata?: Record<string, unknown>;
}

export interface CreateGoalInput {
  session_key: string;
  objective: string;
  token_budget?: number | null;
}

export interface UpdateGoalInput {
  objective?: string;
  token_budget?: number | null;
  metadata?: Record<string, unknown> | null;
}
