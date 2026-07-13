/**
 * 定时任务 API 服务模块
 *
 * 对齐 capability/scheduled/tasks 的结构化自动化任务接口。
 */

import { getAgentApiBaseUrl } from "@/config/runtime-endpoints";
import { requestApi } from "@/lib/api/core/http";
import { toTimestampOrNull } from "@/lib/api/core/timestamp";
import type {
  ApiScheduledTask,
  CreateScheduledTaskParams,
  DeleteScheduledTaskResponse,
  ListScheduledTasksParams,
  RecoverScheduledTaskRunParams,
  ScheduledTaskItem,
  UpdateScheduledTaskParams,
  UpdateScheduledTaskStatusParams,
} from "@/types/capability/scheduled-task/task";
import type {
  ApiScheduledTaskExecutionResult,
  ApiScheduledTaskRun,
  ScheduledTaskRunItem,
  ScheduledTaskRunNowResponse,
} from "@/types/capability/scheduled-task/run";

const AGENT_API_BASE_URL = getAgentApiBaseUrl();
const SCHEDULED_TASKS_API_BASE_URL = `${AGENT_API_BASE_URL}/capability/scheduled/tasks`;

function transformTask(apiTask: ApiScheduledTask): ScheduledTaskItem {
  return {
    ...apiTask,
    expires_at: toTimestampOrNull(apiTask.expires_at),
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
