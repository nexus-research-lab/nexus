// INPUT: 当前 scope 的任务快照、读取/定向任务缺项状态与刷新动作，以及可见页面的分钟时钟。
// OUTPUT: 当前语言的运行/历史/未知状态目录、可读时间及保留快照的单一读取反馈。
// POS: 子智能体目录纯视图；不解释底层异常，也不改变任务执行状态。
"use client";

import { useId, type ReactNode } from "react";
import { ArrowLeft, Loader2 } from "lucide-react";
import { formatRelativeTime } from "@/lib/format/relative-time";
import { useMinuteClock } from "@/shared/lib/react/use-minute-clock";
import { UiBadge } from "@/shared/ui/display/badge";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { UiIconButton } from "@/shared/ui/button/button";
import { UiSeededAvatar } from "@/shared/ui/display/seeded-avatar";
import { UiInlineNotice } from "@/shared/ui/feedback/inline-notice";
import { UiListRow } from "@/shared/ui/list/list-row";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import {
  WORKSPACE_PANEL_HEADER_HEIGHT_CLASS,
  WORKSPACE_PANEL_HEADER_PADDING_CLASS,
} from "@/shared/ui/workspace/surface/workspace-header-layout";
import { WorkspaceSurfaceView } from "@/shared/ui/workspace/surface/workspace-surface-view";
import type {
  SubagentTask,
  SubagentTaskListResponse,
} from "@/types/conversation/subagent-task";

import {
  getSubagentTaskStatus,
  isSubagentTaskActive,
  subagentTaskAvatarSeed,
  subagentTaskTimestamp,
  subagentTaskTitle,
} from "./subagent-task-model";
import {
  buildSubagentTaskListModel,
  type SubagentTaskListEmptyState,
  type SubagentTaskSupportNotice,
} from "./subagent-task-list-model";

const ACTIVE_EMPTY_LABEL: Record<
  SubagentTaskListEmptyState,
  TranslationKey
> = {
  empty: "subagents.no_active",
  loading: "subagents.loading",
};
const SUPPORT_NOTICE_LABEL: Record<
  Exclude<SubagentTaskSupportNotice, null>,
  TranslationKey
> = {
  claude: "subagents.cc_unsupported_description",
  generic: "subagents.unsupported_description",
};
const TASK_EXCEPTION_LABELS = {
  pending: { key: "subagents.status_pending", tone: "idle" },
  failed: { key: "subagents.status_failed", tone: "danger" },
  stopped: { key: "subagents.status_stopped", tone: "default" },
} as const;

interface SubagentTaskListProps {
  data: SubagentTaskListResponse | null;
  error: string | null;
  headerLeading?: ReactNode;
  isLoading: boolean;
  onClose: () => void;
  onRefresh: () => void;
  onSelectTask: (taskId: string) => void;
  requestedTaskUnavailable?: boolean;
  showTitle?: boolean;
  tasks: SubagentTask[];
}

export function SubagentTaskList({
  data,
  error,
  headerLeading,
  isLoading,
  requestedTaskUnavailable = false,
  onClose,
  onRefresh,
  onSelectTask,
  showTitle = true,
  tasks,
}: SubagentTaskListProps) {
  const { t } = useI18n();
  const model = buildSubagentTaskListModel({ data, hasError: Boolean(error), isLoading, tasks });
  const now = useMinuteClock(!model.supportNotice && tasks.some((task) => subagentTaskTimestamp(task) > 0));
  const isDesktopPanel = !showTitle;
  const content = (
    <div aria-busy={isLoading}>
      {!isDesktopPanel && headerLeading ? (
        <div className="mb-4 flex min-h-7 items-center">
          {headerLeading}
        </div>
      ) : null}

      <SubagentTaskSection
        emptyState={requestedTaskUnavailable ? null : model.activeEmptyState}
        label={t("subagents.active_section")}
        now={now}
        onSelectTask={onSelectTask}
        tasks={model.activeTasks}
      />

      {error || requestedTaskUnavailable ? (
        <UiInlineNotice
          action={{
            label: t("subagents.retry"),
            onClick: onRefresh,
            pending: isLoading,
          }}
          className="mt-3"
          message={t(error ? "subagents.list_load_failed_impact" : "subagents.requested_task_missing_detail")}
          title={t(error ? "subagents.list_load_failed_title" : "subagents.requested_task_missing_title")}
          tone={error ? "danger" : "neutral"}
        />
      ) : null}

      {model.supportNotice ? (
        <p className={cn(
          "mt-3 max-w-[420px]",
          getUiTypographyClassName({ role: "supporting", tone: "muted" }),
        )}>
          {t(SUPPORT_NOTICE_LABEL[model.supportNotice])}
        </p>
      ) : null}

      {model.unknownTasks.length > 0 ? (
        <div className="mt-5">
          <SubagentTaskSection
            countInLabel
            label={t("subagents.unknown_section")}
            now={now}
            onSelectTask={onSelectTask}
            tasks={model.unknownTasks}
          />
        </div>
      ) : null}
      {model.historyTasks.length > 0 ? (
        <div className="mt-5">
          <SubagentTaskSection
            countInLabel
            label={t("subagents.history_section")}
            now={now}
            onSelectTask={onSelectTask}
            tasks={model.historyTasks}
          />
        </div>
      ) : null}
    </div>
  );

  return (
    <WorkspaceSurfaceView
      bodyClassName={isDesktopPanel
        ? "flex min-h-0 flex-1 flex-col px-0 py-0"
        : "px-3.5 pb-5 pt-4 sm:px-4"}
      bodyScrollable={!isDesktopPanel}
      contentClassName={isDesktopPanel
        ? "flex h-full min-h-0 flex-col"
        : "min-h-full"}
      header={showTitle ? {
        kind: "mobile",
        leading: (
          <UiIconButton
            aria-label={t("common.back")}
            onClick={onClose}
            shape="round"
            size="lg"
            variant="ghost"
          >
            <ArrowLeft className="h-4 w-4" />
          </UiIconButton>
        ),
      } : undefined}
      maxWidthClassName="max-w-none"
      title={t("subagents.panel_title")}
    >
      {isDesktopPanel ? (
        <>
          <div className={cn(
            "flex min-w-0 shrink-0 items-center border-b border-(--divider-subtle-color)",
            WORKSPACE_PANEL_HEADER_HEIGHT_CLASS,
            WORKSPACE_PANEL_HEADER_PADDING_CLASS,
          )}>
            {headerLeading ?? (
              <span className={cn(
                "truncate",
                getUiTypographyClassName({
                  role: "caption",
                  tone: "soft",
                  weight: "medium",
                }),
              )}>
                {t("subagents.panel_title")}
              </span>
            )}
          </div>
          <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto px-3.5 pb-5 pt-4 sm:px-4">
            {content}
          </div>
        </>
      ) : (
        content
      )}
    </WorkspaceSurfaceView>
  );
}

function SubagentTaskSection({
  countInLabel = false,
  emptyState,
  label,
  now,
  onSelectTask,
  tasks,
}: {
  countInLabel?: boolean;
  emptyState?: SubagentTaskListEmptyState | null;
  label: string;
  now: number;
  onSelectTask: (taskId: string) => void;
  tasks: SubagentTask[];
}) {
  const { locale, t } = useI18n();
  const headingId = useId();
  return (
    <section aria-labelledby={headingId}>
      <h2 id={headingId} className={cn(
        getUiTypographyClassName({
          role: "supporting",
          tone: "soft",
          weight: "semibold",
        }),
      )}>
        {label}{countInLabel ? ` · ${new Intl.NumberFormat(locale).format(tasks.length)}` : ""}
      </h2>

      {tasks.length === 0 && emptyState ? (
        <p className={cn(
          "mt-3 flex items-center gap-2",
          getUiTypographyClassName({ role: "supporting", tone: "soft" }),
        )} role={emptyState === "loading" ? "status" : undefined}>
          {emptyState === "loading" ? <Loader2 aria-hidden="true" className={getUiSpinnerClassName({ size: "sm", tone: "muted" })} /> : null}
          {t(ACTIVE_EMPTY_LABEL[emptyState])}
        </p>
      ) : null}

      {tasks.length > 0 ? (
        <div className="mt-2 space-y-px">
          {tasks.map((task) => (
            <SubagentTaskRow
              key={task.task_id}
              now={now}
              onClick={() => onSelectTask(task.task_id)}
              task={task}
            />
          ))}
        </div>
      ) : null}
    </section>
  );
}

function SubagentTaskRow({
  now,
  onClick,
  task,
}: {
  now: number;
  onClick: () => void;
  task: SubagentTask;
}) {
  const { locale, t } = useI18n();
  const timestamp = subagentTaskTimestamp(task);
  const status = getSubagentTaskStatus(task);
  const exception = status === "pending" || status === "failed" || status === "stopped"
    ? TASK_EXCEPTION_LABELS[status]
    : null;
  const title = subagentTaskTitle(task, t);
  const description = task.description?.trim() ?? "";
  const summary = [
    task.summary,
    description === title ? "" : description,
    task.last_tool_name,
  ]
    .map((value) => value?.trim() ?? "")
    .find(Boolean) ?? t("subagents.no_description");

  return (
    <UiListRow
      data-subagent-task-id={task.task_id}
      className="items-start"
      density="dense"
      leading={(
        <SubagentTaskAvatar
          isActive={isSubagentTaskActive(task)}
          name={title}
          seed={subagentTaskAvatarSeed(task)}
        />
      )}
      onClick={onClick}
      tooltip={t("subagents.open_task")}
    >
      <span className="min-w-0 flex-1">
        <span className="flex min-w-0 items-baseline gap-3">
          <span className={cn(
            "min-w-0 flex-1 truncate",
            getUiTypographyClassName({
              role: "supporting",
              tone: "strong",
              weight: "medium",
            }),
          )}>
            {title}
          </span>
          {timestamp ? (
            <time
              dateTime={new Date(timestamp).toISOString()}
              title={new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeStyle: "short" }).format(timestamp)}
              className={cn(
                "shrink-0 tabular-nums",
                getUiTypographyClassName({ role: "caption", tone: "soft" }),
              )}
            >
              {formatRelativeTime(timestamp, locale, { compact: true, now })}
            </time>
          ) : null}
        </span>
        <span className="flex min-w-0 items-center gap-1.5">
          {exception ? <UiBadge size="sm" tone={exception.tone}>{t(exception.key)}</UiBadge> : null}
          <span className={cn("min-w-0 truncate", getUiTypographyClassName({ role: "metadata", tone: "muted" }))}>{summary}</span>
        </span>
      </span>
    </UiListRow>
  );
}

export function SubagentTaskAvatar({
  className,
  isActive = false,
  name,
  seed,
}: {
  className?: string;
  isActive?: boolean;
  name: string;
  seed: string;
}) {
  return (
    <UiSeededAvatar
      className={cn(
        "mt-0.5",
        className,
      )}
      seed={seed}
      size="2xs"
      state={isActive ? "running" : "default"}
      title={name}
    />
  );
}
