// INPUT: 当前 scope 的子智能体任务快照、读取状态与刷新动作。
// OUTPUT: 保留已有任务快照并完整说明读取失败影响和恢复路径的任务列表。
// POS: 子智能体目录纯视图；不解释底层异常，也不改变任务执行状态。
"use client";

import type { ReactNode } from "react";
import { ArrowLeft } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { UiIconButton } from "@/shared/ui/button/button";
import { UiSeededAvatar } from "@/shared/ui/display/seeded-avatar";
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
const ELAPSED_TIME_UNITS = [
  { milliseconds: 86_400_000, suffix: { en: "d", zh: " 天" } },
  { milliseconds: 3_600_000, suffix: { en: "h", zh: " 小时" } },
  { milliseconds: 60_000, suffix: { en: "m", zh: " 分钟" } },
] as const;

interface SubagentTaskListProps {
  data: SubagentTaskListResponse | null;
  error: string | null;
  headerLeading?: ReactNode;
  isLoading: boolean;
  onClose: () => void;
  onRefresh: () => void;
  onSelectTask: (taskId: string) => void;
  showTitle?: boolean;
  tasks: SubagentTask[];
}

export function SubagentTaskList({
  data,
  error,
  headerLeading,
  isLoading,
  onClose,
  onRefresh,
  onSelectTask,
  showTitle = true,
  tasks,
}: SubagentTaskListProps) {
  const { t } = useI18n();
  const model = buildSubagentTaskListModel({ data, isLoading, tasks });
  const isDesktopPanel = !showTitle;
  const content = (
    <div>
      {!isDesktopPanel && headerLeading ? (
        <div className="mb-4 flex min-h-7 items-center">
          {headerLeading}
        </div>
      ) : null}

      <SubagentTaskSection
        emptyText={t(ACTIVE_EMPTY_LABEL[model.activeEmptyState])}
        label={t("subagents.active_section")}
        onSelectTask={onSelectTask}
        tasks={model.activeTasks}
      />

      {error ? (
        <div
          aria-atomic="true"
          aria-live="polite"
          className="mt-3 rounded-[10px] border border-[color:color-mix(in_srgb,var(--destructive)_24%,transparent)] px-3 py-2.5"
          role="status"
        >
          <p className="text-xs font-semibold leading-5 text-(--destructive)">
            {t("subagents.list_load_failed_title")}
          </p>
          <p className="mt-1 text-xs leading-5 text-(--text-muted)">
            {t("subagents.list_load_failed_impact")}
          </p>
          <p className="mt-1 text-xs font-medium leading-5 text-(--text-default)">
            {t("subagents.list_load_failed_next_step")}
          </p>
          <button
            className="mt-2 text-xs font-semibold text-(--brand-action) hover:underline"
            onClick={onRefresh}
            type="button"
          >
            {t("subagents.retry")}
          </button>
        </div>
      ) : null}

      {model.supportNotice ? (
        <p className="mt-3 max-w-[420px] text-sm leading-6 text-(--text-muted)">
          {t(SUPPORT_NOTICE_LABEL[model.supportNotice])}
        </p>
      ) : null}

      <div className="mt-5">
        <SubagentTaskSection
          countInLabel
          label={t("subagents.completed_section")}
          onSelectTask={onSelectTask}
          tasks={model.completedTasks}
        />
      </div>
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
              <span className="truncate text-xs font-medium text-(--text-soft)">
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
  emptyText,
  label,
  onSelectTask,
  tasks,
}: {
  countInLabel?: boolean;
  emptyText?: string;
  label: string;
  onSelectTask: (taskId: string) => void;
  tasks: SubagentTask[];
}) {
  return (
    <section>
      <h2 className="pr-9 text-compact font-semibold text-(--text-soft)">
        {label}{countInLabel ? ` · ${tasks.length}` : ""}
      </h2>

      {tasks.length === 0 && emptyText ? (
        <p className="mt-3 text-compact text-(--text-soft)">{emptyText}</p>
      ) : null}

      {tasks.length > 0 ? (
        <div className="mt-2 space-y-px">
          {tasks.map((task) => (
            <SubagentTaskRow
              key={task.task_id}
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
  onClick,
  task,
}: {
  onClick: () => void;
  task: SubagentTask;
}) {
  const { locale, t } = useI18n();
  const timestamp = subagentTaskTimestamp(task);
  const title = subagentTaskTitle(task);
  const description = task.description?.trim() ?? "";
  const summary = [
    task.summary,
    description === title ? "" : description,
    task.last_tool_name,
  ]
    .map((value) => value?.trim() ?? "")
    .find(Boolean) ?? t("subagents.no_description");

  return (
    <button
      className="group -mx-1.5 flex w-[calc(100%+0.75rem)] min-w-0 items-start gap-2.5 radius-control-sm px-1.5 py-1.5 text-left transition-colors hover:bg-(--surface-interactive-hover-background) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_28%,transparent)]"
      onClick={onClick}
      title={t("subagents.open_task")}
      type="button"
    >
      <SubagentTaskAvatar
        isActive={isSubagentTaskActive(task)}
        name={title}
        seed={subagentTaskAvatarSeed(task)}
      />
      <span className="min-w-0 flex-1">
        <span className="flex min-w-0 items-baseline gap-3">
          <span className="min-w-0 flex-1 truncate text-sm font-medium leading-5 text-(--text-strong)">
            {title}
          </span>
          {timestamp ? (
            <time className="shrink-0 text-xs tabular-nums text-(--text-soft)">
              {formatCompactElapsedTime(timestamp, locale)}
            </time>
          ) : null}
        </span>
        <span className="block truncate text-compact leading-4.5 text-(--text-muted)">
          {summary}
        </span>
      </span>
    </button>
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
        isActive && "ring-1 ring-[color:color-mix(in_srgb,var(--primary)_18%,transparent)] ring-offset-1 ring-offset-(--background)",
        className,
      )}
      seed={seed}
      size="2xs"
      title={name}
    />
  );
}

function formatCompactElapsedTime(timestamp: number, locale: string): string {
  const elapsedMs = Math.max(0, Date.now() - timestamp);
  const unit = ELAPSED_TIME_UNITS.find(
    ({ milliseconds }) => elapsedMs >= milliseconds,
  );
  if (!unit) {
    return locale === "en" ? "now" : "刚刚";
  }
  const value = Math.floor(elapsedMs / unit.milliseconds);
  return `${value}${locale === "en" ? unit.suffix.en : unit.suffix.zh}`;
}
