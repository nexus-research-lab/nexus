import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";

import type { TaskDialogCreatePreset } from "../dialog/scheduled-task-dialog-types";
import type { Weekday } from "../pickers/picker-types";
import {
  formatScheduledDatetime,
  formatScheduledTaskSchedule,
} from "../scheduled-formatters";

export type ScheduledTaskBoardColumnId =
  | "running"
  | "scheduled"
  | "attention"
  | "stopped";

interface ScheduledTaskBoardColumnDefinition {
  description: string;
  emptyDescription: string;
  id: ScheduledTaskBoardColumnId;
  title: string;
  tone: "primary" | "success" | "warning" | "muted";
}

export interface ScheduledTaskBoardColumn extends ScheduledTaskBoardColumnDefinition {
  items: ScheduledTaskItem[];
}

export interface ScheduledTaskSuggestion {
  description: string;
  icon: "briefing" | "review" | "monitor";
  preset: TaskDialogCreatePreset;
  scheduleLabel: string;
  title: string;
}

interface ScheduledTaskCardPendingState {
  isDeleting: boolean;
  isRunning: boolean;
  isToggling: boolean;
}

export interface ScheduledTaskCardPresentation {
  columnId: ScheduledTaskBoardColumnId;
  contextLabel: string;
  deleteDisabled: boolean;
  historyDisabled: boolean;
  lastError: string | null;
  runAction: {
    disabled: boolean;
    title: string;
  };
  scheduleSummary: string;
  timingSummary: string;
  toggleAction: {
    disabled: boolean;
    label: string;
    title: string;
  };
}

const WORKDAYS: Weekday[] = ["mo", "tu", "we", "th", "fr"];

export const SCHEDULED_TASK_SUGGESTIONS: ScheduledTaskSuggestion[] = [
  {
    description: "汇总最近会话、待办和工作区变化，整理今天最值得优先处理的事项。",
    icon: "briefing",
    preset: {
      dailyTime: "08:30",
      instruction: "汇总最近会话、待办和工作区变化，整理今天最值得优先处理的事项，并给出清晰的行动顺序。",
      selectedWeekdays: WORKDAYS,
      taskName: "每日工作简报",
    },
    scheduleLabel: "工作日 08:30",
    title: "每日工作简报",
  },
  {
    description: "回顾本周完成事项、遗留问题和下周重点，输出一份简短周报。",
    icon: "review",
    preset: {
      dailyTime: "17:00",
      instruction: "回顾本周完成的工作、尚未解决的问题和下一周优先事项，输出一份可以直接使用的简短周报。",
      selectedWeekdays: ["fr"],
      taskName: "每周工作回顾",
    },
    scheduleLabel: "每周五 17:00",
    title: "每周工作回顾",
  },
  {
    description: "检查近期进展和阻塞项，提醒遗漏并给出下一步行动。",
    icon: "monitor",
    preset: {
      dailyTime: "18:00",
      instruction: "检查当前工作区和最近会话的进展，列出阻塞项、可能遗漏的事项和下一步行动。",
      selectedWeekdays: WORKDAYS,
      taskName: "项目进展检查",
    },
    scheduleLabel: "工作日 18:00",
    title: "项目进展检查",
  },
];

export const SCHEDULED_TASK_BOARD_COLUMNS: ScheduledTaskBoardColumnDefinition[] = [
  {
    description: "当前正在执行",
    emptyDescription: "当前没有任务在执行",
    id: "running",
    title: "执行中",
    tone: "primary",
  },
  {
    description: "等待下次触发",
    emptyDescription: "没有等待调度的任务",
    id: "scheduled",
    title: "已计划",
    tone: "success",
  },
  {
    description: "最近执行失败",
    emptyDescription: "没有需要处理的问题",
    id: "attention",
    title: "需处理",
    tone: "warning",
  },
  {
    description: "暂停或单次已结束",
    emptyDescription: "没有停止的任务",
    id: "stopped",
    title: "已停止",
    tone: "muted",
  },
];

function getTaskColumnId(task: ScheduledTaskItem): ScheduledTaskBoardColumnId {
  if (task.running) {
    return "running";
  }
  if (task.failure_streak > 0) {
    return "attention";
  }
  return task.enabled ? "scheduled" : "stopped";
}

function getRunStatusLabel(status: string | null | undefined): string {
  const labels: Record<string, string> = {
    cancelled: "已取消",
    failed: "失败",
    pending: "等待中",
    queued_to_main_session: "已进入主会话",
    running: "运行中",
    skipped: "已跳过",
    succeeded: "成功",
  };
  return status ? labels[status] ?? status : "尚未执行";
}

function getContextLabel(task: ScheduledTaskItem): string {
  const contextLabel = task.source?.context_label?.trim();
  if (contextLabel) {
    return task.source.context_type === "room"
      ? `Room · ${contextLabel}`
      : contextLabel;
  }
  return task.execution_kind === "script" ? "工作区脚本" : "Agent 任务";
}

function getStoppedTimingSummary(task: ScheduledTaskItem): string {
  const lastRun = formatScheduledDatetime(task.last_run_at, { emptyLabel: "尚未执行" });
  if (task.schedule.kind === "at" && task.last_run_status === "succeeded") {
    return `已于 ${lastRun} 完成`;
  }
  return task.last_run_at
    ? `最近${getRunStatusLabel(task.last_run_status)} · ${lastRun}`
    : "尚未执行";
}

function getTimingSummary(
  task: ScheduledTaskItem,
  columnId: ScheduledTaskBoardColumnId,
): string {
  if (columnId === "running") {
    return `开始于 ${formatScheduledDatetime(task.running_started_at, {
      emptyLabel: "刚刚",
      includeSeconds: true,
    })}`;
  }
  if (columnId === "scheduled") {
    return `下次 ${formatScheduledDatetime(task.next_run_at, { emptyLabel: "等待安排" })}`;
  }
  if (columnId === "attention") {
    return `${task.failure_streak} 次失败 · ${formatScheduledDatetime(task.last_run_at, {
      emptyLabel: "时间未知",
    })}`;
  }
  return getStoppedTimingSummary(task);
}

export function getScheduledTaskCardPresentation(
  task: ScheduledTaskItem,
  pending: ScheduledTaskCardPendingState,
): ScheduledTaskCardPresentation {
  const columnId = getTaskColumnId(task);
  return {
    columnId,
    contextLabel: getContextLabel(task),
    deleteDisabled: pending.isDeleting,
    historyDisabled: task.session_target.kind === "main",
    lastError: task.last_error?.trim() || null,
    runAction: {
      disabled: pending.isRunning || task.running,
      title: task.running ? "任务当前正在运行" : "立即运行一次",
    },
    scheduleSummary: formatScheduledTaskSchedule(task.schedule),
    timingSummary: getTimingSummary(task, columnId),
    toggleAction: {
      disabled: pending.isToggling,
      label: task.enabled ? "暂停调度" : "恢复调度",
      title: task.enabled ? "暂停后不再自动触发" : "恢复后重新参与调度",
    },
  };
}

function sortColumnItems(
  columnId: ScheduledTaskBoardColumnId,
  items: ScheduledTaskItem[],
): ScheduledTaskItem[] {
  return [...items].sort((left, right) => {
    if (columnId === "scheduled") {
      return (left.next_run_at ?? Number.MAX_SAFE_INTEGER)
        - (right.next_run_at ?? Number.MAX_SAFE_INTEGER);
    }
    if (columnId === "attention" && left.failure_streak !== right.failure_streak) {
      return right.failure_streak - left.failure_streak;
    }
    const timeDifference = (right.running_started_at ?? right.last_run_at ?? 0)
      - (left.running_started_at ?? left.last_run_at ?? 0);
    return timeDifference || left.name.localeCompare(right.name, "zh-CN");
  });
}

export function buildScheduledTaskBoard(
  items: ScheduledTaskItem[],
): ScheduledTaskBoardColumn[] {
  return SCHEDULED_TASK_BOARD_COLUMNS.map((column) => ({
    ...column,
    items: sortColumnItems(
      column.id,
      items.filter((task) => getTaskColumnId(task) === column.id),
    ),
  }));
}
