// INPUT: 当前 Agent 与共享文件目录缓存/刷新入口。
// OUTPUT: 精确作用域的读取状态、缓存和独立可关闭的失败反馈。
// POS: 目录读取 owner；关闭反馈不把读取失败改写为已确认空目录。

import { useCallback, useEffect, useRef, useState } from "react";

import { useWorkspaceFilesStore } from "@/store/workspace-files";
import type { WorkspaceFileEntry } from "@/types/agent/agent";

const EMPTY_FILES: WorkspaceFileEntry[] = [];

interface WorkspaceFilesResourceState {
  scopeKey: string;
  hasError: boolean;
  errorDismissed: boolean;
  isLoading: boolean;
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
    hasError: false,
    errorDismissed: false,
    isLoading: true,
  });
  scopeRef.current = agentId;

  const reload = useCallback(async (): Promise<WorkspaceFileEntry[] | null> => {
    const token = {scopeKey: agentId, requestId: ++requestSequenceRef.current};
    setState({scopeKey: agentId, hasError: false, errorDismissed: false, isLoading: true});
    try {
      const nextFiles = await refreshFiles(agentId);
      if (scopeRef.current !== token.scopeKey || requestSequenceRef.current !== token.requestId) {
        return null;
      }
      setState({scopeKey: agentId, hasError: false, errorDismissed: false, isLoading: false});
      return nextFiles;
    } catch {
      if (scopeRef.current !== token.scopeKey || requestSequenceRef.current !== token.requestId) {
        return null;
      }
      setState({
        scopeKey: agentId,
        hasError: true,
        errorDismissed: false,
        isLoading: false,
      });
      return null;
    }
  }, [agentId, refreshFiles]);

  useEffect(() => {
    void reload();
  }, [reload]);

  const dismissError = useCallback(() => {
    setState((current) => (
      current.scopeKey === agentId ? {...current, errorDismissed: true} : current
    ));
  }, [agentId]);

  const currentState = state.scopeKey === agentId
    ? state
    : {scopeKey: agentId, hasError: false, errorDismissed: false, isLoading: true};

  return {
    files,
    hasError: currentState.hasError,
    errorFeedbackVisible: currentState.hasError && !currentState.errorDismissed,
    isLoading: currentState.isLoading,
    reload,
    dismissError,
  };
}
