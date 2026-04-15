/**
 * 定时任务 API 服务模块
 *
 * 对齐 capability/scheduled/tasks 的结构化自动化任务接口。
 */

import { get_agent_api_base_url } from "@/config/options";
import { request_api } from "@/lib/http";
import type {
  ApiScheduledTask,
  ApiScheduledTaskExecutionResult,
  ApiScheduledTaskRun,
  CreateScheduledTaskParams,
  DeleteScheduledTaskResponse,
  ListScheduledTasksParams,
  ScheduledTaskItem,
  ScheduledTaskRunItem,
  ScheduledTaskRunNowResponse,
  UpdateScheduledTaskParams,
  UpdateScheduledTaskStatusParams,
} from "@/types/scheduled-task";

const AGENT_API_BASE_URL = get_agent_api_base_url();
const SCHEDULED_TASKS_API_BASE_URL = `${AGENT_API_BASE_URL}/capability/scheduled/tasks`;

function to_timestamp(value?: string | null): number | null {
  if (!value) {
    return null;
  }
  const parsed = new Date(value).getTime();
  return Number.isFinite(parsed) ? parsed : null;
}

function transform_task(api_task: ApiScheduledTask): ScheduledTaskItem {
  return {
    ...api_task,
    next_run_at: to_timestamp(api_task.next_run_at),
    last_run_at: to_timestamp(api_task.last_run_at),
  };
}

function transform_run(api_run: ApiScheduledTaskRun): ScheduledTaskRunItem {
  return {
    ...api_run,
    scheduled_for: to_timestamp(api_run.scheduled_for),
    started_at: to_timestamp(api_run.started_at),
    finished_at: to_timestamp(api_run.finished_at),
  };
}

function transform_run_now_result(
  api_result: ApiScheduledTaskExecutionResult,
): ScheduledTaskRunNowResponse {
  return {
    ...api_result,
    scheduled_for: to_timestamp(api_result.scheduled_for),
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

export async function list_scheduled_tasks_api(
  params?: ListScheduledTasksParams,
): Promise<ScheduledTaskItem[]> {
  const result = await request_api<ApiScheduledTask[]>(
    `${SCHEDULED_TASKS_API_BASE_URL}${build_query({
      agent_id: params?.agent_id,
    })}`,
    {
      method: "GET",
      headers: { "Content-Type": "application/json" },
    },
  );

  return result.map(transform_task);
}

export async function create_scheduled_task_api(
  params: CreateScheduledTaskParams,
): Promise<ScheduledTaskItem> {
  const result = await request_api<ApiScheduledTask>(SCHEDULED_TASKS_API_BASE_URL, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(params),
  });

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
      headers: { "Content-Type": "application/json" },
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
      headers: { "Content-Type": "application/json" },
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
      headers: { "Content-Type": "application/json" },
    },
  );

  return transform_run_now_result(result);
}

export async function update_scheduled_task_status_api(
  job_id: string,
  params: UpdateScheduledTaskStatusParams,
): Promise<ScheduledTaskItem> {
  const result = await request_api<ApiScheduledTask>(
    `${SCHEDULED_TASKS_API_BASE_URL}/${encodeURIComponent(job_id)}/status`,
    {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
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
      headers: { "Content-Type": "application/json" },
    },
  );

  return result.map(transform_run);
}
