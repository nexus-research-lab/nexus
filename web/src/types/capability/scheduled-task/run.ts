/**
 * 定时任务运行记录与即时执行结果契约。
 */

import type { ScheduledTaskDeliveryMode } from "./task";

export type ScheduledTaskRunLedgerStatus =
  | "pending"
  | "running"
  | "succeeded"
  | "failed"
  | "cancelled"
  | "queued_to_main_session"
  | "skipped";

export type ScheduledTaskDeliveryStatus =
  | "not_required"
  | "skipped"
  | "succeeded"
  | "failed"
  | "not_attempted"
  | "pending";

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

export interface ScheduledTaskRunItem extends Omit<
  ApiScheduledTaskRun,
  | "scheduled_for"
  | "started_at"
  | "finished_at"
  | "delivered_at"
  | "delivery_next_attempt_at"
  | "delivery_dead_letter_at"
> {
  scheduled_for: number | null;
  started_at: number | null;
  finished_at: number | null;
  delivered_at: number | null;
  delivery_next_attempt_at: number | null;
  delivery_dead_letter_at: number | null;
}

export interface ApiScheduledTaskExecutionResult {
  job_id: string;
  run_id?: string | null;
  status: ScheduledTaskRunLedgerStatus;
  session_key: string;
  scheduled_for?: string | null;
  round_id?: string | null;
  session_id?: string | null;
  message_count: number;
  error_message?: string | null;
}

export interface ScheduledTaskRunNowResponse extends Omit<
  ApiScheduledTaskExecutionResult,
  "scheduled_for"
> {
  scheduled_for: number | null;
}
