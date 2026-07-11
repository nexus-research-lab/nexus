"use client";

import { Copy, Download, FolderOpen, RotateCcw, X } from "lucide-react";

import { downloadWorkspaceFileApi } from "@/lib/api/agent-manage-api";
import { getWorkspaceFileExternalActionCopy } from "@/lib/workspace-file-action";
import { WorkspaceStatusBadge } from "@/shared/ui/workspace/controls/workspace-status-badge";
import type { ScheduledTaskItem, ScheduledTaskRunItem } from "@/types/capability/scheduled-task";
import { formatScheduledDatetime } from "./scheduled-formatters";
import {
  artifactFileName,
  formatDuration,
  getDeliveryStatusMeta,
  getStatusMeta,
  isRetryableStatus,
  shouldShowAssistantText,
} from "./scheduled-task-run-history-model";

interface ScheduledTaskRunHistoryItemProps {
  task: ScheduledTaskItem;
  run: ScheduledTaskRunItem;
  copiedRunId: string | null;
  retryingRunId: string | null;
  retryingDeliveryRunId: string | null;
  recoveringRunId: string | null;
  canRetryTask: boolean;
  canRetryDelivery: boolean;
  canRecoverTaskRun: boolean;
  onCopyDiagnostic: (run: ScheduledTaskRunItem) => void | Promise<void>;
  onRetry: (run: ScheduledTaskRunItem) => void | Promise<void>;
  onRetryDelivery: (run: ScheduledTaskRunItem) => void | Promise<void>;
  onRecover: (run: ScheduledTaskRunItem) => void | Promise<void>;
}

function downloadRunArtifact(agentId: string, artifactPath: string) {
  void downloadWorkspaceFileApi(
    agentId,
    artifactPath,
    artifactFileName(artifactPath),
  ).catch((error) => {
    console.error("[ScheduledTaskRunHistoryDialog] 处理任务产物失败:", error);
  });
}

function ScheduledRunArtifactButton({
  agentId: agentId,
  artifactPath: artifactPath,
}: {
  agentId: string;
  artifactPath: string;
}) {
  const actionCopy = getWorkspaceFileExternalActionCopy(artifactFileName(artifactPath));
  const Icon = actionCopy.mode === "reveal" ? FolderOpen : Download;
  const label = actionCopy.mode === "reveal" ? "打开产物" : "下载产物";
  return (
    <button
      aria-label={actionCopy.ariaLabel}
      className="mt-2 inline-flex items-center justify-end gap-1.5 text-xs font-semibold text-(--primary) transition duration-(--motion-duration-fast) hover:text-(--primary-hover)"
      onClick={() => downloadRunArtifact(agentId, artifactPath)}
      title={actionCopy.title}
      type="button"
    >
      <Icon className="h-3.5 w-3.5" />
      {label}
    </button>
  );
}

export function ScheduledTaskRunHistoryItem({
  task,
  run,
  copiedRunId: copiedRunId,
  retryingRunId: retryingRunId,
  retryingDeliveryRunId: retryingDeliveryRunId,
  recoveringRunId: recoveringRunId,
  canRetryTask: canRetryTask,
  canRetryDelivery: canRetryDelivery,
  canRecoverTaskRun: canRecoverTaskRun,
  onCopyDiagnostic: onCopyDiagnostic,
  onRetry: onRetry,
  onRetryDelivery: onRetryDelivery,
  onRecover: onRecover,
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
              <WorkspaceStatusBadge label={deliveryStatus.label} size="compact" tone={deliveryStatus.tone} />
            ) : null}
          </div>
          <div className="mt-3 grid gap-3 text-sm text-(--text-default) md:grid-cols-2">
            <div>
              <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-(--text-muted)">
                调度时间
              </p>
              <p className="mt-1.5 font-medium text-(--text-strong)">
                {formatScheduledDatetime(run.scheduled_for, { includeSeconds: true })}
              </p>
            </div>
            <div>
              <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-(--text-muted)">
                执行耗时
              </p>
              <p className="mt-1.5 font-medium text-(--text-strong)">
                {formatDuration(run.started_at, run.finished_at)}
              </p>
            </div>
          </div>
          <details className="mt-3 text-xs text-(--text-muted)">
            <summary className="cursor-pointer list-none font-medium text-(--text-default) hover:text-(--text-strong)">
              诊断详情
            </summary>
            <div className="mt-2 space-y-1.5 rounded-[10px] border border-(--divider-subtle-color) px-3 py-2.5">
              <p className="break-all">Run {run.run_id}</p>
              {run.trigger_kind ? <p>Trigger {run.trigger_kind}</p> : null}
              {typeof run.message_count === "number" ? <p>Messages {run.message_count}</p> : null}
              {run.session_key ? <p className="break-all">Session {run.session_key}</p> : null}
              {run.round_id ? <p className="break-all">Round {run.round_id}</p> : null}
              {run.session_id ? <p className="break-all">Runtime {run.session_id}</p> : null}
              {run.delivery_to ? <p className="break-all">Delivery {run.delivery_to}</p> : null}
              {run.delivered_at ? <p>Delivered {formatScheduledDatetime(run.delivered_at, { includeSeconds: true })}</p> : null}
              {run.delivery_attempts ? <p>Delivery attempts {run.delivery_attempts}</p> : null}
              {run.delivery_next_attempt_at ? <p>Next delivery retry {formatScheduledDatetime(run.delivery_next_attempt_at, { includeSeconds: true })}</p> : null}
              {run.delivery_dead_letter_at ? <p>Delivery dead letter {formatScheduledDatetime(run.delivery_dead_letter_at, { includeSeconds: true })}</p> : null}
              <p>Started {formatScheduledDatetime(run.started_at, { includeSeconds: true })}</p>
              <p>Finished {formatScheduledDatetime(run.finished_at, { includeSeconds: true })}</p>
              <p>Attempts {run.attempts}</p>
              <button
                className="inline-flex items-center gap-1.5 pt-1 font-semibold text-(--text-default) hover:text-(--text-strong)"
                onClick={() => void onCopyDiagnostic(run)}
                type="button"
              >
                <Copy className="h-3.5 w-3.5" />
                {copiedRunId === run.run_id ? "已复制" : "复制诊断"}
              </button>
            </div>
          </details>
        </div>

        <div className="shrink-0 text-right text-sm text-(--text-default)">
          <div className="flex flex-col items-end gap-1.5">
            {isRetryableStatus(run.status) && canRetryTask ? (
              <button
                className="inline-flex items-center justify-end gap-1.5 text-xs font-semibold text-(--primary) transition duration-(--motion-duration-fast) hover:text-(--primary-hover) disabled:opacity-60"
                disabled={retryingRunId === run.run_id || task.running}
                onClick={() => void onRetry(run)}
                title={task.running ? "任务当前正在运行" : "用当前任务配置重新运行一次"}
                type="button"
              >
                <RotateCcw className="h-3.5 w-3.5" />
                {retryingRunId === run.run_id ? "触发中" : "重新运行"}
              </button>
            ) : null}
            {run.delivery_status === "failed" && canRetryDelivery ? (
              <button
                className="inline-flex items-center justify-end gap-1.5 text-xs font-semibold text-(--primary) transition duration-(--motion-duration-fast) hover:text-(--primary-hover) disabled:opacity-60"
                disabled={retryingDeliveryRunId === run.run_id}
                onClick={() => void onRetryDelivery(run)}
                title="只重试这次运行的结果投递，不重新执行任务"
                type="button"
              >
                <RotateCcw className="h-3.5 w-3.5" />
                {retryingDeliveryRunId === run.run_id ? "投递中" : "重试投递"}
              </button>
            ) : null}
            {run.status === "running" && task.running && canRecoverTaskRun ? (
              <button
                className="inline-flex items-center justify-end gap-1.5 text-xs font-semibold text-(--destructive) transition duration-(--motion-duration-fast) hover:text-(--destructive) disabled:opacity-60"
                disabled={recoveringRunId === run.run_id}
                onClick={() => void onRecover(run)}
                title="把该运行标记为取消，并释放任务占用"
                type="button"
              >
                <X className="h-3.5 w-3.5" />
                {recoveringRunId === run.run_id ? "释放中" : "释放占用"}
              </button>
            ) : null}
          </div>
          {run.artifact_path ? (
            <ScheduledRunArtifactButton
              agentId={task.agent_id}
              artifactPath={run.artifact_path}
            />
          ) : null}
        </div>
      </div>
      {run.error_message ? (
        <div className="mt-3 rounded-[14px] border border-[color:color-mix(in_srgb,var(--destructive)_15%,transparent)] px-3 py-2.5 text-sm text-(--destructive)">
          {run.error_message}
        </div>
      ) : null}
      {run.delivery_error ? (
        <div className="mt-3 rounded-[14px] border border-[color:color-mix(in_srgb,var(--destructive)_15%,transparent)] px-3 py-2.5 text-sm text-(--destructive)">
          投递失败：{run.delivery_error}
        </div>
      ) : null}
      {run.result_summary ? (
        <div className="mt-3 rounded-[14px] border border-(--divider-subtle-color) px-3 py-2.5 text-sm leading-6 text-(--text-default)">
          {run.result_summary}
        </div>
      ) : null}
      {run.result_text ? (
        <div className="mt-3 rounded-[14px] border border-(--divider-subtle-color) px-3 py-2.5">
          <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-(--text-muted)">
            运行输出
          </p>
          <pre className="mt-2 max-h-64 whitespace-pre-wrap break-words text-sm leading-6 text-(--text-default)">
            {run.result_text}
          </pre>
        </div>
      ) : null}
      {shouldShowAssistantText(run) ? (
        <div className="mt-3 rounded-[14px] border border-(--divider-subtle-color) px-3 py-2.5">
          <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-(--text-muted)">
            助手回复
          </p>
          <pre className="mt-2 max-h-64 whitespace-pre-wrap break-words text-sm leading-6 text-(--text-default)">
            {run.assistant_text}
          </pre>
        </div>
      ) : null}
    </article>
  );
}
