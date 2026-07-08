/**
 * Room Workspace 控制器
 *
 * 统一管理 workspace 页面中的目录上下文、成员切换、文件操作与错误状态。
 */

import { ChangeEvent, MouseEvent, RefObject, useCallback, useEffect, useMemo, useRef, useState } from "react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import {
  createWorkspaceEntryApi,
  deleteWorkspaceEntryApi,
  renameWorkspaceEntryApi,
  uploadWorkspaceFileApi,
} from "@/lib/api/agent-manage-api";
import { useWorkspaceFilesStore } from "@/store/workspace-files";
import type { WorkspaceFileEntry } from "@/types/agent/agent";

export interface WorkspaceContextMenuState {
  position: { x: number; y: number } | null;
  entry: WorkspaceFileEntry | null;
}

export type WorkspacePromptState =
  | { mode: "create-file"; defaultValue: string; parentPath: string | null }
  | { mode: "create-directory"; defaultValue: string; parentPath: string | null }
  | { mode: "rename"; entry: WorkspaceFileEntry; defaultValue: string }
  | null;

interface UseRoomWorkspaceControllerOptions {
  activeWorkspacePath: string | null;
  agentId: string;
  isDm: boolean;
  onOpenWorkspaceFile: (path: string | null) => void;
  fileInputRef: RefObject<HTMLInputElement | null>;
}

function getParentDirectoryPath(path: string): string | null {
  const lastSlashIndex = path.lastIndexOf("/");
  if (lastSlashIndex === -1) {
    return null;
  }
  return path.slice(0, lastSlashIndex);
}

function getWorkspaceFocusDirectoryPath(path?: string | null): string | null {
  if (!path) {
    return null;
  }
  return getParentDirectoryPath(path);
}

function joinWorkspacePath(parentPath: string | null, name: string): string {
  return parentPath ? `${parentPath}/${name}` : name;
}

function getRenamedActivePath(
  activePath: string | null,
  oldPath: string,
  newPath: string,
): string | null {
  if (!activePath) {
    return null;
  }
  if (activePath === oldPath) {
    return newPath;
  }
  if (activePath.startsWith(`${oldPath}/`)) {
    return `${newPath}${activePath.slice(oldPath.length)}`;
  }
  return null;
}

function isWorkspacePathAffected(
  activePath: string | null,
  targetPath: string,
): boolean {
  if (!activePath) {
    return false;
  }
  return activePath === targetPath || activePath.startsWith(`${targetPath}/`);
}

function resolveWorkspaceMenuPosition(
  event: MouseEvent,
  menuHeight: number,
): { x: number; y: number } {
  const menuWidth = 180;
  return {
    x: Math.min(event.clientX, window.innerWidth - menuWidth),
    y: Math.min(event.clientY, window.innerHeight - menuHeight),
  };
}

export function useRoomWorkspaceController(
  {
    activeWorkspacePath,
    agentId,
    isDm,
    onOpenWorkspaceFile,
    fileInputRef,
}: UseRoomWorkspaceControllerOptions) {
  const [selectedAgentId, setSelectedAgentId] = useResettableState(agentId, agentId);
  const [isUploading, setIsUploading] = useState(false);
  const [isLoadingFiles, setIsLoadingFiles] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [contextMenu, setContextMenu] = useState<WorkspaceContextMenuState>({
    position: null,
    entry: null,
  });
  const [promptState, setPromptState] = useState<WorkspacePromptState>(null);
  const [deleteTarget, setDeleteTarget] = useState<WorkspaceFileEntry | null>(null);
  const [uploadTargetDirectory, setUploadTargetDirectory] = useState<string | null>(null);

  const filesByAgent = useWorkspaceFilesStore((state) => state.files_by_agent);
  const refreshFiles = useWorkspaceFilesStore((state) => state.refresh_files);
  const clearWorkspaceAgent = useWorkspaceFilesStore((state) => state.clear_agent);

  const previousViewAgentIdRef = useRef<string>(isDm ? agentId : selectedAgentId);
  const viewAgentId = isDm ? agentId : selectedAgentId;
  const [focusedDirectoryPath, setFocusedDirectoryPath] = useResettableState<string | null>(null, viewAgentId);
  const files = useMemo(() => filesByAgent[viewAgentId] || [], [filesByAgent, viewAgentId]);

  useEffect(() => {
    const previousViewAgentId = previousViewAgentIdRef.current;
    previousViewAgentIdRef.current = viewAgentId;

    if (previousViewAgentId !== viewAgentId) {
      onOpenWorkspaceFile(null);
    }

    let ignore = false;

    const loadWorkspaceFiles = async () => {
      setIsLoadingFiles(true);
      setErrorMessage(null);
      try {
        await refreshFiles(viewAgentId);
      } catch (error) {
        if (ignore) {
          return;
        }
        clearWorkspaceAgent(viewAgentId);
        setErrorMessage(error instanceof Error ? error.message : "加载文件列表失败");
      } finally {
        if (!ignore) {
          setIsLoadingFiles(false);
        }
      }
    };

    void loadWorkspaceFiles();

    return () => {
      ignore = true;
    };
  }, [clearWorkspaceAgent, onOpenWorkspaceFile, refreshFiles, viewAgentId]);

  useEffect(() => {
    setFocusedDirectoryPath(getWorkspaceFocusDirectoryPath(activeWorkspacePath));
  }, [activeWorkspacePath, setFocusedDirectoryPath, viewAgentId]);

  const handleClickFile = useCallback((path: string) => {
    setFocusedDirectoryPath(getParentDirectoryPath(path));
    onOpenWorkspaceFile(path);
  }, [onOpenWorkspaceFile, setFocusedDirectoryPath]);

  const handleClickDirectory = useCallback((path: string) => {
    setFocusedDirectoryPath(path);
  }, [setFocusedDirectoryPath]);

  const handleUploadClick = useCallback((directoryPath?: string | null) => {
    setUploadTargetDirectory(directoryPath ?? focusedDirectoryPath);
    fileInputRef.current?.click();
  }, [fileInputRef, focusedDirectoryPath]);

  const handleFileSelect = useCallback(async (event: ChangeEvent<HTMLInputElement>) => {
    const selectedFiles = event.target.files;
    if (!selectedFiles || selectedFiles.length === 0) {
      return;
    }

    setIsUploading(true);
    setErrorMessage(null);
    try {
      for (const file of Array.from(selectedFiles)) {
        const targetDirectory = uploadTargetDirectory ? `${uploadTargetDirectory}/` : undefined;
        await uploadWorkspaceFileApi(viewAgentId, file, targetDirectory);
      }
      await refreshFiles(viewAgentId);
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : "上传文件失败");
    } finally {
      setIsUploading(false);
      setUploadTargetDirectory(null);
      if (fileInputRef.current) {
        fileInputRef.current.value = "";
      }
    }
  }, [fileInputRef, refreshFiles, uploadTargetDirectory, viewAgentId]);

  const openCreatePrompt = useCallback((entryType: "file" | "directory", parentPath?: string | null) => {
    setPromptState(
      entryType === "file"
        ? {mode: "create-file", defaultValue: "untitled.txt", parentPath: parentPath ?? focusedDirectoryPath}
        : {mode: "create-directory", defaultValue: "new-folder", parentPath: parentPath ?? focusedDirectoryPath},
    );
  }, [focusedDirectoryPath]);

  const openRenamePrompt = useCallback((entry: WorkspaceFileEntry) => {
    setPromptState({
      mode: "rename",
      entry,
      defaultValue: entry.name,
    });
  }, []);

  const handlePromptConfirm = useCallback(async (value: string) => {
    const normalizedName = value.trim();
    if (!promptState || !normalizedName) {
      return;
    }

    setErrorMessage(null);
    try {
      if (promptState.mode === "rename") {
        if (normalizedName === promptState.entry.name) {
          setPromptState(null);
          return;
        }

        const renamedEntry = await renameWorkspaceEntryApi(
          viewAgentId,
          promptState.entry.path,
          joinWorkspacePath(getParentDirectoryPath(promptState.entry.path), normalizedName),
        );
        await refreshFiles(viewAgentId);

        const nextActivePath = getRenamedActivePath(
          activeWorkspacePath,
          promptState.entry.path,
          renamedEntry.new_path,
        );
        if (nextActivePath) {
          onOpenWorkspaceFile(nextActivePath);
        }
        if (isWorkspacePathAffected(focusedDirectoryPath, promptState.entry.path)) {
          setFocusedDirectoryPath(
            getRenamedActivePath(focusedDirectoryPath, promptState.entry.path, renamedEntry.new_path),
          );
        }
      } else {
        const createdEntry = await createWorkspaceEntryApi(
          viewAgentId,
          joinWorkspacePath(promptState.parentPath, normalizedName),
          promptState.mode === "create-file" ? "file" : "directory",
        );
        await refreshFiles(viewAgentId);

        if (promptState.mode === "create-file") {
          onOpenWorkspaceFile(createdEntry.path);
          setFocusedDirectoryPath(getParentDirectoryPath(createdEntry.path));
        } else {
          setFocusedDirectoryPath(createdEntry.path);
        }
      }
      setPromptState(null);
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : "工作区操作失败");
    }
  }, [
    activeWorkspacePath,
    focusedDirectoryPath,
    onOpenWorkspaceFile,
    promptState,
    refreshFiles,
    setFocusedDirectoryPath,
    viewAgentId,
  ]);

  const handleConfirmDelete = useCallback(async () => {
    if (!deleteTarget) {
      return;
    }

    setErrorMessage(null);
    try {
      await deleteWorkspaceEntryApi(viewAgentId, deleteTarget.path);
      await refreshFiles(viewAgentId);
      if (isWorkspacePathAffected(activeWorkspacePath, deleteTarget.path)) {
        onOpenWorkspaceFile(null);
      }
      if (isWorkspacePathAffected(focusedDirectoryPath, deleteTarget.path)) {
        setFocusedDirectoryPath(getParentDirectoryPath(deleteTarget.path));
      }
      setDeleteTarget(null);
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : "删除失败");
    }
  }, [
    activeWorkspacePath,
    deleteTarget,
    focusedDirectoryPath,
    onOpenWorkspaceFile,
    refreshFiles,
    setFocusedDirectoryPath,
    viewAgentId,
  ]);

  const handleContextMenu = useCallback((event: MouseEvent, entry: WorkspaceFileEntry) => {
    setContextMenu({
      position: resolveWorkspaceMenuPosition(event, entry.is_dir ? 178 : 102),
      entry,
    });
  }, []);

  const handleRootContextMenu = useCallback((event: MouseEvent) => {
    event.preventDefault();
    setContextMenu({
      position: resolveWorkspaceMenuPosition(event, 106),
      entry: null,
    });
  }, []);

  const closeContextMenu = useCallback(() => {
    setContextMenu({position: null, entry: null});
  }, []);

  return {
    viewAgentId,
    files,
    selectedAgentId,
    setSelectedAgentId,
    isUploading,
    isLoadingFiles,
    errorMessage,
    clearErrorMessage: () => setErrorMessage(null),
    contextMenu,
    promptState,
    deleteTarget,
    focusedDirectoryPath,
    currentDirectoryLabel: focusedDirectoryPath ?? "/",
    handleClickFile,
    handleClickDirectory,
    handleUploadClick,
    handleFileSelect,
    openCreatePrompt,
    openRenamePrompt,
    handlePromptConfirm,
    handleConfirmDelete,
    handleContextMenu,
    handleRootContextMenu,
    closeContextMenu,
    setDeleteTarget,
    setPromptState,
  };
}
