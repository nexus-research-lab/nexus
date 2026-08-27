/**
 * INPUT: 运行历史资源状态、任务与运行命令。
 * OUTPUT: 紧凑加载/错误/空态或按时间排列的运行目录。
 * POS: Scheduled 历史弹窗正文，不解释请求身份。
 */
"use client";

import { RefreshCw } from "lucide-react";

import type { ResourceFailure } from "@/lib/error-message";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { UiSkeletonCardList } from "@/shared/ui/display/skeleton";
import type { ScheduledTaskRunItem } from "@/types/capability/scheduled-task/run";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";

import { ScheduledTaskRunHistoryItem } from "./scheduled-task-run-history-item";

interface ScheduledTaskRunHistoryContentProps {
  copiedRunId: string | null;
  failure: ResourceFailure | null;
  hasSnapshot: boolean;
  isLoading: boolean;
  onCopyDiagnostic: (run: ScheduledTaskRunItem) => void | Promise<void>;
  onRecover: (run: ScheduledTaskRunItem) => void | Promise<void>;
  onRefresh: () => void;
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
  failure,
  hasSnapshot,
  isLoading,
  onCopyDiagnostic,
  onRecover,
  onRefresh,
  onRetry,
  onRetryDelivery,
  pendingRecoveries,
  pendingRetries,
  pendingRetryDeliveries,
  runs,
  task,
}: ScheduledTaskRunHistoryContentProps) {
  const { t } = useI18n();
  const accessBlocked = Boolean(failure?.access);
  return (
    <div aria-busy={isLoading}>
      {failure && hasSnapshot && !failure.access ? (
        <UiResourceState
          className="mb-3 min-h-0 py-4"
          description={failure.message}
          impact={t("capability.scheduled_history_stale_impact")}
          nextStep={t("state.retry_next_step")}
          primaryAction={{
            icon: <RefreshCw className="h-3.5 w-3.5" />,
            label: t("state.retry"),
            onClick: onRefresh,
          }}
          role="status"
          size="sm"
          state="error"
          title={t("capability.scheduled_history_refresh_failed")}
        />
      ) : null}
      {accessBlocked && failure ? (
        <UiResourceState
          description={failure.message}
          impact={t("state.access_failure_impact")}
          nextStep={t("state.permission_next_step")}
          primaryAction={{
            icon: <RefreshCw className="h-3.5 w-3.5" />,
            label: t("state.retry"),
            onClick: onRefresh,
          }}
          state="error"
          title={t("state.permission_title")}
        />
      ) : isLoading && !hasSnapshot ? (
        <UiSkeletonCardList cardClassName="min-h-[108px]" count={4} />
      ) : failure && !hasSnapshot ? (
        <UiResourceState
          description={failure.message}
          impact={t("state.read_failure_impact")}
          nextStep={t("state.retry_next_step")}
          primaryAction={{
            icon: <RefreshCw className="h-3.5 w-3.5" />,
            label: t("state.retry"),
            onClick: onRefresh,
          }}
          state="error"
          title={t("capability.scheduled_history_load_failed")}
        />
      ) : runs.length === 0 ? (
        <UiResourceState
          description={t("capability.scheduled_history_empty_description")}
          nextStep={t("capability.scheduled_history_empty_next_step")}
          state="empty"
          title={t("capability.scheduled_history_empty_title")}
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
