/**
 * INPUT: 定时任务协议对象、命令状态与本地化函数。
 * OUTPUT: 看板分列、排序、卡片状态和快速创建预设。
 * POS: 定时任务看板唯一纯投影模型。
 */
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";

import type { TaskDialogCreatePreset } from "../dialog/scheduled-task-dialog-types";
import type { Weekday } from "../pickers/picker-types";
import {
  formatScheduledDatetime,
  formatScheduledTaskSchedule,
} from "../scheduled-formatters";
import {
  getScheduledPermissionDisplayDescription,
  getScheduledPermissionDisplayTitle,
} from "./scheduled-task-attention-model";

export type ScheduledTaskBoardColumnId =
  | "running"
  | "scheduled"
  | "attention"
  | "stopped";

interface ScheduledTaskBoardColumnDefinition {
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
  isPermissionPending: boolean;
  isRunning: boolean;
  isToggling: boolean;
}

export interface ScheduledTaskPermissionPresentation {
  description: string;
  state: string;
  title: string;
}

export interface ScheduledTaskBindingPresentation {
  description: string;
  title: string;
}

export interface ScheduledTaskCardPresentation {
  binding: ScheduledTaskBindingPresentation | null;
  columnId: ScheduledTaskBoardColumnId;
  contextLabel: string;
  deleteDisabled: boolean;
  historyDisabled: boolean;
  lastError: string | null;
  permission: ScheduledTaskPermissionPresentation | null;
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

type Translate = I18nContextValue["t"];

export function buildScheduledTaskSuggestions(
  t: Translate,
): ScheduledTaskSuggestion[] {
  const dailyTitle = t("capability.scheduled_suggestion_daily_title");
  const weeklyTitle = t("capability.scheduled_suggestion_weekly_title");
  const progressTitle = t("capability.scheduled_suggestion_progress_title");
  return [
    {
      description: t("capability.scheduled_suggestion_daily_description"),
      icon: "briefing",
      preset: {
        dailyTime: "08:30",
        instruction: t("capability.scheduled_suggestion_daily_instruction"),
        selectedWeekdays: WORKDAYS,
        taskName: dailyTitle,
      },
      scheduleLabel: t("capability.scheduled_suggestion_daily_schedule"),
      title: dailyTitle,
    },
    {
      description: t("capability.scheduled_suggestion_weekly_description"),
      icon: "review",
      preset: {
        dailyTime: "17:00",
        instruction: t("capability.scheduled_suggestion_weekly_instruction"),
        selectedWeekdays: ["fr"],
        taskName: weeklyTitle,
      },
      scheduleLabel: t("capability.scheduled_suggestion_weekly_schedule"),
      title: weeklyTitle,
    },
    {
      description: t("capability.scheduled_suggestion_progress_description"),
      icon: "monitor",
      preset: {
        dailyTime: "18:00",
        instruction: t("capability.scheduled_suggestion_progress_instruction"),
        selectedWeekdays: WORKDAYS,
        taskName: progressTitle,
      },
      scheduleLabel: t("capability.scheduled_suggestion_progress_schedule"),
      title: progressTitle,
    },
  ];
}

export const SCHEDULED_TASK_BOARD_COLUMNS: ScheduledTaskBoardColumnDefinition[] = [
  {
    id: "running",
    title: "执行中",
    tone: "primary",
  },
  {
    id: "scheduled",
    title: "已计划",
    tone: "success",
  },
  {
    id: "attention",
    title: "需处理",
    tone: "warning",
  },
  {
    id: "stopped",
    title: "已停止",
    tone: "muted",
  },
];

function getTaskColumnId(task: ScheduledTaskItem): ScheduledTaskBoardColumnId {
  if (task.session_binding_state === "rebind_required") {
    return "attention";
  }
  if (isActionablePermissionState(task.permission_state)) {
    return "attention";
  }
  if (task.running) {
    return "running";
  }
  if (task.failure_streak > 0) {
    return "attention";
  }
  return task.enabled ? "scheduled" : "stopped";
}

function getBindingPresentation(
  task: ScheduledTaskItem,
): ScheduledTaskBindingPresentation | null {
  if (task.session_binding_state !== "rebind_required") {
    return null;
  }
  const issues = new Set(task.session_binding_issues ?? []);
  const description = issues.has("execution") && issues.has("delivery")
    ? "执行会话和结果投递会话已删除。编辑任务并重新选择两个会话后才能恢复。"
    : issues.has("delivery")
      ? "结果投递会话已删除。编辑任务并重新选择投递会话后才能恢复。"
      : "执行会话已删除。编辑任务并重新选择执行会话后才能恢复。";
  return {
    description,
    title: "需要重新绑定会话",
  };
}

function isActionablePermissionState(state: string | null | undefined): boolean {
  return [
    "awaiting_approval",
    "awaiting_input",
    "awaiting_reauth",
    "denied",
    "ready_to_retry",
  ].includes(state?.trim() ?? "");
}

function getPermissionPresentation(
  task: ScheduledTaskItem,
): ScheduledTaskPermissionPresentation | null {
  const state = task.permission_state?.trim() ?? "";
  if (!isActionablePermissionState(state)) {
    return null;
  }
  const request = task.pending_permission_request;
  const defaults: Record<string, { description: string; title: string }> = {
    awaiting_approval: {
      description: "确认后才能继续使用本次运行需要的能力。",
      title: "等待权限确认",
    },
    awaiting_input: {
      description: "请编辑任务，把后台运行所需的信息写入任务配置。",
      title: "需要补充任务信息",
    },
    awaiting_reauth: {
      description: "任务授权仍有效，但连接器需要重新连接。",
      title: "连接已失效",
    },
    denied: {
      description: "本次运行已结束；可修改任务或重新手动运行。",
      title: "权限请求已拒绝",
    },
    ready_to_retry: {
      description: "此前运行可能已产生副作用，需要确认后才会重试。",
      title: "等待确认重试",
    },
  };
  const fallback = defaults[state];
  const title = request?.title?.trim() || fallback.title;
  const description = request?.description?.trim() || fallback.description;
  return {
    description: request
      ? getScheduledPermissionDisplayDescription(request, description)
      : description,
    state,
    title: request
      ? getScheduledPermissionDisplayTitle(request, title)
      : title,
  };
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
  if (task.source?.context_type === "room" && contextLabel) {
    return `Room · ${contextLabel}`;
  }
  if (task.source?.context_type === "agent"
    && task.source.context_id?.trim() === task.agent_id.trim()
    && contextLabel) {
    return contextLabel;
  }
  return task.execution_kind === "script" ? "工作区脚本" : task.agent_id;
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
    if (task.session_binding_state === "rebind_required") {
      return "任务已暂停 · 等待重新绑定";
    }
    const permission = getPermissionPresentation(task);
    if (permission) {
      const requestedAt = Date.parse(task.pending_permission_request?.created_at ?? "");
      return Number.isFinite(requestedAt)
        ? `请求于 ${formatScheduledDatetime(requestedAt)}`
        : "等待处理";
    }
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
  const binding = getBindingPresentation(task);
  const permission = getPermissionPresentation(task);
  const permissionBlocksRun = permission !== null && permission.state !== "denied";
  // last_error 描述上一段已经结束的执行；新 attempt 运行期间只呈现当前状态，
  // 若本次仍失败，完成快照会再带回新的诊断。
  const lastError = task.running ? null : task.last_error?.trim() || null;
  return {
    binding,
    columnId,
    contextLabel: getContextLabel(task),
    deleteDisabled: pending.isDeleting,
    historyDisabled: false,
    lastError,
    permission,
    runAction: {
      disabled: pending.isRunning || pending.isPermissionPending || task.running || permissionBlocksRun || binding !== null,
      title: binding
        ? "请先重新绑定有效会话"
        : permissionBlocksRun
        ? "请先处理任务权限"
        : task.running
          ? "任务当前正在运行"
          : "立即运行一次",
    },
    scheduleSummary: formatScheduledTaskSchedule(task.schedule),
    timingSummary: getTimingSummary(task, columnId),
    toggleAction: {
      disabled: pending.isToggling || binding !== null,
      label: binding ? "等待重新绑定" : task.enabled ? "暂停调度" : "恢复调度",
      title: binding
        ? "编辑任务并替换所有已删除会话后才能恢复调度"
        : task.enabled ? "暂停后不再自动触发" : "恢复后重新参与调度",
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
