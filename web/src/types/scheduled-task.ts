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
export type ScheduledTaskSourceContextType = "agent" | "room";
export type ScheduledTaskRunLedgerStatus =
  | "pending"
  | "running"
  | "succeeded"
  | "failed"
  | "cancelled";
export type ScheduledTaskExecutionStatus =
  | ScheduledTaskRunLedgerStatus
  | "queued_to_main_session";

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
  session_target: ScheduledTaskSessionTarget;
  delivery: ScheduledTaskDeliveryTarget;
  source: ScheduledTaskSource;
  enabled: boolean;
  next_run_at?: string | null;
  running: boolean;
  last_run_at?: string | null;
}

export interface ScheduledTaskItem extends Omit<ApiScheduledTask, "next_run_at" | "last_run_at"> {
  next_run_at: number | null;
  last_run_at: number | null;
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
  delivery?: ScheduledTaskDeliveryTarget;
  source?: ScheduledTaskSource;
  enabled?: boolean;
}

export interface UpdateScheduledTaskParams {
  name?: string;
  schedule?: ScheduledTaskSchedule;
  instruction?: string;
  session_target?: ScheduledTaskSessionTarget;
  delivery?: ScheduledTaskDeliveryTarget;
  enabled?: boolean;
}

export interface UpdateScheduledTaskStatusParams {
  enabled: boolean;
}

export interface DeleteScheduledTaskResponse {
  job_id: string;
}

export interface ApiScheduledTaskRun {
  run_id: string;
  job_id: string;
  status: ScheduledTaskRunLedgerStatus;
  scheduled_for?: string | null;
  started_at?: string | null;
  finished_at?: string | null;
  attempts: number;
  error_message?: string | null;
}

export interface ScheduledTaskRunItem extends Omit<ApiScheduledTaskRun, "scheduled_for" | "started_at" | "finished_at"> {
  scheduled_for: number | null;
  started_at: number | null;
  finished_at: number | null;
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
