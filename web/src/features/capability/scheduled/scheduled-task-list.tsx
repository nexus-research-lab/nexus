"use client";

import { Clock3, History, MoreHorizontal, Pencil, Play, Trash2 } from "lucide-react";

import { UiButton } from "@/shared/ui/button";
import { UiMetaGrid, UiMetaItem } from "@/shared/ui/meta-grid";
import { UiSkeleton } from "@/shared/ui/skeleton";
import { UiStateBlock } from "@/shared/ui/state-block";
import { WorkspaceStatusBadge } from "@/shared/ui/workspace/controls/workspace-status-badge";
import {
  WorkspaceCatalogAction,
  WorkspaceCatalogTextAction,
} from "@/shared/ui/workspace/catalog/workspace-catalog-card";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task";
import { formatScheduledDatetime } from "./scheduled-formatters";
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
  sortTasks,
} from "./scheduled-task-list-model";

function ScheduledTaskLoadingRows() {
  return (
    <div className="divide-y divide-(--divider-subtle-color)">
      {Array.from({ length: 3 }, (_, index) => (
        <div className="py-4 first:pt-0" key={index}>
          <div className="flex items-start gap-3">
            <UiSkeleton className="mt-1 h-9 w-9 rounded-[12px]" />
            <div className="min-w-0 flex-1 space-y-2">
              <UiSkeleton className="h-4 w-40" />
              <UiSkeleton className="h-3 w-full max-w-[520px]" />
              <UiSkeleton className="h-3 w-3/5" />
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}

interface ScheduledTaskListProps {
  items: ScheduledTaskItem[];
  isLoading: boolean;
  errorMessage: string | null;
  runPendingJobId?: string | null;
  togglePendingJobId?: string | null;
  deletePendingJobId?: string | null;
  onCreate: () => void;
  onRefresh?: () => void | Promise<void>;
  onRunNow?: (task: ScheduledTaskItem) => void | Promise<void>;
  onToggleEnabled?: (task: ScheduledTaskItem) => void | Promise<void>;
  onDelete?: (task: ScheduledTaskItem) => void | Promise<void>;
  onEdit?: (task: ScheduledTaskItem) => void;
  onOpenHistory?: (task: ScheduledTaskItem) => void;
}

export function ScheduledTaskList({
  items,
  isLoading: isLoading,
  errorMessage: errorMessage,
  runPendingJobId: runPendingJobId = null,
  togglePendingJobId: togglePendingJobId = null,
  deletePendingJobId: deletePendingJobId = null,
  onCreate: onCreate,
  onRefresh: onRefresh,
  onRunNow: onRunNow,
  onToggleEnabled: onToggleEnabled,
  onDelete: onDelete,
  onEdit: onEdit,
  onOpenHistory: onOpenHistory,
}: ScheduledTaskListProps) {
  const sortedItems = sortTasks(items);

  return (
    <section className="min-h-[320px]">
      <div className="mb-3 flex items-start justify-between gap-3 border-b border-(--divider-subtle-color) pb-2">
        <div className="flex min-w-0 items-start gap-2">
          <Clock3 className="mt-1 h-4 w-4 shrink-0 text-(--icon-default)" />
          <div className="min-w-0">
            <h2 className="text-[18px] font-medium tracking-[-0.025em] text-(--text-strong)">
              任务清单
            </h2>
            <p className="text-[12px] leading-5 text-(--text-muted)">
              共 {items.length} 个任务，按下次运行时间排序。
            </p>
          </div>
        </div>
      </div>

      <div className="soft-scrollbar min-h-0">
        {isLoading ? (
          <ScheduledTaskLoadingRows />
        ) : errorMessage ? (
          <UiStateBlock
            actions={(
              <WorkspaceCatalogTextAction onClick={() => void onRefresh?.()} tone="primary">
                重试
              </WorkspaceCatalogTextAction>
            )}
            description={errorMessage}
            title="任务列表加载失败"
            tone="danger"
            variant="plain"
          />
        ) : items.length === 0 ? (
          <UiStateBlock
            actions={(
              <WorkspaceCatalogTextAction onClick={onCreate} tone="primary">
                新建任务
              </WorkspaceCatalogTextAction>
            )}
            description="创建任务后，这里会显示它的执行时间和最近结果。"
            size="sm"
            title="还没有定时任务"
            variant="plain"
          />
        ) : (
          <div className="divide-y divide-(--divider-subtle-color)">
            {sortedItems.map((task) => {
              const status = getPrimaryStatus(task);
              const toggleAction = getToggleAction(task);
              const runPending = runPendingJobId === task.job_id;
              const togglePending = togglePendingJobId === task.job_id;
              const deletePending = deletePendingJobId === task.job_id;
              return (
                <article
                  key={task.job_id}
                  className="py-4 first:pt-0 last:pb-0"
                >
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

                    <div className="flex shrink-0 flex-wrap items-center gap-3 lg:justify-end">
                      <div className="flex items-center justify-end gap-2">
                        <UiButton
                          className="min-w-[92px]"
                          disabled={togglePending}
                          onClick={() => void onToggleEnabled?.(task)}
                          title={task.enabled ? "暂停后不会再按计划自动触发" : "恢复后会重新参与调度"}
                          tone={toggleAction.tone}
                        >
                          {togglePending ? toggleAction.pending_label : toggleAction.label}
                        </UiButton>
                        <WorkspaceCatalogAction
                          aria-label="立即运行"
                          disabled={runPending || task.running}
                          onClick={() => void onRunNow?.(task)}
                          size="md"
                          title={task.running ? "任务当前已经在运行中" : "立即触发一次执行"}
                        >
                          <Play className="h-3.5 w-3.5" />
                        </WorkspaceCatalogAction>
                        <details className="group relative">
                          <summary
                            aria-label="更多操作"
                            className="flex h-9 w-9 cursor-pointer list-none items-center justify-center rounded-[10px] border border-(--divider-subtle-color) text-(--icon-default) transition hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
                            title="更多操作"
                          >
                            <MoreHorizontal className="h-4 w-4" />
                          </summary>
                          <div className="absolute right-0 z-20 mt-2 min-w-[148px] rounded-[12px] border border-(--divider-subtle-color) bg-(--surface-raised-background) p-1.5 shadow-lg">
                            <button
                              className="flex w-full items-center gap-2 rounded-[8px] px-3 py-2 text-left text-sm text-(--text-default) hover:bg-(--surface-interactive-hover-background) disabled:opacity-50"
                              disabled={task.session_target.kind === "main"}
                              onClick={() => onOpenHistory?.(task)}
                              type="button"
                            >
                              <History className="h-3.5 w-3.5" />
                              运行历史
                            </button>
                            <button
                              className="flex w-full items-center gap-2 rounded-[8px] px-3 py-2 text-left text-sm text-(--text-default) hover:bg-(--surface-interactive-hover-background)"
                              onClick={() => onEdit?.(task)}
                              type="button"
                            >
                              <Pencil className="h-3.5 w-3.5" />
                              编辑任务
                            </button>
                            <button
                              className="flex w-full items-center gap-2 rounded-[8px] px-3 py-2 text-left text-sm text-(--destructive) hover:bg-(--surface-interactive-hover-background) disabled:opacity-50"
                              disabled={deletePending}
                              onClick={() => void onDelete?.(task)}
                              type="button"
                            >
                              <Trash2 className="h-3.5 w-3.5" />
                              删除任务
                            </button>
                          </div>
                        </details>
                      </div>
                    </div>
                  </div>
                </article>
              );
            })}
          </div>
        )}
      </div>
    </section>
  );
}
