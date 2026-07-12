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
  cancelled: { label: "已取消", tone: "idle" },
  failed: { label: "失败", tone: "default" },
  pending: { label: "等待中", tone: "default" },
  queued_to_main_session: { label: "已入主会话", tone: "default" },
  running: { label: "运行中", tone: "running" },
  skipped: { label: "已跳过", tone: "idle" },
  succeeded: { label: "成功", tone: "success" },
};

const DELIVERY_STATUS_META: Record<ScheduledTaskDeliveryStatus, RunStatusMeta> = {
  failed: { label: "投递失败", tone: "default" },
  not_attempted: { label: "未投递", tone: "idle" },
  not_required: { label: "无需投递", tone: "idle" },
  pending: { label: "待投递", tone: "running" },
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

export type ScheduledTaskRunActionKind = "recover" | "retry" | "retry_delivery";

interface ScheduledTaskRunActionPresentation {
  disabled: boolean;
  kind: ScheduledTaskRunActionKind;
  label: string;
  title: string;
  tone: "danger" | "primary";
}

interface ScheduledTaskRunActionContext {
  isRecovering: boolean;
  isRetrying: boolean;
  isRetryingDelivery: boolean;
  run: ScheduledTaskRunItem;
  task: ScheduledTaskItem;
}

type RunActionBuilder = (
  context: ScheduledTaskRunActionContext,
) => ScheduledTaskRunActionPresentation | null;

function buildRetryAction({
  isRetrying,
  run,
  task,
}: ScheduledTaskRunActionContext): ScheduledTaskRunActionPresentation | null {
  if (!isRetryableStatus(run.status)) {
    return null;
  }
  return {
    disabled: isRetrying || task.running,
    kind: "retry",
    label: isRetrying ? "触发中" : "重新运行",
    title: task.running ? "任务当前正在运行" : "用当前任务配置重新运行一次",
    tone: "primary",
  };
}

function buildRetryDeliveryAction({
  isRetryingDelivery,
  run,
}: ScheduledTaskRunActionContext): ScheduledTaskRunActionPresentation | null {
  if (run.delivery_status !== "failed") {
    return null;
  }
  return {
    disabled: isRetryingDelivery,
    kind: "retry_delivery",
    label: isRetryingDelivery ? "投递中" : "重试投递",
    title: "只重试这次运行的结果投递，不重新执行任务",
    tone: "primary",
  };
}

function buildRecoverAction({
  isRecovering,
  run,
  task,
}: ScheduledTaskRunActionContext): ScheduledTaskRunActionPresentation | null {
  if (run.status !== "running" || !task.running) {
    return null;
  }
  return {
    disabled: isRecovering,
    kind: "recover",
    label: isRecovering ? "释放中" : "释放占用",
    title: "把该运行标记为取消，并释放任务占用",
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
