// INPUT: history model 投影的动作、运行产物与用户命令。
// OUTPUT: 共享 Button 呈现的重跑、投递恢复、释放占用与产物动作。
// POS: Scheduled 历史动作纯视图；不推断动作资格或命令结果。

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
import { UiButton } from "@/shared/ui/button/button";
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

interface ScheduledTaskRunActionsProps {
  isRecoveryUnconfirmed: boolean;
  isRecovering: boolean;
  isRetryDeliveryUnconfirmed: boolean;
  isRetryUnconfirmed: boolean;
  isRetrying: boolean;
  isRetryingDelivery: boolean;
  onRecover: () => void | Promise<void>;
  onRetry: () => void | Promise<void>;
  onRetryDelivery: () => void | Promise<void>;
  run: ScheduledTaskRunItem;
  task: ScheduledTaskItem;
}

export function ScheduledTaskRunActions({
  isRecoveryUnconfirmed,
  isRecovering,
  isRetryDeliveryUnconfirmed,
  isRetryUnconfirmed,
  isRetrying,
  isRetryingDelivery,
  onRecover,
  onRetry,
  onRetryDelivery,
  run,
  task,
}: ScheduledTaskRunActionsProps) {
  const actions = getRunActionPresentations({
    isRecoveryUnconfirmed,
    isRecovering,
    isRetryDeliveryUnconfirmed,
    isRetryUnconfirmed,
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
    <div className="mt-3 flex flex-wrap items-center justify-end gap-x-4 gap-y-2 border-t border-(--divider-subtle-color) pt-3">
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
    <UiButton
      className="justify-end"
      disabled={disabled}
      onClick={() => void onClick()}
      size="xs"
      title={title}
      tone={tone}
      variant="text"
    >
      {icon}
      {label}
    </UiButton>
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
    <UiButton
      aria-label={actionCopy.ariaLabel}
      className="justify-end"
      onClick={downloadArtifact}
      size="xs"
      title={actionCopy.title}
      tone="primary"
      variant="text"
    >
      <Icon className="h-3.5 w-3.5" />
      {actionCopy.label}
    </UiButton>
  );
}
