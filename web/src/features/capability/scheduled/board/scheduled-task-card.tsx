/**
 * INPUT: 单项定时任务、命令状态与任务动作。
 * OUTPUT: 名称、指令摘要、计划、时间状态和单一注意事项摘要。
 * POS: 定时任务看板卡片；指令用于识别任务，诊断细节延后展示。
 */
"use client";

import { useCallback, useRef, useState } from "react";
import type { LucideIcon } from "lucide-react";
import {
  CalendarCheck2,
  CalendarClock,
  ChevronRight,
  CircleAlert,
  CirclePause,
  Clock3,
  History,
  Link2Off,
  LoaderCircle,
  MoreHorizontal,
  PauseCircle,
  Pencil,
  Play,
  PlayCircle,
  ShieldAlert,
  Trash2,
} from "lucide-react";

import { CapabilityItemIcon } from "@/features/capability/shared/capability-page-layout";
import { UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import {
  UiActionMenu,
  type UiActionMenuItem,
} from "@/shared/ui/menu/action-menu";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";
import type { AutomationPermissionDecision } from "@/types/capability/scheduled-task/permission";

import { getScheduledTaskErrorCopy } from "../scheduled-task-error-copy";
import { ScheduledTaskAttentionDialog } from "./scheduled-task-attention-dialog";
import {
  getScheduledPermissionCapabilityLabel,
  hasScheduledTaskPermissionActions,
} from "./scheduled-task-attention-model";
import { ScheduledTaskPermissionActions } from "./scheduled-task-permission-actions";
import {
  getScheduledTaskCardPresentation,
  type ScheduledTaskBoardColumnId,
} from "./scheduled-task-board-model";

interface ScheduledTaskCardProps {
  isDeleting: boolean;
  isPermissionPending: boolean;
  isRunning: boolean;
  isToggling: boolean;
  onDelete: (task: ScheduledTaskItem) => void;
  onEdit: (task: ScheduledTaskItem) => void;
  onOpenHistory: (task: ScheduledTaskItem) => void;
  onOpenConnector: (connectorId: string) => void;
  onPermissionDecision: (
    task: ScheduledTaskItem,
    decision: AutomationPermissionDecision,
  ) => void;
  onPermissionResume: (task: ScheduledTaskItem) => void;
  onRunNow: (task: ScheduledTaskItem) => void;
  onToggleEnabled: (task: ScheduledTaskItem) => void;
  task: ScheduledTaskItem;
}

type TaskMenuAction = "delete" | "edit" | "toggle";

const TASK_IDENTITY_ICONS: Record<ScheduledTaskBoardColumnId, LucideIcon> = {
  attention: CircleAlert,
  running: LoaderCircle,
  scheduled: CalendarCheck2,
  stopped: CirclePause,
};

const TASK_IDENTITY_TONE_CLASS_NAMES: Record<
  ScheduledTaskBoardColumnId,
  string
> = {
  attention: "text-(--warning)",
  running: "text-(--primary)",
  scheduled: "text-(--success)",
  stopped: "text-(--icon-muted)",
};

export function ScheduledTaskCard({
  isDeleting,
  isPermissionPending,
  isRunning,
  isToggling,
  onDelete,
  onEdit,
  onOpenHistory,
  onOpenConnector,
  onPermissionDecision,
  onPermissionResume,
  onRunNow,
  onToggleEnabled,
  task,
}: ScheduledTaskCardProps) {
  const [isAttentionOpen, setIsAttentionOpen] = useState(false);
  const [isMenuOpen, setIsMenuOpen] = useState(false);
  const menuAnchorRef = useRef<HTMLButtonElement>(null);
  const closeMenu = useCallback(() => setIsMenuOpen(false), []);
  const presentation = getScheduledTaskCardPresentation(task, {
    isDeleting,
    isPermissionPending,
    isRunning,
    isToggling,
  });
  const TaskIdentityIcon = TASK_IDENTITY_ICONS[presentation.columnId];
  const permissionRequest = task.pending_permission_request;
  const errorCopy = getScheduledTaskErrorCopy(presentation.lastError);
  const attentionTitle = presentation.binding?.title
    ?? presentation.permission?.title
    ?? "最近运行异常";
  const attentionDetail = presentation.binding?.description ?? (presentation.permission
    ? permissionRequest
      ? getScheduledPermissionCapabilityLabel(permissionRequest)
      : presentation.permission.description
    : errorCopy?.summary ?? null);
  const hasAttention = Boolean(presentation.binding || presentation.permission || errorCopy);
  const hasPermissionActions = presentation.permission !== null
    && hasScheduledTaskPermissionActions(task);
  const AttentionIcon = presentation.binding
    ? Link2Off
    : presentation.permission ? ShieldAlert : CircleAlert;
  const toggleIcon = task.enabled
    ? <PauseCircle className="h-3.5 w-3.5" />
    : <PlayCircle className="h-3.5 w-3.5" />;
  const menuItems: UiActionMenuItem[] = [
    {
      disabled: presentation.toggleAction.disabled,
      icon: toggleIcon,
      label: presentation.toggleAction.label,
      tone: task.enabled ? "default" : "primary",
      value: "toggle",
    },
    {
      icon: <Pencil className="h-3.5 w-3.5" />,
      label: "编辑任务",
      value: "edit",
    },
    {
      disabled: presentation.deleteDisabled,
      icon: <Trash2 className="h-3.5 w-3.5" />,
      label: "删除任务",
      tone: "danger",
      value: "delete",
    },
  ];
  const actionHandlers: Record<TaskMenuAction, () => void> = {
    delete: () => onDelete(task),
    edit: () => onEdit(task),
    toggle: () => onToggleEnabled(task),
  };

  return (
    <>
      <article
        className={cn(
          "group rounded-[8px] border bg-transparent p-3 transition-[border-color,background-color] duration-(--motion-duration-fast) hover:border-(--surface-interactive-hover-border) hover:bg-(--surface-interactive-hover-background)",
          presentation.columnId === "attention"
            ? "border-[color:color-mix(in_srgb,var(--warning)_30%,var(--divider-subtle-color))]"
            : "border-(--divider-subtle-color)",
        )}
      >
        <div className="flex min-w-0 items-start gap-2.5">
          <CapabilityItemIcon
            className={TASK_IDENTITY_TONE_CLASS_NAMES[presentation.columnId]}
            size="sm"
          >
            <TaskIdentityIcon
              className={cn(
                "h-3.5 w-3.5",
                presentation.columnId === "running" && "motion-safe:animate-spin",
              )}
            />
          </CapabilityItemIcon>
          <div className="min-w-0 flex-1">
            <div className="flex min-w-0 items-start justify-between gap-2">
              <span className="min-w-0 truncate text-xs font-medium text-(--text-soft)">
                {presentation.contextLabel}
              </span>
              <UiIconButton
                ref={menuAnchorRef}
                aria-expanded={isMenuOpen}
                aria-haspopup="menu"
                aria-label="更多操作"
                className="-mr-1 -mt-1 shrink-0"
                onClick={() => setIsMenuOpen((current) => !current)}
                size="sm"
                title="更多操作"
                variant="ghost"
              >
                <MoreHorizontal className="h-4 w-4" />
              </UiIconButton>
              <UiActionMenu
                anchorRef={menuAnchorRef}
                ariaLabel="任务操作"
                isOpen={isMenuOpen}
                items={menuItems}
                minWidth={156}
                onClose={closeMenu}
                onSelect={(value) => actionHandlers[value as TaskMenuAction]()}
              />
            </div>
            <h3 className="mt-1 truncate text-base font-semibold leading-5 text-(--text-strong)">
              {task.name}
            </h3>
          </div>
        </div>
        <p className="mt-1 truncate text-compact leading-5 text-(--text-muted)">
          {task.instruction}
        </p>

        <div className="mt-2 space-y-1 text-xs leading-4 text-(--text-default)">
          <div className="flex min-w-0 items-center gap-1.5">
            <CalendarClock className="h-3.5 w-3.5 shrink-0 text-(--icon-muted)" />
            <span className="truncate font-medium">{presentation.scheduleSummary}</span>
          </div>
          <div className="flex min-w-0 items-center gap-1.5 text-(--text-muted)">
            <Clock3 className="h-3.5 w-3.5 shrink-0 text-(--icon-muted)" />
            <span className="truncate">{presentation.timingSummary}</span>
          </div>
        </div>

        {hasAttention ? (
          <div
            className={cn(
              "mt-2 overflow-hidden rounded-[6px] border",
              presentation.binding || presentation.permission
                ? "border-[color:color-mix(in_srgb,var(--warning)_24%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--warning)_4%,transparent)]"
                : "border-[color:color-mix(in_srgb,var(--destructive)_20%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--destructive)_3%,transparent)]",
            )}
          >
            <button
              aria-label={`查看${attentionTitle}详情`}
              className="flex w-full items-center gap-2 px-2.5 py-2 text-left transition-colors duration-(--motion-duration-fast) hover:bg-[color:color-mix(in_srgb,var(--surface-interactive-hover-background)_70%,transparent)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_24%,transparent)]"
              onClick={() => setIsAttentionOpen(true)}
              type="button"
            >
              <AttentionIcon
                className={cn(
                  "h-3.5 w-3.5 shrink-0",
                  presentation.binding || presentation.permission
                    ? "text-(--warning)"
                    : "text-(--destructive)",
                )}
              />
              <span className="min-w-0 flex-1">
                <span className="block truncate text-xs font-semibold text-(--text-strong)">
                  {attentionTitle}
                </span>
                {attentionDetail ? (
                  <span className="mt-0.5 block truncate text-[11px] leading-4 text-(--text-muted)">
                    {attentionDetail}
                  </span>
                ) : null}
              </span>
              <span className="flex shrink-0 items-center gap-0.5 text-[11px] font-medium text-(--text-soft)">
                详情
                <ChevronRight className="h-3 w-3" />
              </span>
            </button>
            {hasPermissionActions ? (
              <div className="flex items-center gap-1.5 border-t border-[color:color-mix(in_srgb,var(--warning)_18%,var(--divider-subtle-color))] px-2.5 py-2">
                <ScheduledTaskPermissionActions
                  compact
                  isPending={isPermissionPending}
                  onEdit={onEdit}
                  onOpenConnector={onOpenConnector}
                  onPermissionDecision={onPermissionDecision}
                  onPermissionResume={onPermissionResume}
                  task={task}
                />
              </div>
            ) : null}
          </div>
        ) : null}

        <div className="mt-2 flex items-center justify-end gap-1 border-t border-(--divider-subtle-color) pt-2">
          <UiIconButton
            aria-label="运行历史"
            disabled={presentation.historyDisabled}
            onClick={() => onOpenHistory(task)}
            size="sm"
            title="运行历史"
            variant="ghost"
          >
            <History className="h-3.5 w-3.5" />
          </UiIconButton>
          <UiIconButton
            aria-label="立即运行"
            disabled={presentation.runAction.disabled}
            onClick={() => onRunNow(task)}
            size="sm"
            title={presentation.runAction.title}
            tone="primary"
            variant="ghost"
          >
            <Play className="h-3.5 w-3.5 fill-current" />
          </UiIconButton>
        </div>
      </article>

      <ScheduledTaskAttentionDialog
        description={presentation.binding?.description ?? presentation.permission?.description ?? null}
        isBindingAttention={presentation.binding !== null}
        isOpen={isAttentionOpen}
        isPending={isPermissionPending}
        onClose={() => setIsAttentionOpen(false)}
        onEdit={onEdit}
        onOpenConnector={onOpenConnector}
        onOpenHistory={onOpenHistory}
        onPermissionDecision={onPermissionDecision}
        onPermissionResume={onPermissionResume}
        task={task}
        title={presentation.binding?.title ?? presentation.permission?.title ?? null}
      />
    </>
  );
}
