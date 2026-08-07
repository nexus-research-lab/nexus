export interface WorkspaceArtifactExternalAction {
  agentId: string;
  fileName: string;
  path: string;
}

export function buildWorkspaceArtifactExternalAction({
  agentId,
  fileName,
  path,
}: {
  agentId: string | null | undefined;
  fileName: string;
  path: string | null | undefined;
}): WorkspaceArtifactExternalAction | null {
  const normalizedAgentId = agentId?.trim() ?? "";
  const normalizedPath = path?.trim() ?? "";
  if (!normalizedAgentId || !normalizedPath) {
    return null;
  }
  return {
    agentId: normalizedAgentId,
    fileName,
    path: normalizedPath,
  };
}
