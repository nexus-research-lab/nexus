"use client";

// INPUT: The producing reply's structured blocks and exact Agent identity.
// OUTPUT: Durable deliverables and legacy evidence, without working files or foreign producers.
// POS: Reply-tail selection; no Markdown, directory or ambient Agent inference.

import { useMemo } from "react";

import type {
  ContentBlock,
  WorkspaceFileArtifactContent,
} from "@/types/conversation/message/content";

function collectWorkspaceFileArtifactsFromContentBlocks(
  content: ContentBlock[],
  producerAgentId?: string | null,
): WorkspaceFileArtifactContent[] {
  // The same block can appear in both direct and archived projections. This
  // removes that exact duplicate without guessing identity for legacy files.
  return [...new Set(content)].filter(
    (block): block is WorkspaceFileArtifactContent =>
      block.type === "workspace_file_artifact"
      && block.role !== "working_file"
      && (!block.producer_agent_id || block.producer_agent_id === producerAgentId)
      && Boolean(block.path?.trim()),
  );
}

export function useWorkspaceFileArtifactsFromContent(
  content: ContentBlock[],
  producerAgentId?: string | null,
): WorkspaceFileArtifactContent[] {
  return useMemo(
    () => collectWorkspaceFileArtifactsFromContentBlocks(content, producerAgentId),
    [content, producerAgentId],
  );
}
