import { useCallback, useEffect, useRef, useState } from "react";

import { useWorkspaceFilesStore } from "@/store/workspace-files";
import type { WorkspaceFileEntry } from "@/types/agent/agent";

const EMPTY_FILES: WorkspaceFileEntry[] = [];

interface WorkspaceFilesResourceState {
  scopeKey: string;
  errorMessage: string | null;
  isLoading: boolean;
}

function getErrorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "加载文件列表失败";
}

export function useWorkspaceFilesResource(agentId: string) {
  const files = useWorkspaceFilesStore(
    (state) => state.files_by_agent[agentId] ?? EMPTY_FILES,
  );
  const refreshFiles = useWorkspaceFilesStore((state) => state.refresh_files);
  const requestSequenceRef = useRef(0);
  const scopeRef = useRef(agentId);
  const [state, setState] = useState<WorkspaceFilesResourceState>({
    scopeKey: agentId,
    errorMessage: null,
    isLoading: true,
  });
  scopeRef.current = agentId;

  const reload = useCallback(async (): Promise<WorkspaceFileEntry[] | null> => {
    const token = {scopeKey: agentId, requestId: ++requestSequenceRef.current};
    setState({scopeKey: agentId, errorMessage: null, isLoading: true});
    try {
      const nextFiles = await refreshFiles(agentId);
      if (scopeRef.current !== token.scopeKey || requestSequenceRef.current !== token.requestId) {
        return null;
      }
      setState({scopeKey: agentId, errorMessage: null, isLoading: false});
      return nextFiles;
    } catch (error) {
      if (scopeRef.current !== token.scopeKey || requestSequenceRef.current !== token.requestId) {
        return null;
      }
      setState({scopeKey: agentId, errorMessage: getErrorMessage(error), isLoading: false});
      return null;
    }
  }, [agentId, refreshFiles]);

  useEffect(() => {
    void reload();
  }, [reload]);

  const clearError = useCallback(() => {
    setState((current) => (
      current.scopeKey === agentId ? {...current, errorMessage: null} : current
    ));
  }, [agentId]);

  const currentState = state.scopeKey === agentId
    ? state
    : {scopeKey: agentId, errorMessage: null, isLoading: true};

  return {
    files,
    errorMessage: currentState.errorMessage,
    isLoading: currentState.isLoading,
    reload,
    clearError,
  };
}
