// INPUT: Task snapshot, read/loading state and authoritative runtime observation support.
// OUTPUT: Active/history/unknown groups and truthful empty/support states without dropping readable cached tasks on read failure.
// POS: Pure subagent directory projection; no UI styles, clocks or query execution.

import type {
  SubagentTask,
  SubagentTaskListResponse,
} from "@/types/conversation/subagent-task";

import {
  isSubagentTaskActive,
  getSubagentTaskStatus,
  subagentTaskTimestamp,
} from "./subagent-task-model";

export type SubagentTaskListEmptyState = "empty" | "loading";
export type SubagentTaskSupportNotice = "claude" | "generic" | null;

export interface SubagentTaskListModel {
  activeEmptyState: SubagentTaskListEmptyState | null;
  activeTasks: SubagentTask[];
  historyTasks: SubagentTask[];
  unknownTasks: SubagentTask[];
  supportNotice: SubagentTaskSupportNotice;
}

export function filterSubagentTasksByHostAgent(
  tasks: SubagentTask[],
  hostAgentId?: string | null,
): SubagentTask[] {
  const normalizedHostAgentId = hostAgentId?.trim() ?? "";
  if (!normalizedHostAgentId) {
    return tasks;
  }
  return tasks.filter(
    (task) => task.host_agent_id?.trim() === normalizedHostAgentId,
  );
}

interface BuildSubagentTaskListModelOptions {
  data: SubagentTaskListResponse | null;
  isLoading: boolean;
  hasError: boolean;
  tasks: SubagentTask[];
}

export function buildSubagentTaskListModel({
  data,
  isLoading,
  hasError,
  tasks,
}: BuildSubagentTaskListModelOptions): SubagentTaskListModel {
  const supportNotice = resolveSupportNotice(data);
  const groups = groupTasksByActivity(supportNotice ? [] : tasks);
  return {
    activeEmptyState: supportNotice || hasError || groups.unknown.length > 0
      ? null
      : isLoading && !data ? "loading" : "empty",
    activeTasks: groups.active,
    historyTasks: groups.history,
    unknownTasks: groups.unknown,
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
  history: SubagentTask[];
  unknown: SubagentTask[];
} {
  const groups = {
    active: [] as SubagentTask[],
    history: [] as SubagentTask[],
    unknown: [] as SubagentTask[],
  };
  for (const task of tasks) {
    const group = getSubagentTaskStatus(task) === "unknown"
      ? "unknown"
      : isSubagentTaskActive(task) ? "active" : "history";
    groups[group].push(task);
  }
  groups.active.sort(compareTasksByRecentActivity);
  groups.history.sort(compareTasksByRecentActivity);
  groups.unknown.sort(compareTasksByRecentActivity);
  return groups;
}

function compareTasksByRecentActivity(
  left: SubagentTask,
  right: SubagentTask,
): number {
  return subagentTaskTimestamp(right) - subagentTaskTimestamp(left);
}
