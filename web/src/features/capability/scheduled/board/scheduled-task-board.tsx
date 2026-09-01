/**
 * INPUT: 定时任务集合、资源状态、建议预设与任务命令。
 * OUTPUT: 快速创建目录或四列真实状态看板。
 * POS: 定时任务主内容视图；空列由标题与数量自解释。
 */
"use client";

import type { LucideIcon } from "lucide-react";
import {
  BellRing,
  ClipboardList,
  LoaderCircle,
  MonitorCheck,
  Plus,
  RefreshCw,
} from "lucide-react";

import type { ResourceFailure } from "@/lib/error-message";
import { cn } from "@/shared/ui/class-name";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { UiSkeleton } from "@/shared/ui/display/skeleton";
import { useI18n } from "@/shared/i18n/i18n-context";
import {
  WORKSPACE_CATALOG_GRID_CLASS_NAME,
  WORKSPACE_CONTENT_BLEED_CLASS_NAME,
} from "@/shared/ui/layout/workspace-content-layout";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";
import type { AutomationPermissionDecision } from "@/types/capability/scheduled-task/permission";

import type {
  ScheduledTaskPendingCommands,
  ScheduledTaskUnconfirmedCommands,
} from "../controller/scheduled-task-directory-model";
import {
  hasScheduledTaskCommandForJob,
  isScheduledTaskMutationBlocked,
} from "../controller/scheduled-task-directory-model";
import type { TaskDialogCreatePreset } from "../dialog/scheduled-task-dialog-types";
import { ScheduledTaskCard } from "./scheduled-task-card";
import { getScheduledTaskBoardState } from "./scheduled-task-board-state";
import {
  buildScheduledTaskBoard,
  buildScheduledTaskSuggestions,
  type ScheduledTaskBoardColumn,
  type ScheduledTaskSuggestion,
} from "./scheduled-task-board-model";

interface ScheduledTaskBoardProps {
  failure: ResourceFailure | null;
  hasSnapshot: boolean;
  isLoading: boolean;
  isPermissionLoading: boolean;
  items: ScheduledTaskItem[];
  onCreate: () => void;
  onCreateFromPreset: (preset: TaskDialogCreatePreset) => void;
  onConfirmDeletionStopped: (task: ScheduledTaskItem) => void;
  onDelete: (task: ScheduledTaskItem) => void;
  onEdit: (task: ScheduledTaskItem) => void;
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
  pending: ScheduledTaskPendingCommands;
  permissionFailure: ResourceFailure | null;
  unconfirmed: ScheduledTaskUnconfirmedCommands;
}

const COLUMN_TONE_CLASS_NAMES: Record<
  ScheduledTaskBoardColumn["tone"],
  string
> = {
  muted: "bg-(--text-soft)",
  primary: "bg-(--primary)",
  success: "bg-(--success)",
  warning: "bg-(--warning)",
};

const SUGGESTION_ICONS: Record<ScheduledTaskSuggestion["icon"], LucideIcon> = {
  briefing: BellRing,
  monitor: MonitorCheck,
  review: ClipboardList,
};

function ScheduledTaskLoadingBoard() {
  const { t } = useI18n();
  return (
    <div className={cn(
      WORKSPACE_CONTENT_BLEED_CLASS_NAME,
      "soft-scrollbar flex min-h-0 flex-1 overflow-x-auto overflow-y-hidden",
    )} aria-label={t("capability.scheduled_loading")} aria-live="polite" role="status">
      <div className="grid h-full min-w-[1080px] flex-1 grid-cols-4 gap-3">
        {Array.from({ length: 4 }, (_, columnIndex) => (
          <div
            className="h-full min-h-0 border-l border-(--divider-subtle-color) p-3 pl-4 first:border-l-0 first:pl-0"
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
  failure,
  isLoading,
  onRefresh,
}: {
  failure: ResourceFailure;
  isLoading: boolean;
  onRefresh: () => void;
}) {
  const { t } = useI18n();
  const accessBlocked = Boolean(failure.access);

  return (
    <UiResourceState
      className="min-h-0 flex-1 border-y border-(--divider-subtle-color)"
      description={failure.message}
      impact={t(accessBlocked ? "state.access_failure_impact" : "state.read_failure_impact")}
      nextStep={t(accessBlocked ? "state.permission_next_step" : "state.retry_next_step")}
      primaryAction={{
        busy: isLoading,
        busyLabel: t("capability.scheduled_refreshing"),
        icon: <RefreshCw className="h-3.5 w-3.5" />,
        label: t("state.retry"),
        onClick: onRefresh,
      }}
      size="sm"
      state="error"
      title={t(accessBlocked ? "state.permission_title" : "capability.scheduled_load_failed")}
    />
  );
}

function ScheduledTaskSuggestions({
  onCreate,
  onSelect,
}: {
  onCreate: () => void;
  onSelect: (preset: TaskDialogCreatePreset) => void;
}) {
  const { t } = useI18n();
  const suggestions = buildScheduledTaskSuggestions(t);

  return (
    <section
      className="soft-scrollbar min-h-0 flex-1 overflow-y-auto pb-4 pt-3"
      aria-labelledby="scheduled-task-suggestions-title"
    >
      <div className="max-w-[720px]">
        <h2
          className="text-md font-medium tracking-[-0.01em] text-(--text-strong)"
          id="scheduled-task-suggestions-title"
        >
          {t("capability.scheduled_quick_start_title")}
        </h2>
        <p className="mt-1 text-compact leading-5 text-(--text-muted)">
          {t("capability.scheduled_empty_description")}
        </p>
      </div>

      <div className={cn(WORKSPACE_CATALOG_GRID_CLASS_NAME, "mt-4 gap-2")}>
        {suggestions.map((suggestion) => {
          const SuggestionIcon = SUGGESTION_ICONS[suggestion.icon];
          return (
            <button
              className="group flex min-h-[104px] items-start gap-2.5 rounded-[8px] border border-(--divider-subtle-color) bg-transparent p-3 text-left transition-[background,border-color] duration-(--motion-duration-fast) hover:border-(--surface-interactive-hover-border) hover:bg-(--surface-interactive-hover-background) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_24%,transparent)]"
              key={suggestion.title}
              onClick={() => onSelect(suggestion.preset)}
              type="button"
            >
              <span className="flex h-8 w-8 shrink-0 items-center justify-center radius-control-sm border border-(--divider-subtle-color) text-(--primary)">
                <SuggestionIcon className="h-3.5 w-3.5" />
              </span>
              <span className="min-w-0 flex-1">
                <span className="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
                  <span className="text-sm font-medium text-(--text-strong)">
                    {suggestion.title}
                  </span>
                  <span className="text-xs text-(--text-soft)">
                    {suggestion.scheduleLabel}
                  </span>
                </span>
                <span className="mt-1 block text-compact leading-5 text-(--text-muted)">
                  {suggestion.description}
                </span>
              </span>
            </button>
          );
        })}
      </div>

      <button
        className="mt-4 inline-flex items-center gap-1.5 text-compact font-semibold text-(--primary) transition-colors hover:text-(--text-strong) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_24%,transparent)]"
        onClick={onCreate}
        type="button"
      >
        <Plus className="h-3.5 w-3.5" />
        {t("capability.scheduled_create_blank")}
      </button>
    </section>
  );
}

function ScheduledTaskBoardColumnView({
  column,
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
  pending,
  unconfirmed,
}: {
  column: ScheduledTaskBoardColumn;
  onDelete: ScheduledTaskBoardProps["onDelete"];
  onEdit: ScheduledTaskBoardProps["onEdit"];
  onConfirmDeletionStopped: ScheduledTaskBoardProps["onConfirmDeletionStopped"];
  onOpenHistory: ScheduledTaskBoardProps["onOpenHistory"];
  onOpenConnector: ScheduledTaskBoardProps["onOpenConnector"];
  onPermissionDecision: ScheduledTaskBoardProps["onPermissionDecision"];
  onPermissionResume: ScheduledTaskBoardProps["onPermissionResume"];
  onRefresh: ScheduledTaskBoardProps["onRefresh"];
  onRunNow: ScheduledTaskBoardProps["onRunNow"];
  onToggleEnabled: ScheduledTaskBoardProps["onToggleEnabled"];
  pending: ScheduledTaskPendingCommands;
  unconfirmed: ScheduledTaskUnconfirmedCommands;
}) {
  return (
    <section
      className="flex h-full min-h-0 min-w-0 flex-col border-l border-(--divider-subtle-color) pl-3 first:border-l-0 first:pl-0"
      aria-labelledby={`scheduled-column-${column.id}`}
    >
      <header className="flex items-center justify-between gap-3 border-b border-(--divider-subtle-color) px-3 py-2.5">
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <span className={cn("h-2 w-2 shrink-0 rounded-full", COLUMN_TONE_CLASS_NAMES[column.tone])} />
            <h2
              className="truncate text-sm font-semibold text-(--text-strong)"
              id={`scheduled-column-${column.id}`}
            >
              {column.title}
            </h2>
          </div>
        </div>
        <span className="shrink-0 text-xs font-medium tabular-nums text-(--text-muted)">
          {column.items.length}
        </span>
      </header>

      {column.items.length > 0 ? (
        <div className="soft-scrollbar min-h-0 flex-1 space-y-2 overflow-y-auto overscroll-contain p-2">
          {column.items.map((task) => (
            <ScheduledTaskCard
              isDeleting={pending.get("delete")?.has(task.job_id) ?? false}
              isDeleteUnconfirmed={unconfirmed.get("delete")?.has(task.job_id) ?? false}
              isDeletionReviewPending={[
                pending,
                unconfirmed,
              ].some((state) => hasScheduledTaskCommandForJob(
                state,
                "confirmDeletionStopped",
                task.job_id,
              ))}
              isMutationBlocked={isScheduledTaskMutationBlocked(
                pending,
                unconfirmed,
                task.job_id,
              )}
              isPermissionPending={hasScheduledTaskCommandForJob(
                pending,
                "permission",
                task.job_id,
              )}
              isPermissionUnconfirmed={hasScheduledTaskCommandForJob(
                unconfirmed,
                "permission",
                task.job_id,
              )}
              isRunning={pending.get("run")?.has(task.job_id) ?? false}
              isRunUnconfirmed={unconfirmed.get("run")?.has(task.job_id) ?? false}
              isToggling={pending.get("toggle")?.has(task.job_id) ?? false}
              isToggleUnconfirmed={unconfirmed.get("toggle")?.has(task.job_id) ?? false}
              key={task.job_id}
              onDelete={onDelete}
              onEdit={onEdit}
              onConfirmDeletionStopped={onConfirmDeletionStopped}
              onOpenHistory={onOpenHistory}
              onOpenConnector={onOpenConnector}
              onPermissionDecision={onPermissionDecision}
              onPermissionResume={onPermissionResume}
              onRefresh={onRefresh}
              onRunNow={onRunNow}
              onToggleEnabled={onToggleEnabled}
              task={task}
            />
          ))}
        </div>
      ) : (
        <div className="flex-1" aria-hidden="true" />
      )}
    </section>
  );
}

function ScheduledTaskReadyBoard({
  columns,
  ...props
}: Omit<ScheduledTaskBoardProps, "isLoading" | "items" | "onCreate" | "onCreateFromPreset"> & {
  columns: ScheduledTaskBoardColumn[];
}) {
  return (
    <section className="flex min-h-0 flex-1 flex-col" aria-label="定时任务看板">
      <div className={cn(
        WORKSPACE_CONTENT_BLEED_CLASS_NAME,
        "soft-scrollbar min-h-0 flex-1 overflow-x-auto overflow-y-hidden",
      )}>
        <div className="grid h-full min-w-[1080px] grid-cols-4 gap-3">
          {columns.map((column) => (
            <ScheduledTaskBoardColumnView
              column={column}
              key={column.id}
              onDelete={props.onDelete}
              onEdit={props.onEdit}
              onConfirmDeletionStopped={props.onConfirmDeletionStopped}
              onOpenHistory={props.onOpenHistory}
              onOpenConnector={props.onOpenConnector}
              onPermissionDecision={props.onPermissionDecision}
              onPermissionResume={props.onPermissionResume}
              onRefresh={props.onRefresh}
              onRunNow={props.onRunNow}
              onToggleEnabled={props.onToggleEnabled}
              pending={props.pending}
              unconfirmed={props.unconfirmed}
            />
          ))}
        </div>
      </div>
    </section>
  );
}

export function ScheduledTaskBoard(props: ScheduledTaskBoardProps) {
  const { t } = useI18n();
  const state = getScheduledTaskBoardState({
    failure: props.failure,
    hasSnapshot: props.hasSnapshot,
    isLoading: props.isLoading,
    itemCount: props.items.length,
  });
  if (state === "loading") {
    return <ScheduledTaskLoadingBoard />;
  }
  if (state === "error" && props.failure) {
    return (
      <ScheduledTaskErrorState
        failure={props.failure}
        isLoading={props.isLoading}
        onRefresh={props.onRefresh}
      />
    );
  }
  return (
    <div className="flex min-h-0 flex-1 flex-col">
      {props.isLoading || props.isPermissionLoading ? (
        <div
          className="mb-2 flex items-center gap-2 text-xs text-(--text-muted)"
          role="status"
        >
          <LoaderCircle className="h-3.5 w-3.5 motion-safe:animate-spin" />
          {t(props.isLoading
            ? "capability.scheduled_refreshing"
            : "capability.scheduled_permission_refreshing")}
        </div>
      ) : null}
      {props.failure && props.hasSnapshot ? (
        <UiResourceState
          className="mb-3 min-h-0 py-3"
          description={props.failure.message}
          impact={t("state.stale_snapshot_impact")}
          nextStep={t("state.retry_next_step")}
          primaryAction={{
            busy: props.isLoading,
            busyLabel: t("capability.scheduled_refreshing"),
            icon: <RefreshCw className="h-3.5 w-3.5" />,
            label: t("state.retry"),
            onClick: props.onRefresh,
          }}
          role="status"
          size="sm"
          state="error"
          title={t("capability.scheduled_refresh_failed")}
        />
      ) : null}
      {props.permissionFailure ? (
        <UiResourceState
          className="mb-3 min-h-0 py-3"
          description={props.permissionFailure.message}
          impact={t(props.permissionFailure.access
            ? "capability.scheduled_permission_access_impact"
            : "capability.scheduled_permission_stale_impact")}
          nextStep={t(props.permissionFailure.access
            ? "state.permission_next_step"
            : "capability.scheduled_permission_stale_next_step")}
          primaryAction={{
            busy: props.isLoading || props.isPermissionLoading,
            busyLabel: t("capability.scheduled_refreshing"),
            icon: <RefreshCw className="h-3.5 w-3.5" />,
            label: t("state.retry"),
            onClick: props.onRefresh,
          }}
          role="status"
          size="sm"
          state="error"
          title={t(props.permissionFailure.access
            ? "capability.scheduled_permission_access_title"
            : "capability.scheduled_permission_refresh_failed")}
        />
      ) : null}
      {state === "empty" ? (
        <ScheduledTaskSuggestions
          onCreate={props.onCreate}
          onSelect={props.onCreateFromPreset}
        />
      ) : (
        <ScheduledTaskReadyBoard
          {...props}
          columns={buildScheduledTaskBoard(props.items)}
        />
      )}
    </div>
  );
}
