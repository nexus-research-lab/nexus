"use client";

import { WorkspaceStatusBadge } from "@/shared/ui/workspace/controls/workspace-status-badge";
import type {
  ScheduledTaskItem,
  ScheduledTaskRunItem,
} from "@/types/capability/scheduled-task";

import { formatScheduledDatetime } from "../../scheduled-formatters";
import { ScheduledTaskRunActions } from "./scheduled-task-run-actions";
import { ScheduledTaskRunDetails } from "./scheduled-task-run-details";
import {
  formatDuration,
  getDeliveryStatusMeta,
  getStatusMeta,
} from "../scheduled-task-run-history-model";

interface ScheduledTaskRunHistoryItemProps {
  isCopied: boolean;
  isRecovering: boolean;
  isRetrying: boolean;
  isRetryingDelivery: boolean;
  onCopyDiagnostic: (run: ScheduledTaskRunItem) => void | Promise<void>;
  onRecover: (run: ScheduledTaskRunItem) => void | Promise<void>;
  onRetry: (run: ScheduledTaskRunItem) => void | Promise<void>;
  onRetryDelivery: (run: ScheduledTaskRunItem) => void | Promise<void>;
  run: ScheduledTaskRunItem;
  task: ScheduledTaskItem;
}

export function ScheduledTaskRunHistoryItem({
  isCopied,
  isRecovering,
  isRetrying,
  isRetryingDelivery,
  onCopyDiagnostic,
  onRecover,
  onRetry,
  onRetryDelivery,
  run,
  task,
}: ScheduledTaskRunHistoryItemProps) {
  const status = getStatusMeta(run.status);
  const deliveryStatus = getDeliveryStatusMeta(run.delivery_status);
  return (
    <article className="py-4 first:pt-0 last:pb-0">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <WorkspaceStatusBadge label={status.label} size="compact" tone={status.tone} />
            {deliveryStatus ? (
              <WorkspaceStatusBadge
                label={deliveryStatus.label}
                size="compact"
                tone={deliveryStatus.tone}
              />
            ) : null}
          </div>
          <div className="mt-3 grid gap-3 text-sm text-(--text-default) md:grid-cols-2">
            <RunTimingMetric
              label="调度时间"
              value={formatScheduledDatetime(run.scheduled_for, { includeSeconds: true })}
            />
            <RunTimingMetric
              label="执行耗时"
              value={formatDuration(run.started_at, run.finished_at)}
            />
          </div>
          <ScheduledTaskRunDetails
            isCopied={isCopied}
            onCopyDiagnostic={() => onCopyDiagnostic(run)}
            run={run}
          />
        </div>
        <ScheduledTaskRunActions
          isRecovering={isRecovering}
          isRetrying={isRetrying}
          isRetryingDelivery={isRetryingDelivery}
          onRecover={() => onRecover(run)}
          onRetry={() => onRetry(run)}
          onRetryDelivery={() => onRetryDelivery(run)}
          run={run}
          task={task}
        />
      </div>
    </article>
  );
}

function RunTimingMetric({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-(--text-muted)">
        {label}
      </p>
      <p className="mt-1.5 font-medium text-(--text-strong)">
        {value}
      </p>
    </div>
  );
}
