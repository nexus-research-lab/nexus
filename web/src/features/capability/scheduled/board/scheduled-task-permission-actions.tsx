// INPUT: 当前任务权限事实、动作保护与显式命令。
// OUTPUT: 卡片/详情共用的双语权限按钮；结果未知仍保持禁用。
// POS: 权限动作纯视图，不推断运行状态或发起资源请求。
"use client";

import { ExternalLink, Pencil, RotateCcw } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import type { AutomationPermissionDecision } from "@/types/capability/scheduled-task/permission";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";

interface ScheduledTaskPermissionActionsProps {
  compact?: boolean;
  isPending: boolean;
  onEdit: (task: ScheduledTaskItem) => void;
  onOpenConnector: (connectorId: string) => void;
  onPermissionDecision: (
    task: ScheduledTaskItem,
    decision: AutomationPermissionDecision,
  ) => void;
  onPermissionResume: (task: ScheduledTaskItem) => void;
  task: ScheduledTaskItem;
}

export function ScheduledTaskPermissionActions({
  compact = false,
  isPending,
  onEdit,
  onOpenConnector,
  onPermissionDecision,
  onPermissionResume,
  task,
}: ScheduledTaskPermissionActionsProps) {
  const { t } = useI18n();
  const request = task.pending_permission_request;
  const state = task.permission_state?.trim() ?? "";
  const size = compact ? "xs" : "sm";
  const actionClassName = compact ? "min-w-0 flex-1 whitespace-nowrap" : undefined;
  const denyButton = request?.status === "pending" ? (
    <UiButton
      disabled={isPending}
      onClick={() => onPermissionDecision(task, "deny")}
      size={size}
      tone="danger"
      variant="ghost"
    >
      {t("capability.scheduled_permission_deny")}
    </UiButton>
  ) : null;

  if (state === "ready_to_retry") {
    if (request?.status !== "approved" || !request.run_id) {
      return null;
    }
    return (
      <UiButton
        className={actionClassName}
        disabled={isPending}
        onClick={() => onPermissionResume(task)}
        size={size}
        tone="primary"
        variant="surface"
      >
        <RotateCcw className="h-3.5 w-3.5" />
        {t("capability.scheduled_permission_resume")}
      </UiButton>
    );
  }

  if (state === "awaiting_input") {
    return (
      <>
        <UiButton
          className={actionClassName}
          disabled={isPending}
          onClick={() => onEdit(task)}
          size={size}
          variant="surface"
        >
          <Pencil className="h-3.5 w-3.5" />
          {t("capability.scheduled_dialog_edit_title")}
        </UiButton>
        {denyButton}
      </>
    );
  }

  if (state === "awaiting_reauth" && request) {
    return (
      <>
        {request.capability.connector_id ? (
          <UiButton
            className={actionClassName}
            disabled={isPending}
            onClick={() => onOpenConnector(request.capability.connector_id ?? "")}
            size={size}
            variant="surface"
          >
            <ExternalLink className="h-3.5 w-3.5" />
            {t("capability.scheduled_permission_reconnect")}
          </UiButton>
        ) : null}
        <UiButton
          className={actionClassName}
          disabled={isPending}
          onClick={() => onPermissionDecision(task, "retry")}
          size={size}
          tone="primary"
          variant="surface"
        >
          {t("capability.scheduled_permission_continue")}
        </UiButton>
        {denyButton}
      </>
    );
  }

  if (state === "awaiting_approval" && request?.status === "pending") {
    return (
      <>
        <UiButton
          className={actionClassName}
          disabled={isPending}
          onClick={() => onPermissionDecision(task, "allow_once")}
          size={size}
          tone="primary"
          variant="surface"
        >
          {t(compact ? "capability.scheduled_permission_once_short" : "capability.scheduled_permission_once")}
        </UiButton>
        <UiButton
          className={actionClassName}
          disabled={isPending}
          onClick={() => onPermissionDecision(task, "allow_task")}
          size={size}
          variant="surface"
        >
          {t(compact ? "capability.scheduled_permission_always_short" : "capability.scheduled_permission_always")}
        </UiButton>
        {denyButton}
      </>
    );
  }

  if (state === "denied") {
    return (
      <UiButton
        className={actionClassName}
        disabled={isPending}
        onClick={() => onEdit(task)}
        size={size}
        variant="surface"
      >
        <Pencil className="h-3.5 w-3.5" />
        {t("capability.scheduled_dialog_edit_title")}
      </UiButton>
    );
  }

  return null;
}
