import {
  firstNonEmptyArtifactValue,
  getArtifactFileName,
  getArtifactParentPath,
} from "../artifact-path-model";
import {
  buildWorkspaceArtifactExternalAction,
  type WorkspaceArtifactExternalAction,
} from "../workspace-artifact-action-model";

interface FileArtifactDensityStyle {
  card: string;
  externalAction: string;
  externalIcon: string;
  fileIcon: string;
  fileName: string;
  iconFrame: string;
  label: string;
  openBadge: string;
  wrapper: string;
}

export interface FileArtifactProjection {
  action: WorkspaceArtifactExternalAction | null;
  canOpen: boolean;
  fileName: string;
  openAgentId: string;
  parentPath: string;
  style: FileArtifactDensityStyle;
}

const DENSITY_STYLE: Record<"compact" | "regular", FileArtifactDensityStyle> = {
  compact: {
    card: "max-w-[28rem] gap-1.5 px-2.5 py-2",
    externalAction: "px-1.5 py-1 text-2xs",
    externalIcon: "h-3 w-3",
    fileIcon: "h-3.5 w-3.5",
    fileName: "text-sm leading-5",
    iconFrame: "h-8 w-8",
    label: "text-compact leading-5",
    openBadge: "px-1.5 py-0.5 text-2xs",
    wrapper: "my-0",
  },
  regular: {
    card: "max-w-[32rem] gap-2 px-3 py-2.5",
    externalAction: "px-2 py-1 text-xs",
    externalIcon: "h-3.5 w-3.5",
    fileIcon: "h-4 w-4",
    fileName: "text-base leading-5",
    iconFrame: "h-9 w-9",
    label: "text-base leading-6",
    openBadge: "px-2 py-1 text-xs",
    wrapper: "my-2",
  },
};

export function projectFileArtifact({
  compact,
  currentAgentId,
  displayPath,
  hasOpenHandler,
  path,
  workspaceAgentId,
}: {
  compact: boolean;
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
    style: DENSITY_STYLE[compact ? "compact" : "regular"],
  };
}
