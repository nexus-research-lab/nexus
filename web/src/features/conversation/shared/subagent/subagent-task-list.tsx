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
        <UiInlineNotice
          action={{
            label: t("subagents.retry"),
            onClick: onRefresh,
          }}
          className="mt-3"
          message={t("subagents.list_load_failed_impact")}
          title={t("subagents.list_load_failed_title")}
          tone="danger"
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
      <h2 className={cn(
        "pr-9",
        getUiTypographyClassName({
          role: "supporting",
          tone: "soft",
          weight: "semibold",
        }),
      )}>
        {label}{countInLabel ? ` · ${tasks.length}` : ""}
      </h2>

      {tasks.length === 0 && emptyText ? (
        <p className={cn(
          "mt-3",
          getUiTypographyClassName({ role: "supporting", tone: "soft" }),
        )}>{emptyText}</p>
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
    <UiListRow
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
            <time className={cn(
              "shrink-0 tabular-nums",
              getUiTypographyClassName({ role: "caption", tone: "soft" }),
            )}>
              {formatCompactElapsedTime(timestamp, locale)}
            </time>
          ) : null}
        </span>
        <span className={cn(
          "block truncate",
          getUiTypographyClassName({ role: "metadata", tone: "muted" }),
        )}>
          {summary}
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
