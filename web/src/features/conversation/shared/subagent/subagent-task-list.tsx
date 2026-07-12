"use client";

import { X } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { WorkspaceSurfaceToolbarAction } from "@/shared/ui/workspace/surface/workspace-surface-toolbar-action";
import { WorkspaceSurfaceView } from "@/shared/ui/workspace/surface/workspace-surface-view";
import type {
  SubagentTask,
  SubagentTaskListResponse,
} from "@/types/conversation/subagent-task";

import {
  isSubagentTaskActive,
  subagentTaskAvatarColor,
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
  isLoading,
  onClose,
  onRefresh,
  onSelectTask,
  showTitle = true,
  tasks,
}: SubagentTaskListProps) {
  const { t } = useI18n();
  const model = buildSubagentTaskListModel({ data, isLoading, tasks });

  return (
    <WorkspaceSurfaceView
      bodyClassName="px-3.5 pb-5 pt-4 sm:px-4"
      bodyScrollable
      contentClassName="min-h-full"
      header={showTitle ? {
        action: (
          <WorkspaceSurfaceToolbarAction
            ariaLabel={t("common.close")}
            onClick={onClose}
            title={t("common.close")}
          >
            <X className="h-3.5 w-3.5" />
            {t("common.close")}
          </WorkspaceSurfaceToolbarAction>
        ),
        kind: "page",
      } : undefined}
      maxWidthClassName="max-w-none"
      title={t("subagents.panel_title")}
    >
      <div>
        <SubagentTaskSection
          emptyText={t(ACTIVE_EMPTY_LABEL[model.activeEmptyState])}
          label={t("subagents.active_section")}
          onSelectTask={onSelectTask}
          tasks={model.activeTasks}
        />

        {error ? (
          <div className="mt-3 flex items-start gap-3 text-xs leading-5 text-(--destructive)">
            <p className="min-w-0 flex-1">{error}</p>
            <button
              className="shrink-0 font-semibold hover:underline"
              onClick={onRefresh}
              type="button"
            >
              {t("subagents.retry")}
            </button>
          </div>
        ) : null}

        {model.supportNotice ? (
          <p className="mt-3 max-w-[420px] text-[13px] leading-6 text-(--text-muted)">
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
      <h2 className="pr-9 text-[12px] font-semibold text-(--text-soft)">
        {label}{countInLabel ? ` · ${tasks.length}` : ""}
      </h2>

      {tasks.length === 0 && emptyText ? (
        <p className="mt-3 text-[12px] text-(--text-soft)">{emptyText}</p>
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
  const summary = [task.summary, task.description, task.last_tool_name]
    .map((value) => value?.trim() ?? "")
    .find(Boolean) ?? t("subagents.no_description");

  return (
    <button
      className="group -mx-1.5 flex w-[calc(100%+0.75rem)] min-w-0 items-start gap-2.5 rounded-[7px] px-1.5 py-1.5 text-left transition-colors hover:bg-(--surface-interactive-hover-background) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_28%,transparent)]"
      onClick={onClick}
      title={t("subagents.open_task")}
      type="button"
    >
      <SubagentTaskAvatar
        isActive={isSubagentTaskActive(task)}
        name={subagentTaskTitle(task)}
        taskId={task.task_id}
      />
      <span className="min-w-0 flex-1">
        <span className="flex min-w-0 items-baseline gap-3">
          <span className="min-w-0 flex-1 truncate text-[13px] font-medium leading-5 text-(--text-strong)">
            {subagentTaskTitle(task)}
          </span>
          {timestamp ? (
            <time className="shrink-0 text-[10.5px] tabular-nums text-(--text-soft)">
              {formatCompactElapsedTime(timestamp, locale)}
            </time>
          ) : null}
        </span>
        <span className="block truncate text-[11.5px] leading-4.5 text-(--text-muted)">
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
  taskId,
}: {
  className?: string;
  isActive?: boolean;
  name: string;
  taskId: string;
}) {
  const color = subagentTaskAvatarColor(taskId);

  return (
    <span
      aria-label={name}
      className={cn(
        "relative mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center overflow-hidden rounded-full shadow-[inset_0_0_0_1px_rgba(255,255,255,0.26)]",
        isActive && "ring-1 ring-[color:color-mix(in_srgb,var(--primary)_18%,transparent)] ring-offset-1 ring-offset-(--background)",
        className,
      )}
      style={{ backgroundColor: color }}
      title={name}
    >
      <span className="absolute h-[19px] w-1 rounded-full bg-white/72" />
      <span className="absolute h-[19px] w-1 rotate-45 rounded-full bg-white/72" />
      <span className="absolute h-[19px] w-1 rotate-90 rounded-full bg-white/72" />
      <span className="absolute h-[19px] w-1 -rotate-45 rounded-full bg-white/72" />
      <span className="absolute h-2 w-2 rounded-full border border-white/70 bg-white/18" />
    </span>
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
