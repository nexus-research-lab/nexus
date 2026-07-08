/**
 * 定时任务 API 服务模块
 *
 * 对齐 capability/scheduled/tasks 的结构化自动化任务接口。
 */

import { getAgentApiBaseUrl } from "@/config/options";
import { requestApi } from "@/lib/api/http";
import { toTimestampOrNull } from "@/lib/api/timestamp-utils";
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

const AGENT_API_BASE_URL = getAgentApiBaseUrl();
const SCHEDULED_TASKS_API_BASE_URL = `${AGENT_API_BASE_URL}/capability/scheduled/tasks`;

function transformTask(apiTask: ApiScheduledTask): ScheduledTaskItem {
  return {
    ...apiTask,
    next_run_at: toTimestampOrNull(apiTask.next_run_at),
    running_started_at: toTimestampOrNull(apiTask.running_started_at),
    last_run_at: toTimestampOrNull(apiTask.last_run_at),
    failure_streak: apiTask.failure_streak ?? 0,
  };
}

function transformRun(apiRun: ApiScheduledTaskRun): ScheduledTaskRunItem {
  return {
    ...apiRun,
    scheduled_for: toTimestampOrNull(apiRun.scheduled_for),
    started_at: toTimestampOrNull(apiRun.started_at),
    finished_at: toTimestampOrNull(apiRun.finished_at),
    delivered_at: toTimestampOrNull(apiRun.delivered_at),
    delivery_next_attempt_at: toTimestampOrNull(apiRun.delivery_next_attempt_at),
    delivery_dead_letter_at: toTimestampOrNull(apiRun.delivery_dead_letter_at),
  };
}

function transformEvent(apiEvent: ApiScheduledTaskEvent): ScheduledTaskEventItem {
  return {
    ...apiEvent,
    created_at: toTimestampOrNull(apiEvent.created_at),
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
    next_run_at: toTimestampOrNull(apiTask.next_run_at),
    last_run_at: toTimestampOrNull(apiTask.last_run_at),
    failure_streak: apiTask.failure_streak ?? 0,
    runs: apiTask.runs.map(transformRun),
  };
}

function transformDailyReport(
  apiReport: ApiScheduledTaskDailyReport,
): ScheduledTaskDailyReport {
  return {
    ...apiReport,
    start_at: toTimestampOrNull(apiReport.start_at),
    end_at: toTimestampOrNull(apiReport.end_at),
    tasks: apiReport.tasks.map(transformDailyReportTask),
  };
}

function transformRunNowResult(
  apiResult: ApiScheduledTaskExecutionResult,
): ScheduledTaskRunNowResponse {
  return {
    ...apiResult,
    scheduled_for: toTimestampOrNull(apiResult.scheduled_for),
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

export async function listScheduledTasksApi(
  params?: ListScheduledTasksParams,
): Promise<ScheduledTaskItem[]> {
  const result = await requestApi<ApiScheduledTask[]>(
    `${SCHEDULED_TASKS_API_BASE_URL}${buildQuery({
      agent_id: params?.agent_id,
    })}`,
    {
      method: "GET",
    },
  );

  return result.map(transformTask);
}

export async function createScheduledTaskApi(
  params: CreateScheduledTaskParams,
): Promise<ScheduledTaskItem> {
  const result = await requestApi<ApiScheduledTask>(
    SCHEDULED_TASKS_API_BASE_URL,
    {
      method: "POST",
      body: JSON.stringify(params),
    },
  );

  return transformTask(result);
}

export async function updateScheduledTaskApi(
  jobId: string,
  params: UpdateScheduledTaskParams,
): Promise<ScheduledTaskItem> {
  const result = await requestApi<ApiScheduledTask>(
    `${SCHEDULED_TASKS_API_BASE_URL}/${encodeURIComponent(jobId)}`,
    {
      method: "PATCH",
      body: JSON.stringify(params),
    },
  );

  return transformTask(result);
}

export async function deleteScheduledTaskApi(
  jobId: string,
): Promise<DeleteScheduledTaskResponse> {
  return requestApi<DeleteScheduledTaskResponse>(
    `${SCHEDULED_TASKS_API_BASE_URL}/${encodeURIComponent(jobId)}`,
    {
      method: "DELETE",
    },
  );
}

export async function runScheduledTaskApi(
  jobId: string,
): Promise<ScheduledTaskRunNowResponse> {
  const result = await requestApi<ApiScheduledTaskExecutionResult>(
    `${SCHEDULED_TASKS_API_BASE_URL}/${encodeURIComponent(jobId)}/run`,
    {
      method: "POST",
    },
  );

  return transformRunNowResult(result);
}

export async function recoverScheduledTaskRunApi(
  jobId: string,
  params: RecoverScheduledTaskRunParams = {},
): Promise<ScheduledTaskItem> {
  const result = await requestApi<ApiScheduledTask>(
    `${SCHEDULED_TASKS_API_BASE_URL}/${encodeURIComponent(jobId)}/recover`,
    {
      method: "POST",
      body: JSON.stringify(params),
    },
  );

  return transformTask(result);
}

export async function updateScheduledTaskStatusApi(
  jobId: string,
  params: UpdateScheduledTaskStatusParams,
): Promise<ScheduledTaskItem> {
  const result = await requestApi<ApiScheduledTask>(
    `${SCHEDULED_TASKS_API_BASE_URL}/${encodeURIComponent(jobId)}/status`,
    {
      method: "PATCH",
      body: JSON.stringify(params),
    },
  );

  return transformTask(result);
}

export async function listScheduledTaskRunsApi(
  jobId: string,
): Promise<ScheduledTaskRunItem[]> {
  const result = await requestApi<ApiScheduledTaskRun[]>(
    `${SCHEDULED_TASKS_API_BASE_URL}/${encodeURIComponent(jobId)}/runs`,
    {
      method: "GET",
    },
  );

  return result.map(transformRun);
}

export async function retryScheduledTaskRunDeliveryApi(
  jobId: string,
  runId: string,
): Promise<ScheduledTaskRunItem> {
  const result = await requestApi<ApiScheduledTaskRun>(
    `${SCHEDULED_TASKS_API_BASE_URL}/${encodeURIComponent(jobId)}/runs/${encodeURIComponent(runId)}/delivery/retry`,
    {
      method: "POST",
      body: JSON.stringify({}),
    },
  );

  return transformRun(result);
}
