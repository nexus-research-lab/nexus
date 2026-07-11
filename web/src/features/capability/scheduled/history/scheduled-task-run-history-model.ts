import type {
  ScheduledTaskDeliveryStatus,
  ScheduledTaskRunItem,
  ScheduledTaskRunLedgerStatus,
} from "@/types/capability/scheduled-task/run";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";

import { formatScheduledDatetime } from "../scheduled-formatters";

interface RunStatusMeta {
  label: string;
  tone: "active" | "default" | "idle" | "running" | "success";
}

export interface RunDiagnosticRow {
  breakAll?: boolean;
  label: string;
  value: string;
}

export interface RunOutputSection {
  content: string;
  label?: string;
  tone: "danger" | "default";
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

function shouldShowAssistantText(run: ScheduledTaskRunItem): boolean {
  return Boolean(
    run.assistant_text
    && run.assistant_text.trim() !== (run.result_text ?? "").trim(),
  );
}

export function artifactFileName(path: string): string {
  return path.split("/").filter(Boolean).at(-1) ?? "automation-run.md";
}

export function isRetryableStatus(status: ScheduledTaskRunLedgerStatus): boolean {
  return status === "failed" || status === "cancelled" || status === "skipped";
}

export function getRunDiagnosticRows(run: ScheduledTaskRunItem): RunDiagnosticRow[] {
  const rows: Array<RunDiagnosticRow | null> = [
    { breakAll: true, label: "Run", value: run.run_id },
    run.trigger_kind ? { label: "Trigger", value: run.trigger_kind } : null,
    typeof run.message_count === "number"
      ? { label: "Messages", value: String(run.message_count) }
      : null,
    run.session_key ? { breakAll: true, label: "Session", value: run.session_key } : null,
    run.round_id ? { breakAll: true, label: "Round", value: run.round_id } : null,
    run.session_id ? { breakAll: true, label: "Runtime", value: run.session_id } : null,
    run.delivery_to ? { breakAll: true, label: "Delivery", value: run.delivery_to } : null,
    run.delivered_at ? {
      label: "Delivered",
      value: formatScheduledDatetime(run.delivered_at, { includeSeconds: true }),
    } : null,
    run.delivery_attempts
      ? { label: "Delivery attempts", value: String(run.delivery_attempts) }
      : null,
    run.delivery_next_attempt_at ? {
      label: "Next delivery retry",
      value: formatScheduledDatetime(run.delivery_next_attempt_at, { includeSeconds: true }),
    } : null,
    run.delivery_dead_letter_at ? {
      label: "Delivery dead letter",
      value: formatScheduledDatetime(run.delivery_dead_letter_at, { includeSeconds: true }),
    } : null,
    {
      label: "Started",
      value: formatScheduledDatetime(run.started_at, { includeSeconds: true }),
    },
    {
      label: "Finished",
      value: formatScheduledDatetime(run.finished_at, { includeSeconds: true }),
    },
    { label: "Attempts", value: String(run.attempts) },
  ];
  return rows.filter((row): row is RunDiagnosticRow => row !== null);
}

export function getRunOutputSections(run: ScheduledTaskRunItem): RunOutputSection[] {
  const sections: Array<RunOutputSection | null> = [
    run.error_message
      ? { content: run.error_message, tone: "danger" }
      : null,
    run.delivery_error
      ? { content: `投递失败：${run.delivery_error}`, tone: "danger" }
      : null,
    run.result_summary
      ? { content: run.result_summary, tone: "default" }
      : null,
    run.result_text
      ? { content: run.result_text, label: "运行输出", tone: "default" }
      : null,
    shouldShowAssistantText(run)
      ? { content: run.assistant_text ?? "", label: "助手回复", tone: "default" }
      : null,
  ];
  return sections.filter((section): section is RunOutputSection => section !== null);
}

export function buildRunDiagnostic(
  task: ScheduledTaskItem,
  run: ScheduledTaskRunItem,
): string {
  const fields = [
    ["Task", task.name],
    ["Job ID", task.job_id],
    ["Agent ID", task.agent_id],
    ["Execution", task.execution_kind ?? "agent"],
    ["Run ID", run.run_id],
    ["Status", run.status],
    ["Delivery Status", run.delivery_status || ""],
    ["Delivery Attempts", String(run.delivery_attempts ?? 0)],
    ["Delivered At", formatScheduledDatetime(run.delivered_at, { includeSeconds: true })],
    ["Delivery Next Attempt", formatScheduledDatetime(run.delivery_next_attempt_at, { includeSeconds: true })],
    ["Delivery Dead Letter At", formatScheduledDatetime(run.delivery_dead_letter_at, { includeSeconds: true })],
    ["Trigger", run.trigger_kind || ""],
    ["Scheduled", formatScheduledDatetime(run.scheduled_for, { includeSeconds: true })],
    ["Started", formatScheduledDatetime(run.started_at, { includeSeconds: true })],
    ["Finished", formatScheduledDatetime(run.finished_at, { includeSeconds: true })],
    ["Duration", formatDuration(run.started_at, run.finished_at)],
    ["Attempts", String(run.attempts)],
    ["Session", run.session_key || ""],
    ["Round", run.round_id || ""],
    ["Runtime", run.session_id || ""],
    ["Artifact", run.artifact_path || ""],
  ];
  const sections: Array<[string, string | null | undefined]> = [
    ["Delivery Error", run.delivery_error],
    ["Error", run.error_message],
    ["Summary", run.result_summary],
    ["Result", run.result_text],
    ["Assistant", shouldShowAssistantText(run) ? run.assistant_text : null],
  ].filter((section): section is [string, string] => Boolean(section[1]));
  return [
    ...fields.map(([label, value]) => `${label}: ${value}`),
    ...sections.flatMap(([label, value]) => ["", `${label}:`, value]),
  ].join("\n");
}
