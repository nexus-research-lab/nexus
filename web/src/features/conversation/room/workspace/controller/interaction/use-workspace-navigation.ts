import { useCallback, useEffect } from "react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";

import {
  getParentWorkspacePath,
  getWorkspaceFocusPath,
  isWorkspacePathWithin,
  replaceWorkspacePathPrefix,
} from "../workspace-path-model";

interface UseWorkspaceNavigationOptions {
  activeWorkspacePath: string | null;
  onOpenWorkspaceFile: (path: string | null) => void;
  scopeKey: string;
}

export function useWorkspaceNavigation({
  activeWorkspacePath,
  onOpenWorkspaceFile,
  scopeKey,
}: UseWorkspaceNavigationOptions) {
  const [focusedDirectoryPath, setFocusedDirectoryPath] =
    useResettableState<string | null>(null, scopeKey);

  useEffect(() => {
    setFocusedDirectoryPath(getWorkspaceFocusPath(activeWorkspacePath));
  }, [activeWorkspacePath, setFocusedDirectoryPath]);

  const openFile = useCallback((path: string) => {
    setFocusedDirectoryPath(getParentWorkspacePath(path));
    onOpenWorkspaceFile(path);
  }, [onOpenWorkspaceFile, setFocusedDirectoryPath]);
  const applyRename = useCallback((previousPath: string, nextPath: string) => {
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
  }, [
    activeWorkspacePath,
    focusedDirectoryPath,
    onOpenWorkspaceFile,
    setFocusedDirectoryPath,
  ]);
  const applyCreate = useCallback((entryType: "file" | "directory", path: string) => {
    if (entryType === "file") {
      openFile(path);
      return;
    }
    setFocusedDirectoryPath(path);
  }, [openFile, setFocusedDirectoryPath]);
  const applyDelete = useCallback((deletedPath: string) => {
    if (isWorkspacePathWithin(activeWorkspacePath, deletedPath)) {
      onOpenWorkspaceFile(null);
    }
    if (isWorkspacePathWithin(focusedDirectoryPath, deletedPath)) {
      setFocusedDirectoryPath(getParentWorkspacePath(deletedPath));
    }
  }, [
    activeWorkspacePath,
    focusedDirectoryPath,
    onOpenWorkspaceFile,
    setFocusedDirectoryPath,
  ]);

  return {
    applyCreate,
    applyDelete,
    applyRename,
    focusedDirectoryPath,
    focusDirectory: setFocusedDirectoryPath,
    openFile,
  };
}
