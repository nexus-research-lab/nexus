/**
 * 定时任务类型定义
 *
 * 对齐 automation cron 后端的结构化 schedule / session target / delivery 契约。
 */

export type ScheduledTaskScheduleKind = "every" | "cron" | "at";
export type ScheduledTaskSessionTargetKind = "isolated" | "main" | "bound" | "named";
export type ScheduledTaskWakeMode = "now" | "next-heartbeat";
export type ScheduledTaskDeliveryMode = "none" | "last" | "explicit";
export type ScheduledTaskSourceKind = "user_page" | "agent" | "cli" | "system";
export type ScheduledTaskSourceContextType = "agent" | "room" | "chat";
export type ScheduledTaskOverlapPolicy = "skip" | "allow";
export type ScheduledTaskExecutionKind = "agent" | "script";
export type ScheduledTaskRunLedgerStatus =
  | "pending"
  | "running"
  | "succeeded"
  | "failed"
  | "cancelled"
  | "queued_to_main_session"
  | "skipped";
export type ScheduledTaskExecutionStatus = ScheduledTaskRunLedgerStatus;
export type ScheduledTaskDeliveryStatus =
  | "not_required"
  | "skipped"
  | "succeeded"
  | "failed"
  | "not_attempted"
  | "pending";

export type ScheduledTaskSchedule =
  | {
      kind: "every";
      interval_seconds: number;
      run_at?: null;
      cron_expression?: null;
      timezone?: string | null;
    }
  | {
      kind: "cron";
      cron_expression: string;
      timezone: string;
      run_at?: null;
      interval_seconds?: null;
    }
  | {
      kind: "at";
      run_at: string;
      interval_seconds?: null;
      cron_expression?: null;
      timezone?: string | null;
    };

export type ScheduledTaskSessionTarget =
  | {
      kind: "isolated";
      bound_session_key?: null;
      named_session_key?: null;
      wake_mode?: ScheduledTaskWakeMode;
    }
  | {
      kind: "main";
      bound_session_key?: null;
      named_session_key?: null;
      wake_mode?: ScheduledTaskWakeMode;
    }
  | {
      kind: "bound";
      bound_session_key: string;
      named_session_key?: null;
      wake_mode?: ScheduledTaskWakeMode;
    }
  | {
      kind: "named";
      bound_session_key?: null;
      named_session_key: string;
      wake_mode?: ScheduledTaskWakeMode;
    };

export interface ScheduledTaskDeliveryTarget {
  mode: ScheduledTaskDeliveryMode;
  channel?: string | null;
  to?: string | null;
  account_id?: string | null;
  thread_id?: string | null;
}

export interface ScheduledTaskSource {
  kind: ScheduledTaskSourceKind;
  creator_agent_id?: string | null;
  context_type?: ScheduledTaskSourceContextType | null;
  context_id?: string | null;
  context_label?: string | null;
  session_key?: string | null;
  session_label?: string | null;
}

export interface ApiScheduledTask {
  job_id: string;
  name: string;
  agent_id: string;
  schedule: ScheduledTaskSchedule;
  instruction: string;
  execution_kind?: ScheduledTaskExecutionKind | null;
  session_target: ScheduledTaskSessionTarget;
  delivery: ScheduledTaskDeliveryTarget;
  source: ScheduledTaskSource;
  overlap_policy?: ScheduledTaskOverlapPolicy | null;
  enabled: boolean;
  next_run_at?: string | null;
  running: boolean;
  running_run_id?: string | null;
  running_started_at?: string | null;
  last_run_at?: string | null;
  last_run_status?: ScheduledTaskRunLedgerStatus | string | null;
  failure_streak?: number | null;
  last_error?: string | null;
  last_delivery_status?: string | null;
}

export interface ScheduledTaskItem extends Omit<ApiScheduledTask, "next_run_at" | "running_started_at" | "last_run_at" | "failure_streak"> {
  next_run_at: number | null;
  running_started_at: number | null;
  last_run_at: number | null;
  failure_streak: number;
}

export interface ListScheduledTasksParams {
  agent_id?: string;
}

export interface CreateScheduledTaskParams {
  name: string;
  agent_id: string;
  schedule: ScheduledTaskSchedule;
  session_target?: ScheduledTaskSessionTarget;
  instruction: string;
  execution_kind?: ScheduledTaskExecutionKind;
  delivery?: ScheduledTaskDeliveryTarget;
  source?: ScheduledTaskSource;
  overlap_policy?: ScheduledTaskOverlapPolicy;
  enabled?: boolean;
}

export interface UpdateScheduledTaskParams {
  name?: string;
  agent_id?: string;
  schedule?: ScheduledTaskSchedule;
  instruction?: string;
  execution_kind?: ScheduledTaskExecutionKind;
  session_target?: ScheduledTaskSessionTarget;
  delivery?: ScheduledTaskDeliveryTarget;
  source?: ScheduledTaskSource;
  overlap_policy?: ScheduledTaskOverlapPolicy;
  enabled?: boolean;
}

export interface UpdateScheduledTaskStatusParams {
  enabled: boolean;
}

export interface RecoverScheduledTaskRunParams {
  run_id?: string;
}

export interface DeleteScheduledTaskResponse {
  job_id: string;
  agent_id?: string | null;
  deleted?: boolean;
  active_run_id?: string | null;
  cancelled_run_id?: string | null;
  cancelled_active_run?: boolean;
}

export interface ApiScheduledTaskRun {
  run_id: string;
  job_id: string;
  status: ScheduledTaskRunLedgerStatus;
  trigger_kind?: string | null;
  session_key?: string | null;
  round_id?: string | null;
  session_id?: string | null;
  message_count?: number | null;
  delivery_mode?: ScheduledTaskDeliveryMode | string | null;
  delivery_to?: string | null;
  delivery_status?: ScheduledTaskDeliveryStatus | string | null;
  delivery_error?: string | null;
  delivered_at?: string | null;
  delivery_attempts?: number | null;
  delivery_next_attempt_at?: string | null;
  delivery_dead_letter_at?: string | null;
  scheduled_for?: string | null;
  started_at?: string | null;
  finished_at?: string | null;
  attempts: number;
  error_message?: string | null;
  result_summary?: string | null;
  assistant_text?: string | null;
  result_text?: string | null;
  artifact_path?: string | null;
}

export interface ScheduledTaskRunItem extends Omit<ApiScheduledTaskRun, "scheduled_for" | "started_at" | "finished_at" | "delivered_at" | "delivery_next_attempt_at" | "delivery_dead_letter_at"> {
  scheduled_for: number | null;
  started_at: number | null;
  finished_at: number | null;
  delivered_at: number | null;
  delivery_next_attempt_at: number | null;
  delivery_dead_letter_at: number | null;
}

export interface ApiScheduledTaskEvent {
  event_id: string;
  job_id: string;
  agent_id: string;
  action: string;
  actor_user_id?: string | null;
  actor_agent_id?: string | null;
  run_id?: string | null;
  detail?: Record<string, unknown> | null;
  created_at?: string | null;
}

export interface ScheduledTaskEventItem extends Omit<ApiScheduledTaskEvent, "created_at"> {
  created_at: number | null;
}

export interface ScheduledTaskHealth {
  state: string;
  signals?: string[];
  suggested_tools?: string[];
  recovery_available: boolean;
  recovery_run_id?: string;
  manual_redelivery_available: boolean;
  manual_redelivery_run_ids?: string[];
  delivery_failed_run_count?: number;
  delivery_pending_run_count?: number;
  delivery_pending_run_ids?: string[];
  delivery_skipped_run_count?: number;
  delivery_skipped_run_ids?: string[];
  delivery_dead_letter_count?: number;
  delivery_dead_letter_run_ids?: string[];
  failed_run_count?: number;
  execution_failed_run_ids?: string[];
  latest_execution_error?: string | null;
  latest_delivery_error?: string | null;
  running_for_seconds?: number;
}

export interface ApiScheduledTaskStatus {
  job: ApiScheduledTask;
  health: ScheduledTaskHealth;
  recent_runs: ApiScheduledTaskRun[];
  recent_events: ApiScheduledTaskEvent[];
}

export interface ScheduledTaskStatusItem {
  job: ScheduledTaskItem;
  health: ScheduledTaskHealth;
  recent_runs: ScheduledTaskRunItem[];
  recent_events: ScheduledTaskEventItem[];
}

export interface ScheduledTaskDailyReportTotals {
  task_count: number;
  enabled_task_count: number;
  running_task_count: number;
  run_count: number;
  succeeded_run_count: number;
  failed_run_count: number;
  cancelled_run_count: number;
  skipped_run_count: number;
  delivered_run_count: number;
  delivery_failed_run_count: number;
  delivery_pending_run_count: number;
  delivery_skipped_run_count: number;
  delivery_dead_letter_run_count: number;
  delivery_not_needed_count: number;
  delivery_not_attempted_count: number;
}

export interface ApiScheduledTaskDailyReportTask {
  job_id: string;
  name: string;
  agent_id: string;
  deleted?: boolean;
  enabled: boolean;
  running: boolean;
  running_run_id?: string | null;
  recovery_run_id?: string | null;
  next_run_at?: string | null;
  last_run_at?: string | null;
  last_run_status?: ScheduledTaskRunLedgerStatus | string | null;
  last_delivery_status?: string | null;
  failure_streak?: number | null;
  last_error?: string | null;
  latest_execution_error?: string | null;
  latest_delivery_error?: string | null;
  signals?: string[];
  suggested_tools?: string[];
  execution_failed_run_ids?: string[];
  manual_redelivery_run_ids?: string[];
  delivery_pending_run_ids?: string[];
  delivery_skipped_run_ids?: string[];
  delivery_dead_letter_run_ids?: string[];
  runs: ApiScheduledTaskRun[];
  totals: ScheduledTaskDailyReportTotals;
}

export interface ScheduledTaskDailyReportTask extends Omit<ApiScheduledTaskDailyReportTask, "next_run_at" | "last_run_at" | "failure_streak" | "runs"> {
  next_run_at: number | null;
  last_run_at: number | null;
  failure_streak: number;
  runs: ScheduledTaskRunItem[];
}

export interface ApiScheduledTaskDailyReport {
  date: string;
  timezone: string;
  agent_id?: string | null;
  job_id?: string | null;
  start_at: string;
  end_at: string;
  totals: ScheduledTaskDailyReportTotals;
  tasks: ApiScheduledTaskDailyReportTask[];
}

export interface ScheduledTaskDailyReport extends Omit<ApiScheduledTaskDailyReport, "start_at" | "end_at" | "tasks"> {
  start_at: number | null;
  end_at: number | null;
  tasks: ScheduledTaskDailyReportTask[];
}

export interface GetScheduledTaskStatusParams {
  run_limit?: number;
  event_limit?: number;
}

export interface ListScheduledTaskEventsParams {
  limit?: number;
}

export interface GetScheduledTaskDailyReportParams {
  date?: string;
  timezone?: string;
  agent_id?: string;
  job_id?: string;
}

export interface ApiScheduledTaskExecutionResult {
  job_id: string;
  run_id?: string | null;
  status: ScheduledTaskExecutionStatus;
  session_key: string;
  scheduled_for?: string | null;
  round_id?: string | null;
  session_id?: string | null;
  message_count: number;
  error_message?: string | null;
}

export interface ScheduledTaskRunNowResponse extends Omit<ApiScheduledTaskExecutionResult, "scheduled_for"> {
  scheduled_for: number | null;
}
