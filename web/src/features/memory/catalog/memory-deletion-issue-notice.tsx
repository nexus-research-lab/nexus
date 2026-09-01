/**
 * INPUT: 当前 owner/Agent 下按 path 保留的删除异常，以及显式核对、重试和新意图动作。
 * OUTPUT: 每个未解决删除的持久 Problem / Impact / Recovery 展示。
 * POS: Memory Catalog 删除反馈面；不执行分类，也不把普通刷新当作结果核对。
 */
import { RefreshCw } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiResourceState } from "@/shared/ui/display/resource-state";

import {
  getMemoryDeletionRecoveryPresentation,
  type MemoryDeletionIssue,
  type MemoryDeletionRecoveryAction,
} from "./memory-deletion-recovery";

interface MemoryDeletionIssueNoticesProps {
  action: "delete" | "reconcile" | null;
  commandPath: string;
  issues: MemoryDeletionIssue[];
  onBeginNewIntent: (path: string) => void;
  onReconcile: (path: string) => void;
  onRetry: (path: string) => void;
}

export function MemoryDeletionIssueNotices({
  action,
  commandPath,
  issues,
  onBeginNewIntent,
  onReconcile,
  onRetry,
}: MemoryDeletionIssueNoticesProps) {
  const { t } = useI18n();
  if (issues.length === 0) {
    return null;
  }
  return (
    <div className="mx-3 mt-3 space-y-2">
      {issues.map((issue) => {
        const presentation = getMemoryDeletionRecoveryPresentation(issue);
        const commandBusy = Boolean(commandPath);
        const issueBusy = commandPath === issue.path;
        const issueReconciling = issueBusy && action === "reconcile";
        return (
          <UiResourceState
            className="min-h-0 py-3"
            impact={t(presentation.impactKey, { name: issue.title })}
            key={`${issue.ownerGeneration}:${issue.agentId}:${issue.path}`}
            nextStep={t(presentation.nextStepKey)}
            primaryAction={getRecoveryAction({
              action: presentation.primaryAction,
              busy: issueReconciling,
              disabled: commandBusy && !issueReconciling,
              onBeginNewIntent,
              onReconcile,
              onRetry,
              path: issue.path,
              t,
            })}
            secondaryAction={presentation.secondaryAction
              ? getRecoveryAction({
                  action: presentation.secondaryAction,
                  busy: false,
                  disabled: commandBusy,
                  onBeginNewIntent,
                  onReconcile,
                  onRetry,
                  path: issue.path,
                  t,
                })
              : undefined}
            size="sm"
            state="error"
            title={t(presentation.titleKey, { name: issue.title })}
            urgency="polite"
            variant="card"
          />
        );
      })}
    </div>
  );
}

function getRecoveryAction({
  action,
  busy,
  disabled,
  onBeginNewIntent,
  onReconcile,
  onRetry,
  path,
  t,
}: {
  action: MemoryDeletionRecoveryAction;
  busy: boolean;
  disabled: boolean;
  onBeginNewIntent: (path: string) => void;
  onReconcile: (path: string) => void;
  onRetry: (path: string) => void;
  path: string;
  t: ReturnType<typeof useI18n>["t"];
}) {
  switch (action) {
    case "reconcile":
      return {
        busy,
        busyLabel: t("capability.memory_delete_checking_result"),
        disabled,
        icon: <RefreshCw className="h-3.5 w-3.5" />,
        label: t("capability.memory_delete_check_result"),
        onClick: () => onReconcile(path),
      };
    case "retry":
      return {
        disabled,
        label: t("capability.memory_delete_retry"),
        onClick: () => onRetry(path),
      };
    case "start_new_intent":
      return {
        disabled,
        label: t("capability.memory_delete_start_new_intent"),
        onClick: () => onBeginNewIntent(path),
        tone: "danger" as const,
      };
  }
}
