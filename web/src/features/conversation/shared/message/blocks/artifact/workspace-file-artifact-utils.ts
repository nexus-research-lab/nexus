"use client";

import { useMemo } from "react";

import type {
  ContentBlock,
  WorkspaceFileArtifactContent,
} from "@/types/conversation/message";

function collectWorkspaceFileArtifactsFromContentBlocks(
  content: ContentBlock[],
): WorkspaceFileArtifactContent[] {
  return content.filter(
    (block): block is WorkspaceFileArtifactContent =>
      block.type === "workspace_file_artifact" && Boolean(block.path?.trim()),
  );
}

export function useWorkspaceFileArtifactsFromContent(
  content: ContentBlock[],
): WorkspaceFileArtifactContent[] {
  return useMemo(
    () => collectWorkspaceFileArtifactsFromContentBlocks(content),
    [content],
  );
}
