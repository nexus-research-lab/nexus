"use client";

import { ExternalLink, Pencil, RotateCcw } from "lucide-react";

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
      拒绝
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
        确认重试
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
          编辑任务
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
            重新连接
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
          已连接，继续
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
          {compact ? "仅本次" : "仅本次允许"}
        </UiButton>
        <UiButton
          className={actionClassName}
          disabled={isPending}
          onClick={() => onPermissionDecision(task, "allow_task")}
          size={size}
          variant="surface"
        >
          {compact ? "始终允许" : "此任务始终允许"}
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
        编辑任务
      </UiButton>
    );
  }

  return null;
}
