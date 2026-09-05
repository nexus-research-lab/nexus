// INPUT: Artifact path, display name, explicit/current Agent and open capability.
// OUTPUT: File identity, parent path and exact-scope open/external-action eligibility.
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
}

export function projectFileArtifact({
  currentAgentId,
  displayPath,
  hasOpenHandler,
  path,
  workspaceAgentId,
}: {
  currentAgentId: string | null;
  displayPath?: string;
  hasOpenHandler: boolean;
  path: string;
  workspaceAgentId?: string | null;
}): FileArtifactProjection {
  const visiblePath = firstNonEmptyArtifactValue(displayPath, path);
  const normalizedPath = path.trim();
  const openAgentId = firstNonEmptyArtifactValue(
    workspaceAgentId,
    currentAgentId,
  );
  const fileName = getArtifactFileName(visiblePath);
  return {
    action: buildWorkspaceArtifactExternalAction({
      agentId: openAgentId,
      fileName,
      path: normalizedPath,
    }),
    canOpen: [hasOpenHandler, Boolean(normalizedPath)].every(Boolean),
    fileName,
    openAgentId,
    parentPath: getArtifactParentPath(visiblePath),
  };
}
