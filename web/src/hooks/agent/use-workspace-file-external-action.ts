// INPUT: 显式 Agent、原文件路径/名称、可选来源作用域与当前 owner/语言。
// OUTPUT: 下载/桌面定位命令、原生禁用事实与只属于当前作用域最近操作的公共反馈。
// POS: Workspace 文件按钮共用的领域动作生命周期；不推断 Agent、不取消/重放已发出的操作。

import { useCallback, useEffect, useRef, useSyncExternalStore } from "react";
import { downloadWorkspaceFileApi } from "@/lib/api/agent/agent-api";
import { getWorkspaceFileExternalActionCopy } from "@/lib/workspace-file-action";
import { captureAuthOwnerScopeGeneration, isAuthOwnerScopeGenerationCurrent, subscribeAuthOwnerScopeGeneration } from "@/shared/auth/auth-owner-generation";
import { useI18n } from "@/shared/i18n/i18n-context";
import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import type { FeedbackBannerProps } from "@/shared/ui/feedback/feedback-banner-contract";

export function useWorkspaceFileExternalAction({
  agentId, path, fileName, sourceKey,
}: {
  agentId: string | null | undefined;
  path: string | null | undefined;
  fileName: string;
  sourceKey?: string;
}) {
  const { t } = useI18n();
  const copy = getWorkspaceFileExternalActionCopy(t, fileName);
  const ownerGeneration = useSyncExternalStore(subscribeAuthOwnerScopeGeneration, captureAuthOwnerScopeGeneration, captureAuthOwnerScopeGeneration);
  const scopeKey = JSON.stringify([ownerGeneration, sourceKey, agentId, path, fileName]);
  const scopeRef = useRef(scopeKey);
  scopeRef.current = scopeKey;
  const requestRef = useRef(0);
  const [hasFailure, setHasFailure] = useResettableState(false, scopeKey);
  const disabled = !agentId?.trim() || !path?.trim();
  useEffect(() => () => { requestRef.current += 1; }, [scopeKey]);

  const onAction = useCallback(() => {
    if (!agentId?.trim() || !path?.trim() || !isAuthOwnerScopeGenerationCurrent(ownerGeneration)) return;
    const requestId = ++requestRef.current;
    setHasFailure(false);
    void downloadWorkspaceFileApi(agentId, path, fileName).catch((error) => {
      // 只限制迟到反馈；已发出的文件动作不会因为切页或换 owner 而重放。
      if (scopeRef.current !== scopeKey || requestRef.current !== requestId
        || !isAuthOwnerScopeGenerationCurrent(ownerGeneration)) return;
      console.error("[WorkspaceFileExternalAction] 文件操作失败:", error);
      setHasFailure(true);
    });
  }, [agentId, fileName, ownerGeneration, path, scopeKey, setHasFailure]);
  const failure: FeedbackBannerProps | null = hasFailure ? {
    impact: t("workspace_file.external_action_failed_impact"),
    nextStep: t("workspace_file.external_action_failed_next_step"),
    onDismiss: () => setHasFailure(false),
    title: t("workspace_file.external_action_failed"),
    tone: "error",
    urgency: "polite",
  } : null;
  return { copy, disabled, failure, onAction };
}
