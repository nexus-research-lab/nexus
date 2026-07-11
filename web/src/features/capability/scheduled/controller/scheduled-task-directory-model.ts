import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";

import type { PendingCommandState } from "./pending-command-model";

export const SCHEDULED_TASK_COMMAND_KINDS = ["delete", "run", "toggle"] as const;
export type ScheduledTaskCommandKind = typeof SCHEDULED_TASK_COMMAND_KINDS[number];

export interface ScheduledTaskFeedback {
  message: string;
  title: string;
  tone: "success" | "warning" | "error";
}

export interface ScheduledTaskMetrics {
  enabled: number;
  paused: number;
  running: number;
}

export type ScheduledTaskPendingCommands = PendingCommandState<ScheduledTaskCommandKind>;

export function getScheduledTaskMetrics(
  items: ScheduledTaskItem[],
): ScheduledTaskMetrics {
  const counts = items.reduce(
    (current, task) => ({
      enabled: current.enabled + Number(task.enabled),
      running: current.running + Number(task.running),
    }),
    { enabled: 0, running: 0 },
  );
  return {
    ...counts,
    paused: items.length - counts.enabled,
  };
}
