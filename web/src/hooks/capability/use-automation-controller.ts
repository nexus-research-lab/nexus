"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { resolve_agent_id } from "@/config/options";
import { useResettableState } from "@/hooks/ui/use-resettable-state";
import {
  get_heartbeat_config_api,
  update_heartbeat_api,
  wake_heartbeat_api,
} from "@/lib/api/heartbeat-api";
import {
  create_scheduled_task_api,
  delete_scheduled_task_api,
  list_scheduled_tasks_api,
  run_scheduled_task_api,
  update_scheduled_task_api,
  update_scheduled_task_status_api,
} from "@/lib/api/scheduled-task-api";
import type {
  HeartbeatConfig,
  HeartbeatUpdateInput,
  HeartbeatWakeResult,
  WakeHeartbeatRequest,
} from "@/types/capability/heartbeat";
import type {
  CreateScheduledTaskParams,
  DeleteScheduledTaskResponse,
  ScheduledTaskItem,
  ScheduledTaskRunNowResponse,
  UpdateScheduledTaskParams,
} from "@/types/capability/scheduled-task";

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
  update_heartbeat: (payload: HeartbeatUpdateInput) => Promise<HeartbeatConfig>;
  create_task: (params: CreateScheduledTaskParams) => Promise<ScheduledTaskItem>;
  update_task: (jobId: string, params: UpdateScheduledTaskParams) => Promise<ScheduledTaskItem>;
  delete_task: (jobId: string) => Promise<DeleteScheduledTaskResponse>;
  toggle_task: (task: ScheduledTaskItem) => Promise<ScheduledTaskItem>;
  run_task: (task: ScheduledTaskItem) => Promise<ScheduledTaskRunNowResponse>;
}

function upsertTask(items: ScheduledTaskItem[], nextTask: ScheduledTaskItem): ScheduledTaskItem[] {
  const nextIndex = items.findIndex((item) => item.job_id === nextTask.job_id);
  if (nextIndex < 0) {
    return [nextTask, ...items];
  }

  return items.map((item, index) => (index === nextIndex ? nextTask : item));
}

export function useAutomationController(
  options: UseAutomationControllerOptions = {},
): AutomationController {
  const agentId = resolve_agent_id(options.agent_id);
  const includeAllTasks = Boolean(options.include_all_tasks);
  const [heartbeat, setHeartbeat] = useResettableState<HeartbeatConfig | null>(null, agentId);
  const [scheduledTasks, setScheduledTasks] = useResettableState<ScheduledTaskItem[]>([], agentId);
  const [heartbeatLoading, setHeartbeatLoading] = useResettableState(true, agentId);
  const [tasksLoading, setTasksLoading] = useResettableState(true, agentId);
  const [heartbeatError, setHeartbeatError] = useResettableState<string | null>(null, agentId);
  const [tasksError, setTasksError] = useResettableState<string | null>(null, agentId);
  const activeAgentIdRef = useRef(agentId);
  const heartbeatRequestTokenRef = useRef(0);
  const tasksRequestTokenRef = useRef(0);

  const commitTasksState = useCallback(
    (updater: (currentItems: ScheduledTaskItem[]) => ScheduledTaskItem[]) => {
      tasksRequestTokenRef.current += 1;
      setTasksLoading(false);
      setTasksError(null);
      setScheduledTasks((currentItems) => updater(currentItems));
    },
    [],
  );

  function isActiveHeartbeatRequest(requestAgentId: string, requestToken: number): boolean {
    return (
      activeAgentIdRef.current === requestAgentId
      && heartbeatRequestTokenRef.current === requestToken
    );
  }

  function isActiveTasksRequest(requestAgentId: string, requestToken: number): boolean {
    return (
      activeAgentIdRef.current === requestAgentId
      && tasksRequestTokenRef.current === requestToken
    );
  }

  useEffect(() => {
    activeAgentIdRef.current = agentId;
    heartbeatRequestTokenRef.current += 1;
    tasksRequestTokenRef.current += 1;
  }, [agentId]);

  const refreshHeartbeat = useCallback(async () => {
    const requestAgentId = agentId;
    const requestToken = heartbeatRequestTokenRef.current + 1;
    heartbeatRequestTokenRef.current = requestToken;
    setHeartbeatLoading(true);
    setHeartbeatError(null);
    try {
      const result = await get_heartbeat_config_api(requestAgentId);
      // agent 切换或新的刷新请求会推进 token，旧响应必须被静默丢弃，避免串写到当前视图。
      if (!isActiveHeartbeatRequest(requestAgentId, requestToken)) {
        return;
      }
      setHeartbeat(result);
    } catch (error) {
      if (!isActiveHeartbeatRequest(requestAgentId, requestToken)) {
        return;
      }
      setHeartbeatError(error instanceof Error ? error.message : "加载 heartbeat 失败");
    } finally {
      if (!isActiveHeartbeatRequest(requestAgentId, requestToken)) {
        return;
      }
      setHeartbeatLoading(false);
    }
  }, [agentId]);

  const refreshTasks = useCallback(async (options?: { silent?: boolean }) => {
    const requestAgentId = agentId;
    const requestToken = tasksRequestTokenRef.current + 1;
    tasksRequestTokenRef.current = requestToken;
    if (!options?.silent) {
      setTasksLoading(true);
    }
    setTasksError(null);
    try {
      const result = await list_scheduled_tasks_api(includeAllTasks ? undefined : { agent_id: requestAgentId });
      // 任务列表同样按 agent_id 绑定，只允许最后一次有效请求落状态。
      if (!isActiveTasksRequest(requestAgentId, requestToken)) {
        return;
      }
      setScheduledTasks(result);
    } catch (error) {
      if (!isActiveTasksRequest(requestAgentId, requestToken)) {
        return;
      }
      setTasksError(error instanceof Error ? error.message : "加载定时任务失败");
      throw error;
    } finally {
      if (!isActiveTasksRequest(requestAgentId, requestToken)) {
        return;
      }
      if (!options?.silent) {
        setTasksLoading(false);
      }
    }
  }, [agentId, includeAllTasks]);

  const refreshAll = useCallback(async () => {
    const results = await Promise.allSettled([refreshHeartbeat(), refreshTasks()]);
    const failed = results.filter((r): r is PromiseRejectedResult => r.status === "rejected");
    if (failed.length > 0) {
      console.warn("[useAutomationController] refresh_all partial failure:", failed.map((r) => r.reason));
    }
  }, [refreshHeartbeat, refreshTasks]);

  const wakeHeartbeat = useCallback(async (params: WakeHeartbeatRequest = {}) => {
    const requestAgentId = agentId;
    const result = await wake_heartbeat_api(requestAgentId, params);
    // wake 只会改变运行态，不会改写持久化配置，因此触发后立即刷新 heartbeat 即可。
    if (activeAgentIdRef.current === requestAgentId) {
      await refreshHeartbeat();
    }
    return result;
  }, [agentId, refreshHeartbeat]);

  const updateHeartbeat = useCallback(async (payload: HeartbeatUpdateInput) => {
    const requestAgentId = agentId;
    const nextConfig = await update_heartbeat_api(requestAgentId, payload);
    // PUT 直接返回最新状态，落到当前 agent 的视图里；旧 agent 响应不能串写。
    if (activeAgentIdRef.current === requestAgentId) {
      heartbeatRequestTokenRef.current += 1;
      setHeartbeat(nextConfig);
      setHeartbeatError(null);
    }
    return nextConfig;
  }, [agentId]);

  const createTask = useCallback(async (params: CreateScheduledTaskParams) => {
    const requestAgentId = agentId;
    const createdTask = await create_scheduled_task_api(params);
    if (
      activeAgentIdRef.current === requestAgentId
      && (includeAllTasks || requestAgentId === createdTask.agent_id)
    ) {
      // 本地写入会推进 token，确保较早发起的列表刷新结果不会回滚最新任务状态。
      commitTasksState((currentItems) => upsertTask(currentItems, createdTask));
      await refreshTasks().catch((err: unknown) => console.debug("[useAutomationController] background refresh failed:", err));
    }
    return createdTask;
  }, [agentId, commitTasksState, includeAllTasks, refreshTasks]);

  const updateTask = useCallback(async (jobId: string, params: UpdateScheduledTaskParams) => {
    const requestAgentId = agentId;
    const updatedTask = await update_scheduled_task_api(jobId, params);
    if (
      activeAgentIdRef.current === requestAgentId
      && (includeAllTasks || requestAgentId === updatedTask.agent_id)
    ) {
      commitTasksState((currentItems) => upsertTask(currentItems, updatedTask));
      await refreshTasks().catch((err: unknown) => console.debug("[useAutomationController] background refresh failed:", err));
    }
    return updatedTask;
  }, [agentId, commitTasksState, includeAllTasks, refreshTasks]);

  const deleteTask = useCallback(async (jobId: string) => {
    const requestAgentId = agentId;
    const deletedTask = await delete_scheduled_task_api(jobId);
    if (activeAgentIdRef.current === requestAgentId) {
      commitTasksState((currentItems) => currentItems.filter((item) => item.job_id !== jobId));
      await refreshTasks().catch((err: unknown) => console.debug("[useAutomationController] background refresh failed:", err));
    }
    return deletedTask;
  }, [agentId, commitTasksState, refreshTasks]);

  const toggleTask = useCallback(async (task: ScheduledTaskItem) => {
    const requestAgentId = agentId;
    const updatedTask = await update_scheduled_task_status_api(task.job_id, {
      enabled: !task.enabled,
    });
    if (
      activeAgentIdRef.current === requestAgentId
      && (includeAllTasks || requestAgentId === updatedTask.agent_id)
    ) {
      commitTasksState((currentItems) => upsertTask(currentItems, updatedTask));
      await refreshTasks().catch((err: unknown) => console.debug("[useAutomationController] background refresh failed:", err));
    }
    return updatedTask;
  }, [agentId, commitTasksState, includeAllTasks, refreshTasks]);

  const runTask = useCallback(async (task: ScheduledTaskItem) => {
    const requestAgentId = agentId;
    const result = await run_scheduled_task_api(task.job_id);
    if (activeAgentIdRef.current === requestAgentId) {
      await refreshTasks().catch((err: unknown) => console.debug("[useAutomationController] background refresh failed:", err));
    }
    return result;
  }, [agentId, refreshTasks]);

  useEffect(() => {
    void refreshAll().catch((err: unknown) => console.debug("[useAutomationController] initial load failed:", err));
  }, [refreshAll]);

  const visibleHeartbeat = heartbeat?.agent_id === agentId ? heartbeat : null;
  const visibleScheduledTasks = includeAllTasks
    ? scheduledTasks
    : (scheduledTasks.every((item) => item.agent_id === agentId) ? scheduledTasks : []);

  return {
    agent_id: agentId,
    heartbeat: visibleHeartbeat,
    scheduled_tasks: visibleScheduledTasks,
    loading: heartbeatLoading || tasksLoading,
    heartbeat_loading: heartbeatLoading,
    tasks_loading: tasksLoading,
    heartbeat_error: heartbeatError,
    tasks_error: tasksError,
    refresh_heartbeat: refreshHeartbeat,
    refresh_tasks: refreshTasks,
    refresh_all: refreshAll,
    wake_heartbeat: wakeHeartbeat,
    update_heartbeat: updateHeartbeat,
    create_task: createTask,
    update_task: updateTask,
    delete_task: deleteTask,
    toggle_task: toggleTask,
    run_task: runTask,
  };
}
