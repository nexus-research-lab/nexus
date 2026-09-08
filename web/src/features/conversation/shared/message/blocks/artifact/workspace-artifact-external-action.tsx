/**
 * INPUT: 已验证的 workspace 外部动作、布局 class 与紧凑尺寸。
 * OUTPUT: 使用共享 Button 和公共反馈的下载或文件管理器定位动作。
 * POS: Workspace Artifact 内容适配器；文件动作生命周期归公共领域 Hook。
 */
import { Download, FolderOpen } from "lucide-react";

import { useWorkspaceFileExternalAction } from "@/hooks/agent/use-workspace-file-external-action";
import { UiButton } from "@/shared/ui/button/button";
import { FeedbackBannerViewport } from "@/shared/ui/feedback/feedback-banner-viewport";
import type { WorkspaceArtifactExternalAction } from "./workspace-artifact-action-model";

const ACTION_ICON = {
  download: Download,
  reveal: FolderOpen,
} as const;

export function WorkspaceArtifactExternalActionButton({
  action,
  className,
  size = "xs",
}: {
  action: WorkspaceArtifactExternalAction | null;
  className?: string;
  size?: "2xs" | "xs";
}) {
  const { copy, disabled, failure, onAction } = useWorkspaceFileExternalAction({
    agentId: action?.agentId, path: action?.path, fileName: action?.fileName ?? "",
  });
  if (!action) {
    return null;
  }
  const ActionIcon = ACTION_ICON[copy.mode];
  return (
    <>
      <UiButton
        aria-label={copy.ariaLabel}
        className={className}
        disabled={disabled}
        onClick={onAction}
        size={size}
        title={copy.title}
        variant="text"
      >
        <ActionIcon className={size === "2xs" ? "h-3 w-3" : "h-3.5 w-3.5"} />
        <span>{copy.label}</span>
      </UiButton>
      <FeedbackBannerViewport item={failure} />
    </>
  );
}
