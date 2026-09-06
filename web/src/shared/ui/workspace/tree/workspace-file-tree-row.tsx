// INPUT: 文件节点、真实深度、受控展开/选中投影与稳定动作。
// OUTPUT: 原生按钮组成的嵌套目录，具名展开、完整路径提示和独立行内动作。
// POS: 文件树递归布局；展开状态归 Tree，Button/行次动作归共享 owner。

"use client";

import { memo, useCallback, useId, type MouseEvent } from "react";
import { ChevronRight, Pencil, Trash2 } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { UiButton } from "@/shared/ui/button/button";
import { UiListActionButton } from "@/shared/ui/list/list-action";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { WorkspaceFileEntry } from "@/types/agent/agent";

import {
  getWorkspaceFileTreeRowPresentation,
  getWorkspaceDirectoryIcon,
  getWorkspaceFileVisual,
  type WorkspaceFileTreeNode,
} from "./workspace-file-tree-model";

export interface WorkspaceFileTreeActions {
  onClickDirectory: (path: string) => void;
  onClickFile: (path: string) => void;
  onContextMenu: (event: MouseEvent, entry: WorkspaceFileEntry) => void;
  onDeleteEntry: (entry: WorkspaceFileEntry) => void;
  onRenameEntry: (entry: WorkspaceFileEntry) => void;
}

interface WorkspaceFileTreeRowProps {
  actions: WorkspaceFileTreeActions;
  activePath: string | null;
  depth: number;
  expandedDirectories: ReadonlyMap<string, boolean>;
  focusedDirectoryPath: string | null;
  node: WorkspaceFileTreeNode;
}

export const WorkspaceFileTreeRow = memo(function WorkspaceFileTreeRow({
  actions,
  activePath,
  depth,
  expandedDirectories,
  focusedDirectoryPath,
  node,
}: WorkspaceFileTreeRowProps) {
  const { entry, children } = node;
  const isOpen = expandedDirectories.get(entry.path) ?? depth === 0;
  const entryId = useId();
  const childrenId = useId();
  const presentation = getWorkspaceFileTreeRowPresentation({
    activePath,
    depth,
    entry,
    focusedDirectoryPath,
    isOpen,
  });

  const handleClick = useCallback(() => {
    if (entry.is_dir) {
      actions.onClickDirectory(entry.path);
      return;
    }
    actions.onClickFile(entry.path);
  }, [actions, entry]);
  const handleContextMenu = useCallback((event: MouseEvent) => {
    event.preventDefault();
    event.stopPropagation();
    actions.onContextMenu(event, entry);
  }, [actions, entry]);

  return (
    <li className="min-w-0">
      <div
        className={presentation.rowClassName}
        onContextMenu={handleContextMenu}
      >
        <UiButton
          aria-controls={presentation.showChildren ? childrenId : undefined}
          aria-current={presentation.isSelected ? "true" : undefined}
          aria-expanded={entry.is_dir ? isOpen : undefined}
          aria-label={entry.name}
          className="min-w-0 flex-1 justify-start gap-1.25 px-0 text-left focus-visible:ring-inset"
          id={entryId}
          onClick={handleClick}
          size="xs"
          style={{ paddingLeft: `min(${presentation.paddingLeft}px, 35%)` }}
          title={entry.path}
          variant="text"
          type="button"
        >
          <WorkspaceTreeExpandIndicator
            className={presentation.chevronClassName}
            isDirectory={entry.is_dir}
          />
          <WorkspaceTreeEntryIcon
            entry={entry}
            isDirectoryTarget={presentation.isDirectoryTarget}
            isOpen={isOpen}
          />
          <span className={presentation.nameClassName}>
            {entry.name}
          </span>
        </UiButton>
        <WorkspaceFileTreeRowActions
          actions={actions}
          entry={entry}
          visible={presentation.actionsVisible}
        />
      </div>
      <WorkspaceFileTreeChildren
        actions={actions}
        activePath={activePath}
        children={children}
        depth={depth}
        expandedDirectories={expandedDirectories}
        id={childrenId}
        labelledBy={entryId}
        focusedDirectoryPath={focusedDirectoryPath}
        visible={presentation.showChildren}
      />
    </li>
  );
});

function WorkspaceTreeExpandIndicator({
  className,
  isDirectory,
}: {
  className: string;
  isDirectory: boolean;
}) {
  return isDirectory
    ? <ChevronRight aria-hidden className={className} />
    : <span className="w-3 shrink-0" />;
}

function WorkspaceFileTreeChildren({
  actions,
  activePath,
  children,
  depth,
  expandedDirectories,
  focusedDirectoryPath,
  visible,
  id,
  labelledBy,
}: {
  actions: WorkspaceFileTreeActions;
  activePath: string | null;
  children: WorkspaceFileTreeNode[];
  depth: number;
  expandedDirectories: ReadonlyMap<string, boolean>;
  focusedDirectoryPath: string | null;
  visible: boolean;
  id: string;
  labelledBy: string;
}) {
  if (!visible) {
    return null;
  }
  return (
    // eslint-disable-next-line jsx-a11y/no-redundant-roles -- WebKit needs an explicit role for unmarked lists: https://bugs.webkit.org/show_bug.cgi?id=170179#c1
    <ul aria-labelledby={labelledBy} className="m-0 min-w-0 list-none p-0" id={id} role="list">
      {children.map((child) => (
        <WorkspaceFileTreeRow
          actions={actions}
          activePath={activePath}
          depth={depth + 1}
          expandedDirectories={expandedDirectories}
          focusedDirectoryPath={focusedDirectoryPath}
          key={child.entry.path}
          node={child}
        />
      ))}
    </ul>
  );
}

function WorkspaceTreeEntryIcon({
  entry,
  isDirectoryTarget,
  isOpen,
}: {
  entry: WorkspaceFileEntry;
  isDirectoryTarget: boolean;
  isOpen: boolean;
}) {
  const iconSrc = entry.is_dir
    ? getWorkspaceDirectoryIcon(isOpen)
    : getWorkspaceFileVisual(entry.name).iconSrc;
  return (
    <img
      alt=""
      aria-hidden="true"
      className={cn(
        "h-4 w-4 shrink-0 object-contain",
        entry.is_dir && !isDirectoryTarget && "opacity-90",
      )}
      draggable={false}
      src={iconSrc}
    />
  );
}

function WorkspaceFileTreeRowActions({
  actions,
  entry,
  visible,
}: {
  actions: WorkspaceFileTreeActions;
  entry: WorkspaceFileEntry;
  visible: boolean;
}) {
  const { t } = useI18n();

  return (
    <div className="ml-auto flex shrink-0 items-center gap-0.5 pl-1">
      <UiListActionButton
        aria-label={`${t("home.rename")} ${entry.path}`}
        onClick={() => actions.onRenameEntry(entry)}
        size="xs"
        stopPropagation
        title={t("home.rename")}
        visibility={visible ? "visible" : "hover"}
      >
        <Pencil aria-hidden className="h-3 w-3" />
      </UiListActionButton>
      <UiListActionButton
        aria-label={`${t("common.delete")} ${entry.path}`}
        onClick={() => actions.onDeleteEntry(entry)}
        size="xs"
        stopPropagation
        title={t("common.delete")}
        tone="danger"
        visibility={visible ? "visible" : "hover"}
      >
        <Trash2 aria-hidden className="h-3 w-3" />
      </UiListActionButton>
    </div>
  );
}
