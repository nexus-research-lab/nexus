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

function transformTask(apiTask: ApiScheduledTask): ScheduledTaskItem {
  return {
    ...apiTask,
    next_run_at: to_timestamp_or_null(apiTask.next_run_at),
    running_started_at: to_timestamp_or_null(apiTask.running_started_at),
    last_run_at: to_timestamp_or_null(apiTask.last_run_at),
    failure_streak: apiTask.failure_streak ?? 0,
  };
}

function transformRun(apiRun: ApiScheduledTaskRun): ScheduledTaskRunItem {
  return {
    ...apiRun,
    scheduled_for: to_timestamp_or_null(apiRun.scheduled_for),
    started_at: to_timestamp_or_null(apiRun.started_at),
    finished_at: to_timestamp_or_null(apiRun.finished_at),
    delivered_at: to_timestamp_or_null(apiRun.delivered_at),
    delivery_next_attempt_at: to_timestamp_or_null(apiRun.delivery_next_attempt_at),
    delivery_dead_letter_at: to_timestamp_or_null(apiRun.delivery_dead_letter_at),
  };
}

function transformEvent(apiEvent: ApiScheduledTaskEvent): ScheduledTaskEventItem {
  return {
    ...apiEvent,
    created_at: to_timestamp_or_null(apiEvent.created_at),
  };
}

function transformStatus(apiStatus: ApiScheduledTaskStatus): ScheduledTaskStatusItem {
  return {
    ...apiStatus,
    job: transformTask(apiStatus.job),
    recent_runs: apiStatus.recent_runs.map(transformRun),
    recent_events: apiStatus.recent_events.map(transformEvent),
  };
}

function transformDailyReportTask(
  apiTask: ApiScheduledTaskDailyReport["tasks"][number],
): ScheduledTaskDailyReportTask {
  return {
    ...apiTask,
    next_run_at: to_timestamp_or_null(apiTask.next_run_at),
    last_run_at: to_timestamp_or_null(apiTask.last_run_at),
    failure_streak: apiTask.failure_streak ?? 0,
    runs: apiTask.runs.map(transformRun),
  };
}

function transformDailyReport(
  apiReport: ApiScheduledTaskDailyReport,
): ScheduledTaskDailyReport {
  return {
    ...apiReport,
    start_at: to_timestamp_or_null(apiReport.start_at),
    end_at: to_timestamp_or_null(apiReport.end_at),
    tasks: apiReport.tasks.map(transformDailyReportTask),
  };
}

function transformRunNowResult(
  apiResult: ApiScheduledTaskExecutionResult,
): ScheduledTaskRunNowResponse {
  return {
    ...apiResult,
    scheduled_for: to_timestamp_or_null(apiResult.scheduled_for),
  };
}

function buildQuery(params: Record<string, string | undefined>): string {
  const searchParams = new URLSearchParams();
  Object.entries(params).forEach(([key, value]) => {
    if (value) {
      searchParams.set(key, value);
    }
  });
  const queryString = searchParams.toString();
  return queryString ? `?${queryString}` : "";
}

function numberQueryValue(value: number | undefined): string | undefined {
  if (typeof value !== "number" || !Number.isFinite(value) || value <= 0) {
    return undefined;
  }
  return String(Math.floor(value));
}

export async function list_scheduled_tasks_api(
  params?: ListScheduledTasksParams,
): Promise<ScheduledTaskItem[]> {
  const result = await request_api<ApiScheduledTask[]>(
    `${SCHEDULED_TASKS_API_BASE_URL}${buildQuery({
      agent_id: params?.agent_id,
    })}`,
    {
      method: "GET",
    },
  );

  return result.map(transformTask);
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

  return transformTask(result);
}

export async function update_scheduled_task_api(
  jobId: string,
  params: UpdateScheduledTaskParams,
): Promise<ScheduledTaskItem> {
  const result = await request_api<ApiScheduledTask>(
    `${SCHEDULED_TASKS_API_BASE_URL}/${encodeURIComponent(jobId)}`,
    {
      method: "PATCH",
      body: JSON.stringify(params),
    },
  );

  return transformTask(result);
}

export async function delete_scheduled_task_api(
  jobId: string,
): Promise<DeleteScheduledTaskResponse> {
  return request_api<DeleteScheduledTaskResponse>(
    `${SCHEDULED_TASKS_API_BASE_URL}/${encodeURIComponent(jobId)}`,
    {
      method: "DELETE",
    },
  );
}

export async function run_scheduled_task_api(
  jobId: string,
): Promise<ScheduledTaskRunNowResponse> {
  const result = await request_api<ApiScheduledTaskExecutionResult>(
    `${SCHEDULED_TASKS_API_BASE_URL}/${encodeURIComponent(jobId)}/run`,
    {
      method: "POST",
    },
  );

  return transformRunNowResult(result);
}

export async function recover_scheduled_task_run_api(
  jobId: string,
  params: RecoverScheduledTaskRunParams = {},
): Promise<ScheduledTaskItem> {
  const result = await request_api<ApiScheduledTask>(
    `${SCHEDULED_TASKS_API_BASE_URL}/${encodeURIComponent(jobId)}/recover`,
    {
      method: "POST",
      body: JSON.stringify(params),
    },
  );

  return transformTask(result);
}

export async function update_scheduled_task_status_api(
  jobId: string,
  params: UpdateScheduledTaskStatusParams,
): Promise<ScheduledTaskItem> {
  const result = await request_api<ApiScheduledTask>(
    `${SCHEDULED_TASKS_API_BASE_URL}/${encodeURIComponent(jobId)}/status`,
    {
      method: "PATCH",
      body: JSON.stringify(params),
    },
  );

  return transformTask(result);
}

export async function list_scheduled_task_runs_api(
  jobId: string,
): Promise<ScheduledTaskRunItem[]> {
  const result = await request_api<ApiScheduledTaskRun[]>(
    `${SCHEDULED_TASKS_API_BASE_URL}/${encodeURIComponent(jobId)}/runs`,
    {
      method: "GET",
    },
  );

  return result.map(transformRun);
}

export async function retry_scheduled_task_run_delivery_api(
  jobId: string,
  runId: string,
): Promise<ScheduledTaskRunItem> {
  const result = await request_api<ApiScheduledTaskRun>(
    `${SCHEDULED_TASKS_API_BASE_URL}/${encodeURIComponent(jobId)}/runs/${encodeURIComponent(runId)}/delivery/retry`,
    {
      method: "POST",
      body: JSON.stringify({}),
    },
  );

  return transformRun(result);
}
