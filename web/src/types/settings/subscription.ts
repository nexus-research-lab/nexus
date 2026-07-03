export interface SubscriptionPlan {
  plan_key: string;
  display_name: string;
  status: string;
  monthly_token_limit: number | null;
  notes: string;
  sort_order: number;
}

export interface SubscriptionAccount {
  owner_user_id: string;
  username: string;
  display_name: string;
  role: string;
  user_status: string;
  plan_key: string;
  plan_name: string;
  monthly_token_limit: number | null;
  used_tokens: number;
  used_percent: number | null;
  session_count: number;
  message_count: number;
  period_start: string;
  period_end: string;
  created_at: string;
  updated_at: string;
}

export interface SubscriptionOverview {
  plans: SubscriptionPlan[];
  accounts: SubscriptionAccount[];
  period_start: string;
  period_end: string;
  updated_at: string;
}

export interface UpdateUserSubscriptionPayload {
  plan_key: string;
}

export interface UpsertSubscriptionPlanPayload {
  plan_key: string;
  display_name: string;
  status: string;
  monthly_token_limit: number | null;
  notes: string;
  sort_order: number;
}
