"use client";

import { useMemo } from "react";

import type {
  ContentBlock,
  WorkspaceFileArtifactContent,
} from "@/types/conversation/message";

export function collect_workspace_file_artifacts_from_content_blocks(
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
    () => collect_workspace_file_artifacts_from_content_blocks(content),
    [content],
  );
}
