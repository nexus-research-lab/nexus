import type { AutomationPermissionRequest } from "@/types/capability/scheduled-task/permission";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";

const PERMISSION_RESOURCE_KEYS = [
  "url",
  "document_url",
  "document_id",
  "doc_id",
  "doc_token",
  "wiki_token",
  "file_token",
] as const;

function isPendingFeishuDocumentToolRequest(
  request: AutomationPermissionRequest,
): boolean {
  return request.status === "pending"
    && request.kind === "tool"
    && request.capability.connector_id === "feishu-docx";
}

export function hasScheduledTaskPermissionActions(task: ScheduledTaskItem): boolean {
  const request = task.pending_permission_request;
  switch (task.permission_state?.trim()) {
    case "ready_to_retry":
      return request?.status === "approved" && Boolean(request.run_id);
    case "awaiting_input":
    case "denied":
      return true;
    case "awaiting_reauth":
      return Boolean(request);
    case "awaiting_approval":
      return request?.status === "pending";
    default:
      return false;
  }
}

export function hasScheduledTaskPermissionAttention(task: ScheduledTaskItem): boolean {
  return [
    "awaiting_approval",
    "awaiting_input",
    "awaiting_reauth",
    "denied",
    "ready_to_retry",
  ].includes(task.permission_state?.trim() ?? "");
}

export function getScheduledPermissionDisplayTitle(
  request: AutomationPermissionRequest,
  fallback: string,
): string {
  if (!isPendingFeishuDocumentToolRequest(request)) {
    return fallback;
  }
  if (request.capability.effect === "read") {
    return "飞书文档读取需要确认";
  }
  if (request.capability.effect === "write") {
    return "飞书文档修改需要确认";
  }
  return "飞书文档操作需要确认";
}

export function getScheduledPermissionDisplayDescription(
  request: AutomationPermissionRequest,
  fallback: string,
): string {
  if (!isPendingFeishuDocumentToolRequest(request)) {
    return fallback;
  }
  if (request.capability.effect === "read") {
    return "任务需要读取指定的飞书文档内容。请确认是否允许本次运行继续。";
  }
  if (request.capability.effect === "write") {
    return "任务需要修改指定的飞书文档内容，可能产生外部副作用。请确认是否允许本次运行继续。";
  }
  return "任务需要操作指定的飞书文档。请确认是否允许本次运行继续。";
}

export function getScheduledPermissionCapabilityLabel(
  request: AutomationPermissionRequest,
): string {
  const target =
    request.capability.connector_id === "feishu-docx"
      ? "飞书文档"
      : request.capability.connector_id?.trim() || "外部工具";
  const effects: Record<string, string> = {
    execute: "执行",
    read: "只读",
    write: "写入 / 修改",
  };
  return `${target} · ${effects[request.capability.effect] ?? request.capability.effect}`;
}

export function getScheduledPermissionResourceSummary(
  request: AutomationPermissionRequest,
): string | null {
  const summary = request.input_summary;
  if (!summary) {
    return null;
  }
  const entries = Object.entries(summary);
  for (const expectedKey of PERMISSION_RESOURCE_KEYS) {
    const entry = entries.find(
      ([key]) => key.trim().toLowerCase() === expectedKey,
    );
    const value = entry?.[1];
    if (typeof value === "string" && value.trim()) {
      return value.trim();
    }
  }
  return null;
}
