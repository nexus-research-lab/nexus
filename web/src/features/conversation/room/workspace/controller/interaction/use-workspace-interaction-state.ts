import { useCallback, type MouseEvent, type RefObject } from "react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import type { WorkspaceFileEntry } from "@/types/agent/agent";

import {
  createWorkspacePrompt,
  resolveWorkspaceMenuPosition,
  type WorkspaceContextMenuState,
  type WorkspacePromptState,
} from "./workspace-interaction-model";

interface UseWorkspaceInteractionStateOptions {
  fileInputRef: RefObject<HTMLInputElement | null>;
  focusedDirectoryPath: string | null;
  scopeKey: string;
}

const CLOSED_CONTEXT_MENU: WorkspaceContextMenuState = {
  entry: null,
  position: null,
};

export function useWorkspaceInteractionState({
  fileInputRef,
  focusedDirectoryPath,
  scopeKey,
}: UseWorkspaceInteractionStateOptions) {
  const [contextMenu, setContextMenu] =
    useResettableState<WorkspaceContextMenuState>(CLOSED_CONTEXT_MENU, scopeKey);
  const [promptState, setPromptState] =
    useResettableState<WorkspacePromptState>(null, scopeKey);
  const [deleteTarget, setDeleteTarget] =
    useResettableState<WorkspaceFileEntry | null>(null, scopeKey);
  const [uploadTargetDirectory, setUploadTargetDirectory] =
    useResettableState<string | null>(null, scopeKey);

  const openUpload = useCallback((directoryPath?: string | null) => {
    setUploadTargetDirectory(directoryPath ?? focusedDirectoryPath);
    fileInputRef.current?.click();
  }, [fileInputRef, focusedDirectoryPath, setUploadTargetDirectory]);
  const clearUploadTarget = useCallback(() => {
    setUploadTargetDirectory(null);
  }, [setUploadTargetDirectory]);
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
    setPromptState({ defaultValue: entry.name, entry, mode: "rename" });
  }, [setPromptState]);
  const openContextMenu = useCallback((
    event: MouseEvent,
    entry: WorkspaceFileEntry,
  ) => {
    setContextMenu({ entry, position: getMenuPosition(event, entry) });
  }, [setContextMenu]);
  const openRootContextMenu = useCallback((event: MouseEvent) => {
    event.preventDefault();
    setContextMenu({ entry: null, position: getMenuPosition(event, null) });
  }, [setContextMenu]);
  const closeContextMenu = useCallback(() => {
    setContextMenu(CLOSED_CONTEXT_MENU);
  }, [setContextMenu]);
  const closePrompt = useCallback(() => {
    setPromptState(null);
  }, [setPromptState]);
  const clearDeleteTarget = useCallback(() => {
    setDeleteTarget(null);
  }, [setDeleteTarget]);
  const openDeletePrompt = useCallback((entry: WorkspaceFileEntry) => {
    setDeleteTarget(entry);
  }, [setDeleteTarget]);

  return {
    clearDeleteTarget,
    clearUploadTarget,
    closeContextMenu,
    closePrompt,
    contextMenu,
    deleteTarget,
    openContextMenu,
    openCreatePrompt,
    openDeletePrompt,
    openRenamePrompt,
    openRootContextMenu,
    openUpload,
    promptState,
    uploadTargetDirectory,
  };
}

function getMenuPosition(
  event: MouseEvent,
  entry: WorkspaceFileEntry | null,
): { x: number; y: number } {
  return resolveWorkspaceMenuPosition(
    { x: event.clientX, y: event.clientY },
    { height: window.innerHeight, width: window.innerWidth },
    entry,
  );
}
