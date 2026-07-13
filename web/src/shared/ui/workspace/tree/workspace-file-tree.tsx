"use client";

import { useMemo } from "react";

import type { WorkspaceFileEntry } from "@/types/agent/agent";

import { buildWorkspaceFileTree } from "./workspace-file-tree-model";
import {
  WorkspaceFileTreeRow,
  type WorkspaceFileTreeActions,
} from "./workspace-file-tree-row";

interface WorkspaceFileTreeProps extends WorkspaceFileTreeActions {
  activePath: string | null;
  entries: WorkspaceFileEntry[];
  focusedDirectoryPath: string | null;
}

export function WorkspaceFileTree({
  activePath,
  entries,
  focusedDirectoryPath,
  onClickDirectory,
  onClickFile,
  onContextMenu,
  onDeleteEntry,
  onRenameEntry,
}: WorkspaceFileTreeProps) {
  const tree = useMemo(() => buildWorkspaceFileTree(entries), [entries]);
  const actions = useMemo<WorkspaceFileTreeActions>(() => ({
    onClickDirectory,
    onClickFile,
    onContextMenu,
    onDeleteEntry,
    onRenameEntry,
  }), [
    onClickDirectory,
    onClickFile,
    onContextMenu,
    onDeleteEntry,
    onRenameEntry,
  ]);

  return tree.map((node) => (
    <WorkspaceFileTreeRow
      actions={actions}
      activePath={activePath}
      depth={0}
      focusedDirectoryPath={focusedDirectoryPath}
      key={node.entry.path}
      node={node}
    />
  ));
}
