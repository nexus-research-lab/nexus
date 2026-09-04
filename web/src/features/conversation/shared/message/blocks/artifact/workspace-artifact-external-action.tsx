/**
 * INPUT: 已验证的 workspace 外部动作、布局 class 与紧凑尺寸。
 * OUTPUT: 使用共享 Button 状态的下载或在文件管理器中显示动作。
 * POS: Workspace Artifact 外部动作适配器，不拥有按钮颜色、圆角或焦点样式。
 */
import { Download, FolderOpen } from "lucide-react";

import { downloadWorkspaceFileApi } from "@/lib/api/agent/agent-api";
import { getWorkspaceFileExternalActionCopy } from "@/lib/workspace-file-action";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
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
  const { t } = useI18n();
  if (!action) {
    return null;
  }
  const copy = getWorkspaceFileExternalActionCopy(t, action.fileName);
  const ActionIcon = ACTION_ICON[copy.mode];
  return (
    <UiButton
      aria-label={copy.ariaLabel}
      className={className}
      onClick={() => runWorkspaceArtifactExternalAction(action)}
      size={size}
      title={copy.title}
      variant="text"
    >
      <ActionIcon className={size === "2xs" ? "h-3 w-3" : "h-3.5 w-3.5"} />
      <span>{copy.label}</span>
    </UiButton>
  );
}

function runWorkspaceArtifactExternalAction(
  action: WorkspaceArtifactExternalAction,
): void {
  void downloadWorkspaceFileApi(
    action.agentId,
    action.path,
    action.fileName,
  ).catch((error) => {
    console.error(
      "[WorkspaceArtifactAction] 处理 workspace 文件失败:",
      error,
    );
  });
}
