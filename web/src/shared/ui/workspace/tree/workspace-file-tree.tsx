// INPUT: 文件快照、当前路径与调用方导航/文件管理命令。
// OUTPUT: 具名层级目录；路径展开偏好在父级收起和同目录刷新后保留。
// POS: 文件树展开状态与稳定动作 owner；不请求文件或执行持久写入。

"use client";

import { useCallback, useEffect, useMemo, useState } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
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
  const { t } = useI18n();
  const tree = useMemo(() => buildWorkspaceFileTree(entries), [entries]);
  const [expandedDirectories, setExpandedDirectories] = useState<ReadonlyMap<string, boolean>>(
    () => new Map(),
  );
  const rootDirectories = useMemo(
    () => new Set(tree.filter(({ entry }) => entry.is_dir).map(({ entry }) => entry.path)),
    [tree],
  );
  useEffect(() => {
    const directories = new Set(entries.filter((entry) => entry.is_dir).map((entry) => entry.path));
    setExpandedDirectories((current) => [...current.keys()].some((path) => !directories.has(path))
      ? new Map([...current].filter(([path]) => directories.has(path)))
      : current);
  }, [entries]);
  const handleClickDirectory = useCallback((path: string) => {
    setExpandedDirectories((current) => new Map(current).set(
      path,
      !(current.get(path) ?? rootDirectories.has(path)),
    ));
    onClickDirectory(path);
  }, [onClickDirectory, rootDirectories]);
  const actions = useMemo<WorkspaceFileTreeActions>(() => ({
    onClickDirectory: handleClickDirectory,
    onClickFile,
    onContextMenu,
    onDeleteEntry,
    onRenameEntry,
  }), [
    handleClickDirectory,
    onClickFile,
    onContextMenu,
    onDeleteEntry,
    onRenameEntry,
  ]);

  return (
    // eslint-disable-next-line jsx-a11y/no-redundant-roles -- WebKit needs an explicit role for unmarked lists: https://bugs.webkit.org/show_bug.cgi?id=170179#c1
    <ul aria-label={t("common.file_tree")} className="m-0 min-w-0 list-none p-0" role="list">
      {tree.map((node) => (
        <WorkspaceFileTreeRow
          actions={actions}
          activePath={activePath}
          depth={0}
          expandedDirectories={expandedDirectories}
          focusedDirectoryPath={focusedDirectoryPath}
          key={node.entry.path}
          node={node}
        />
      ))}
    </ul>
  );
}
