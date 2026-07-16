"use client";

import { useCallback, useRef, useState } from "react";
import {
  CalendarClock,
  Clock3,
  History,
  MoreHorizontal,
  PauseCircle,
  Pencil,
  Play,
  PlayCircle,
  Trash2,
} from "lucide-react";

import { UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import {
  UiActionMenu,
  type UiActionMenuItem,
} from "@/shared/ui/menu/action-menu";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";

import {
  getScheduledTaskCardPresentation,
} from "./scheduled-task-board-model";

interface ScheduledTaskCardProps {
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

type TaskMenuAction = "delete" | "edit" | "toggle";

export function ScheduledTaskCard({
  isDeleting,
  isRunning,
  isToggling,
  onDelete,
  onEdit,
  onOpenHistory,
  onRunNow,
  onToggleEnabled,
  task,
}: ScheduledTaskCardProps) {
  const [isMenuOpen, setIsMenuOpen] = useState(false);
  const menuAnchorRef = useRef<HTMLButtonElement>(null);
  const closeMenu = useCallback(() => setIsMenuOpen(false), []);
  const presentation = getScheduledTaskCardPresentation(task, {
    isDeleting,
    isRunning,
    isToggling,
  });
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
    <article
      className={cn(
        "group rounded-[8px] border bg-(--surface-raised-background) p-3.5 shadow-[0_1px_2px_rgba(15,23,42,0.04)] transition-[border-color,background-color] duration-(--motion-duration-fast) hover:border-(--surface-interactive-hover-border)",
        presentation.columnId === "attention"
          ? "border-[color:color-mix(in_srgb,var(--warning)_30%,var(--divider-subtle-color))]"
          : "border-(--divider-subtle-color)",
      )}
    >
      <div className="flex min-w-0 items-start justify-between gap-2">
        <span className="min-w-0 truncate text-[10.5px] font-medium text-(--text-soft)">
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

      <h3 className="mt-1 line-clamp-2 text-[14px] font-semibold leading-5 text-(--text-strong)">
        {task.name}
      </h3>
      <p className="mt-2 line-clamp-2 whitespace-pre-line text-[12px] leading-5 text-(--text-muted)">
        {task.instruction}
      </p>

      <div className="mt-3 space-y-1.5 text-[11px] leading-4 text-(--text-default)">
        <div className="flex min-w-0 items-center gap-1.5">
          <CalendarClock className="h-3.5 w-3.5 shrink-0 text-(--icon-muted)" />
          <span className="truncate font-medium">{presentation.scheduleSummary}</span>
        </div>
        <div className="flex min-w-0 items-center gap-1.5 text-(--text-muted)">
          <Clock3 className="h-3.5 w-3.5 shrink-0 text-(--icon-muted)" />
          <span className="truncate">{presentation.timingSummary}</span>
        </div>
      </div>

      {presentation.lastError ? (
        <p
          className="mt-3 line-clamp-2 border-l-2 border-(--destructive) pl-2 text-[11px] leading-4 text-(--destructive)"
          title={presentation.lastError}
        >
          {presentation.lastError}
        </p>
      ) : null}

      <div className="mt-3 flex items-center justify-end gap-1 border-t border-(--divider-subtle-color) pt-2.5">
        <UiIconButton
          aria-label="运行历史"
          disabled={presentation.historyDisabled}
          onClick={() => onOpenHistory(task)}
          size="sm"
          title={presentation.historyDisabled ? "主会话任务没有独立运行历史" : "运行历史"}
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
  );
}
