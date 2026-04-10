"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { resolveAgentId } from "@/config/options";
import { getHeartbeatConfigApi, wakeHeartbeatApi } from "@/lib/heartbeat-api";
import { listScheduledTasksApi } from "@/lib/scheduled-task-api";
import type { HeartbeatConfig, HeartbeatWakeResult, WakeHeartbeatRequest } from "@/types/heartbeat";
import type { ScheduledTaskItem } from "@/types/scheduled-task";

export interface UseAutomationControllerOptions {
  agent_id?: string | null;
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
  refresh_tasks: () => Promise<void>;
  refresh_all: () => Promise<void>;
  wake_heartbeat: (params?: WakeHeartbeatRequest) => Promise<HeartbeatWakeResult>;
}

export function useAutomationController(
  options: UseAutomationControllerOptions = {},
): AutomationController {
  const agent_id = resolveAgentId(options.agent_id);
  const [heartbeat, set_heartbeat] = useState<HeartbeatConfig | null>(null);
  const [scheduled_tasks, set_scheduled_tasks] = useState<ScheduledTaskItem[]>([]);
  const [heartbeat_loading, set_heartbeat_loading] = useState(true);
  const [tasks_loading, set_tasks_loading] = useState(true);
  const [heartbeat_error, set_heartbeat_error] = useState<string | null>(null);
  const [tasks_error, set_tasks_error] = useState<string | null>(null);
  const active_agent_id_ref = useRef(agent_id);
  const heartbeat_request_token_ref = useRef(0);
  const tasks_request_token_ref = useRef(0);

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
      const result = await getHeartbeatConfigApi(request_agent_id);
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

  const refresh_tasks = useCallback(async () => {
    const request_agent_id = agent_id;
    const request_token = tasks_request_token_ref.current + 1;
    tasks_request_token_ref.current = request_token;
    set_tasks_loading(true);
    set_tasks_error(null);
    try {
      const result = await listScheduledTasksApi({ agent_id: request_agent_id });
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
      set_tasks_loading(false);
    }
  }, [agent_id]);

  const refresh_all = useCallback(async () => {
    await Promise.all([refresh_heartbeat(), refresh_tasks()]);
  }, [refresh_heartbeat, refresh_tasks]);

  const wake_heartbeat = useCallback(async (params: WakeHeartbeatRequest = {}) => {
    const request_agent_id = agent_id;
    const result = await wakeHeartbeatApi(request_agent_id, params);
    // 中文注释：wake 只会改变运行态，不会改写持久化配置，因此触发后立即刷新 heartbeat 即可。
    if (active_agent_id_ref.current === request_agent_id) {
      await refresh_heartbeat();
    }
    return result;
  }, [agent_id, refresh_heartbeat]);

  useEffect(() => {
    void refresh_all().catch(() => undefined);
  }, [refresh_all]);

  const visible_heartbeat = heartbeat?.agent_id === agent_id ? heartbeat : null;
  const visible_scheduled_tasks = scheduled_tasks.every((item) => item.agent_id === agent_id)
    ? scheduled_tasks
    : [];

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
  };
}
