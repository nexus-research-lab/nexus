"use client";

import type { LucideIcon } from "lucide-react";
import {
  BellRing,
  CalendarCheck2,
  CircleAlert,
  CirclePause,
  ClipboardList,
  LoaderCircle,
  MonitorCheck,
  Plus,
  RefreshCw,
} from "lucide-react";

import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiSkeleton } from "@/shared/ui/display/skeleton";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";

import type { ScheduledTaskPendingCommands } from "../controller/scheduled-task-directory-model";
import type { TaskDialogCreatePreset } from "../dialog/scheduled-task-dialog-types";
import { ScheduledTaskCard } from "./scheduled-task-card";
import {
  buildScheduledTaskBoard,
  SCHEDULED_TASK_SUGGESTIONS,
  type ScheduledTaskBoardColumn,
  type ScheduledTaskSuggestion,
} from "./scheduled-task-board-model";

interface ScheduledTaskBoardProps {
  errorMessage: string | null;
  isLoading: boolean;
  items: ScheduledTaskItem[];
  onCreate: () => void;
  onCreateFromPreset: (preset: TaskDialogCreatePreset) => void;
  onDelete: (task: ScheduledTaskItem) => void;
  onEdit: (task: ScheduledTaskItem) => void;
  onOpenHistory: (task: ScheduledTaskItem) => void;
  onRefresh: () => void;
  onRunNow: (task: ScheduledTaskItem) => void;
  onToggleEnabled: (task: ScheduledTaskItem) => void;
  pending: ScheduledTaskPendingCommands;
}

type ScheduledTaskBoardState =
  | { kind: "loading" }
  | { kind: "error"; message: string }
  | { kind: "empty" }
  | { columns: ScheduledTaskBoardColumn[]; kind: "ready" };

const COLUMN_TONE_CLASS_NAMES: Record<
  ScheduledTaskBoardColumn["tone"],
  string
> = {
  muted: "bg-(--text-soft)",
  primary: "bg-(--primary)",
  success: "bg-(--success)",
  warning: "bg-(--warning)",
};

const COLUMN_EMPTY_ICONS: Record<ScheduledTaskBoardColumn["id"], LucideIcon> = {
  attention: CircleAlert,
  running: LoaderCircle,
  scheduled: CalendarCheck2,
  stopped: CirclePause,
};

const SUGGESTION_ICONS: Record<ScheduledTaskSuggestion["icon"], LucideIcon> = {
  briefing: BellRing,
  monitor: MonitorCheck,
  review: ClipboardList,
};

function getScheduledTaskBoardState({
  errorMessage,
  isLoading,
  items,
}: Pick<ScheduledTaskBoardProps, "errorMessage" | "isLoading" | "items">): ScheduledTaskBoardState {
  if (isLoading) {
    return { kind: "loading" };
  }
  if (errorMessage) {
    return { kind: "error", message: errorMessage };
  }
  if (items.length === 0) {
    return { kind: "empty" };
  }
  return { columns: buildScheduledTaskBoard(items), kind: "ready" };
}

function ScheduledTaskLoadingBoard() {
  return (
    <div className="soft-scrollbar -mx-5 flex min-h-0 flex-1 overflow-x-auto overflow-y-hidden px-5 xl:-mx-6 xl:px-6">
      <div className="grid h-full min-w-[1080px] flex-1 grid-cols-4 gap-3">
        {Array.from({ length: 4 }, (_, columnIndex) => (
          <div
            className="h-full min-h-0 rounded-[8px] bg-[color:color-mix(in_srgb,var(--surface-interactive-hover-background)_58%,transparent)] p-3"
            key={columnIndex}
          >
            <div className="mb-4 flex items-center justify-between">
              <UiSkeleton className="h-4 w-24" />
              <UiSkeleton className="h-4 w-5 rounded-full" />
            </div>
            <UiSkeleton className="h-36 w-full rounded-[8px]" />
          </div>
        ))}
      </div>
    </div>
  );
}

function ScheduledTaskErrorState({
  message,
  onRefresh,
}: {
  message: string;
  onRefresh: () => void;
}) {
  return (
    <div className="flex min-h-0 flex-1 flex-col items-center justify-center border-y border-(--divider-subtle-color) px-6 text-center">
      <CircleAlert className="h-8 w-8 text-(--destructive)" />
      <h2 className="mt-4 text-[16px] font-semibold text-(--text-strong)">任务加载失败</h2>
      <p className="mt-1 max-w-md text-[12px] leading-5 text-(--text-muted)">{message}</p>
      <UiButton className="mt-4" onClick={onRefresh} size="sm" tone="primary" variant="surface">
        <RefreshCw className="h-3.5 w-3.5" />
        重新加载
      </UiButton>
    </div>
  );
}

function ScheduledTaskSuggestions({
  onCreate,
  onSelect,
}: {
  onCreate: () => void;
  onSelect: (preset: TaskDialogCreatePreset) => void;
}) {
  return (
    <section
      className="soft-scrollbar min-h-0 flex-1 overflow-y-auto pb-4 pt-3"
      aria-labelledby="scheduled-task-suggestions-title"
    >
      <div className="max-w-[720px]">
        <h2
          className="text-[18px] font-semibold tracking-[-0.02em] text-(--text-strong)"
          id="scheduled-task-suggestions-title"
        >
          从一个常用任务开始
        </h2>
        <p className="mt-1 text-[12px] leading-5 text-(--text-muted)">
          选择建议后仍可修改执行对象、时间和回传位置。
        </p>
      </div>

      <div className="mt-5 grid [grid-template-columns:repeat(auto-fit,minmax(min(100%,280px),1fr))] gap-2.5">
        {SCHEDULED_TASK_SUGGESTIONS.map((suggestion) => {
          const SuggestionIcon = SUGGESTION_ICONS[suggestion.icon];
          return (
            <button
              className="group flex min-h-[118px] items-start gap-3 rounded-[8px] border border-(--divider-subtle-color) bg-transparent p-4 text-left transition-[background,border-color] duration-(--motion-duration-fast) hover:border-(--surface-interactive-hover-border) hover:bg-(--surface-interactive-hover-background) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_24%,transparent)]"
              key={suggestion.title}
              onClick={() => onSelect(suggestion.preset)}
              type="button"
            >
              <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-[8px] border border-(--divider-subtle-color) text-(--primary)">
                <SuggestionIcon className="h-4 w-4" />
              </span>
              <span className="min-w-0 flex-1">
                <span className="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
                  <span className="text-[14px] font-semibold text-(--text-strong)">
                    {suggestion.title}
                  </span>
                  <span className="text-[11px] text-(--text-soft)">
                    {suggestion.scheduleLabel}
                  </span>
                </span>
                <span className="mt-1.5 block text-[12px] leading-5 text-(--text-muted)">
                  {suggestion.description}
                </span>
              </span>
            </button>
          );
        })}
      </div>

      <button
        className="mt-4 inline-flex items-center gap-1.5 text-[12px] font-semibold text-(--primary) transition-colors hover:text-(--text-strong) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_24%,transparent)]"
        onClick={onCreate}
        type="button"
      >
        <Plus className="h-3.5 w-3.5" />
        创建自定义任务
      </button>
    </section>
  );
}

function ScheduledTaskBoardColumnView({
  column,
  onDelete,
  onEdit,
  onOpenHistory,
  onRunNow,
  onToggleEnabled,
  pending,
}: {
  column: ScheduledTaskBoardColumn;
  onDelete: ScheduledTaskBoardProps["onDelete"];
  onEdit: ScheduledTaskBoardProps["onEdit"];
  onOpenHistory: ScheduledTaskBoardProps["onOpenHistory"];
  onRunNow: ScheduledTaskBoardProps["onRunNow"];
  onToggleEnabled: ScheduledTaskBoardProps["onToggleEnabled"];
  pending: ScheduledTaskPendingCommands;
}) {
  const EmptyIcon = COLUMN_EMPTY_ICONS[column.id];
  return (
    <section
      className="flex h-full min-h-0 min-w-0 flex-col rounded-[8px] bg-[color:color-mix(in_srgb,var(--surface-interactive-hover-background)_58%,transparent)]"
      aria-labelledby={`scheduled-column-${column.id}`}
    >
      <header className="flex min-h-16 items-start justify-between gap-3 border-b border-[color:color-mix(in_srgb,var(--divider-subtle-color)_72%,transparent)] px-3.5 py-3">
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <span className={cn("h-2 w-2 shrink-0 rounded-full", COLUMN_TONE_CLASS_NAMES[column.tone])} />
            <h2
              className="truncate text-[13px] font-semibold text-(--text-strong)"
              id={`scheduled-column-${column.id}`}
            >
              {column.title}
            </h2>
          </div>
          <p className="mt-1 truncate pl-4 text-[10.5px] text-(--text-soft)">
            {column.description}
          </p>
        </div>
        <span className="shrink-0 text-[11px] font-medium tabular-nums text-(--text-muted)">
          {column.items.length}
        </span>
      </header>

      {column.items.length > 0 ? (
        <div className="soft-scrollbar min-h-0 flex-1 space-y-2.5 overflow-y-auto overscroll-contain p-2.5">
          {column.items.map((task) => (
            <ScheduledTaskCard
              isDeleting={pending.get("delete")?.has(task.job_id) ?? false}
              isRunning={pending.get("run")?.has(task.job_id) ?? false}
              isToggling={pending.get("toggle")?.has(task.job_id) ?? false}
              key={task.job_id}
              onDelete={onDelete}
              onEdit={onEdit}
              onOpenHistory={onOpenHistory}
              onRunNow={onRunNow}
              onToggleEnabled={onToggleEnabled}
              task={task}
            />
          ))}
        </div>
      ) : (
        <div className="flex flex-1 flex-col items-center justify-center px-5 pb-12 text-center">
          <EmptyIcon className="h-6 w-6 text-(--icon-muted)" />
          <p className="mt-2 text-[11px] leading-5 text-(--text-soft)">
            {column.emptyDescription}
          </p>
        </div>
      )}
    </section>
  );
}

function ScheduledTaskReadyBoard({
  columns,
  ...props
}: Omit<ScheduledTaskBoardProps, "errorMessage" | "isLoading" | "items" | "onCreate" | "onCreateFromPreset" | "onRefresh"> & {
  columns: ScheduledTaskBoardColumn[];
}) {
  return (
    <section className="flex min-h-0 flex-1 flex-col" aria-label="定时任务看板">
      <div className="mb-3 flex items-end justify-between gap-4">
        <div>
          <h2 className="text-[18px] font-semibold tracking-[-0.02em] text-(--text-strong)">
            任务看板
          </h2>
          <p className="mt-0.5 text-[11px] leading-5 text-(--text-muted)">
            查看任务的当前状态、下次执行时间和异常情况。
          </p>
        </div>
      </div>
      <div className="soft-scrollbar -mx-5 min-h-0 flex-1 overflow-x-auto overflow-y-hidden px-5 xl:-mx-6 xl:px-6">
        <div className="grid h-full min-w-[1080px] grid-cols-4 gap-3">
          {columns.map((column) => (
            <ScheduledTaskBoardColumnView
              column={column}
              key={column.id}
              onDelete={props.onDelete}
              onEdit={props.onEdit}
              onOpenHistory={props.onOpenHistory}
              onRunNow={props.onRunNow}
              onToggleEnabled={props.onToggleEnabled}
              pending={props.pending}
            />
          ))}
        </div>
      </div>
    </section>
  );
}

export function ScheduledTaskBoard(props: ScheduledTaskBoardProps) {
  const state = getScheduledTaskBoardState(props);
  if (state.kind === "loading") {
    return <ScheduledTaskLoadingBoard />;
  }
  if (state.kind === "error") {
    return <ScheduledTaskErrorState message={state.message} onRefresh={props.onRefresh} />;
  }
  if (state.kind === "empty") {
    return (
      <ScheduledTaskSuggestions
        onCreate={props.onCreate}
        onSelect={props.onCreateFromPreset}
      />
    );
  }
  return <ScheduledTaskReadyBoard {...props} columns={state.columns} />;
}
