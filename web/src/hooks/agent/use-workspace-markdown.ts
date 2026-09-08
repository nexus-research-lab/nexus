/**
 * INPUT: 消费侧的 exact Agent/显式空 scope、未提供 scope 时的当前 Agent 与 owner-scoped 文件快照。
 * OUTPUT: 同一 Agent 绑定的 Markdown 文件解析器和归属身份。
 * POS: Agent 工作区业务资源适配；共享 Markdown 只接收结果，不订阅 Store。
 */
"use client";

import { useMemo } from "react";

import { getWorkspaceFilePreviewUrl } from "@/lib/api/agent/agent-api";
import { createWorkspaceFileResolver } from "@/shared/ui/markdown/workspace/markdown-workspace-artifact-model";
import { useAgentStore } from "@/store/agent";
import { useWorkspaceFilesStore } from "@/store/workspace-files";

export function useWorkspaceMarkdown(
  workspaceAgentId?: string | null,
  onOpenWorkspaceFile?: (path: string, workspaceAgentId?: string | null) => void,
) {
  const currentAgentId = useAgentStore((state) => (
    workspaceAgentId === undefined ? state.current_agent_id : null
  ));
  const resolvedAgentId = workspaceAgentId === undefined
    ? currentAgentId
    : workspaceAgentId?.trim() || null;
  const files = useWorkspaceFilesStore((state) => (
    resolvedAgentId ? state.files_by_agent[resolvedAgentId] : undefined
  ));
  const resolveFilePath = useMemo(
    () => createWorkspaceFileResolver(files ?? []),
    [files],
  );

  const getFilePreviewUrl = useMemo(
    () => resolvedAgentId
      ? (path: string) => getWorkspaceFilePreviewUrl(resolvedAgentId, path)
      : undefined,
    [resolvedAgentId],
  );
  const openFile = useMemo(
    () => onOpenWorkspaceFile && resolvedAgentId
      ? (path: string) => onOpenWorkspaceFile(path, resolvedAgentId)
      : undefined,
    [onOpenWorkspaceFile, resolvedAgentId],
  );

  return {
    currentAgentId: resolvedAgentId,
    getFilePreviewUrl,
    onOpenWorkspaceFile: openFile,
    resolveFilePath,
  };
}
