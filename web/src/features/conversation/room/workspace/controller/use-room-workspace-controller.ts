import type { ChangeEvent, MouseEvent, RefObject } from "react";
import { useCallback, useEffect } from "react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import type { WorkspaceFileEntry } from "@/types/agent/agent";

import {
  createWorkspacePrompt,
  resolveWorkspaceMenuPosition,
  type WorkspaceContextMenuState,
  type WorkspacePromptState,
} from "./workspace-interaction-model";
import {
  getParentWorkspacePath,
  getWorkspaceFocusPath,
  isWorkspacePathWithin,
  replaceWorkspacePathPrefix,
} from "./workspace-path-model";
import { useWorkspaceAgentScope } from "./use-workspace-agent-scope";
import { useWorkspaceCommands } from "./use-workspace-commands";
import { useWorkspaceFilesResource } from "./use-workspace-files-resource";

interface UseRoomWorkspaceControllerOptions {
  activeWorkspacePath: string | null;
  agentId: string;
  isDm: boolean;
  onOpenWorkspaceFile: (path: string | null) => void;
  fileInputRef: RefObject<HTMLInputElement | null>;
}

function getMenuPosition(event: MouseEvent, entry: WorkspaceFileEntry | null) {
  return resolveWorkspaceMenuPosition(
    {x: event.clientX, y: event.clientY},
    {width: window.innerWidth, height: window.innerHeight},
    entry,
  );
}

export function useRoomWorkspaceController({
  activeWorkspacePath,
  agentId,
  isDm,
  onOpenWorkspaceFile,
  fileInputRef,
}: UseRoomWorkspaceControllerOptions) {
  const agentScope = useWorkspaceAgentScope({agentId, isDm, onOpenWorkspaceFile});
  const resource = useWorkspaceFilesResource(agentScope.viewAgentId);
  const commands = useWorkspaceCommands({
    agentId: agentScope.viewAgentId,
    refreshFiles: resource.reload,
  });
  const [focusedDirectoryPath, setFocusedDirectoryPath] = useResettableState<string | null>(
    null,
    agentScope.viewAgentId,
  );
  const [contextMenu, setContextMenu] = useResettableState<WorkspaceContextMenuState>(
    {position: null, entry: null},
    agentScope.viewAgentId,
  );
  const [promptState, setPromptState] = useResettableState<WorkspacePromptState>(
    null,
    agentScope.viewAgentId,
  );
  const [deleteTarget, setDeleteTarget] = useResettableState<WorkspaceFileEntry | null>(
    null,
    agentScope.viewAgentId,
  );
  const [uploadTargetDirectory, setUploadTargetDirectory] = useResettableState<string | null>(
    null,
    agentScope.viewAgentId,
  );

  useEffect(() => {
    setFocusedDirectoryPath(getWorkspaceFocusPath(activeWorkspacePath));
  }, [activeWorkspacePath, setFocusedDirectoryPath]);

  const handleClickFile = useCallback((path: string) => {
    setFocusedDirectoryPath(getParentWorkspacePath(path));
    onOpenWorkspaceFile(path);
  }, [onOpenWorkspaceFile, setFocusedDirectoryPath]);

  const handleUploadClick = useCallback((directoryPath?: string | null) => {
    setUploadTargetDirectory(directoryPath ?? focusedDirectoryPath);
    fileInputRef.current?.click();
  }, [fileInputRef, focusedDirectoryPath, setUploadTargetDirectory]);

  const handleFileSelect = useCallback(async (event: ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(event.currentTarget.files ?? []);
    event.currentTarget.value = "";
    const targetDirectory = uploadTargetDirectory;
    setUploadTargetDirectory(null);
    if (files.length > 0) {
      await commands.uploadFiles(files, targetDirectory);
    }
  }, [commands, setUploadTargetDirectory, uploadTargetDirectory]);

  const openCreatePrompt = useCallback((
    entryType: "file" | "directory",
    parentPath?: string | null,
  ) => {
    setPromptState(createWorkspacePrompt(
      entryType,
      parentPath ?? focusedDirectoryPath,
    ));
  }, [focusedDirectoryPath, setPromptState]);

  const openRenamePrompt = useCallback((entry: WorkspaceFileEntry) => {
    setPromptState({mode: "rename", entry, defaultValue: entry.name});
  }, [setPromptState]);

  const applyRenameResult = useCallback((
    previousPath: string,
    nextPath: string,
  ) => {
    const nextActivePath = replaceWorkspacePathPrefix(
      activeWorkspacePath,
      previousPath,
      nextPath,
    );
    if (nextActivePath) {
      onOpenWorkspaceFile(nextActivePath);
    }

    const nextFocusedPath = replaceWorkspacePathPrefix(
      focusedDirectoryPath,
      previousPath,
      nextPath,
    );
    if (nextFocusedPath) {
      setFocusedDirectoryPath(nextFocusedPath);
    }
  }, [activeWorkspacePath, focusedDirectoryPath, onOpenWorkspaceFile, setFocusedDirectoryPath]);

  const handlePromptConfirm = useCallback(async (value: string) => {
    const name = value.trim();
    if (!promptState || !name) {
      return;
    }

    if (promptState.mode === "rename") {
      if (name === promptState.entry.name) {
        setPromptState(null);
        return;
      }
      const result = await commands.renameEntry(promptState.entry, name);
      if (!result) {
        return;
      }
      applyRenameResult(promptState.entry.path, result.new_path);
      setPromptState(null);
      return;
    }

    const entryType = promptState.mode === "create-file" ? "file" : "directory";
    const result = await commands.createEntry(entryType, promptState.parentPath, name);
    if (!result) {
      return;
    }
    if (entryType === "file") {
      onOpenWorkspaceFile(result.path);
      setFocusedDirectoryPath(getParentWorkspacePath(result.path));
    } else {
      setFocusedDirectoryPath(result.path);
    }
    setPromptState(null);
  }, [
    applyRenameResult,
    commands,
    onOpenWorkspaceFile,
    promptState,
    setFocusedDirectoryPath,
    setPromptState,
  ]);

  const handleConfirmDelete = useCallback(async () => {
    if (!deleteTarget) {
      return;
    }
    const result = await commands.deleteEntry(deleteTarget);
    if (!result) {
      return;
    }
    if (isWorkspacePathWithin(activeWorkspacePath, deleteTarget.path)) {
      onOpenWorkspaceFile(null);
    }
    if (isWorkspacePathWithin(focusedDirectoryPath, deleteTarget.path)) {
      setFocusedDirectoryPath(getParentWorkspacePath(deleteTarget.path));
    }
    setDeleteTarget(null);
  }, [
    activeWorkspacePath,
    commands,
    deleteTarget,
    focusedDirectoryPath,
    onOpenWorkspaceFile,
    setDeleteTarget,
    setFocusedDirectoryPath,
  ]);

  const handleDownloadContextEntry = useCallback(async () => {
    const entry = contextMenu.entry;
    if (entry && !entry.is_dir) {
      await commands.downloadEntry(entry);
    }
  }, [commands, contextMenu.entry]);

  const handleContextMenu = useCallback((event: MouseEvent, entry: WorkspaceFileEntry) => {
    setContextMenu({position: getMenuPosition(event, entry), entry});
  }, [setContextMenu]);

  const handleRootContextMenu = useCallback((event: MouseEvent) => {
    event.preventDefault();
    setContextMenu({position: getMenuPosition(event, null), entry: null});
  }, [setContextMenu]);

  const clearErrorMessage = useCallback(() => {
    resource.clearError();
    commands.clearError();
  }, [commands, resource]);

  return {
    viewAgentId: agentScope.viewAgentId,
    selectedAgentId: agentScope.selectedAgentId,
    setSelectedAgentId: agentScope.selectAgent,
    files: resource.files,
    isLoadingFiles: resource.isLoading,
    isUploading: commands.activeCommand === "upload",
    errorMessage: commands.errorMessage ?? resource.errorMessage,
    clearErrorMessage,
    contextMenu,
    promptState,
    deleteTarget,
    focusedDirectoryPath,
    currentDirectoryLabel: focusedDirectoryPath ?? "/",
    handleClickFile,
    handleClickDirectory: setFocusedDirectoryPath,
    handleUploadClick,
    handleFileSelect,
    openCreatePrompt,
    openRenamePrompt,
    handlePromptConfirm,
    handleConfirmDelete,
    handleDownloadContextEntry,
    handleContextMenu,
    handleRootContextMenu,
    closeContextMenu: () => setContextMenu({position: null, entry: null}),
    setDeleteTarget,
    setPromptState,
  };
}
