"use client";

import { useCallback, useMemo, useRef, useState } from "react";
import { History, MoreHorizontal, Pencil, Play, Trash2 } from "lucide-react";

import { UiButton, UiIconButton } from "@/shared/ui/button/button";
import { UiMetaGrid, UiMetaItem } from "@/shared/ui/display/meta-grid";
import {
  UiActionMenu,
  type UiActionMenuItem,
} from "@/shared/ui/menu/action-menu";
import { WorkspaceStatusBadge } from "@/shared/ui/workspace/controls/workspace-status-badge";
import { WorkspaceCatalogAction } from "@/shared/ui/workspace/catalog/workspace-catalog-actions";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task";

import { formatScheduledDatetime } from "../scheduled-formatters";
import {
  getBehaviorSummary,
  getContextSummary,
  getDeliverySummary,
  getPrimaryStatus,
  getRunStatusLabel,
  getScheduleSummary,
  getSessionSummary,
  getSourceKindLabel,
  getToggleAction,
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
  const status = getPrimaryStatus(task);
  const toggleAction = getToggleAction(task);
  const menuItems = useMemo<UiActionMenuItem[]>(() => [
    {
      disabled: task.session_target.kind === "main",
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
      disabled: isDeleting,
      icon: <Trash2 className="h-3.5 w-3.5" />,
      label: "删除任务",
      tone: "danger",
      value: "delete",
    },
  ], [isDeleting, task.session_target.kind]);
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
            <WorkspaceStatusBadge label={status.label} size="compact" tone={status.tone} />
            {task.running ? (
              <WorkspaceStatusBadge label="执行占用中" size="compact" tone="running" />
            ) : null}
            {task.failure_streak > 0 ? (
              <WorkspaceStatusBadge label={`连续失败 ${task.failure_streak} 次`} size="compact" tone="default" />
            ) : null}
          </div>
          <div className="mt-3 flex flex-wrap gap-x-5 gap-y-2 text-sm text-(--text-default)">
            <span className="font-medium text-(--text-strong)">
              {getScheduleSummary(task.schedule)}
            </span>
            <span>下次 {formatScheduledDatetime(task.next_run_at, { emptyLabel: "未安排" })}</span>
            <span>
              最近 {getRunStatusLabel(task.last_run_status)} · {formatScheduledDatetime(task.last_run_at, { emptyLabel: "尚未执行" })}
            </span>
          </div>
          <p className="mt-3 text-sm leading-6 text-(--text-default)">
            {getBehaviorSummary(task)}
          </p>
          {task.last_error ? (
            <p className="mt-2 break-words rounded-[8px] border border-[color:color-mix(in_srgb,var(--destructive)_18%,transparent)] px-3 py-2 text-xs leading-5 text-(--destructive)">
              {task.last_error}
            </p>
          ) : null}
          <details className="group mt-3 text-xs text-(--text-muted)">
            <summary className="cursor-pointer list-none font-medium text-(--text-default) hover:text-(--text-strong)">
              查看执行与投递设置
            </summary>
            <div className="mt-3 rounded-[10px] border border-(--divider-subtle-color) px-3 py-3">
              <UiMetaGrid>
                <UiMetaItem label="归属对象" value={getContextSummary(task)} />
                <UiMetaItem label="执行会话" value={getSessionSummary(task)} />
                <UiMetaItem label="结果回传" value={getDeliverySummary(task.delivery, task.source)} />
                <UiMetaItem label="重叠策略" value={task.overlap_policy === "allow" ? "允许并行" : "跳过重叠"} />
              </UiMetaGrid>
              <div className="mt-3 flex flex-wrap gap-x-5 gap-y-2">
                <span>Agent {task.agent_id}</span>
                <span>来源 {getSourceKindLabel(task.source)}</span>
                {task.running_started_at ? (
                  <span>本次开始 {formatScheduledDatetime(task.running_started_at, { includeSeconds: true })}</span>
                ) : null}
                {task.expires_at ? (
                  <span>有效期至 {formatScheduledDatetime(task.expires_at, { includeSeconds: true })}</span>
                ) : null}
              </div>
            </div>
          </details>
        </div>

        <div className="flex shrink-0 items-center justify-end gap-2">
          <UiButton
            className="min-w-[92px]"
            disabled={isToggling}
            onClick={() => onToggleEnabled(task)}
            title={task.enabled ? "暂停后不会再按计划自动触发" : "恢复后会重新参与调度"}
            tone={toggleAction.tone}
          >
            {isToggling ? toggleAction.pending_label : toggleAction.label}
          </UiButton>
          <WorkspaceCatalogAction
            aria-label="立即运行"
            disabled={isRunning || task.running}
            onClick={() => onRunNow(task)}
            size="md"
            title={task.running ? "任务当前已经在运行中" : "立即触发一次执行"}
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
