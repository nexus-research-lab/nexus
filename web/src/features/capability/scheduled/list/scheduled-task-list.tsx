"use client";

import { Clock3 } from "lucide-react";

import { UiSkeleton } from "@/shared/ui/display/skeleton";
import { UiStateBlock } from "@/shared/ui/display/state-block";
import { WorkspaceCatalogTextAction } from "@/shared/ui/workspace/catalog/workspace-catalog-actions";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";

import type { ScheduledTaskPendingCommands } from "../controller/scheduled-task-directory-model";
import { ScheduledTaskListItem } from "./scheduled-task-list-item";
import { sortTasks } from "./scheduled-task-list-model";

interface ScheduledTaskListProps {
  errorMessage: string | null;
  isLoading: boolean;
  items: ScheduledTaskItem[];
  onCreate: () => void;
  onDelete: (task: ScheduledTaskItem) => void;
  onEdit: (task: ScheduledTaskItem) => void;
  onOpenHistory: (task: ScheduledTaskItem) => void;
  onRefresh: () => void;
  onRunNow: (task: ScheduledTaskItem) => void;
  onToggleEnabled: (task: ScheduledTaskItem) => void;
  pending: ScheduledTaskPendingCommands;
}

type ScheduledTaskListState =
  | { kind: "loading" }
  | { kind: "error"; message: string }
  | { kind: "empty" }
  | { items: ScheduledTaskItem[]; kind: "ready" };

function getScheduledTaskListState({
  errorMessage,
  isLoading,
  items,
}: Pick<ScheduledTaskListProps, "errorMessage" | "isLoading" | "items">): ScheduledTaskListState {
  if (isLoading) {
    return { kind: "loading" };
  }
  if (errorMessage) {
    return { kind: "error", message: errorMessage };
  }
  if (items.length === 0) {
    return { kind: "empty" };
  }
  return { items: sortTasks(items), kind: "ready" };
}

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

function ScheduledTaskListContent({
  onCreate,
  onDelete,
  onEdit,
  onOpenHistory,
  onRefresh,
  onRunNow,
  onToggleEnabled,
  pending,
  state,
}: Omit<ScheduledTaskListProps, "errorMessage" | "isLoading" | "items"> & {
  state: ScheduledTaskListState;
}) {
  switch (state.kind) {
    case "loading":
      return <ScheduledTaskLoadingRows />;
    case "error":
      return (
        <UiStateBlock
          actions={(
            <WorkspaceCatalogTextAction onClick={onRefresh} tone="primary">
              重试
            </WorkspaceCatalogTextAction>
          )}
          description={state.message}
          title="任务列表加载失败"
          tone="danger"
          variant="plain"
        />
      );
    case "empty":
      return (
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
      );
    case "ready":
      return (
        <div className="divide-y divide-(--divider-subtle-color)">
          {state.items.map((task) => (
            <ScheduledTaskListItem
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
      );
  }
}

export function ScheduledTaskList(props: ScheduledTaskListProps) {
  const state = getScheduledTaskListState(props);
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
              共 {props.items.length} 个任务，按下次运行时间排序。
            </p>
          </div>
        </div>
      </div>
      <div className="soft-scrollbar min-h-0">
        <ScheduledTaskListContent
          onCreate={props.onCreate}
          onDelete={props.onDelete}
          onEdit={props.onEdit}
          onOpenHistory={props.onOpenHistory}
          onRefresh={props.onRefresh}
          onRunNow={props.onRunNow}
          onToggleEnabled={props.onToggleEnabled}
          pending={props.pending}
          state={state}
        />
      </div>
    </section>
  );
}
