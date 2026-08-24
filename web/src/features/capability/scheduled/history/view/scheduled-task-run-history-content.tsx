/**
 * INPUT: 运行历史资源状态、任务与运行命令。
 * OUTPUT: 紧凑加载/错误/空态或按时间排列的运行目录。
 * POS: Scheduled 历史弹窗正文，不解释请求身份。
 */
"use client";

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
          description="任务运行后，记录会显示在这里。"
          title="暂无运行记录"
        />
      ) : (
        <div className="divide-y divide-(--divider-subtle-color)">
          {runs.map((run, index) => (
            <ScheduledTaskRunHistoryItem
              defaultOpen={index === 0}
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
