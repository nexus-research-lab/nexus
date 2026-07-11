"use client";

import { memo, useCallback, useState, type MouseEvent } from "react";
import { ChevronRight, Folder, FolderOpen, Pencil, Trash2 } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { WorkspaceFileEntry } from "@/types/agent/agent";

import {
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
  const isActive = entry.path === activePath;
  const isDirectoryTarget = entry.is_dir && entry.path === focusedDirectoryPath;
  const isSelected = isActive || isDirectoryTarget;

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
        className={cn(
          "group relative flex min-w-full w-max items-center rounded-xl pr-2 text-left transition-colors",
          "hover:bg-(--surface-interactive-hover-background)",
          isSelected
            ? "bg-[color:color-mix(in_srgb,var(--foreground)_4%,transparent)] text-(--text-strong)"
            : "text-(--text-default)",
        )}
        onContextMenu={handleContextMenu}
      >
        {isSelected ? (
          <span
            aria-hidden="true"
            className="absolute left-1 top-2 bottom-2 w-px rounded-full bg-[color:color-mix(in_srgb,var(--primary)_72%,white_28%)]"
          />
        ) : null}

        <button
          className="flex min-w-0 flex-1 items-center gap-1.25 py-1.25 text-left"
          onClick={handleClick}
          style={{ paddingLeft: `${8 + depth * 12}px` }}
          type="button"
        >
          {entry.is_dir ? (
            <ChevronRight
              className={cn(
                "h-3 w-3 shrink-0 transition-transform",
                isSelected ? "text-(--icon-default)" : "text-(--icon-muted)",
                isOpen && "rotate-90",
              )}
            />
          ) : (
            <span className="w-3 shrink-0" />
          )}
          <WorkspaceTreeEntryIcon
            entry={entry}
            isDirectoryTarget={isDirectoryTarget}
            isOpen={isOpen}
          />
          <span
            className={cn(
              "shrink-0 whitespace-nowrap text-[13px] leading-[1.3rem]",
              entry.is_dir || isSelected ? "font-medium" : "font-normal",
            )}
          >
            {entry.name}
          </span>
        </button>
        <WorkspaceFileTreeRowActions
          actions={actions}
          entry={entry}
          visible={isSelected}
        />
      </div>

      {entry.is_dir && isOpen ? children.map((child) => (
        <WorkspaceFileTreeRow
          actions={actions}
          activePath={activePath}
          depth={depth + 1}
          focusedDirectoryPath={focusedDirectoryPath}
          key={child.entry.path}
          node={child}
        />
      )) : null}
    </div>
  );
});

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
