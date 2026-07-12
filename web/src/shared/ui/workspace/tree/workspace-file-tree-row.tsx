"use client";

import { memo, useCallback, useState, type MouseEvent } from "react";
import { ChevronRight, Folder, FolderOpen, Pencil, Trash2 } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { WorkspaceFileEntry } from "@/types/agent/agent";

import {
  getWorkspaceFileTreeRowPresentation,
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
  focusedDirectoryPath: string | null;
  node: WorkspaceFileTreeNode;
}

export const WorkspaceFileTreeRow = memo(function WorkspaceFileTreeRow({
  actions,
  activePath,
  depth,
  focusedDirectoryPath,
  node,
}: WorkspaceFileTreeRowProps) {
  const { entry, children } = node;
  const [isOpen, setIsOpen] = useState(depth === 0);
  const presentation = getWorkspaceFileTreeRowPresentation({
    activePath,
    depth,
    entry,
    focusedDirectoryPath,
    isOpen,
  });

  const handleClick = useCallback(() => {
    if (entry.is_dir) {
      setIsOpen((value) => !value);
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
    <div>
      <div
        className={presentation.rowClassName}
        onContextMenu={handleContextMenu}
      >
        <WorkspaceTreeSelectionIndicator visible={presentation.isSelected} />

        <button
          className="flex min-w-0 flex-1 items-center gap-1.25 py-1.25 text-left"
          onClick={handleClick}
          style={{ paddingLeft: `${presentation.paddingLeft}px` }}
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
        </button>
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
        focusedDirectoryPath={focusedDirectoryPath}
        visible={presentation.showChildren}
      />
    </div>
  );
});

function WorkspaceTreeSelectionIndicator({ visible }: { visible: boolean }) {
  if (!visible) {
    return null;
  }
  return (
    <span
      aria-hidden="true"
      className="absolute left-1 top-2 bottom-2 w-px rounded-full bg-[color:color-mix(in_srgb,var(--primary)_72%,white_28%)]"
    />
  );
}

function WorkspaceTreeExpandIndicator({
  className,
  isDirectory,
}: {
  className: string;
  isDirectory: boolean;
}) {
  return isDirectory
    ? <ChevronRight className={className} />
    : <span className="w-3 shrink-0" />;
}

function WorkspaceFileTreeChildren({
  actions,
  activePath,
  children,
  depth,
  focusedDirectoryPath,
  visible,
}: {
  actions: WorkspaceFileTreeActions;
  activePath: string | null;
  children: WorkspaceFileTreeNode[];
  depth: number;
  focusedDirectoryPath: string | null;
  visible: boolean;
}) {
  if (!visible) {
    return null;
  }
  return children.map((child) => (
    <WorkspaceFileTreeRow
      actions={actions}
      activePath={activePath}
      depth={depth + 1}
      focusedDirectoryPath={focusedDirectoryPath}
      key={child.entry.path}
      node={child}
    />
  ));
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
  if (entry.is_dir) {
    const DirectoryIcon = isOpen ? FolderOpen : Folder;
    return (
      <DirectoryIcon
        className={cn(
          "h-3.5 w-3.5 shrink-0",
          isDirectoryTarget
            ? "text-[color:color-mix(in_srgb,var(--accent)_82%,var(--foreground)_18%)]"
            : "text-[var(--accent)]",
        )}
      />
    );
  }
  const { Icon, iconClassName } = getWorkspaceFileVisual(entry.name);
  return <Icon className={cn("h-3.5 w-3.5 shrink-0", iconClassName)} />;
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
  const handleRename = useCallback((event: MouseEvent) => {
    event.stopPropagation();
    actions.onRenameEntry(entry);
  }, [actions, entry]);
  const handleDelete = useCallback((event: MouseEvent) => {
    event.stopPropagation();
    actions.onDeleteEntry(entry);
  }, [actions, entry]);

  return (
    <div
      className={cn(
        "ml-auto flex shrink-0 items-center gap-0.5 pl-2 transition-opacity",
        visible ? "opacity-100" : "opacity-0 group-hover:opacity-100",
      )}
    >
      <button
        aria-label={t("home.rename")}
        className="flex h-5.5 w-5.5 items-center justify-center rounded-md text-(--icon-muted) transition hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-default)"
        onClick={handleRename}
        title={t("home.rename")}
        type="button"
      >
        <Pencil className="h-3 w-3" />
      </button>
      <button
        aria-label={t("common.delete")}
        className="flex h-5.5 w-5.5 items-center justify-center rounded-md text-(--icon-muted) transition hover:bg-[color:color-mix(in_srgb,var(--destructive)_8%,transparent)] hover:text-(--destructive)"
        onClick={handleDelete}
        title={t("common.delete")}
        type="button"
      >
        <Trash2 className="h-3 w-3" />
      </button>
    </div>
  );
}
