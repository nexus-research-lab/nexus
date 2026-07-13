import { Download, FolderOpen } from "lucide-react";

import { downloadWorkspaceFileApi } from "@/lib/api/agent/agent-api";
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
  if (!action) {
    return null;
  }
  const ActionIcon = ACTION_ICON[action.copy.mode];
  return (
    <button
      aria-label={action.copy.ariaLabel}
      className={className}
      onClick={() => runWorkspaceArtifactExternalAction(action)}
      title={action.copy.title}
      type="button"
    >
      <ActionIcon className={iconClassName} />
      <span>{action.copy.label}</span>
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
      `[WorkspaceArtifactAction] ${action.copy.label} workspace 文件失败:`,
      error,
    );
  });
}
