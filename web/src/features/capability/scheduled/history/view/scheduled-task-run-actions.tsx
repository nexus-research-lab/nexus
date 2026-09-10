// INPUT: history model 投影的动作、运行产物与用户命令。
// OUTPUT: 随当前语言投影的共享 Button 动作/忙碌状态与绑定历史执行身份的文件动作/反馈。
// POS: Scheduled 历史动作适配；运行资格归模型，文件动作生命周期归公共领域 Hook。

"use client";

import { useId } from "react";
import {
  Download,
  FolderOpen,
  RotateCcw,
  X,
  type LucideIcon,
} from "lucide-react";

import { useWorkspaceFileExternalAction } from "@/hooks/agent/use-workspace-file-external-action";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { FeedbackBannerViewport } from "@/shared/ui/feedback/feedback-banner-viewport";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import type { ScheduledTaskRunItem } from "@/types/capability/scheduled-task/run";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";

import {
  artifactFileName,
  getRunActionPresentations,
  type ScheduledTaskRunActionKind,
} from "../scheduled-task-run-history-model";
import { getRunWorkspaceAgentID } from "../scheduled-task-run-diagnostic-model";

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
  const { t } = useI18n();
  const actions = getRunActionPresentations({
    isRecoveryUnconfirmed,
    isRecovering,
    isRetryDeliveryUnconfirmed,
    isRetryUnconfirmed,
    isRetrying,
    isRetryingDelivery,
    run,
    task,
  }, t);
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
            <UiButton
              aria-busy={action.busy}
              className="justify-end"
              disabled={action.disabled}
              key={action.kind}
              onClick={() => void actionHandlers[action.kind]()}
              size="xs"
              title={action.title}
              tone={action.tone}
              variant="text"
            >
              <Icon className="h-3.5 w-3.5" />
              {action.label}
            </UiButton>
          );
        })}
      </div>
      {run.artifact_path ? (
        <ScheduledRunArtifactButton run={run} />
      ) : null}
    </div>
  );
}

function ScheduledRunArtifactButton({ run }: { run: ScheduledTaskRunItem }) {
  const { t } = useI18n();
  const reasonId = useId();
  const { copy: actionCopy, disabled, failure, onAction } = useWorkspaceFileExternalAction({
    agentId: getRunWorkspaceAgentID(run),
    path: run.artifact_path,
    fileName: artifactFileName(run.artifact_path ?? ""),
    sourceKey: JSON.stringify([run.job_id, run.run_id]),
  });
  const Icon = actionCopy.mode === "reveal" ? FolderOpen : Download;
  return (
    <>
      {disabled ? <span className={getUiTypographyClassName({ role: "supporting", tone: "muted" })} id={reasonId}>
        {t("capability.scheduled_history_artifact_unavailable")}
      </span> : null}
      <UiButton
        aria-describedby={disabled ? reasonId : undefined}
        aria-label={actionCopy.ariaLabel}
        className="justify-end"
        disabled={disabled}
        onClick={onAction}
        size="xs"
        title={actionCopy.title}
        tone="primary"
        variant="text"
      >
        <Icon className="h-3.5 w-3.5" />
        {actionCopy.label}
      </UiButton>
      <FeedbackBannerViewport item={failure} />
    </>
  );
}
