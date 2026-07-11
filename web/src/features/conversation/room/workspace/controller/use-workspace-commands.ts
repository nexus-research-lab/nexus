import { useCallback, useRef, useState } from "react";

import {
  createWorkspaceEntryApi,
  deleteWorkspaceEntryApi,
  downloadWorkspaceFileApi,
  renameWorkspaceEntryApi,
  uploadWorkspaceFileApi,
} from "@/lib/api/agent-manage-api";
import type {
  WorkspaceEntryMutationResponse,
  WorkspaceEntryRenameResponse,
  WorkspaceFileEntry,
} from "@/types/agent/agent";

import { getParentWorkspacePath, joinWorkspacePath } from "./workspace-path-model";

type WorkspaceCommand = "upload" | "create" | "rename" | "delete" | "download";

interface WorkspaceCommandState {
  scopeKey: string;
  activeCommand: WorkspaceCommand | null;
  errorMessage: string | null;
}

interface WorkspaceCommandToken {
  scopeKey: string;
  commandId: number;
}

interface UseWorkspaceCommandsOptions {
  agentId: string;
  refreshFiles: () => Promise<WorkspaceFileEntry[] | null>;
}

const COMMAND_ERROR_MESSAGES: Record<WorkspaceCommand, string> = {
  upload: "上传文件失败",
  create: "创建工作区条目失败",
  rename: "重命名失败",
  delete: "删除失败",
  download: "处理文件失败",
};

const COMMAND_REFRESH_POLICY: Record<WorkspaceCommand, boolean> = {
  upload: true,
  create: true,
  rename: true,
  delete: true,
  download: false,
};

function getCommandErrorMessage(error: unknown, command: WorkspaceCommand): string {
  return error instanceof Error ? error.message : COMMAND_ERROR_MESSAGES[command];
}

export function useWorkspaceCommands({agentId, refreshFiles}: UseWorkspaceCommandsOptions) {
  const scopeRef = useRef(agentId);
  const commandSequenceRef = useRef(0);
  const activeTokenRef = useRef<WorkspaceCommandToken | null>(null);
  const [state, setState] = useState<WorkspaceCommandState>({
    scopeKey: agentId,
    activeCommand: null,
    errorMessage: null,
  });
  scopeRef.current = agentId;

  const isCurrentToken = useCallback((token: WorkspaceCommandToken): boolean => (
    scopeRef.current === token.scopeKey
      && activeTokenRef.current?.scopeKey === token.scopeKey
      && activeTokenRef.current.commandId === token.commandId
  ), []);

  const runCommand = useCallback(async <Result,>(
    command: WorkspaceCommand,
    mutation: (scopeKey: string) => Promise<Result>,
  ): Promise<Result | null> => {
    if (activeTokenRef.current?.scopeKey === agentId) {
      return null;
    }

    const token = {scopeKey: agentId, commandId: ++commandSequenceRef.current};
    activeTokenRef.current = token;
    setState({scopeKey: agentId, activeCommand: command, errorMessage: null});

    try {
      const result = await mutation(token.scopeKey);
      if (COMMAND_REFRESH_POLICY[command]) {
        await refreshFiles();
      }
      return isCurrentToken(token) ? result : null;
    } catch (error) {
      if (isCurrentToken(token)) {
        setState({
          scopeKey: agentId,
          activeCommand: null,
          errorMessage: getCommandErrorMessage(error, command),
        });
      }
      return null;
    } finally {
      if (isCurrentToken(token)) {
        activeTokenRef.current = null;
        setState((current) => ({...current, activeCommand: null}));
      }
    }
  }, [agentId, isCurrentToken, refreshFiles]);

  const uploadFiles = useCallback((
    files: File[],
    targetDirectory: string | null,
  ): Promise<true | null> => runCommand("upload", async (scopeKey) => {
    const targetPath = targetDirectory ? `${targetDirectory}/` : undefined;
    for (const file of files) {
      await uploadWorkspaceFileApi(scopeKey, file, targetPath);
    }
    return true as const;
  }), [runCommand]);

  const createEntry = useCallback((
    entryType: "file" | "directory",
    parentPath: string | null,
    name: string,
  ): Promise<WorkspaceEntryMutationResponse | null> => runCommand(
    "create",
    (scopeKey) => createWorkspaceEntryApi(
      scopeKey,
      joinWorkspacePath(parentPath, name),
      entryType,
    ),
  ), [runCommand]);

  const renameEntry = useCallback((
    entry: WorkspaceFileEntry,
    name: string,
  ): Promise<WorkspaceEntryRenameResponse | null> => runCommand(
    "rename",
    (scopeKey) => renameWorkspaceEntryApi(
      scopeKey,
      entry.path,
      joinWorkspacePath(getParentWorkspacePath(entry.path), name),
    ),
  ), [runCommand]);

  const deleteEntry = useCallback((
    entry: WorkspaceFileEntry,
  ): Promise<WorkspaceEntryMutationResponse | null> => runCommand(
    "delete",
    (scopeKey) => deleteWorkspaceEntryApi(scopeKey, entry.path),
  ), [runCommand]);

  const downloadEntry = useCallback((
    entry: WorkspaceFileEntry,
  ): Promise<true | null> => runCommand("download", async (scopeKey) => {
    await downloadWorkspaceFileApi(scopeKey, entry.path, entry.name);
    return true as const;
  }), [runCommand]);

  const clearError = useCallback(() => {
    setState((current) => (
      current.scopeKey === agentId ? {...current, errorMessage: null} : current
    ));
  }, [agentId]);

  const currentState = state.scopeKey === agentId
    ? state
    : {scopeKey: agentId, activeCommand: null, errorMessage: null};

  return {
    activeCommand: currentState.activeCommand,
    errorMessage: currentState.errorMessage,
    uploadFiles,
    createEntry,
    renameEntry,
    deleteEntry,
    downloadEntry,
    clearError,
  };
}
