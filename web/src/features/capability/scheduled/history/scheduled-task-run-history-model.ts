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

export function isRetryableStatus(status: ScheduledTaskRunLedgerStatus): boolean {
  return status === "failed" || status === "cancelled" || status === "skipped";
}
