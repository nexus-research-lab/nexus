// INPUT: 当前任务权限请求、明确能力事实和当前语言。
// OUTPUT: 权限动作资格及本地化能力、目标摘要。
// POS: Scheduled注意事项纯投影，不执行审批或推断副作用。
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
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
  t: I18nContextValue["t"],
): string {
  if (!isPendingFeishuDocumentToolRequest(request)) {
    return fallback;
  }
  if (request.capability.effect === "read") {
    return t("capability.scheduled_board_feishu_read_title");
  }
  if (request.capability.effect === "write") {
    return t("capability.scheduled_board_feishu_write_title");
  }
  return t("capability.scheduled_board_feishu_title");
}

export function getScheduledPermissionDisplayDescription(
  request: AutomationPermissionRequest,
  fallback: string,
  t: I18nContextValue["t"],
): string {
  if (!isPendingFeishuDocumentToolRequest(request)) {
    return fallback;
  }
  if (request.capability.effect === "read") {
    return t("capability.scheduled_board_feishu_read_description");
  }
  if (request.capability.effect === "write") {
    return t("capability.scheduled_board_feishu_write_description");
  }
  return t("capability.scheduled_board_feishu_description");
}

export function getScheduledPermissionCapabilityLabel(
  request: AutomationPermissionRequest,
  t: I18nContextValue["t"],
): string {
  const target =
    request.capability.connector_id === "feishu-docx"
      ? t("capability.scheduled_board_feishu")
      : request.capability.connector_id?.trim() || t("capability.scheduled_board_external_tool");
  const effects: Record<string, string> = {
    execute: t("capability.scheduled_board_effect_execute"),
    read: t("capability.scheduled_board_effect_read"),
    write: t("capability.scheduled_board_effect_write"),
  };
  return `${target} · ${effects[request.capability.effect] ?? t("capability.scheduled_board_effect_unknown")}`;
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
