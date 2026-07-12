"use client";

import { useCallback, useRef, useState } from "react";
import { History, MoreHorizontal, Pencil, Play, Trash2 } from "lucide-react";

import { UiButton, UiIconButton } from "@/shared/ui/button/button";
import { UiMetaGrid, UiMetaItem } from "@/shared/ui/display/meta-grid";
import {
  UiActionMenu,
  type UiActionMenuItem,
} from "@/shared/ui/menu/action-menu";
import { WorkspaceStatusBadge } from "@/shared/ui/workspace/controls/workspace-status-badge";
import { WorkspaceCatalogAction } from "@/shared/ui/workspace/catalog/workspace-catalog-actions";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";

import {
  getScheduledTaskListItemPresentation,
} from "./scheduled-task-list-model";

interface ScheduledTaskListItemProps {
  isDeleting: boolean;
  isRunning: boolean;
  isToggling: boolean;
  onDelete: (task: ScheduledTaskItem) => void;
  onEdit: (task: ScheduledTaskItem) => void;
  onOpenHistory: (task: ScheduledTaskItem) => void;
  onRunNow: (task: ScheduledTaskItem) => void;
  onToggleEnabled: (task: ScheduledTaskItem) => void;
  task: ScheduledTaskItem;
}

type TaskMenuAction = "delete" | "edit" | "history";

export function ScheduledTaskListItem({
  isDeleting,
  isRunning,
  isToggling,
  onDelete,
  onEdit,
  onOpenHistory,
  onRunNow,
  onToggleEnabled,
  task,
}: ScheduledTaskListItemProps) {
  const [isMenuOpen, setIsMenuOpen] = useState(false);
  const menuAnchorRef = useRef<HTMLButtonElement>(null);
  const closeMenu = useCallback(() => setIsMenuOpen(false), []);
  const presentation = getScheduledTaskListItemPresentation(task, {
    isDeleting,
    isRunning,
    isToggling,
  });
  const menuItems: UiActionMenuItem[] = [
    {
      disabled: presentation.historyDisabled,
      icon: <History className="h-3.5 w-3.5" />,
      label: "运行历史",
      value: "history",
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
    history: () => onOpenHistory(task),
  };

  return (
    <article className="py-4 first:pt-0 last:pb-0">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <h3 className="text-[15px] font-semibold text-(--text-strong)">
              {task.name}
            </h3>
            {presentation.statusBadges.map((badge) => (
              <WorkspaceStatusBadge
                key={`${badge.tone}:${badge.label}`}
                label={badge.label}
                size="compact"
                tone={badge.tone}
              />
            ))}
          </div>
          <div className="mt-3 flex flex-wrap gap-x-5 gap-y-2 text-sm text-(--text-default)">
            <span className="font-medium text-(--text-strong)">
              {presentation.scheduleSummary}
            </span>
            <span>{presentation.nextRunSummary}</span>
            <span>{presentation.lastRunSummary}</span>
          </div>
          <p className="mt-3 text-sm leading-6 text-(--text-default)">
            {presentation.behaviorSummary}
          </p>
          {presentation.lastError ? (
            <p className="mt-2 break-words rounded-[8px] border border-[color:color-mix(in_srgb,var(--destructive)_18%,transparent)] px-3 py-2 text-xs leading-5 text-(--destructive)">
              {presentation.lastError}
            </p>
          ) : null}
          <details className="group mt-3 text-xs text-(--text-muted)">
            <summary className="cursor-pointer list-none font-medium text-(--text-default) hover:text-(--text-strong)">
              查看执行与投递设置
            </summary>
            <div className="mt-3 rounded-[10px] border border-(--divider-subtle-color) px-3 py-3">
              <UiMetaGrid>
                <UiMetaItem label="归属对象" value={presentation.contextSummary} />
                <UiMetaItem label="执行会话" value={presentation.sessionSummary} />
                <UiMetaItem label="结果回传" value={presentation.deliverySummary} />
                <UiMetaItem label="重叠策略" value={presentation.overlapPolicyLabel} />
              </UiMetaGrid>
              <div className="mt-3 flex flex-wrap gap-x-5 gap-y-2">
                <span>Agent {task.agent_id}</span>
                <span>来源 {presentation.sourceKindLabel}</span>
                {presentation.detailItems.map((item) => (
                  <span key={item.label}>{item.label} {item.value}</span>
                ))}
              </div>
            </div>
          </details>
        </div>

        <div className="flex shrink-0 items-center justify-end gap-2">
          <UiButton
            className="min-w-[92px]"
            disabled={presentation.toggleAction.disabled}
            onClick={() => onToggleEnabled(task)}
            title={presentation.toggleAction.title}
            tone={presentation.toggleAction.tone}
          >
            {presentation.toggleAction.label}
          </UiButton>
          <WorkspaceCatalogAction
            aria-label="立即运行"
            disabled={presentation.runAction.disabled}
            onClick={() => onRunNow(task)}
            size="md"
            title={presentation.runAction.title}
          >
            <Play className="h-3.5 w-3.5" />
          </WorkspaceCatalogAction>
          <UiIconButton
            ref={menuAnchorRef}
            aria-expanded={isMenuOpen}
            aria-haspopup="menu"
            aria-label="更多操作"
            onClick={() => setIsMenuOpen((current) => !current)}
            size="md"
            title="更多操作"
            variant="surface"
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
      </div>
    </article>
  );
}
