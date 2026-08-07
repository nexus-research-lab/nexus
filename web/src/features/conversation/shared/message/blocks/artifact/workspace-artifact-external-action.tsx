import { Download, FolderOpen } from "lucide-react";

import { downloadWorkspaceFileApi } from "@/lib/api/agent/agent-api";
import { getWorkspaceFileExternalActionCopy } from "@/lib/workspace-file-action";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { WorkspaceArtifactExternalAction } from "./workspace-artifact-action-model";

const ACTION_ICON = {
  download: Download,
  reveal: FolderOpen,
} as const;

export function WorkspaceArtifactExternalActionButton({
  action,
  className,
  iconClassName,
}: {
  action: WorkspaceArtifactExternalAction | null;
  className: string;
  iconClassName: string;
}) {
  const { t } = useI18n();
  if (!action) {
    return null;
  }
  const copy = getWorkspaceFileExternalActionCopy(t, action.fileName);
  const ActionIcon = ACTION_ICON[copy.mode];
  return (
    <button
      aria-label={copy.ariaLabel}
      className={className}
      onClick={() => runWorkspaceArtifactExternalAction(action)}
      title={copy.title}
      type="button"
    >
      <ActionIcon className={iconClassName} />
      <span>{copy.label}</span>
    </button>
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
