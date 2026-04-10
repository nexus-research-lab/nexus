"use client";

import { useCallback, useEffect, useState } from "react";

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

  const refresh_heartbeat = useCallback(async () => {
    set_heartbeat_loading(true);
    set_heartbeat_error(null);
    try {
      const result = await getHeartbeatConfigApi(agent_id);
      set_heartbeat(result);
    } catch (error) {
      set_heartbeat_error(error instanceof Error ? error.message : "加载 heartbeat 失败");
    } finally {
      set_heartbeat_loading(false);
    }
  }, [agent_id]);

  const refresh_tasks = useCallback(async () => {
    set_tasks_loading(true);
    set_tasks_error(null);
    try {
      const result = await listScheduledTasksApi({ agent_id });
      set_scheduled_tasks(result);
    } catch (error) {
      set_tasks_error(error instanceof Error ? error.message : "加载定时任务失败");
    } finally {
      set_tasks_loading(false);
    }
  }, [agent_id]);

  const refresh_all = useCallback(async () => {
    await Promise.all([refresh_heartbeat(), refresh_tasks()]);
  }, [refresh_heartbeat, refresh_tasks]);

  const wake_heartbeat = useCallback(async (params: WakeHeartbeatRequest = {}) => {
    const result = await wakeHeartbeatApi(agent_id, params);
    // 中文注释：wake 只会改变运行态，不会改写持久化配置，因此触发后立即刷新 heartbeat 即可。
    await refresh_heartbeat();
    return result;
  }, [agent_id, refresh_heartbeat]);

  useEffect(() => {
    void refresh_all();
  }, [refresh_all]);

  return {
    agent_id,
    heartbeat,
    scheduled_tasks,
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
