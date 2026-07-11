"use client";

import type { ReactNode } from "react";
import { Download, FolderOpen, RotateCcw, X } from "lucide-react";

import { downloadWorkspaceFileApi } from "@/lib/api/agent/agent-api";
import { getWorkspaceFileExternalActionCopy } from "@/lib/workspace-file-action";
import type { ScheduledTaskRunItem } from "@/types/capability/scheduled-task/run";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";

import {
  artifactFileName,
  isRetryableStatus,
} from "../scheduled-task-run-history-model";

interface ScheduledTaskRunActionsProps {
  isRecovering: boolean;
  isRetrying: boolean;
  isRetryingDelivery: boolean;
  onRecover: () => void | Promise<void>;
  onRetry: () => void | Promise<void>;
  onRetryDelivery: () => void | Promise<void>;
  run: ScheduledTaskRunItem;
  task: ScheduledTaskItem;
}

export function ScheduledTaskRunActions({
  isRecovering,
  isRetrying,
  isRetryingDelivery,
  onRecover,
  onRetry,
  onRetryDelivery,
  run,
  task,
}: ScheduledTaskRunActionsProps) {
  return (
    <div className="shrink-0 text-right text-sm text-(--text-default)">
      <div className="flex flex-col items-end gap-1.5">
        {isRetryableStatus(run.status) ? (
          <RunActionButton
            disabled={isRetrying || task.running}
            icon={<RotateCcw className="h-3.5 w-3.5" />}
            label={isRetrying ? "触发中" : "重新运行"}
            onClick={onRetry}
            title={task.running ? "任务当前正在运行" : "用当前任务配置重新运行一次"}
          />
        ) : null}
        {run.delivery_status === "failed" ? (
          <RunActionButton
            disabled={isRetryingDelivery}
            icon={<RotateCcw className="h-3.5 w-3.5" />}
            label={isRetryingDelivery ? "投递中" : "重试投递"}
            onClick={onRetryDelivery}
            title="只重试这次运行的结果投递，不重新执行任务"
          />
        ) : null}
        {run.status === "running" && task.running ? (
          <RunActionButton
            disabled={isRecovering}
            icon={<X className="h-3.5 w-3.5" />}
            label={isRecovering ? "释放中" : "释放占用"}
            onClick={onRecover}
            title="把该运行标记为取消，并释放任务占用"
            tone="danger"
          />
        ) : null}
      </div>
      {run.artifact_path ? (
        <ScheduledRunArtifactButton
          agentId={task.agent_id}
          artifactPath={run.artifact_path}
        />
      ) : null}
    </div>
  );
}

function RunActionButton({
  disabled,
  icon,
  label,
  onClick,
  title,
  tone = "primary",
}: {
  disabled: boolean;
  icon: ReactNode;
  label: string;
  onClick: () => void | Promise<void>;
  title: string;
  tone?: "danger" | "primary";
}) {
  return (
    <button
      className={tone === "danger"
        ? "inline-flex items-center justify-end gap-1.5 text-xs font-semibold text-(--destructive) transition duration-(--motion-duration-fast) hover:text-(--destructive) disabled:opacity-60"
        : "inline-flex items-center justify-end gap-1.5 text-xs font-semibold text-(--primary) transition duration-(--motion-duration-fast) hover:text-(--primary-hover) disabled:opacity-60"}
      disabled={disabled}
      onClick={() => void onClick()}
      title={title}
      type="button"
    >
      {icon}
      {label}
    </button>
  );
}

function ScheduledRunArtifactButton({
  agentId,
  artifactPath,
}: {
  agentId: string;
  artifactPath: string;
}) {
  const actionCopy = getWorkspaceFileExternalActionCopy(artifactFileName(artifactPath));
  const Icon = actionCopy.mode === "reveal" ? FolderOpen : Download;
  const label = actionCopy.mode === "reveal" ? "打开产物" : "下载产物";
  const downloadArtifact = () => {
    void downloadWorkspaceFileApi(
      agentId,
      artifactPath,
      artifactFileName(artifactPath),
    ).catch((error) => {
      console.error("[scheduled-task-history] 处理任务产物失败:", error);
    });
  };
  return (
    <button
      aria-label={actionCopy.ariaLabel}
      className="mt-2 inline-flex items-center justify-end gap-1.5 text-xs font-semibold text-(--primary) transition duration-(--motion-duration-fast) hover:text-(--primary-hover)"
      onClick={downloadArtifact}
      title={actionCopy.title}
      type="button"
    >
      <Icon className="h-3.5 w-3.5" />
      {label}
    </button>
  );
}
