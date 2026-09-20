// INPUT: Compact or regular inline file-artifact presentation.
// OUTPUT: File-card geometry and content typography, independent of path and Agent identity.
// POS: File Artifact reading layout; the common content recipe owns its surface and shared Button owns the external action.

import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

interface FileArtifactLayout {
  card: string;
  fileIcon: string;
  fileName: string;
  iconFrame: string;
  label: string;
  wrapper: string;
}

const DENSITY_STYLE: Record<"compact" | "regular", FileArtifactLayout> = {
  compact: {
    card: "gap-1.5 px-2.5 py-2",
    fileIcon: "h-3.5 w-3.5",
    fileName: getUiTypographyClassName({ role: "supporting", tone: "strong", weight: "medium" }),
    iconFrame: "h-8 w-8",
    label: getUiTypographyClassName({ role: "metadata" }),
    wrapper: "my-0",
  },
  regular: {
    card: "gap-2 px-3 py-2.5",
    fileIcon: "h-4 w-4",
    fileName: getUiTypographyClassName({ role: "body", tone: "strong", weight: "medium" }),
    iconFrame: "h-9 w-9",
    label: getUiTypographyClassName({ role: "body" }),
    wrapper: "my-2",
  },
};

export function resolveFileArtifactLayout(compact: boolean): FileArtifactLayout {
  return DENSITY_STYLE[compact ? "compact" : "regular"];
}
