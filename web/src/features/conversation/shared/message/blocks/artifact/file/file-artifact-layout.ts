// INPUT: Compact or regular inline file-artifact presentation.
// OUTPUT: File-card geometry and content typography, independent of path and Agent identity.
// POS: File Artifact reading layout; the common content recipe owns its surface and shared Button owns the external action.

interface FileArtifactLayout {
  card: string;
  fileIcon: string;
  fileName: string;
  iconFrame: string;
  label: string;
  openBadge: string;
  wrapper: string;
}

const DENSITY_STYLE: Record<"compact" | "regular", FileArtifactLayout> = {
  compact: {
    card: "max-w-[28rem] gap-1.5 px-2.5 py-2",
    fileIcon: "h-3.5 w-3.5",
    fileName: "text-sm leading-5",
    iconFrame: "h-8 w-8",
    label: "text-compact leading-5",
    openBadge: "px-1.5 py-0.5 text-2xs",
    wrapper: "my-0",
  },
  regular: {
    card: "max-w-[32rem] gap-2 px-3 py-2.5",
    fileIcon: "h-4 w-4",
    fileName: "text-base leading-5",
    iconFrame: "h-9 w-9",
    label: "text-base leading-6",
    openBadge: "px-2 py-1 text-xs",
    wrapper: "my-2",
  },
};

export function resolveFileArtifactLayout(compact: boolean): FileArtifactLayout {
  return DENSITY_STYLE[compact ? "compact" : "regular"];
}
