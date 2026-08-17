import type { PendingCommandState } from "./pending-command-model";

export const SCHEDULED_TASK_COMMAND_KINDS = ["delete", "permission", "run", "toggle"] as const;
export type ScheduledTaskCommandKind = typeof SCHEDULED_TASK_COMMAND_KINDS[number];

export interface ScheduledTaskFeedback {
  message: string;
  title: string;
  tone: "success" | "warning" | "error";
}

export type ScheduledTaskPendingCommands = PendingCommandState<ScheduledTaskCommandKind>;
