// INPUT: 单次运行、所属任务、命令状态与诊断/恢复动作。
// OUTPUT: 原生可折叠历史行，组合状态、结果详情与合法动作。
// POS: Scheduled 历史单项装配层；状态与动作资格来自 history model。

"use client";

import { useState } from "react";
import { ChevronDown } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import { WorkspaceStatusBadge } from "@/shared/ui/workspace/controls/workspace-status-badge";
import type { ScheduledTaskRunItem } from "@/types/capability/scheduled-task/run";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";

import { formatScheduledDatetime } from "../../scheduled-formatters";
import { ScheduledTaskRunActions } from "./scheduled-task-run-actions";
import { ScheduledTaskRunDetails } from "./scheduled-task-run-details";
import {
  formatDuration,
  getDeliveryStatusMeta,
  getStatusMeta,
} from "../scheduled-task-run-history-model";

interface ScheduledTaskRunHistoryItemProps {
  defaultOpen: boolean;
  isCopied: boolean;
  isRecoveryUnconfirmed: boolean;
  isRecovering: boolean;
  isRetryDeliveryUnconfirmed: boolean;
  isRetryUnconfirmed: boolean;
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
  defaultOpen,
  isCopied,
  isRecoveryUnconfirmed,
  isRecovering,
  isRetryDeliveryUnconfirmed,
  isRetryUnconfirmed,
  isRetrying,
  isRetryingDelivery,
  onCopyDiagnostic,
  onRecover,
  onRetry,
  onRetryDelivery,
  run,
  task,
}: ScheduledTaskRunHistoryItemProps) {
  const [open, setOpen] = useState(defaultOpen);
  const status = getStatusMeta(run.status);
  const deliveryStatus = getDeliveryStatusMeta(run.delivery_status);
  const showDeliveryStatus = run.delivery_status !== "not_required"
    && run.delivery_status !== "skipped";
  return (
    <article className="py-1.5 first:pt-0 last:pb-0">
      <details
        className="group"
        onToggle={(event) => setOpen(event.currentTarget.open)}
        open={open}
      >
        <summary className="flex cursor-pointer list-none items-center gap-3 radius-control-md px-2 py-2.5 transition-colors hover:bg-(--surface-control-background) [&::-webkit-details-marker]:hidden">
          <WorkspaceStatusBadge label={status.label} size="compact" tone={status.tone} />
          <div className="min-w-0 flex-1">
            <p className={cn(
              "truncate",
              getUiTypographyClassName({
                role: "supporting",
                tone: "strong",
                weight: "medium",
              }),
            )}>
              {formatScheduledDatetime(run.scheduled_for, { includeSeconds: true })}
            </p>
            <p className={cn(
              "mt-0.5 flex flex-wrap items-center gap-x-1.5",
              getUiTypographyClassName({ role: "caption", tone: "muted" }),
            )}>
              <span>{formatDuration(run.started_at, run.finished_at)}</span>
              {showDeliveryStatus && deliveryStatus ? (
                <>
                  <span aria-hidden="true">·</span>
                  <span>{deliveryStatus.label}</span>
                </>
              ) : null}
            </p>
          </div>
          <ChevronDown className="h-4 w-4 shrink-0 text-(--icon-muted) transition-transform group-open:rotate-180" />
        </summary>
        <div className="mx-2 border-l border-(--divider-subtle-color) pb-3 pl-4 pr-2">
          <ScheduledTaskRunDetails
            isCopied={isCopied}
            onCopyDiagnostic={() => onCopyDiagnostic(run)}
            run={run}
          />
          <ScheduledTaskRunActions
            isRecoveryUnconfirmed={isRecoveryUnconfirmed}
            isRecovering={isRecovering}
            isRetryDeliveryUnconfirmed={isRetryDeliveryUnconfirmed}
            isRetryUnconfirmed={isRetryUnconfirmed}
            isRetrying={isRetrying}
            isRetryingDelivery={isRetryingDelivery}
            onRecover={() => onRecover(run)}
            onRetry={() => onRetry(run)}
            onRetryDelivery={() => onRetryDelivery(run)}
            run={run}
            task={task}
          />
        </div>
      </details>
    </article>
  );
}
