"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { resolveAgentId } from "@/config/options";
import { listScheduledTasksApi } from "@/lib/api/capability/scheduled-task-api";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";

interface RefreshScheduledTasksOptions {
  silent?: boolean;
}

function upsertScheduledTask(
  items: ScheduledTaskItem[],
  nextTask: ScheduledTaskItem,
): ScheduledTaskItem[] {
  const taskIndex = items.findIndex((item) => item.job_id === nextTask.job_id);
  if (taskIndex < 0) {
    return [nextTask, ...items];
  }
  return items.map((item, index) => (index === taskIndex ? nextTask : item));
}

export function useScheduledTasksResource() {
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [items, setItems] = useState<ScheduledTaskItem[]>([]);
  const isMountedRef = useRef(false);
  const requestVersionRef = useRef(0);

  const isCurrentRequest = useCallback((version: number): boolean => (
    isMountedRef.current && requestVersionRef.current === version
  ), []);

  const refresh = useCallback(async (
    options: RefreshScheduledTasksOptions = {},
  ): Promise<void> => {
    const version = requestVersionRef.current + 1;
    requestVersionRef.current = version;
    if (!options.silent) {
      setIsLoading(true);
      setErrorMessage(null);
    }
    try {
      const result = await listScheduledTasksApi();
      if (isCurrentRequest(version)) {
        setItems(result);
      }
    } catch (error) {
      if (isCurrentRequest(version) && !options.silent) {
        setErrorMessage(error instanceof Error ? error.message : "加载定时任务失败");
      }
      throw error;
    } finally {
      if (isCurrentRequest(version) && !options.silent) {
        setIsLoading(false);
      }
    }
  }, [isCurrentRequest]);

  const commitItems = useCallback((
    update: (currentItems: ScheduledTaskItem[]) => ScheduledTaskItem[],
  ): void => {
    // 本地命令结果推进请求代次，避免更早的列表响应回滚已确认状态。
    requestVersionRef.current += 1;
    setErrorMessage(null);
    setIsLoading(false);
    setItems(update);
  }, []);

  const upsertTask = useCallback((task: ScheduledTaskItem): void => {
    commitItems((currentItems) => upsertScheduledTask(currentItems, task));
  }, [commitItems]);

  const removeTask = useCallback((jobId: string): void => {
    commitItems((currentItems) => (
      currentItems.filter((item) => item.job_id !== jobId)
    ));
  }, [commitItems]);

  useEffect(() => {
    isMountedRef.current = true;
    void refresh().catch((error: unknown) => {
      console.debug("[scheduled-tasks] Initial load failed:", error);
    });
    return () => {
      isMountedRef.current = false;
      requestVersionRef.current += 1;
    };
  }, [refresh]);

  return {
    agentId: resolveAgentId(),
    errorMessage,
    isLoading,
    items,
    refresh,
    removeTask,
    upsertTask,
  };
}
