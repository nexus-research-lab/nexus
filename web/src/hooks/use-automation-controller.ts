"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { resolve_agent_id } from "@/config/options";
import { get_heartbeat_config_api, wake_heartbeat_api } from "@/lib/heartbeat-api";
import {
  create_scheduled_task_api,
  delete_scheduled_task_api,
  list_scheduled_tasks_api,
  run_scheduled_task_api,
  update_scheduled_task_api,
  update_scheduled_task_status_api,
} from "@/lib/scheduled-task-api";
import type { HeartbeatConfig, HeartbeatWakeResult, WakeHeartbeatRequest } from "@/types/heartbeat";
import type {
  CreateScheduledTaskParams,
  DeleteScheduledTaskResponse,
  ScheduledTaskItem,
  ScheduledTaskRunNowResponse,
  UpdateScheduledTaskParams,
} from "@/types/scheduled-task";

export interface UseAutomationControllerOptions {
  agent_id?: string | null;
  include_all_tasks?: boolean;
}

export interface AutomationController {
  agent_id: string;
  heartbeat: HeartbeatConfig | null;
  scheduled_tasks: ScheduledTaskItem[];
  loading: boolean;
  heartbeat_loading: boolean;
  tasks_loading: boolean;
  heartbeat_error: string | null;
  tasks_error: string | null;
  refresh_heartbeat: () => Promise<void>;
  refresh_tasks: (options?: { silent?: boolean }) => Promise<void>;
  refresh_all: () => Promise<void>;
  wake_heartbeat: (params?: WakeHeartbeatRequest) => Promise<HeartbeatWakeResult>;
  create_task: (params: CreateScheduledTaskParams) => Promise<ScheduledTaskItem>;
  update_task: (job_id: string, params: UpdateScheduledTaskParams) => Promise<ScheduledTaskItem>;
  delete_task: (job_id: string) => Promise<DeleteScheduledTaskResponse>;
  toggle_task: (task: ScheduledTaskItem) => Promise<ScheduledTaskItem>;
  run_task: (task: ScheduledTaskItem) => Promise<ScheduledTaskRunNowResponse>;
}

function upsert_task(items: ScheduledTaskItem[], next_task: ScheduledTaskItem): ScheduledTaskItem[] {
  const next_index = items.findIndex((item) => item.job_id === next_task.job_id);
  if (next_index < 0) {
    return [next_task, ...items];
  }

  return items.map((item, index) => (index === next_index ? next_task : item));
}

export function useAutomationController(
  options: UseAutomationControllerOptions = {},
): AutomationController {
  const agent_id = resolve_agent_id(options.agent_id);
  const include_all_tasks = Boolean(options.include_all_tasks);
  const [heartbeat, set_heartbeat] = useState<HeartbeatConfig | null>(null);
  const [scheduled_tasks, set_scheduled_tasks] = useState<ScheduledTaskItem[]>([]);
  const [heartbeat_loading, set_heartbeat_loading] = useState(true);
  const [tasks_loading, set_tasks_loading] = useState(true);
  const [heartbeat_error, set_heartbeat_error] = useState<string | null>(null);
  const [tasks_error, set_tasks_error] = useState<string | null>(null);
  const active_agent_id_ref = useRef(agent_id);
  const heartbeat_request_token_ref = useRef(0);
  const tasks_request_token_ref = useRef(0);

  const commit_tasks_state = useCallback(
    (updater: (current_items: ScheduledTaskItem[]) => ScheduledTaskItem[]) => {
      tasks_request_token_ref.current += 1;
      set_tasks_loading(false);
      set_tasks_error(null);
      set_scheduled_tasks((current_items) => updater(current_items));
    },
    [],
  );

  function is_active_heartbeat_request(request_agent_id: string, request_token: number): boolean {
    return (
      active_agent_id_ref.current === request_agent_id
      && heartbeat_request_token_ref.current === request_token
    );
  }

  function is_active_tasks_request(request_agent_id: string, request_token: number): boolean {
    return (
      active_agent_id_ref.current === request_agent_id
      && tasks_request_token_ref.current === request_token
    );
  }

  useEffect(() => {
    active_agent_id_ref.current = agent_id;
    heartbeat_request_token_ref.current += 1;
    tasks_request_token_ref.current += 1;
    set_heartbeat(null);
    set_scheduled_tasks([]);
    set_heartbeat_error(null);
    set_tasks_error(null);
    set_heartbeat_loading(true);
    set_tasks_loading(true);
  }, [agent_id]);

  const refresh_heartbeat = useCallback(async () => {
    const request_agent_id = agent_id;
    const request_token = heartbeat_request_token_ref.current + 1;
    heartbeat_request_token_ref.current = request_token;
    set_heartbeat_loading(true);
    set_heartbeat_error(null);
    try {
      const result = await get_heartbeat_config_api(request_agent_id);
      // 中文注释：agent 切换或新的刷新请求会推进 token，旧响应必须被静默丢弃，避免串写到当前视图。
      if (!is_active_heartbeat_request(request_agent_id, request_token)) {
        return;
      }
      set_heartbeat(result);
    } catch (error) {
      if (!is_active_heartbeat_request(request_agent_id, request_token)) {
        return;
      }
      set_heartbeat_error(error instanceof Error ? error.message : "加载 heartbeat 失败");
    } finally {
      if (!is_active_heartbeat_request(request_agent_id, request_token)) {
        return;
      }
      set_heartbeat_loading(false);
    }
  }, [agent_id]);

  const refresh_tasks = useCallback(async (options?: { silent?: boolean }) => {
    const request_agent_id = agent_id;
    const request_token = tasks_request_token_ref.current + 1;
    tasks_request_token_ref.current = request_token;
    if (!options?.silent) {
      set_tasks_loading(true);
    }
    set_tasks_error(null);
    try {
      const result = await list_scheduled_tasks_api(include_all_tasks ? undefined : { agent_id: request_agent_id });
      // 中文注释：任务列表同样按 agent_id 绑定，只允许最后一次有效请求落状态。
      if (!is_active_tasks_request(request_agent_id, request_token)) {
        return;
      }
      set_scheduled_tasks(result);
    } catch (error) {
      if (!is_active_tasks_request(request_agent_id, request_token)) {
        return;
      }
      set_tasks_error(error instanceof Error ? error.message : "加载定时任务失败");
      throw error;
    } finally {
      if (!is_active_tasks_request(request_agent_id, request_token)) {
        return;
      }
      if (!options?.silent) {
        set_tasks_loading(false);
      }
    }
  }, [agent_id, include_all_tasks]);

  const refresh_all = useCallback(async () => {
    await Promise.all([refresh_heartbeat(), refresh_tasks()]);
  }, [refresh_heartbeat, refresh_tasks]);

  const wake_heartbeat = useCallback(async (params: WakeHeartbeatRequest = {}) => {
    const request_agent_id = agent_id;
    const result = await wake_heartbeat_api(request_agent_id, params);
    // 中文注释：wake 只会改变运行态，不会改写持久化配置，因此触发后立即刷新 heartbeat 即可。
    if (active_agent_id_ref.current === request_agent_id) {
      await refresh_heartbeat();
    }
    return result;
  }, [agent_id, refresh_heartbeat]);

  const create_task = useCallback(async (params: CreateScheduledTaskParams) => {
    const request_agent_id = agent_id;
    const created_task = await create_scheduled_task_api(params);
    if (
      active_agent_id_ref.current === request_agent_id
      && (include_all_tasks || request_agent_id === created_task.agent_id)
    ) {
      // 中文注释：本地写入会推进 token，确保较早发起的列表刷新结果不会回滚最新任务状态。
      commit_tasks_state((current_items) => upsert_task(current_items, created_task));
      await refresh_tasks().catch(() => undefined);
    }
    return created_task;
  }, [agent_id, commit_tasks_state, include_all_tasks, refresh_tasks]);

  const update_task = useCallback(async (job_id: string, params: UpdateScheduledTaskParams) => {
    const request_agent_id = agent_id;
    const updated_task = await update_scheduled_task_api(job_id, params);
    if (
      active_agent_id_ref.current === request_agent_id
      && (include_all_tasks || request_agent_id === updated_task.agent_id)
    ) {
      commit_tasks_state((current_items) => upsert_task(current_items, updated_task));
      await refresh_tasks().catch(() => undefined);
    }
    return updated_task;
  }, [agent_id, commit_tasks_state, include_all_tasks, refresh_tasks]);

  const delete_task = useCallback(async (job_id: string) => {
    const request_agent_id = agent_id;
    const deleted_task = await delete_scheduled_task_api(job_id);
    if (active_agent_id_ref.current === request_agent_id) {
      commit_tasks_state((current_items) => current_items.filter((item) => item.job_id !== job_id));
      await refresh_tasks().catch(() => undefined);
    }
    return deleted_task;
  }, [agent_id, commit_tasks_state, refresh_tasks]);

  const toggle_task = useCallback(async (task: ScheduledTaskItem) => {
    const request_agent_id = agent_id;
    const updated_task = await update_scheduled_task_status_api(task.job_id, {
      enabled: !task.enabled,
    });
    if (
      active_agent_id_ref.current === request_agent_id
      && (include_all_tasks || request_agent_id === updated_task.agent_id)
    ) {
      commit_tasks_state((current_items) => upsert_task(current_items, updated_task));
      await refresh_tasks().catch(() => undefined);
    }
    return updated_task;
  }, [agent_id, commit_tasks_state, include_all_tasks, refresh_tasks]);

  const run_task = useCallback(async (task: ScheduledTaskItem) => {
    const request_agent_id = agent_id;
    const result = await run_scheduled_task_api(task.job_id);
    if (active_agent_id_ref.current === request_agent_id) {
      await refresh_tasks().catch(() => undefined);
    }
    return result;
  }, [agent_id, refresh_tasks]);

  useEffect(() => {
    void refresh_all().catch(() => undefined);
  }, [refresh_all]);

  const visible_heartbeat = heartbeat?.agent_id === agent_id ? heartbeat : null;
  const visible_scheduled_tasks = include_all_tasks
    ? scheduled_tasks
    : (scheduled_tasks.every((item) => item.agent_id === agent_id) ? scheduled_tasks : []);

  return {
    agent_id,
    heartbeat: visible_heartbeat,
    scheduled_tasks: visible_scheduled_tasks,
    loading: heartbeat_loading || tasks_loading,
    heartbeat_loading,
    tasks_loading,
    heartbeat_error,
    tasks_error,
    refresh_heartbeat,
    refresh_tasks,
    refresh_all,
    wake_heartbeat,
    create_task,
    update_task,
    delete_task,
    toggle_task,
    run_task,
  };
}
