"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { resolveAgentId } from "@/config/options";
import { useResettableState } from "@/hooks/ui/use-resettable-state";
import {
  getHeartbeatConfigApi,
  updateHeartbeatApi,
  wakeHeartbeatApi,
} from "@/lib/api/heartbeat-api";
import {
  createScheduledTaskApi,
  deleteScheduledTaskApi,
  listScheduledTasksApi,
  runScheduledTaskApi,
  updateScheduledTaskApi,
  updateScheduledTaskStatusApi,
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
  agentId?: string | null;
  includeAllTasks?: boolean;
}

export interface AutomationController {
  agentId: string;
  heartbeat: HeartbeatConfig | null;
  scheduledTasks: ScheduledTaskItem[];
  loading: boolean;
  heartbeatLoading: boolean;
  tasksLoading: boolean;
  heartbeatError: string | null;
  tasksError: string | null;
  refreshHeartbeat: () => Promise<void>;
  refreshTasks: (options?: { silent?: boolean }) => Promise<void>;
  refreshAll: () => Promise<void>;
  wakeHeartbeat: (params?: WakeHeartbeatRequest) => Promise<HeartbeatWakeResult>;
  updateHeartbeat: (payload: HeartbeatUpdateInput) => Promise<HeartbeatConfig>;
  createTask: (params: CreateScheduledTaskParams) => Promise<ScheduledTaskItem>;
  updateTask: (jobId: string, params: UpdateScheduledTaskParams) => Promise<ScheduledTaskItem>;
  deleteTask: (jobId: string) => Promise<DeleteScheduledTaskResponse>;
  toggleTask: (task: ScheduledTaskItem) => Promise<ScheduledTaskItem>;
  runTask: (task: ScheduledTaskItem) => Promise<ScheduledTaskRunNowResponse>;
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
  const agentId = resolveAgentId(options.agentId);
  const includeAllTasks = Boolean(options.includeAllTasks);
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
    [setScheduledTasks, setTasksError, setTasksLoading],
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
  }, [agentId, setHeartbeat, setHeartbeatError, setHeartbeatLoading]);

  const refreshHeartbeat = useCallback(async () => {
    const requestAgentId = agentId;
    const requestToken = heartbeatRequestTokenRef.current + 1;
    heartbeatRequestTokenRef.current = requestToken;
    setHeartbeatLoading(true);
    setHeartbeatError(null);
    try {
      const result = await getHeartbeatConfigApi(requestAgentId);
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
  }, [agentId, setHeartbeat, setHeartbeatError, setHeartbeatLoading]);

  const refreshTasks = useCallback(async (options?: { silent?: boolean }) => {
    const requestAgentId = agentId;
    const requestToken = tasksRequestTokenRef.current + 1;
    tasksRequestTokenRef.current = requestToken;
    if (!options?.silent) {
      setTasksLoading(true);
    }
    setTasksError(null);
    try {
      const result = await listScheduledTasksApi(includeAllTasks ? undefined : { agent_id: requestAgentId });
      // 任务列表同样按 agentId 绑定，只允许最后一次有效请求落状态。
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
  }, [
    agentId,
    includeAllTasks,
    setScheduledTasks,
    setTasksError,
    setTasksLoading,
  ]);

  const refreshAll = useCallback(async () => {
    const results = await Promise.allSettled([refreshHeartbeat(), refreshTasks()]);
    const failed = results.filter((r): r is PromiseRejectedResult => r.status === "rejected");
    if (failed.length > 0) {
      console.warn("[useAutomationController] refresh_all partial failure:", failed.map((r) => r.reason));
    }
  }, [refreshHeartbeat, refreshTasks]);

  const wakeHeartbeat = useCallback(async (params: WakeHeartbeatRequest = {}) => {
    const requestAgentId = agentId;
    const result = await wakeHeartbeatApi(requestAgentId, params);
    // wake 只会改变运行态，不会改写持久化配置，因此触发后立即刷新 heartbeat 即可。
    if (activeAgentIdRef.current === requestAgentId) {
      await refreshHeartbeat();
    }
    return result;
  }, [agentId, refreshHeartbeat]);

  const updateHeartbeat = useCallback(async (payload: HeartbeatUpdateInput) => {
    const requestAgentId = agentId;
    const nextConfig = await updateHeartbeatApi(requestAgentId, payload);
    // PUT 直接返回最新状态，落到当前 agent 的视图里；旧 agent 响应不能串写。
    if (activeAgentIdRef.current === requestAgentId) {
      heartbeatRequestTokenRef.current += 1;
      setHeartbeat(nextConfig);
      setHeartbeatError(null);
    }
    return nextConfig;
  }, [agentId, setHeartbeat, setHeartbeatError]);

  const createTask = useCallback(async (params: CreateScheduledTaskParams) => {
    const requestAgentId = agentId;
    const createdTask = await createScheduledTaskApi(params);
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
    const updatedTask = await updateScheduledTaskApi(jobId, params);
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
    const deletedTask = await deleteScheduledTaskApi(jobId);
    if (activeAgentIdRef.current === requestAgentId) {
      commitTasksState((currentItems) => currentItems.filter((item) => item.job_id !== jobId));
      await refreshTasks().catch((err: unknown) => console.debug("[useAutomationController] background refresh failed:", err));
    }
    return deletedTask;
  }, [agentId, commitTasksState, refreshTasks]);

  const toggleTask = useCallback(async (task: ScheduledTaskItem) => {
    const requestAgentId = agentId;
    const updatedTask = await updateScheduledTaskStatusApi(task.job_id, {
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
    const result = await runScheduledTaskApi(task.job_id);
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
    agentId: agentId,
    heartbeat: visibleHeartbeat,
    scheduledTasks: visibleScheduledTasks,
    loading: heartbeatLoading || tasksLoading,
    heartbeatLoading: heartbeatLoading,
    tasksLoading: tasksLoading,
    heartbeatError: heartbeatError,
    tasksError: tasksError,
    refreshHeartbeat: refreshHeartbeat,
    refreshTasks: refreshTasks,
    refreshAll: refreshAll,
    wakeHeartbeat: wakeHeartbeat,
    updateHeartbeat: updateHeartbeat,
    createTask: createTask,
    updateTask: updateTask,
    deleteTask: deleteTask,
    toggleTask: toggleTask,
    runTask: runTask,
  };
}
