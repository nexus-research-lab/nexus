"use client";

import type { ReactNode } from "react";
import {
  Download,
  FolderOpen,
  RotateCcw,
  X,
  type LucideIcon,
} from "lucide-react";

import { downloadWorkspaceFileApi } from "@/lib/api/agent/agent-api";
import { getWorkspaceFileExternalActionCopy } from "@/lib/workspace-file-action";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { ScheduledTaskRunItem } from "@/types/capability/scheduled-task/run";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";

import {
  artifactFileName,
  getRunActionPresentations,
  type ScheduledTaskRunActionKind,
} from "../scheduled-task-run-history-model";

const RUN_ACTION_ICONS: Record<ScheduledTaskRunActionKind, LucideIcon> = {
  recover: X,
  retry: RotateCcw,
  retry_delivery: RotateCcw,
};

const RUN_ACTION_TONE_CLASS_NAMES = {
  danger: "inline-flex items-center justify-end gap-1.5 text-xs font-semibold text-(--destructive) transition duration-(--motion-duration-fast) hover:text-(--destructive) disabled:opacity-60",
  primary: "inline-flex items-center justify-end gap-1.5 text-xs font-semibold text-(--primary) transition duration-(--motion-duration-fast) hover:text-(--primary-hover) disabled:opacity-60",
} as const;

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
  const actions = getRunActionPresentations({
    isRecovering,
    isRetrying,
    isRetryingDelivery,
    run,
    task,
  });
  const actionHandlers: Record<ScheduledTaskRunActionKind, () => void | Promise<void>> = {
    recover: onRecover,
    retry: onRetry,
    retry_delivery: onRetryDelivery,
  };
  if (actions.length === 0 && !run.artifact_path) {
    return null;
  }
  return (
    <div className="mt-3 flex flex-wrap items-center justify-end gap-x-4 gap-y-2 border-t border-(--divider-subtle-color) pt-3 text-sm text-(--text-default)">
      <div className="flex flex-wrap items-center justify-end gap-x-4 gap-y-2">
        {actions.map((action) => {
          const Icon = RUN_ACTION_ICONS[action.kind];
          return (
            <RunActionButton
              disabled={action.disabled}
              icon={<Icon className="h-3.5 w-3.5" />}
              key={action.kind}
              label={action.label}
              onClick={actionHandlers[action.kind]}
              title={action.title}
              tone={action.tone}
            />
          );
        })}
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
  tone,
}: {
  disabled: boolean;
  icon: ReactNode;
  label: string;
  onClick: () => void | Promise<void>;
  title: string;
  tone: "danger" | "primary";
}) {
  return (
    <button
      className={RUN_ACTION_TONE_CLASS_NAMES[tone]}
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
  const { t } = useI18n();
  const actionCopy = getWorkspaceFileExternalActionCopy(
    t,
    artifactFileName(artifactPath),
  );
  const Icon = actionCopy.mode === "reveal" ? FolderOpen : Download;
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
      className="inline-flex items-center justify-end gap-1.5 text-xs font-semibold text-(--primary) transition duration-(--motion-duration-fast) hover:text-(--primary-hover)"
      onClick={downloadArtifact}
      title={actionCopy.title}
      type="button"
    >
      <Icon className="h-3.5 w-3.5" />
      {actionCopy.label}
    </button>
  );
}
