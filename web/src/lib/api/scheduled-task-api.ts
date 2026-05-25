/**
 * 定时任务 API 服务模块
 *
 * 对齐 capability/scheduled/tasks 的结构化自动化任务接口。
 */

import { get_agent_api_base_url } from "@/config/options";
import { request_api } from "@/lib/api/http";
import { to_timestamp_or_null } from "@/lib/api/timestamp-utils";
import type {
  ApiScheduledTask,
  ApiScheduledTaskDailyReport,
  ApiScheduledTaskEvent,
  ApiScheduledTaskExecutionResult,
  ApiScheduledTaskRun,
  ApiScheduledTaskStatus,
  CreateScheduledTaskParams,
  DeleteScheduledTaskResponse,
  GetScheduledTaskDailyReportParams,
  GetScheduledTaskStatusParams,
  ListScheduledTaskEventsParams,
  ListScheduledTasksParams,
  RecoverScheduledTaskRunParams,
  ScheduledTaskDailyReport,
  ScheduledTaskDailyReportTask,
  ScheduledTaskEventItem,
  ScheduledTaskItem,
  ScheduledTaskRunItem,
  ScheduledTaskRunNowResponse,
  ScheduledTaskStatusItem,
  UpdateScheduledTaskParams,
  UpdateScheduledTaskStatusParams,
} from "@/types/capability/scheduled-task";

const AGENT_API_BASE_URL = get_agent_api_base_url();
const SCHEDULED_TASKS_API_BASE_URL = `${AGENT_API_BASE_URL}/capability/scheduled/tasks`;

function transform_task(api_task: ApiScheduledTask): ScheduledTaskItem {
  return {
    ...api_task,
    next_run_at: to_timestamp_or_null(api_task.next_run_at),
    running_started_at: to_timestamp_or_null(api_task.running_started_at),
    last_run_at: to_timestamp_or_null(api_task.last_run_at),
    failure_streak: api_task.failure_streak ?? 0,
  };
}

function transform_run(api_run: ApiScheduledTaskRun): ScheduledTaskRunItem {
  return {
    ...api_run,
    scheduled_for: to_timestamp_or_null(api_run.scheduled_for),
    started_at: to_timestamp_or_null(api_run.started_at),
    finished_at: to_timestamp_or_null(api_run.finished_at),
    delivered_at: to_timestamp_or_null(api_run.delivered_at),
    delivery_next_attempt_at: to_timestamp_or_null(api_run.delivery_next_attempt_at),
    delivery_dead_letter_at: to_timestamp_or_null(api_run.delivery_dead_letter_at),
  };
}

function transform_event(api_event: ApiScheduledTaskEvent): ScheduledTaskEventItem {
  return {
    ...api_event,
    created_at: to_timestamp_or_null(api_event.created_at),
  };
}

function transform_status(api_status: ApiScheduledTaskStatus): ScheduledTaskStatusItem {
  return {
    ...api_status,
    job: transform_task(api_status.job),
    recent_runs: api_status.recent_runs.map(transform_run),
    recent_events: api_status.recent_events.map(transform_event),
  };
}

function transform_daily_report_task(
  api_task: ApiScheduledTaskDailyReport["tasks"][number],
): ScheduledTaskDailyReportTask {
  return {
    ...api_task,
    next_run_at: to_timestamp_or_null(api_task.next_run_at),
    last_run_at: to_timestamp_or_null(api_task.last_run_at),
    failure_streak: api_task.failure_streak ?? 0,
    runs: api_task.runs.map(transform_run),
  };
}

function transform_daily_report(
  api_report: ApiScheduledTaskDailyReport,
): ScheduledTaskDailyReport {
  return {
    ...api_report,
    start_at: to_timestamp_or_null(api_report.start_at),
    end_at: to_timestamp_or_null(api_report.end_at),
    tasks: api_report.tasks.map(transform_daily_report_task),
  };
}

function transform_run_now_result(
  api_result: ApiScheduledTaskExecutionResult,
): ScheduledTaskRunNowResponse {
  return {
    ...api_result,
    scheduled_for: to_timestamp_or_null(api_result.scheduled_for),
  };
}

function build_query(params: Record<string, string | undefined>): string {
  const search_params = new URLSearchParams();
  Object.entries(params).forEach(([key, value]) => {
    if (value) {
      search_params.set(key, value);
    }
  });
  const query_string = search_params.toString();
  return query_string ? `?${query_string}` : "";
}

function number_query_value(value: number | undefined): string | undefined {
  if (typeof value !== "number" || !Number.isFinite(value) || value <= 0) {
    return undefined;
  }
  return String(Math.floor(value));
}

export async function list_scheduled_tasks_api(
  params?: ListScheduledTasksParams,
): Promise<ScheduledTaskItem[]> {
  const result = await request_api<ApiScheduledTask[]>(
    `${SCHEDULED_TASKS_API_BASE_URL}${build_query({
      agent_id: params?.agent_id,
    })}`,
    {
      method: "GET",
    },
  );

  return result.map(transform_task);
}

export async function create_scheduled_task_api(
  params: CreateScheduledTaskParams,
): Promise<ScheduledTaskItem> {
  const result = await request_api<ApiScheduledTask>(
    SCHEDULED_TASKS_API_BASE_URL,
    {
      method: "POST",
      body: JSON.stringify(params),
    },
  );

  return transform_task(result);
}

export async function update_scheduled_task_api(
  job_id: string,
  params: UpdateScheduledTaskParams,
): Promise<ScheduledTaskItem> {
  const result = await request_api<ApiScheduledTask>(
    `${SCHEDULED_TASKS_API_BASE_URL}/${encodeURIComponent(job_id)}`,
    {
      method: "PATCH",
      body: JSON.stringify(params),
    },
  );

  return transform_task(result);
}

export async function delete_scheduled_task_api(
  job_id: string,
): Promise<DeleteScheduledTaskResponse> {
  return request_api<DeleteScheduledTaskResponse>(
    `${SCHEDULED_TASKS_API_BASE_URL}/${encodeURIComponent(job_id)}`,
    {
      method: "DELETE",
    },
  );
}

export async function run_scheduled_task_api(
  job_id: string,
): Promise<ScheduledTaskRunNowResponse> {
  const result = await request_api<ApiScheduledTaskExecutionResult>(
    `${SCHEDULED_TASKS_API_BASE_URL}/${encodeURIComponent(job_id)}/run`,
    {
      method: "POST",
    },
  );

  return transform_run_now_result(result);
}

export async function recover_scheduled_task_run_api(
  job_id: string,
  params: RecoverScheduledTaskRunParams = {},
): Promise<ScheduledTaskItem> {
  const result = await request_api<ApiScheduledTask>(
    `${SCHEDULED_TASKS_API_BASE_URL}/${encodeURIComponent(job_id)}/recover`,
    {
      method: "POST",
      body: JSON.stringify(params),
    },
  );

  return transform_task(result);
}

export async function update_scheduled_task_status_api(
  job_id: string,
  params: UpdateScheduledTaskStatusParams,
): Promise<ScheduledTaskItem> {
  const result = await request_api<ApiScheduledTask>(
    `${SCHEDULED_TASKS_API_BASE_URL}/${encodeURIComponent(job_id)}/status`,
    {
      method: "PATCH",
      body: JSON.stringify(params),
    },
  );

  return transform_task(result);
}

export async function list_scheduled_task_runs_api(
  job_id: string,
): Promise<ScheduledTaskRunItem[]> {
  const result = await request_api<ApiScheduledTaskRun[]>(
    `${SCHEDULED_TASKS_API_BASE_URL}/${encodeURIComponent(job_id)}/runs`,
    {
      method: "GET",
    },
  );

  return result.map(transform_run);
}

export async function get_scheduled_task_status_api(
  job_id: string,
  params: GetScheduledTaskStatusParams = {},
): Promise<ScheduledTaskStatusItem> {
  const result = await request_api<ApiScheduledTaskStatus>(
    `${SCHEDULED_TASKS_API_BASE_URL}/${encodeURIComponent(job_id)}/status${build_query({
      run_limit: number_query_value(params.run_limit),
      event_limit: number_query_value(params.event_limit),
    })}`,
    {
      method: "GET",
    },
  );

  return transform_status(result);
}

export async function list_scheduled_task_events_api(
  job_id: string,
  params: ListScheduledTaskEventsParams = {},
): Promise<ScheduledTaskEventItem[]> {
  const result = await request_api<ApiScheduledTaskEvent[]>(
    `${SCHEDULED_TASKS_API_BASE_URL}/${encodeURIComponent(job_id)}/events${build_query({
      limit: number_query_value(params.limit),
    })}`,
    {
      method: "GET",
    },
  );

  return result.map(transform_event);
}

export async function get_scheduled_task_daily_report_api(
  params: GetScheduledTaskDailyReportParams = {},
): Promise<ScheduledTaskDailyReport> {
  const result = await request_api<ApiScheduledTaskDailyReport>(
    `${AGENT_API_BASE_URL}/capability/scheduled/reports/daily${build_query({
      date: params.date,
      timezone: params.timezone,
      agent_id: params.agent_id,
      job_id: params.job_id,
    })}`,
    {
      method: "GET",
    },
  );

  return transform_daily_report(result);
}

export async function retry_scheduled_task_run_delivery_api(
  job_id: string,
  run_id: string,
): Promise<ScheduledTaskRunItem> {
  const result = await request_api<ApiScheduledTaskRun>(
    `${SCHEDULED_TASKS_API_BASE_URL}/${encodeURIComponent(job_id)}/runs/${encodeURIComponent(run_id)}/delivery/retry`,
    {
      method: "POST",
      body: JSON.stringify({}),
    },
  );

  return transform_run(result);
}
