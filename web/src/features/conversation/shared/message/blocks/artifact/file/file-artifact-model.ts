// INPUT: Artifact path, display name, resolved source workspace and open capability.
// OUTPUT: File identity and shared preview/external eligibility; missing scope never uses global selection.
// POS: Pure File Artifact projection; density and visual recipes belong to file-artifact-layout.

import {
  firstNonEmptyArtifactValue,
  getArtifactFileName,
  getArtifactParentPath,
} from "../artifact-path-model";
import {
  buildWorkspaceArtifactExternalAction,
  type WorkspaceArtifactExternalAction,
} from "../workspace-artifact-action-model";

export interface FileArtifactProjection {
  action: WorkspaceArtifactExternalAction | null;
  canOpen: boolean;
  fileName: string;
  openAgentId: string;
  parentPath: string;
  unavailableReason: "path" | "workspace" | null;
}

export function projectFileArtifact({
  defaultFileName,
  displayPath,
  hasOpenHandler,
  path,
  workspaceAgentId,
}: {
  defaultFileName: string;
  displayPath?: string;
  hasOpenHandler: boolean;
  path: string;
  workspaceAgentId?: string | null;
}): FileArtifactProjection {
  const visiblePath = firstNonEmptyArtifactValue(displayPath, path);
  const normalizedPath = path.trim();
  const openAgentId = firstNonEmptyArtifactValue(workspaceAgentId);
  const fileName = getArtifactFileName(visiblePath, defaultFileName);
  const action = buildWorkspaceArtifactExternalAction({
    agentId: openAgentId,
    fileName,
    path: normalizedPath,
  });
  return {
    action,
    canOpen: hasOpenHandler && action !== null,
    fileName,
    openAgentId,
    parentPath: getArtifactParentPath(visiblePath),
    unavailableReason: !normalizedPath ? "path" : !openAgentId ? "workspace" : null,
  };
}
