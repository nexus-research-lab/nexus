/**
 * INPUT: 单项定时任务、durable 删除/命令状态与任务动作。
 * OUTPUT: 共享目录卡中的名称、指令摘要、计划、时间状态与单一注意事项。
 * POS: 定时任务看板卡片；复用 CatalogCard/Panel/Button，不拥有业务状态视觉 recipe。
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
import { UiButton, UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import {
  UiActionMenu,
  type UiActionMenuItem,
} from "@/shared/ui/menu/action-menu";
import { UiPanel } from "@/shared/ui/panel";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import { WorkspaceCatalogCard } from "@/shared/ui/workspace/catalog/workspace-catalog-card";
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
  isDeleteUnconfirmed: boolean;
  isDeletionReviewPending: boolean;
  isMutationBlocked: boolean;
  isPermissionPending: boolean;
  isPermissionUnconfirmed: boolean;
  isRunning: boolean;
  isRunUnconfirmed: boolean;
  isToggling: boolean;
  isToggleUnconfirmed: boolean;
  onDelete: (task: ScheduledTaskItem) => void;
  onEdit: (task: ScheduledTaskItem) => void;
  onConfirmDeletionStopped: (task: ScheduledTaskItem) => void;
  onOpenHistory: (task: ScheduledTaskItem) => void;
  onOpenConnector: (connectorId: string) => void;
  onPermissionDecision: (
    task: ScheduledTaskItem,
    decision: AutomationPermissionDecision,
  ) => void;
  onPermissionResume: (task: ScheduledTaskItem) => void;
  onRefresh: () => void;
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
  isDeleteUnconfirmed,
  isDeletionReviewPending,
  isMutationBlocked,
  isPermissionPending,
  isPermissionUnconfirmed,
  isRunning,
  isRunUnconfirmed,
  isToggling,
  isToggleUnconfirmed,
  onDelete,
  onEdit,
  onConfirmDeletionStopped,
  onOpenHistory,
  onOpenConnector,
  onPermissionDecision,
  onPermissionResume,
  onRefresh,
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
    isDeleteUnconfirmed,
    isMutationBlocked,
    isPermissionPending,
    isPermissionUnconfirmed,
    isRunning,
    isRunUnconfirmed,
    isToggling,
    isToggleUnconfirmed,
  });
  const TaskIdentityIcon = TASK_IDENTITY_ICONS[presentation.columnId];
  const permissionRequest = task.pending_permission_request;
  const errorCopy = getScheduledTaskErrorCopy(presentation.lastError);
  const attentionTitle = presentation.deletion?.title
    ?? presentation.binding?.title
    ?? presentation.permission?.title
    ?? "最近运行异常";
  const attentionDetail = presentation.deletion?.description
    ?? presentation.binding?.description ?? (presentation.permission
    ? permissionRequest
      ? getScheduledPermissionCapabilityLabel(permissionRequest)
      : presentation.permission.description
    : errorCopy?.summary ?? null);
  const hasAttention = Boolean(
    presentation.deletion || presentation.binding || presentation.permission || errorCopy,
  );
  const hasPermissionActions = presentation.permission !== null
    && hasScheduledTaskPermissionActions(task);
  const AttentionIcon = presentation.deletion
    ? Trash2
    : presentation.binding
    ? Link2Off
    : presentation.permission ? ShieldAlert : CircleAlert;
  const toggleIcon = task.enabled
    ? <PauseCircle className="h-3.5 w-3.5" />
    : <PlayCircle className="h-3.5 w-3.5" />;
  const menuItems: UiActionMenuItem[] = [
    {
      description: presentation.deletion
        ? `${presentation.deletion.title}，任务不再接受修改`
        : isMutationBlocked && !isToggleUnconfirmed
        ? "该任务的另一个修改仍在处理或待确认"
        : isToggleUnconfirmed ? presentation.toggleAction.title : undefined,
      disabled: presentation.toggleAction.disabled,
      icon: toggleIcon,
      label: presentation.toggleAction.label,
      tone: task.enabled ? "default" : "primary",
      value: "toggle",
    },
    {
      description: presentation.deletion
        ? `${presentation.deletion.title}，任务不再接受修改`
        : isMutationBlocked
        ? "该任务的另一个修改仍在处理或待确认"
        : undefined,
      disabled: isMutationBlocked || presentation.deletion !== null,
      icon: <Pencil className="h-3.5 w-3.5" />,
      label: "编辑任务",
      value: "edit",
    },
    {
      description: presentation.deletion
        ? presentation.deletion.nextStep
        : isDeleteUnconfirmed
        ? "上次删除请求结果待确认，请先刷新任务状态"
        : undefined,
      disabled: presentation.deleteDisabled,
      icon: <Trash2 className="h-3.5 w-3.5" />,
      label: "删除任务",
      tone: "danger",
      value: "delete",
    },
  ];
  const actionHandlers: Record<TaskMenuAction, () => void> = {
    delete: () => {
      if (!presentation.deletion) onDelete(task);
    },
    edit: () => {
      if (!presentation.deletion) onEdit(task);
    },
    toggle: () => {
      if (!presentation.deletion) onToggleEnabled(task);
    },
  };

  return (
    <>
      <WorkspaceCatalogCard
        className={cn(
          "group min-h-0 p-3 hover:border-(--surface-interactive-hover-border) hover:bg-(--surface-interactive-hover-background)",
          presentation.columnId === "attention"
            ? "border-[color:color-mix(in_srgb,var(--warning)_30%,var(--divider-subtle-color))]"
            : "border-(--divider-subtle-color)",
        )}
        size="compact"
      >
        <div className="flex min-w-0 items-start gap-2.5">
          <CapabilityItemIcon
            className={TASK_IDENTITY_TONE_CLASS_NAMES[presentation.columnId]}
            size="sm"
          >
            <TaskIdentityIcon
              className={presentation.columnId === "running"
                ? getUiSpinnerClassName({ size: "sm", tone: "current" })
                : "h-3.5 w-3.5"}
            />
          </CapabilityItemIcon>
          <div className="min-w-0 flex-1">
            <div className="flex min-w-0 items-start justify-between gap-2">
              <span className={cn(
                "min-w-0 truncate",
                getUiTypographyClassName({
                  role: "caption",
                  tone: "soft",
                  weight: "medium",
                }),
              )}>
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
            <h3 className={cn(
              "mt-1 truncate",
              getUiTypographyClassName({ role: "sectionTitle", tone: "strong" }),
            )}>
              {task.name}
            </h3>
          </div>
        </div>
        <p className={cn(
          "mt-1 truncate",
          getUiTypographyClassName({ role: "metadata", tone: "muted" }),
        )}>
          {task.instruction}
        </p>

        <div className={cn(
          "mt-2 space-y-1",
          getUiTypographyClassName({ role: "caption", tone: "default" }),
        )}>
          <div className="flex min-w-0 items-center gap-1.5">
            <CalendarClock className="h-3.5 w-3.5 shrink-0 text-(--icon-muted)" />
            <span className="truncate ui-type-weight-medium">
              {presentation.scheduleSummary}
            </span>
          </div>
          <div className="flex min-w-0 items-center gap-1.5 ui-type-tone-muted">
            <Clock3 className="h-3.5 w-3.5 shrink-0 text-(--icon-muted)" />
            <span className="truncate">{presentation.timingSummary}</span>
          </div>
        </div>

        {hasAttention ? (
          <UiPanel
            className={cn(
              "mt-2 overflow-hidden",
              presentation.deletion || presentation.binding || presentation.permission
                ? "border-[color:color-mix(in_srgb,var(--warning)_24%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--warning)_4%,transparent)]"
                : "border-[color:color-mix(in_srgb,var(--destructive)_20%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--destructive)_3%,transparent)]",
            )}
            padding="none"
            radius="sm"
          >
            <UiButton
              aria-label={`查看${attentionTitle}详情`}
              className="min-h-0 w-full justify-start gap-2 px-2.5 py-2 text-left hover:bg-[color:color-mix(in_srgb,var(--surface-interactive-hover-background)_70%,transparent)] focus-visible:ring-inset"
              onClick={() => setIsAttentionOpen(true)}
              size="sm"
              variant="ghost"
            >
              <AttentionIcon
                className={cn(
                  "h-3.5 w-3.5 shrink-0",
                  presentation.deletion || presentation.binding || presentation.permission
                    ? "text-(--warning)"
                    : "text-(--destructive)",
                )}
              />
              <span className="min-w-0 flex-1">
                <span className={cn(
                  "block truncate",
                  getUiTypographyClassName({
                    role: "caption",
                    tone: "strong",
                    weight: "semibold",
                  }),
                )}>
                  {attentionTitle}
                </span>
                {attentionDetail ? (
                  <span className={cn(
                    "mt-0.5 block truncate",
                    getUiTypographyClassName({ role: "caption", tone: "muted" }),
                  )}>
                    {attentionDetail}
                  </span>
                ) : null}
              </span>
              <span className={cn(
                "flex shrink-0 items-center gap-0.5",
                getUiTypographyClassName({
                  role: "caption",
                  tone: "soft",
                  weight: "medium",
                }),
              )}>
                详情
                <ChevronRight className="h-3 w-3" />
              </span>
            </UiButton>
            {hasPermissionActions ? (
              <div className="flex items-center gap-1.5 border-t border-[color:color-mix(in_srgb,var(--warning)_18%,var(--divider-subtle-color))] px-2.5 py-2">
                <ScheduledTaskPermissionActions
                  compact
                  isPending={isMutationBlocked || isPermissionPending || isPermissionUnconfirmed}
                  onEdit={onEdit}
                  onOpenConnector={onOpenConnector}
                  onPermissionDecision={onPermissionDecision}
                  onPermissionResume={onPermissionResume}
                  task={task}
                />
              </div>
            ) : null}
          </UiPanel>
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
      </WorkspaceCatalogCard>

      <ScheduledTaskAttentionDialog
        deletionImpact={presentation.deletion?.impact ?? null}
        deletionNextStep={presentation.deletion?.nextStep ?? null}
        description={presentation.deletion?.description
          ?? presentation.binding?.description
          ?? presentation.permission?.description
          ?? null}
        isBindingAttention={presentation.binding !== null}
        isDeletionAttention={presentation.deletion !== null}
        isDeletionReviewPending={isDeletionReviewPending}
        isOpen={isAttentionOpen}
        isPending={isMutationBlocked || isPermissionPending || isPermissionUnconfirmed}
        onClose={() => setIsAttentionOpen(false)}
        onConfirmDeletionStopped={onConfirmDeletionStopped}
        onEdit={onEdit}
        onOpenConnector={onOpenConnector}
        onOpenHistory={onOpenHistory}
        onPermissionDecision={onPermissionDecision}
        onPermissionResume={onPermissionResume}
        onRefresh={onRefresh}
        task={task}
        title={presentation.deletion?.title
          ?? presentation.binding?.title
          ?? presentation.permission?.title
          ?? null}
      />
    </>
  );
}
