// INPUT: Scheduled 变更事件与任务列表静默读取动作。
// OUTPUT: 由 `scheduled_task_changed` 驱动的无轮询刷新，并同步能力摘要失效信号。
// POS: Scheduled 实时失效适配层；合并、单飞与重连补读由 useAppEventRefresh 统一持有。

"use client";

import { useCallback } from "react";

import { useAppEventRefresh } from "@/lib/websocket/use-app-event-refresh";
import type { EventMessage } from "@/types/generated/protocol";

import { notifyCapabilitySummaryMutated } from "../capability-summary-events";

interface ScheduledTaskRealtimeRefreshOptions {
  refreshTasks: (options?: { silent?: boolean }) => Promise<unknown>;
}

export function useScheduledTaskRealtimeRefresh({
  refreshTasks,
}: ScheduledTaskRealtimeRefreshOptions): void {
  const handleEvent = useCallback((event: EventMessage): boolean => {
    if (event.event_type !== "scheduled_task_changed") {
      return false;
    }
    // 摘要失效与页面可见性无关；列表重读由协调器按可见性延后。
    notifyCapabilitySummaryMutated({
      agent_id: event.agent_id ?? "",
      source: "scheduled_tasks",
    });
    return true;
  }, []);
  const refresh = useCallback(() => refreshTasks({ silent: true }), [refreshTasks]);

  useAppEventRefresh({ onEvent: handleEvent, refresh, revalidateOnFocus: true });
}
