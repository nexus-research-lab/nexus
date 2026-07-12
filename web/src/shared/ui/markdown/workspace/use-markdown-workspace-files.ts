"use client";

import { useMemo } from "react";

import { useAgentStore } from "@/store/agent";
import { useWorkspaceFilesStore } from "@/store/workspace-files";

import {
  createWorkspaceFileResolver,
  type ResolveWorkspaceFilePath,
} from "./markdown-workspace-artifact-model";

export function useMarkdownFileResolver(
  workspaceAgentId?: string | null,
): ResolveWorkspaceFilePath {
  const currentAgentId = useAgentStore((state) => state.current_agent_id);
  const filesByAgent = useWorkspaceFilesStore((state) => state.files_by_agent);
  const resolvedAgentId = workspaceAgentId?.trim() || currentAgentId || "";

  return useMemo(
    () => createWorkspaceFileResolver(filesByAgent[resolvedAgentId] ?? []),
    [filesByAgent, resolvedAgentId],
  );
}

export function useMarkdownCurrentAgentID(
  workspaceAgentId?: string | null,
): string | null {
  const currentAgentId = useAgentStore((state) => state.current_agent_id);
  return workspaceAgentId?.trim() || currentAgentId;
}
