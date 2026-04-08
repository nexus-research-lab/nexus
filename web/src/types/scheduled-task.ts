/**
 * 定时任务类型定义
 *
 * 兼容后端原始响应（字符串时间）与前端展示模型（时间戳）。
 */

export type ScheduledTaskStatus = "active" | "paused" | "completed" | "failed";

export interface ApiScheduledTaskItem {
  task_id: string;
  name: string;
  source_type: string;
  source_id?: string | null;
  cron_expression: string;
  timezone: string;
  status: ScheduledTaskStatus;
  instruction: string;
  last_run_at?: string | null;
  next_run_at?: string | null;
  created_at: string;
  updated_at: string;
}

export interface ScheduledTaskItem extends Omit<ApiScheduledTaskItem, "last_run_at" | "next_run_at" | "created_at" | "updated_at"> {
  last_run_at: number | null;
  next_run_at: number | null;
  created_at: number;
  updated_at: number;
}

export interface ScheduledTaskSummary {
  total: number;
  active: number;
  paused: number;
  completed: number;
  failed: number;
}

export interface ApiScheduledTaskListResponse {
  items: ApiScheduledTaskItem[];
  summary: ScheduledTaskSummary;
}

export interface ScheduledTaskListResponse {
  items: ScheduledTaskItem[];
  summary: ScheduledTaskSummary;
}

export interface CreateScheduledTaskParams {
  name: string;
  source_type: string;
  source_id?: string | null;
  cron_expression: string;
  timezone: string;
  instruction: string;
}
