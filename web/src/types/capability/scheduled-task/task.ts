/**
 * 定时任务定义与写操作契约。
 *
 * 这里只描述任务本身；运行记录和执行结果由 `run.ts` 持有。
 */

export type ScheduledTaskWakeMode = "now" | "next-heartbeat";
export type ScheduledTaskDeliveryMode = "none" | "last" | "explicit";
export type ScheduledTaskSourceKind = "user_page" | "agent" | "cli" | "system";
export type ScheduledTaskSourceContextType = "agent" | "room" | "chat";
export type ScheduledTaskOverlapPolicy = "skip" | "allow";
export type ScheduledTaskExecutionKind = "agent" | "script";

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
  expires_at?: string | null;
  enabled: boolean;
  next_run_at?: string | null;
  running: boolean;
  running_run_id?: string | null;
  running_started_at?: string | null;
  last_run_at?: string | null;
  last_run_status?: string | null;
  failure_streak?: number | null;
  last_error?: string | null;
  last_delivery_status?: string | null;
}

export interface ScheduledTaskItem extends Omit<
  ApiScheduledTask,
  "expires_at" | "next_run_at" | "running_started_at" | "last_run_at" | "failure_streak"
> {
  expires_at: number | null;
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
  expires_at?: string;
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
  expires_at?: string;
  clear_expires_at?: boolean;
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
