// INPUT: Scheduled task/run ledger 快照、当前翻译函数与 exact 在途/未确认动作。
// OUTPUT: 本地化状态/时长、有序动作及独立 busy/disabled 投影；未知状态不暴露 wire 值。
// POS: Scheduled 运行历史纯模型；任一删除态只允许读取，不发起 run/delivery mutation。

import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";

import type {
  ScheduledTaskDeliveryStatus,
  ScheduledTaskRunItem,
  ScheduledTaskRunLedgerStatus,
} from "@/types/capability/scheduled-task/run";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";

type Translate = I18nContextValue["t"];

interface RunStatusMeta {
  label: string;
  tone: "active" | "default" | "idle" | "running" | "success";
}

type RunStatusDefinition = Omit<RunStatusMeta, "label"> & { label: TranslationKey };

const RUN_STATUS_META: Record<ScheduledTaskRunLedgerStatus, RunStatusDefinition> = {
  cancelled: { label: "capability.scheduled_history_run_cancelled", tone: "idle" },
  failed: { label: "capability.scheduled_history_run_failed", tone: "default" },
  pending: { label: "capability.scheduled_history_run_pending", tone: "default" },
  queued_to_main_session: { label: "capability.scheduled_history_run_queued", tone: "running" },
  running: { label: "capability.scheduled_history_run_running", tone: "running" },
  skipped: { label: "capability.scheduled_history_run_skipped", tone: "idle" },
  succeeded: { label: "capability.scheduled_history_run_succeeded", tone: "success" },
};

const DELIVERY_STATUS_META: Record<ScheduledTaskDeliveryStatus, RunStatusDefinition> = {
  failed: { label: "capability.scheduled_history_delivery_failed", tone: "default" },
  not_attempted: { label: "capability.scheduled_history_delivery_not_attempted", tone: "idle" },
  not_required: { label: "capability.scheduled_history_delivery_not_required", tone: "idle" },
  pending: { label: "capability.scheduled_history_delivery_pending", tone: "running" },
  retrying: { label: "capability.scheduled_history_delivery_unconfirmed", tone: "default" },
  skipped: { label: "capability.scheduled_history_delivery_not_required", tone: "idle" },
  succeeded: { label: "capability.scheduled_history_delivery_succeeded", tone: "success" },
};

export function formatDuration(
  startedAt: number | null,
  finishedAt: number | null,
  t: Translate,
): string {
  if (startedAt === null || finishedAt === null || !Number.isFinite(startedAt) || !Number.isFinite(finishedAt)) {
    return t("capability.scheduled_history_duration_incomplete");
  }
  const diffSeconds = Math.max(0, Math.round((finishedAt - startedAt) / 1000));
  if (diffSeconds < 60) {
    return t("capability.scheduled_history_duration_seconds", { seconds: diffSeconds });
  }
  const minutes = Math.floor(diffSeconds / 60);
  const seconds = diffSeconds % 60;
  return t("capability.scheduled_history_duration_minutes", { minutes, seconds });
}

export function getStatusMeta(status: ScheduledTaskRunLedgerStatus, t: Translate): RunStatusMeta {
  const meta = Object.hasOwn(RUN_STATUS_META, status) ? RUN_STATUS_META[status] : null;
  return meta
    ? { ...meta, label: t(meta.label) }
    : { label: t("capability.scheduled_history_run_unknown"), tone: "default" };
}

export function getDeliveryStatusMeta(
  status: ScheduledTaskRunItem["delivery_status"],
  t: Translate,
): RunStatusMeta | null {
  if (!status) {
    return null;
  }
  const meta = Object.hasOwn(DELIVERY_STATUS_META, status)
    ? DELIVERY_STATUS_META[status as ScheduledTaskDeliveryStatus]
    : null;
  return meta
    ? { ...meta, label: t(meta.label) }
    : { label: t("capability.scheduled_history_delivery_unknown"), tone: "default" };
}

export function getTaskStatusMeta(task: ScheduledTaskItem, t: Translate): RunStatusMeta {
  if (task.deletion_state?.trim() === "review_required") {
    return { label: t("capability.scheduled_history_deletion_review"), tone: "idle" };
  }
  if (task.deletion_state?.trim()) {
    return { label: t("capability.scheduled_history_deleting"), tone: "idle" };
  }
  if (task.running) {
    return { label: t("capability.scheduled_history_task_running"), tone: "running" };
  }
  if (task.enabled) {
    return { label: t("capability.scheduled_history_task_enabled"), tone: "active" };
  }
  return { label: t("capability.scheduled_history_task_paused"), tone: "idle" };
}

export function artifactFileName(path: string): string {
  return path.split("/").filter(Boolean).at(-1) ?? "automation-run.md";
}

function isRetryableStatus(status: ScheduledTaskRunLedgerStatus): boolean {
  return status === "failed" || status === "cancelled" || status === "skipped";
}

function deletionActionLabel(task: ScheduledTaskItem, t: Translate): string {
  return task.deletion_state?.trim() === "review_required"
    ? t("capability.scheduled_history_deletion_review")
    : t("capability.scheduled_history_deletion_finishing");
}

export type ScheduledTaskRunActionKind = "recover" | "retry" | "retry_delivery";

interface ScheduledTaskRunActionPresentation {
  busy: boolean;
  disabled: boolean;
  kind: ScheduledTaskRunActionKind;
  label: string;
  title: string;
  tone: "danger" | "primary";
}

interface ScheduledTaskRunActionContext {
  isRecoveryUnconfirmed: boolean;
  isRecovering: boolean;
  isRetryDeliveryUnconfirmed: boolean;
  isRetryUnconfirmed: boolean;
  isRetrying: boolean;
  isRetryingDelivery: boolean;
  run: ScheduledTaskRunItem;
  task: ScheduledTaskItem;
}

type RunActionBuilder = (
  context: ScheduledTaskRunActionContext,
  t: Translate,
) => ScheduledTaskRunActionPresentation | null;

function buildRetryAction({
  isRetryUnconfirmed,
  isRetrying,
  run,
  task,
}: ScheduledTaskRunActionContext, t: Translate): ScheduledTaskRunActionPresentation | null {
  if (!isRetryableStatus(run.status)) {
    return null;
  }
  const taskDeleting = Boolean(task.deletion_state?.trim());
  return {
    disabled: taskDeleting || isRetrying || isRetryUnconfirmed || task.running,
    busy: isRetrying,
    kind: "retry",
    label: taskDeleting
      ? deletionActionLabel(task, t)
      : t(isRetryUnconfirmed
        ? "capability.scheduled_history_retry_unconfirmed"
        : isRetrying ? "capability.scheduled_history_retry_busy" : "capability.scheduled_history_retry"),
    title: taskDeleting
      ? task.deletion_state?.trim() === "review_required"
        ? t("capability.scheduled_history_retry_deletion_review")
        : t("capability.scheduled_history_retry_deleting")
      : isRetryUnconfirmed
      ? t("capability.scheduled_history_retry_check")
      : task.running ? t("capability.scheduled_history_retry_running") : t("capability.scheduled_history_retry_hint"),
    tone: "primary",
  };
}

function buildRetryDeliveryAction({
  isRetryDeliveryUnconfirmed,
  isRetryingDelivery,
  run,
  task,
}: ScheduledTaskRunActionContext, t: Translate): ScheduledTaskRunActionPresentation | null {
  const taskDeleting = Boolean(task.deletion_state?.trim());
  if (run.delivery_status === "retrying") {
    const hasExactAttempt = typeof run.delivery_attempts === "number";
    return {
      disabled: taskDeleting
        || isRetryingDelivery
        || isRetryDeliveryUnconfirmed
        || !hasExactAttempt,
      busy: isRetryingDelivery,
      kind: "retry_delivery",
      label: taskDeleting
        ? deletionActionLabel(task, t)
        : isRetryDeliveryUnconfirmed
        ? t("capability.scheduled_history_delivery_unconfirmed")
        : isRetryingDelivery
          ? t("capability.scheduled_history_delivery_busy")
          : hasExactAttempt
            ? t("capability.scheduled_history_delivery_verified_retry")
            : t("capability.scheduled_history_delivery_refresh_check"),
      title: taskDeleting
        ? task.deletion_state?.trim() === "review_required"
          ? t("capability.scheduled_history_delivery_deletion_review")
          : t("capability.scheduled_history_delivery_deleting")
        : hasExactAttempt
        ? t("capability.scheduled_history_delivery_verify_hint")
        : t("capability.scheduled_history_delivery_missing_attempt"),
      tone: "primary",
    };
  }
  if (run.delivery_status !== "failed") {
    return null;
  }
  return {
    disabled: taskDeleting || isRetryingDelivery || isRetryDeliveryUnconfirmed,
    busy: isRetryingDelivery,
    kind: "retry_delivery",
    label: taskDeleting
      ? deletionActionLabel(task, t)
      : t(isRetryDeliveryUnconfirmed
        ? "capability.scheduled_history_delivery_unconfirmed"
        : isRetryingDelivery ? "capability.scheduled_history_delivery_busy" : "capability.scheduled_history_delivery_retry"),
    title: taskDeleting
      ? task.deletion_state?.trim() === "review_required"
        ? t("capability.scheduled_history_delivery_deletion_review")
        : t("capability.scheduled_history_delivery_deleting")
      : isRetryDeliveryUnconfirmed
      ? t("capability.scheduled_history_delivery_check")
      : t("capability.scheduled_history_delivery_retry_hint"),
    tone: "primary",
  };
}

function buildRecoverAction({
  isRecoveryUnconfirmed,
  isRecovering,
  run,
  task,
}: ScheduledTaskRunActionContext, t: Translate): ScheduledTaskRunActionPresentation | null {
  if (!["queued_to_main_session", "running"].includes(run.status) || !task.running) {
    return null;
  }
  return {
    disabled: Boolean(task.deletion_state?.trim())
      || isRecovering
      || isRecoveryUnconfirmed,
    busy: isRecovering,
    kind: "recover",
    label: task.deletion_state?.trim()
      ? deletionActionLabel(task, t)
      : t(isRecoveryUnconfirmed
        ? "capability.scheduled_history_recover_unconfirmed"
        : isRecovering ? "capability.scheduled_history_recover_busy" : "capability.scheduled_history_recover"),
    title: task.deletion_state?.trim()
      ? task.deletion_state?.trim() === "review_required"
        ? t("capability.scheduled_history_recover_deletion_review")
        : t("capability.scheduled_history_recover_deleting")
      : isRecoveryUnconfirmed
      ? t("capability.scheduled_history_recover_check")
      : t("capability.scheduled_history_recover_hint"),
    tone: "danger",
  };
}

const RUN_ACTION_BUILDERS: RunActionBuilder[] = [
  buildRetryAction,
  buildRetryDeliveryAction,
  buildRecoverAction,
];

export function getRunActionPresentations(
  context: ScheduledTaskRunActionContext,
  t: Translate,
): ScheduledTaskRunActionPresentation[] {
  return RUN_ACTION_BUILDERS.flatMap((buildAction) => {
    const action = buildAction(context, t);
    return action ? [action] : [];
  });
}
