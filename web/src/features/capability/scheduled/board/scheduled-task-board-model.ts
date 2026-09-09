/**
 * INPUT: 定时任务协议对象、命令状态与本地化函数。
 * OUTPUT: 看板分列、排序、卡片状态、删除收尾/人工复核语义和快速创建预设。
 * POS: 定时任务看板唯一纯投影模型。
 */
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";
import type { TranslationKey } from "@/shared/i18n/messages";
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
  isDeleteUnconfirmed?: boolean;
  isMutationBlocked?: boolean;
  isPermissionPending: boolean;
  isPermissionUnconfirmed?: boolean;
  isRunning: boolean;
  isRunUnconfirmed?: boolean;
  isToggling: boolean;
  isToggleUnconfirmed?: boolean;
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

export interface ScheduledTaskDeletionPresentation {
  description: string;
  impact: string;
  nextStep: string;
  title: string;
}

export interface ScheduledTaskCardPresentation {
  binding: ScheduledTaskBindingPresentation | null;
  columnId: ScheduledTaskBoardColumnId;
  contextLabel: string;
  deletion: ScheduledTaskDeletionPresentation | null;
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

const SCHEDULED_TASK_BOARD_COLUMNS: (Omit<ScheduledTaskBoardColumnDefinition, "title"> & { titleKey: TranslationKey })[] = [
  {
    id: "running",
    titleKey: "capability.scheduled_column_running",
    tone: "primary",
  },
  {
    id: "scheduled",
    titleKey: "capability.scheduled_column_scheduled",
    tone: "success",
  },
  {
    id: "attention",
    titleKey: "capability.scheduled_column_attention",
    tone: "warning",
  },
  {
    id: "stopped",
    titleKey: "capability.scheduled_column_stopped",
    tone: "muted",
  },
];

function getTaskColumnId(task: ScheduledTaskItem): ScheduledTaskBoardColumnId {
  if (isScheduledTaskDeleting(task)) {
    return "attention";
  }
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

export function isScheduledTaskDeleting(task: ScheduledTaskItem): boolean {
  // 后端以非空 deletion_state 统一拒绝新 mutation；Web 对未来收尾
  // 子状态也必须 fail closed，不能只认当前的 `deleting` 字面值。
  return Boolean(task.deletion_state?.trim());
}

function getDeletionPresentation(
  task: ScheduledTaskItem,
  t: Translate,
): ScheduledTaskDeletionPresentation | null {
  if (!isScheduledTaskDeleting(task)) {
    return null;
  }
  if (task.deletion_state?.trim() === "review_required") {
    return {
      description: t("capability.scheduled_board_review_description"),
      impact: t("capability.scheduled_board_review_impact"),
      nextStep: t("capability.scheduled_delete_review_required_next_step"),
      title: t("capability.scheduled_delete_review_required_title"),
    };
  }
  return {
    description: t("capability.scheduled_board_finishing_description"),
    impact: t("capability.scheduled_board_finishing_impact"),
    nextStep: t("capability.scheduled_board_finishing_next"),
    title: t("capability.scheduled_board_deleting_title"),
  };
}

function getBindingPresentation(
  task: ScheduledTaskItem,
  t: Translate,
): ScheduledTaskBindingPresentation | null {
  if (task.session_binding_state !== "rebind_required") {
    return null;
  }
  const issues = new Set(task.session_binding_issues ?? []);
  const description = issues.has("execution") && issues.has("delivery")
    ? t("capability.scheduled_board_rebind_both")
    : issues.has("delivery")
      ? t("capability.scheduled_board_rebind_delivery")
      : t("capability.scheduled_board_rebind_execution");
  return {
    description,
    title: t("capability.scheduled_board_rebind_required"),
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
  t: Translate,
): ScheduledTaskPermissionPresentation | null {
  const state = task.permission_state?.trim() ?? "";
  if (!isActionablePermissionState(state)) {
    return null;
  }
  const request = task.pending_permission_request;
  const defaults: Record<string, { description: string; title: string }> = {
    awaiting_approval: {
      description: t("capability.scheduled_board_approval_description"),
      title: t("capability.scheduled_board_approval_title"),
    },
    awaiting_input: {
      description: t("capability.scheduled_board_input_description"),
      title: t("capability.scheduled_board_input_title"),
    },
    awaiting_reauth: {
      description: t("capability.scheduled_board_reauth_description"),
      title: t("capability.scheduled_board_reauth_title"),
    },
    denied: {
      description: t("capability.scheduled_board_denied_description"),
      title: t("capability.scheduled_board_denied_title"),
    },
    ready_to_retry: {
      description: t("capability.scheduled_board_retry_description"),
      title: t("capability.scheduled_board_retry_title"),
    },
  };
  const fallback = defaults[state];
  const title = request?.title?.trim() || fallback.title;
  const description = request?.description?.trim() || fallback.description;
  return {
    description: request
      ? getScheduledPermissionDisplayDescription(request, description, t)
      : description,
    state,
    title: request
      ? getScheduledPermissionDisplayTitle(request, title, t)
      : title,
  };
}

function getRunStatusLabel(status: string | null | undefined, t: Translate): string {
  const labels: Record<string, string> = {
    cancelled: t("capability.scheduled_board_run_cancelled"),
    failed: t("capability.scheduled_board_run_failed"),
    pending: t("capability.scheduled_board_run_pending"),
    queued_to_main_session: t("capability.scheduled_board_run_queued"),
    running: t("capability.scheduled_history_task_running"),
    skipped: t("capability.scheduled_board_run_skipped"),
    succeeded: t("capability.scheduled_history_run_succeeded"),
  };
  return status ? labels[status] ?? t("capability.scheduled_board_run_unknown_status") : t("capability.scheduled_board_never_run");
}

function getContextLabel(task: ScheduledTaskItem, t: Translate): string {
  const contextLabel = task.source?.context_label?.trim();
  if (task.source?.context_type === "room" && contextLabel) {
    return t("capability.scheduled_board_room_context", { name: contextLabel });
  }
  if (task.source?.context_type === "agent"
    && task.source.context_id?.trim() === task.agent_id.trim()
    && contextLabel) {
    return contextLabel;
  }
  return t(task.execution_kind === "script"
    ? "capability.scheduled_context_script"
    : "capability.scheduled_context_agent");
}

function getStoppedTimingSummary(task: ScheduledTaskItem, t: Translate, locale: string): string {
  const lastRun = formatScheduledDatetime(task.last_run_at, { locale, emptyLabel: t("capability.scheduled_board_never_run") });
  if (task.schedule.kind === "at" && task.last_run_status === "succeeded") {
    return t("capability.scheduled_board_completed_at", { time: lastRun });
  }
  return task.last_run_at
    ? t("capability.scheduled_board_last_run", { status: getRunStatusLabel(task.last_run_status, t), time: lastRun })
    : t("capability.scheduled_board_never_run");
}

function getTimingSummary(
  task: ScheduledTaskItem,
  columnId: ScheduledTaskBoardColumnId,
  t: Translate,
  locale: string,
): string {
  if (columnId === "running") {
    return t("capability.scheduled_board_started_at", { time: formatScheduledDatetime(task.running_started_at, { locale,
      emptyLabel: t("capability.scheduled_board_time_missing"),
      includeSeconds: true,
    }) });
  }
  if (columnId === "scheduled") {
    return t("capability.scheduled_board_next_at", { time: formatScheduledDatetime(task.next_run_at, { locale, emptyLabel: t("capability.scheduled_board_unscheduled") }) });
  }
  if (columnId === "attention") {
    if (isScheduledTaskDeleting(task)) {
      return task.deletion_state?.trim() === "review_required"
        ? t("capability.scheduled_board_delete_paused")
        : t("capability.scheduled_board_delete_finishing");
    }
    if (task.session_binding_state === "rebind_required") {
      return t("capability.scheduled_board_paused_rebind");
    }
    const permission = getPermissionPresentation(task, t);
    if (permission) {
      const requestedAt = Date.parse(task.pending_permission_request?.created_at ?? "");
      return Number.isFinite(requestedAt)
        ? t("capability.scheduled_board_requested_at", { time: formatScheduledDatetime(requestedAt, { locale }) })
        : t("capability.scheduled_board_waiting");
    }
    return t("capability.scheduled_board_failures_at", { count: task.failure_streak, time: formatScheduledDatetime(task.last_run_at, { locale,
      emptyLabel: t("capability.scheduled_board_time_unknown"),
    }) });
  }
  return getStoppedTimingSummary(task, t, locale);
}

export function getScheduledTaskCardPresentation(
  task: ScheduledTaskItem,
  pending: ScheduledTaskCardPendingState,
  t: Translate,
  locale = "zh",
): ScheduledTaskCardPresentation {
  const columnId = getTaskColumnId(task);
  const deletion = getDeletionPresentation(task, t);
  const deletionNeedsReview = task.deletion_state?.trim() === "review_required";
  // durable 删除态拥有专用语义：收尾期间不误展示旧权限或绑定动作。
  const binding = deletion ? null : getBindingPresentation(task, t);
  const permission = deletion ? null : getPermissionPresentation(task, t);
  const permissionBlocksRun = permission !== null && permission.state !== "denied";
  // last_error 描述上一段已经结束的执行；新 attempt 运行期间只呈现当前状态，
  // 若本次仍失败，完成快照会再带回新的诊断。
  const lastError = task.running || deletion ? null : task.last_error?.trim() || null;
  return {
    binding,
    columnId,
    contextLabel: getContextLabel(task, t),
    deletion,
    deleteDisabled: deletion !== null
      || pending.isDeleting
      || Boolean(pending.isDeleteUnconfirmed)
      || Boolean(pending.isMutationBlocked),
    historyDisabled: false,
    lastError,
    permission,
    runAction: {
      disabled: pending.isRunning
        || pending.isPermissionPending
        || Boolean(pending.isPermissionUnconfirmed)
        || Boolean(pending.isRunUnconfirmed)
        || Boolean(pending.isMutationBlocked)
        || task.running
        || permissionBlocksRun
        || binding !== null
        || deletion !== null,
      title: deletion
        ? deletionNeedsReview
          ? t("capability.scheduled_board_run_delete_review")
          : t("capability.scheduled_board_run_deleting")
        : pending.isRunUnconfirmed
        ? t("capability.scheduled_history_retry_check")
        : pending.isPermissionUnconfirmed
          ? t("capability.scheduled_board_permission_unknown")
          : binding
        ? t("capability.scheduled_board_run_rebind")
        : permissionBlocksRun
        ? t("capability.scheduled_board_run_permission")
        : task.running
          ? t("capability.scheduled_history_retry_running")
          : t("capability.scheduled_board_run_now"),
    },
    scheduleSummary: formatScheduledTaskSchedule(task.schedule, t, locale),
    timingSummary: getTimingSummary(task, columnId, t, locale),
    toggleAction: {
      disabled: pending.isToggling
        || Boolean(pending.isToggleUnconfirmed)
        || Boolean(pending.isMutationBlocked)
        || binding !== null
        || deletion !== null,
      label: deletion
        ? deletionNeedsReview ? t("capability.scheduled_history_deletion_review") : t("capability.scheduled_history_deletion_finishing")
        : pending.isToggleUnconfirmed
        ? t("capability.scheduled_board_waiting")
        : binding ? t("capability.scheduled_board_awaiting_rebind") : task.enabled ? t("capability.scheduled_board_pause") : t("capability.scheduled_board_resume"),
      title: deletion
        ? deletionNeedsReview
          ? t("capability.scheduled_board_toggle_delete_review")
          : t("capability.scheduled_board_toggle_deleting")
        : pending.isToggleUnconfirmed
        ? t("capability.scheduled_board_toggle_unknown")
        : binding
        ? t("capability.scheduled_board_toggle_rebind")
        : task.enabled ? t("capability.scheduled_board_pause_hint") : t("capability.scheduled_board_resume_hint"),
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
  t: Translate,
): ScheduledTaskBoardColumn[] {
  return SCHEDULED_TASK_BOARD_COLUMNS.map((column) => ({
    id: column.id,
    tone: column.tone,
    title: t(column.titleKey),
    items: sortColumnItems(
      column.id,
      items.filter((task) => getTaskColumnId(task) === column.id),
    ),
  }));
}
