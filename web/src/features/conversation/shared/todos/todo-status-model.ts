import type { TaskProgressContent } from "@/types/conversation/message";
import type { TodoItem } from "@/types/conversation/todo";

const TERMINAL_STATUSES = new Set([
  "completed",
  "complete",
  "success",
  "done",
  "stopped",
  "cancelled",
  "canceled",
  "killed",
  "interrupted",
  "failed",
  "error",
]);
const PENDING_STATUSES = new Set(["pending", "queued", "created"]);
const ACTIVE_STATUSES = new Set(["running", "in_progress", "in progress", "started"]);
const TERMINAL_PROGRESS_MARKERS = [
  "completed",
  "complete",
  "finished",
  "done",
  "已完成",
  "完成",
];
const ACTIVE_PROGRESS_MARKERS = [
  "in_progress",
  "in progress",
  "running",
  "正在",
  "处理中",
];

export function inferSystemTaskStatus(
  subtype: string,
  status: string | null,
  fallback?: TodoItem["status"],
): TodoItem["status"] {
  const normalizedStatus = status?.toLowerCase().trim() ?? "";
  if (TERMINAL_STATUSES.has(normalizedStatus)) {
    return "completed";
  }
  if (PENDING_STATUSES.has(normalizedStatus)) {
    return "pending";
  }
  if (ACTIVE_STATUSES.has(normalizedStatus)) {
    return "in_progress";
  }
  // task_notification 是子任务最终回报，必须覆盖旧的运行中状态。
  if (subtype === "task_notification") {
    return "completed";
  }
  return fallback ?? "in_progress";
}

export function inferTaskProgressStatus(
  block: TaskProgressContent,
  fallback?: TodoItem["status"],
): TodoItem["status"] {
  const progressText = `${block.last_tool_name ?? ""} ${block.description ?? ""}`.toLowerCase();
  if (TERMINAL_PROGRESS_MARKERS.some((marker) => progressText.includes(marker))) {
    return "completed";
  }
  if (ACTIVE_PROGRESS_MARKERS.some((marker) => progressText.includes(marker))) {
    return "in_progress";
  }
  return fallback ?? (block.last_tool_name === "TaskCreate" ? "pending" : "in_progress");
}
