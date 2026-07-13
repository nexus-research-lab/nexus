import type {
  SubagentTask,
  SubagentTaskListResponse,
} from "@/types/conversation/subagent-task";

import {
  isSubagentTaskActive,
  subagentTaskTimestamp,
} from "./subagent-task-model";

export type SubagentTaskListEmptyState = "empty" | "loading";
export type SubagentTaskSupportNotice = "claude" | "generic" | null;

export interface SubagentTaskListModel {
  activeEmptyState: SubagentTaskListEmptyState;
  activeTasks: SubagentTask[];
  completedTasks: SubagentTask[];
  supportNotice: SubagentTaskSupportNotice;
}

interface BuildSubagentTaskListModelOptions {
  data: SubagentTaskListResponse | null;
  isLoading: boolean;
  tasks: SubagentTask[];
}

export function buildSubagentTaskListModel({
  data,
  isLoading,
  tasks,
}: BuildSubagentTaskListModelOptions): SubagentTaskListModel {
  const supportNotice = resolveSupportNotice(data);
  const groups = groupTasksByActivity(supportNotice ? [] : tasks);
  return {
    activeEmptyState: isLoading && !data ? "loading" : "empty",
    activeTasks: groups.active,
    completedTasks: groups.completed,
    supportNotice,
  };
}

function resolveSupportNotice(
  data: SubagentTaskListResponse | null,
): SubagentTaskSupportNotice {
  if (!data || data.capabilities.observe) {
    return null;
  }
  return data.runtime_kind === "claude" ? "claude" : "generic";
}

function groupTasksByActivity(tasks: SubagentTask[]): {
  active: SubagentTask[];
  completed: SubagentTask[];
} {
  const groups = {
    active: [] as SubagentTask[],
    completed: [] as SubagentTask[],
  };
  for (const task of tasks) {
    groups[isSubagentTaskActive(task) ? "active" : "completed"].push(task);
  }
  groups.active.sort(compareTasksByRecentActivity);
  groups.completed.sort(compareTasksByRecentActivity);
  return groups;
}

function compareTasksByRecentActivity(
  left: SubagentTask,
  right: SubagentTask,
): number {
  return subagentTaskTimestamp(right) - subagentTaskTimestamp(left);
}
