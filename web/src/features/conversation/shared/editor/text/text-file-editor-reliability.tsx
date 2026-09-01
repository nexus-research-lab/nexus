// INPUT: 文本文件读取失败、保存结果事实与只读/显式恢复动作。
// OUTPUT: 窄屏可换行的 Problem / Impact / Recovery 状态面。
// POS: 通用文本编辑器异常视图；不推断结果、不触发自动写入。
"use client";

import { RefreshCw, Save } from "lucide-react";

import type { ResourceFailure } from "@/lib/error-message";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiResourceState } from "@/shared/ui/display/resource-state";

import type { TextFileSaveIssue } from "./text-file-editor-recovery";

interface TextFileEditorReliabilityProps {
  hasLoadedContent: boolean;
  isLoading: boolean;
  isReconciling: boolean;
  isSaving: boolean;
  onAdoptLatest: () => void;
  onLoadLatest: () => void;
  onOverwrite: () => void;
  onReconcile: () => void;
  onRetrySave: () => void;
  resourceFailure: ResourceFailure | null;
  revisionReady: boolean;
  saveIssue: TextFileSaveIssue | null;
}

export function TextFileEditorReliability({
  hasLoadedContent,
  isLoading,
  isReconciling,
  isSaving,
  onAdoptLatest,
  onLoadLatest,
  onOverwrite,
  onReconcile,
  onRetrySave,
  resourceFailure,
  revisionReady,
  saveIssue,
}: TextFileEditorReliabilityProps) {
  if (!resourceFailure && !saveIssue) {
    return null;
  }
  return (
    <div className="min-w-0 shrink-0 px-3 py-2">
      {resourceFailure ? (
        <TextFileResourceFailure
          failure={resourceFailure}
          hasLoadedContent={hasLoadedContent}
          isLoading={isLoading}
          onReload={onLoadLatest}
        />
      ) : saveIssue ? (
        <TextFileSaveIssueNotice
          isLoading={isLoading}
          isReconciling={isReconciling}
          isSaving={isSaving}
          issue={saveIssue}
          onAdoptLatest={onAdoptLatest}
          onLoadLatest={onLoadLatest}
          onOverwrite={onOverwrite}
          onReconcile={onReconcile}
          onRetrySave={onRetrySave}
          revisionReady={revisionReady}
        />
      ) : null}
    </div>
  );
}

function TextFileResourceFailure({
  failure,
  hasLoadedContent,
  isLoading,
  onReload,
}: {
  failure: ResourceFailure;
  hasLoadedContent: boolean;
  isLoading: boolean;
  onReload: () => void;
}) {
  const { t } = useI18n();
  const accessBlocked = Boolean(failure.access);
  return (
    <UiResourceState
      className="min-h-0 w-full px-3 py-3 [overflow-wrap:anywhere]"
      impact={t(accessBlocked
        ? "workspace_file.access_failure_impact"
        : hasLoadedContent
          ? "workspace_file.load_failed_stale_impact"
          : "workspace_file.load_failed_empty_impact")}
      primaryAction={{
        busy: isLoading,
        busyLabel: t("workspace_file.loading_latest"),
        icon: <RefreshCw className="h-3.5 w-3.5" />,
        label: t("state.retry"),
        onClick: onReload,
      }}
      size="sm"
      state="error"
      title={t(accessBlocked
        ? "state.permission_title"
        : "workspace_file.load_failed_title")}
      variant="card"
    />
  );
}

function TextFileSaveIssueNotice({
  isLoading,
  isReconciling,
  isSaving,
  issue,
  onAdoptLatest,
  onLoadLatest,
  onOverwrite,
  onReconcile,
  onRetrySave,
  revisionReady,
}: {
  isLoading: boolean;
  isReconciling: boolean;
  isSaving: boolean;
  issue: TextFileSaveIssue;
  onAdoptLatest: () => void;
  onLoadLatest: () => void;
  onOverwrite: () => void;
  onReconcile: () => void;
  onRetrySave: () => void;
  revisionReady: boolean;
}) {
  const { t } = useI18n();
  if (issue.kind === "conflict") {
    const reviewing = issue.phase === "review";
    return (
      <UiResourceState
        className="min-h-0 w-full px-3 py-3 [overflow-wrap:anywhere]"
        impact={t(reviewing
          ? "workspace_file.conflict_review_impact"
          : "workspace_file.conflict_impact")}
        primaryAction={reviewing
          ? {
              label: t("workspace_file.adopt_latest"),
              onClick: onAdoptLatest,
            }
          : {
              busy: isLoading,
              busyLabel: t("workspace_file.loading_latest"),
              icon: <RefreshCw className="h-3.5 w-3.5" />,
              label: t("workspace_file.load_latest"),
              onClick: onLoadLatest,
            }}
        secondaryAction={reviewing
          ? {
              busy: isSaving,
              busyLabel: t("workspace_file.overwriting"),
              disabled: !revisionReady,
              label: t("workspace_file.overwrite_latest"),
              onClick: onOverwrite,
              tone: "danger",
            }
          : undefined}
        size="sm"
        state="error"
        title={t(reviewing
          ? "workspace_file.conflict_review_title"
          : "workspace_file.conflict_title")}
        tone="warning"
        variant="card"
      />
    );
  }
  if (issue.kind === "outcome_unknown") {
    return (
      <UiResourceState
        className="min-h-0 w-full px-3 py-3 [overflow-wrap:anywhere]"
        impact={t("workspace_file.save_unknown_impact")}
        primaryAction={{
          busy: isReconciling,
          busyLabel: t("workspace_file.checking_save_result"),
          icon: <RefreshCw className="h-3.5 w-3.5" />,
          label: t("workspace_file.check_save_result"),
          onClick: onReconcile,
        }}
        size="sm"
        state="error"
        title={t(issue.reconciliationFailed
          ? "workspace_file.save_check_failed_title"
          : "workspace_file.save_unknown_title")}
        tone="warning"
        variant="card"
      />
    );
  }
  if (issue.kind === "retry_ready") {
    return (
      <UiResourceState
        className="min-h-0 w-full px-3 py-3 [overflow-wrap:anywhere]"
        impact={t("workspace_file.save_retry_ready_impact")}
        primaryAction={{
          busy: isSaving,
          busyLabel: t("common.saving"),
          icon: <Save className="h-3.5 w-3.5" />,
          label: t("workspace_file.retry_save"),
          onClick: onRetrySave,
        }}
        size="sm"
        state="error"
        title={t("workspace_file.save_retry_ready_title")}
        tone="warning"
        variant="card"
      />
    );
  }
  return (
    <UiResourceState
      className="min-h-0 w-full px-3 py-3 [overflow-wrap:anywhere]"
      impact={t("workspace_file.save_not_applied_impact")}
      primaryAction={{
        busy: isSaving,
        busyLabel: t("common.saving"),
        icon: <Save className="h-3.5 w-3.5" />,
        label: t("workspace_file.retry_save"),
        onClick: onRetrySave,
      }}
      size="sm"
      state="error"
      title={t("workspace_file.save_not_applied_title")}
      variant="card"
    />
  );
}
