"use client";

import { Copy, Download, FolderOpen, RotateCcw, X } from "lucide-react";

import { download_workspace_file_api } from "@/lib/api/agent-manage-api";
import { get_workspace_file_external_action_copy } from "@/lib/workspace-file-action";
import { WorkspaceStatusBadge } from "@/shared/ui/workspace/controls/workspace-status-badge";
import type { ScheduledTaskItem, ScheduledTaskRunItem } from "@/types/capability/scheduled-task";
import { format_scheduled_datetime } from "./scheduled-formatters";
import {
  artifact_file_name,
  format_duration,
  get_delivery_status_meta,
  get_status_meta,
  is_retryable_status,
  should_show_assistant_text,
} from "./scheduled-task-run-history-model";

interface ScheduledTaskRunHistoryItemProps {
  task: ScheduledTaskItem;
  run: ScheduledTaskRunItem;
  copied_run_id: string | null;
  retrying_run_id: string | null;
  retrying_delivery_run_id: string | null;
  recovering_run_id: string | null;
  can_retry_task: boolean;
  can_retry_delivery: boolean;
  can_recover_task_run: boolean;
  on_copy_diagnostic: (run: ScheduledTaskRunItem) => void | Promise<void>;
  on_retry: (run: ScheduledTaskRunItem) => void | Promise<void>;
  on_retry_delivery: (run: ScheduledTaskRunItem) => void | Promise<void>;
  on_recover: (run: ScheduledTaskRunItem) => void | Promise<void>;
}

function download_run_artifact(agent_id: string, artifact_path: string) {
  void download_workspace_file_api(
    agent_id,
    artifact_path,
    artifact_file_name(artifact_path),
  ).catch((error) => {
    console.error("[ScheduledTaskRunHistoryDialog] 处理任务产物失败:", error);
  });
}

function ScheduledRunArtifactButton({
  agent_id,
  artifact_path,
}: {
  agent_id: string;
  artifact_path: string;
}) {
  const action_copy = get_workspace_file_external_action_copy(artifact_file_name(artifact_path));
  const Icon = action_copy.mode === "reveal" ? FolderOpen : Download;
  const label = action_copy.mode === "reveal" ? "打开产物" : "下载产物";
  return (
    <button
      aria-label={action_copy.aria_label}
      className="mt-2 inline-flex items-center justify-end gap-1.5 text-xs font-semibold text-(--primary) transition duration-(--motion-duration-fast) hover:text-(--primary-hover)"
      onClick={() => download_run_artifact(agent_id, artifact_path)}
      title={action_copy.title}
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
  copied_run_id,
  retrying_run_id,
  retrying_delivery_run_id,
  recovering_run_id,
  can_retry_task,
  can_retry_delivery,
  can_recover_task_run,
  on_copy_diagnostic,
  on_retry,
  on_retry_delivery,
  on_recover,
}: ScheduledTaskRunHistoryItemProps) {
  const status = get_status_meta(run.status);
  const delivery_status = get_delivery_status_meta(run.delivery_status);

  return (
    <article className="py-4 first:pt-0 last:pb-0">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <WorkspaceStatusBadge label={status.label} size="compact" tone={status.tone} />
            {delivery_status ? (
              <WorkspaceStatusBadge label={delivery_status.label} size="compact" tone={delivery_status.tone} />
            ) : null}
            <span className="text-xs font-medium text-(--text-default)">
              Run ID {run.run_id}
            </span>
          </div>
          <div className="mt-3 grid gap-3 text-sm text-(--text-default) md:grid-cols-2">
            <div>
              <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-(--text-muted)">
                调度时间
              </p>
              <p className="mt-1.5 font-medium text-(--text-strong)">
                {format_scheduled_datetime(run.scheduled_for, { include_seconds: true })}
              </p>
            </div>
            <div>
              <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-(--text-muted)">
                执行耗时
              </p>
              <p className="mt-1.5 font-medium text-(--text-strong)">
                {format_duration(run.started_at, run.finished_at)}
              </p>
            </div>
            {run.trigger_kind ? (
              <div>
                <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-(--text-muted)">
                  触发方式
                </p>
                <p className="mt-1.5 font-medium text-(--text-strong)">
                  {run.trigger_kind}
                </p>
              </div>
            ) : null}
            {typeof run.message_count === "number" ? (
              <div>
                <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-(--text-muted)">
                  消息数
                </p>
                <p className="mt-1.5 font-medium text-(--text-strong)">
                  {run.message_count}
                </p>
              </div>
            ) : null}
          </div>
          {(run.session_key || run.round_id || run.session_id || run.delivery_to || run.delivered_at || run.delivery_attempts || run.delivery_next_attempt_at || run.delivery_dead_letter_at) ? (
            <div className="mt-3 space-y-1.5 text-xs text-(--text-default)">
              {run.session_key ? <p className="break-all">Session {run.session_key}</p> : null}
              {run.round_id ? <p className="break-all">Round {run.round_id}</p> : null}
              {run.session_id ? <p className="break-all">Runtime {run.session_id}</p> : null}
              {run.delivery_to ? <p className="break-all">Delivery {run.delivery_to}</p> : null}
              {run.delivered_at ? <p>Delivered {format_scheduled_datetime(run.delivered_at, { include_seconds: true })}</p> : null}
              {run.delivery_attempts ? <p>Delivery attempts {run.delivery_attempts}</p> : null}
              {run.delivery_next_attempt_at ? <p>Next delivery retry {format_scheduled_datetime(run.delivery_next_attempt_at, { include_seconds: true })}</p> : null}
              {run.delivery_dead_letter_at ? <p>Delivery dead letter {format_scheduled_datetime(run.delivery_dead_letter_at, { include_seconds: true })}</p> : null}
            </div>
          ) : null}
        </div>

        <div className="shrink-0 text-right text-sm text-(--text-default)">
          <p>开始 {format_scheduled_datetime(run.started_at, { include_seconds: true })}</p>
          <p className="mt-1">结束 {format_scheduled_datetime(run.finished_at, { include_seconds: true })}</p>
          <p className="mt-1">尝试次数 {run.attempts}</p>
          <div className="mt-2 flex flex-col items-end gap-1.5">
            <button
              className="inline-flex items-center justify-end gap-1.5 text-xs font-semibold text-(--text-default) transition duration-(--motion-duration-fast) hover:text-(--text-strong)"
              onClick={() => void on_copy_diagnostic(run)}
              type="button"
            >
              <Copy className="h-3.5 w-3.5" />
              {copied_run_id === run.run_id ? "已复制" : "复制诊断"}
            </button>
            {is_retryable_status(run.status) && can_retry_task ? (
              <button
                className="inline-flex items-center justify-end gap-1.5 text-xs font-semibold text-(--primary) transition duration-(--motion-duration-fast) hover:text-(--primary-hover) disabled:opacity-60"
                disabled={retrying_run_id === run.run_id || task.running}
                onClick={() => void on_retry(run)}
                title={task.running ? "任务当前正在运行" : "用当前任务配置重新运行一次"}
                type="button"
              >
                <RotateCcw className="h-3.5 w-3.5" />
                {retrying_run_id === run.run_id ? "触发中" : "重新运行"}
              </button>
            ) : null}
            {run.delivery_status === "failed" && can_retry_delivery ? (
              <button
                className="inline-flex items-center justify-end gap-1.5 text-xs font-semibold text-(--primary) transition duration-(--motion-duration-fast) hover:text-(--primary-hover) disabled:opacity-60"
                disabled={retrying_delivery_run_id === run.run_id}
                onClick={() => void on_retry_delivery(run)}
                title="只重试这次运行的结果投递，不重新执行任务"
                type="button"
              >
                <RotateCcw className="h-3.5 w-3.5" />
                {retrying_delivery_run_id === run.run_id ? "投递中" : "重试投递"}
              </button>
            ) : null}
            {run.status === "running" && task.running && can_recover_task_run ? (
              <button
                className="inline-flex items-center justify-end gap-1.5 text-xs font-semibold text-(--destructive) transition duration-(--motion-duration-fast) hover:text-(--destructive) disabled:opacity-60"
                disabled={recovering_run_id === run.run_id}
                onClick={() => void on_recover(run)}
                title="把该运行标记为取消，并释放任务占用"
                type="button"
              >
                <X className="h-3.5 w-3.5" />
                {recovering_run_id === run.run_id ? "释放中" : "释放占用"}
              </button>
            ) : null}
          </div>
          {run.artifact_path ? (
            <ScheduledRunArtifactButton
              agent_id={task.agent_id}
              artifact_path={run.artifact_path}
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
      {should_show_assistant_text(run) ? (
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
