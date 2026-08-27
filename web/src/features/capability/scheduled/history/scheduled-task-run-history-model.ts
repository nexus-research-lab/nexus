// INPUT: Scheduled task/run ledger 快照与 exact 未确认动作。
// OUTPUT: 运行/投递状态、历史动作与 durable 删除收尾/人工处理的纯展示投影。
// POS: Scheduled 运行历史纯模型；任一删除态只允许读取，不发起 run/delivery mutation。

import type {
  ScheduledTaskDeliveryStatus,
  ScheduledTaskRunItem,
  ScheduledTaskRunLedgerStatus,
} from "@/types/capability/scheduled-task/run";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";

interface RunStatusMeta {
  label: string;
  tone: "active" | "default" | "idle" | "running" | "success";
}

const RUN_STATUS_META: Record<ScheduledTaskRunLedgerStatus, RunStatusMeta> = {
  cancelled: { label: "取消", tone: "idle" },
  failed: { label: "失败", tone: "default" },
  pending: { label: "等待", tone: "default" },
  queued_to_main_session: { label: "排队", tone: "running" },
  running: { label: "运行", tone: "running" },
  skipped: { label: "跳过", tone: "idle" },
  succeeded: { label: "成功", tone: "success" },
};

const DELIVERY_STATUS_META: Record<ScheduledTaskDeliveryStatus, RunStatusMeta> = {
  failed: { label: "投递失败", tone: "default" },
  not_attempted: { label: "未投递", tone: "idle" },
  not_required: { label: "无需投递", tone: "idle" },
  pending: { label: "待投递", tone: "running" },
  retrying: { label: "投递结果待确认", tone: "default" },
  skipped: { label: "无需投递", tone: "idle" },
  succeeded: { label: "投递成功", tone: "success" },
};

export function formatDuration(
  startedAt: number | null,
  finishedAt: number | null,
): string {
  if (!startedAt || !finishedAt) {
    return "未完成";
  }
  const diffSeconds = Math.max(0, Math.round((finishedAt - startedAt) / 1000));
  if (diffSeconds < 60) {
    return `${diffSeconds} 秒`;
  }
  const minutes = Math.floor(diffSeconds / 60);
  const seconds = diffSeconds % 60;
  return `${minutes} 分 ${seconds} 秒`;
}

export function getStatusMeta(status: ScheduledTaskRunLedgerStatus): RunStatusMeta {
  return RUN_STATUS_META[status];
}

export function getDeliveryStatusMeta(
  status: ScheduledTaskRunItem["delivery_status"],
): RunStatusMeta | null {
  if (!status) {
    return null;
  }
  return DELIVERY_STATUS_META[status as ScheduledTaskDeliveryStatus] ?? null;
}

export function getTaskStatusMeta(task: ScheduledTaskItem): RunStatusMeta {
  if (task.deletion_state?.trim() === "review_required") {
    return { label: "删除待处理", tone: "idle" };
  }
  if (task.deletion_state?.trim()) {
    return { label: "删除中", tone: "idle" };
  }
  if (task.running) {
    return { label: "运行中", tone: "running" };
  }
  if (task.enabled) {
    return { label: "已启用", tone: "active" };
  }
  return { label: "已暂停", tone: "idle" };
}

export function artifactFileName(path: string): string {
  return path.split("/").filter(Boolean).at(-1) ?? "automation-run.md";
}

function isRetryableStatus(status: ScheduledTaskRunLedgerStatus): boolean {
  return status === "failed" || status === "cancelled" || status === "skipped";
}

function deletionActionLabel(task: ScheduledTaskItem): string {
  return task.deletion_state?.trim() === "review_required"
    ? "删除待处理"
    : "删除收尾中";
}

export type ScheduledTaskRunActionKind = "recover" | "retry" | "retry_delivery";

interface ScheduledTaskRunActionPresentation {
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
) => ScheduledTaskRunActionPresentation | null;

function buildRetryAction({
  isRetryUnconfirmed,
  isRetrying,
  run,
  task,
}: ScheduledTaskRunActionContext): ScheduledTaskRunActionPresentation | null {
  if (!isRetryableStatus(run.status)) {
    return null;
  }
  const taskDeleting = Boolean(task.deletion_state?.trim());
  return {
    disabled: taskDeleting || isRetrying || isRetryUnconfirmed || task.running,
    kind: "retry",
    label: taskDeleting
      ? deletionActionLabel(task)
      : isRetryUnconfirmed ? "运行结果待确认" : isRetrying ? "触发中" : "重新运行",
    title: taskDeleting
      ? task.deletion_state?.trim() === "review_required"
        ? "删除正在等待管理员处理，任务不再接受新的运行"
        : "删除已受理，任务不再接受新的运行"
      : isRetryUnconfirmed
      ? "上次运行请求结果待确认，请先刷新任务状态"
      : task.running ? "任务当前正在运行" : "用当前任务配置重新运行一次",
    tone: "primary",
  };
}

function buildRetryDeliveryAction({
  isRetryDeliveryUnconfirmed,
  isRetryingDelivery,
  run,
  task,
}: ScheduledTaskRunActionContext): ScheduledTaskRunActionPresentation | null {
  const taskDeleting = Boolean(task.deletion_state?.trim());
  if (run.delivery_status === "retrying") {
    const hasExactAttempt = typeof run.delivery_attempts === "number";
    return {
      disabled: taskDeleting
        || isRetryingDelivery
        || isRetryDeliveryUnconfirmed
        || !hasExactAttempt,
      kind: "retry_delivery",
      label: taskDeleting
        ? deletionActionLabel(task)
        : isRetryDeliveryUnconfirmed
        ? "投递结果待确认"
        : isRetryingDelivery
          ? "投递中"
          : hasExactAttempt
            ? "我已核对，重新投递"
            : "请刷新后核对",
      title: taskDeleting
        ? task.deletion_state?.trim() === "review_required"
          ? "删除正在等待管理员处理，不会再发起结果投递"
          : "删除已受理，不会再发起结果投递"
        : hasExactAttempt
        ? "这次投递可能已经送达。请先核对接收位置，确认没有收到后再重新投递。"
        : "当前记录缺少可靠的投递次数，请刷新运行历史后再核对。",
      tone: "primary",
    };
  }
  if (run.delivery_status !== "failed") {
    return null;
  }
  return {
    disabled: taskDeleting || isRetryingDelivery || isRetryDeliveryUnconfirmed,
    kind: "retry_delivery",
    label: taskDeleting
      ? deletionActionLabel(task)
      : isRetryDeliveryUnconfirmed ? "投递结果待确认" : isRetryingDelivery ? "投递中" : "重试投递",
    title: taskDeleting
      ? task.deletion_state?.trim() === "review_required"
        ? "删除正在等待管理员处理，不会再发起结果投递"
        : "删除已受理，不会再发起结果投递"
      : isRetryDeliveryUnconfirmed
      ? "上次投递请求结果待确认，请先刷新任务状态"
      : "只重试这次运行的结果投递，不重新执行任务",
    tone: "primary",
  };
}

function buildRecoverAction({
  isRecoveryUnconfirmed,
  isRecovering,
  run,
  task,
}: ScheduledTaskRunActionContext): ScheduledTaskRunActionPresentation | null {
  if (!["queued_to_main_session", "running"].includes(run.status) || !task.running) {
    return null;
  }
  return {
    disabled: Boolean(task.deletion_state?.trim())
      || isRecovering
      || isRecoveryUnconfirmed,
    kind: "recover",
    label: task.deletion_state?.trim()
      ? deletionActionLabel(task)
      : isRecoveryUnconfirmed ? "释放结果待确认" : isRecovering ? "释放中" : "释放占用",
    title: task.deletion_state?.trim()
      ? task.deletion_state?.trim() === "review_required"
        ? "删除正在等待管理员处理，不能再手动修改当前运行"
        : "删除收尾会自动处理当前运行，无需再手动释放"
      : isRecoveryUnconfirmed
      ? "上次释放请求结果待确认，请先刷新任务状态"
      : "把该运行标记为取消，并释放任务占用",
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
): ScheduledTaskRunActionPresentation[] {
  return RUN_ACTION_BUILDERS.flatMap((buildAction) => {
    const action = buildAction(context);
    return action ? [action] : [];
  });
}
