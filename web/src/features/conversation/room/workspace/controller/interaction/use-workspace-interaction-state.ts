// INPUT: Workspace scope、上传 input、目录焦点与用户原生交互事件。
// OUTPUT: 按 scope 隔离的菜单调用点/DOM 锚点、Prompt、删除和上传状态。
// POS: Workspace 本地交互适配；只记录调用身份，菜单尺寸与视口边界由 UI owner 处理。
import { useCallback, type MouseEvent, type RefObject } from "react";

import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import type { WorkspaceFileEntry } from "@/types/agent/agent";

import {
  createWorkspacePrompt,
  type WorkspaceContextMenuState,
  type WorkspacePromptState,
} from "./workspace-interaction-model";

interface UseWorkspaceInteractionStateOptions {
  fileInputRef: RefObject<HTMLInputElement | null>;
  focusedDirectoryPath: string | null;
  scopeKey: string;
}

const CLOSED_CONTEXT_MENU: WorkspaceContextMenuState = {
  anchor: null,
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
    setContextMenu(getContextMenuState(event, entry));
  }, [setContextMenu]);
  const openRootContextMenu = useCallback((event: MouseEvent) => {
    setContextMenu(getContextMenuState(event, null));
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

function getContextMenuState(event: MouseEvent, entry: WorkspaceFileEntry | null): WorkspaceContextMenuState {
  event.preventDefault();
  const anchor = event.currentTarget instanceof HTMLElement ? event.currentTarget : null;
  const rect = anchor?.getBoundingClientRect();
  const keyboardInvocation = event.button !== 2 && event.clientX === 0 && event.clientY === 0;
  return {
    anchor,
    entry,
    position: keyboardInvocation && rect
      ? { x: rect.left, y: rect.bottom }
      : { x: event.clientX, y: event.clientY },
  };
}
