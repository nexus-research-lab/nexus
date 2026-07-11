"use client";

import { History } from "lucide-react";

import { UiSkeletonCardList } from "@/shared/ui/display/skeleton";
import { UiStateBlock } from "@/shared/ui/display/state-block";
import type { ScheduledTaskRunItem } from "@/types/capability/scheduled-task/run";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";

import { ScheduledTaskRunHistoryItem } from "./scheduled-task-run-history-item";

interface ScheduledTaskRunHistoryContentProps {
  copiedRunId: string | null;
  errorMessage: string | null;
  isLoading: boolean;
  onCopyDiagnostic: (run: ScheduledTaskRunItem) => void | Promise<void>;
  onRecover: (run: ScheduledTaskRunItem) => void | Promise<void>;
  onRetry: (run: ScheduledTaskRunItem) => void | Promise<void>;
  onRetryDelivery: (run: ScheduledTaskRunItem) => void | Promise<void>;
  pendingRecoveries: ReadonlySet<string>;
  pendingRetries: ReadonlySet<string>;
  pendingRetryDeliveries: ReadonlySet<string>;
  runs: ScheduledTaskRunItem[];
  task: ScheduledTaskItem;
}

export function ScheduledTaskRunHistoryContent({
  copiedRunId,
  errorMessage,
  isLoading,
  onCopyDiagnostic,
  onRecover,
  onRetry,
  onRetryDelivery,
  pendingRecoveries,
  pendingRetries,
  pendingRetryDeliveries,
  runs,
  task,
}: ScheduledTaskRunHistoryContentProps) {
  return (
    <div>
      {isLoading ? (
        <UiSkeletonCardList cardClassName="min-h-[108px]" count={4} />
      ) : errorMessage ? (
        <UiStateBlock description={errorMessage} title="运行历史加载失败" tone="danger" />
      ) : runs.length === 0 ? (
        <UiStateBlock
          description="手动执行或等调度器首次触发后，这里会显示每次运行的状态、耗时和错误信息。"
          icon={<History className="h-6 w-6 text-(--icon-strong)" />}
          title="还没有运行记录"
        />
      ) : (
        <div className="divide-y divide-(--divider-subtle-color)">
          {runs.map((run) => (
            <ScheduledTaskRunHistoryItem
              isCopied={copiedRunId === run.run_id}
              isRecovering={pendingRecoveries.has(run.run_id)}
              isRetrying={pendingRetries.has(run.run_id)}
              isRetryingDelivery={pendingRetryDeliveries.has(run.run_id)}
              key={run.run_id}
              onCopyDiagnostic={onCopyDiagnostic}
              onRecover={onRecover}
              onRetry={onRetry}
              onRetryDelivery={onRetryDelivery}
              run={run}
              task={task}
            />
          ))}
        </div>
      )}
    </div>
  );
}
