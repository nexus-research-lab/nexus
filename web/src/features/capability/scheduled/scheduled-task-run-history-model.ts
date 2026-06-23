import type { ScheduledTaskItem, ScheduledTaskRunItem } from "@/types/capability/scheduled-task";
import { format_scheduled_datetime } from "./scheduled-formatters";

export function format_duration(started_at: number | null, finished_at: number | null): string {
  if (!started_at || !finished_at) {
    return "未完成";
  }
  const diff_seconds = Math.max(0, Math.round((finished_at - started_at) / 1000));
  if (diff_seconds < 60) {
    return `${diff_seconds} 秒`;
  }
  const minutes = Math.floor(diff_seconds / 60);
  const seconds = diff_seconds % 60;
  return `${minutes} 分 ${seconds} 秒`;
}

export function get_status_meta(status: ScheduledTaskRunItem["status"]) {
  if (status === "succeeded") {
    return { label: "成功", tone: "success" as const };
  }
  if (status === "running") {
    return { label: "运行中", tone: "running" as const };
  }
  if (status === "pending") {
    return { label: "等待中", tone: "default" as const };
  }
  if (status === "cancelled") {
    return { label: "已取消", tone: "idle" as const };
  }
  if (status === "queued_to_main_session") {
    return { label: "已入主会话", tone: "default" as const };
  }
  if (status === "skipped") {
    return { label: "已跳过", tone: "idle" as const };
  }
  return { label: "失败", tone: "default" as const };
}

export function get_delivery_status_meta(status: ScheduledTaskRunItem["delivery_status"]) {
  if (status === "succeeded") {
    return { label: "投递成功", tone: "success" as const };
  }
  if (status === "failed") {
    return { label: "投递失败", tone: "default" as const };
  }
  if (status === "pending") {
    return { label: "待投递", tone: "running" as const };
  }
  if (status === "not_attempted") {
    return { label: "未投递", tone: "idle" as const };
  }
  if (status === "not_required" || status === "skipped") {
    return { label: "无需投递", tone: "idle" as const };
  }
  return null;
}

export function should_show_assistant_text(run: ScheduledTaskRunItem): boolean {
  if (!run.assistant_text) {
    return false;
  }
  return run.assistant_text.trim() !== (run.result_text ?? "").trim();
}

export function artifact_file_name(path: string): string {
  return path.split("/").filter(Boolean).at(-1) ?? "automation-run.md";
}

export function is_retryable_status(status: ScheduledTaskRunItem["status"]): boolean {
  return status === "failed" || status === "cancelled" || status === "skipped";
}

export function build_run_diagnostic(task: ScheduledTaskItem, run: ScheduledTaskRunItem): string {
  const lines = [
    `Task: ${task.name}`,
    `Job ID: ${task.job_id}`,
    `Agent ID: ${task.agent_id}`,
    `Execution: ${task.execution_kind ?? "agent"}`,
    `Run ID: ${run.run_id}`,
    `Status: ${run.status}`,
    `Delivery Status: ${run.delivery_status || ""}`,
    `Delivery Attempts: ${run.delivery_attempts ?? 0}`,
    `Delivered At: ${format_scheduled_datetime(run.delivered_at, { include_seconds: true })}`,
    `Delivery Next Attempt: ${format_scheduled_datetime(run.delivery_next_attempt_at, { include_seconds: true })}`,
    `Delivery Dead Letter At: ${format_scheduled_datetime(run.delivery_dead_letter_at, { include_seconds: true })}`,
    `Trigger: ${run.trigger_kind || ""}`,
    `Scheduled: ${format_scheduled_datetime(run.scheduled_for, { include_seconds: true })}`,
    `Started: ${format_scheduled_datetime(run.started_at, { include_seconds: true })}`,
    `Finished: ${format_scheduled_datetime(run.finished_at, { include_seconds: true })}`,
    `Duration: ${format_duration(run.started_at, run.finished_at)}`,
    `Attempts: ${run.attempts}`,
    `Session: ${run.session_key || ""}`,
    `Round: ${run.round_id || ""}`,
    `Runtime: ${run.session_id || ""}`,
    `Artifact: ${run.artifact_path || ""}`,
  ];
  if (run.delivery_error) {
    lines.push("", "Delivery Error:", run.delivery_error);
  }
  if (run.error_message) {
    lines.push("", "Error:", run.error_message);
  }
  if (run.result_summary) {
    lines.push("", "Summary:", run.result_summary);
  }
  if (run.result_text) {
    lines.push("", "Result:", run.result_text);
  }
  if (run.assistant_text && run.assistant_text.trim() !== (run.result_text ?? "").trim()) {
    lines.push("", "Assistant:", run.assistant_text);
  }
  return lines.join("\n");
}
