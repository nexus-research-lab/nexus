/**
 * =====================================================
 * @File   : workspace-file-tree.tsx
 * @Date   : 2026-04-15 17:44
 * @Author : leemysw
 * 2026-04-15 17:44   Create
 * =====================================================
 */

"use client";

import { memo, useCallback, useMemo, useState } from "react";
import { ChevronRight, Folder, FolderOpen, Pencil, Trash2 } from "lucide-react";

import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import { WorkspaceFileEntry } from "@/types/agent/agent";

import { getWorkspaceFileVisual } from "./workspace-file-visuals";

interface TreeNode {
  entry: WorkspaceFileEntry;
  children: TreeNode[];
}

interface WorkspaceFileTreeProps {
  entries: WorkspaceFileEntry[];
  activePath: string | null;
  focusedDirectoryPath: string | null;
  onClickFile: (path: string) => void;
  onClickDirectory: (path: string) => void;
  onRenameEntry: (entry: WorkspaceFileEntry) => void;
  onDeleteEntry: (entry: WorkspaceFileEntry) => void;
  onContextMenu: (event: React.MouseEvent, entry: WorkspaceFileEntry) => void;
}

function buildTree(entries: WorkspaceFileEntry[]): TreeNode[] {
  const sortedEntries = [...entries].sort((left, right) => {
    if (left.is_dir !== right.is_dir) {
      return left.is_dir ? -1 : 1;
    }
    return left.path.localeCompare(right.path);
  });

  const roots: TreeNode[] = [];
  const nodeMap = new Map<string, TreeNode>();

  for (const entry of sortedEntries) {
    const node: TreeNode = { entry, children: [] };
    nodeMap.set(entry.path, node);

    const parentPath = entry.path.substring(0, entry.path.lastIndexOf("/"));
    const parent = nodeMap.get(parentPath);
    if (parent) {
      parent.children.push(node);
    } else {
      roots.push(node);
    }
  }

  return roots;
}

interface WorkspaceFileTreeRowProps {
  node: TreeNode;
  activePath: string | null;
  focusedDirectoryPath: string | null;
  depth: number;
  onClickFile: (path: string) => void;
  onClickDirectory: (path: string) => void;
  onRenameEntry: (entry: WorkspaceFileEntry) => void;
  onDeleteEntry: (entry: WorkspaceFileEntry) => void;
  onContextMenu: (event: React.MouseEvent, entry: WorkspaceFileEntry) => void;
}

const WorkspaceFileTreeRow = memo(function WorkspaceFileTreeRow({
  node,
  activePath: activePath,
  focusedDirectoryPath: focusedDirectoryPath,
  depth,
  onClickFile: onClickFile,
  onClickDirectory: onClickDirectory,
  onRenameEntry: onRenameEntry,
  onDeleteEntry: onDeleteEntry,
  onContextMenu: onContextMenu,
}: WorkspaceFileTreeRowProps) {
  const { t } = useI18n();
  const { entry, children } = node;
  const isActive = entry.path === activePath;
  const isDirectoryTarget = entry.is_dir && entry.path === focusedDirectoryPath;
  const isSelected = isActive || isDirectoryTarget;
  const { Icon: FileIcon, iconClassName: iconClassName } = getWorkspaceFileVisual(entry.name);
  const [isOpen, setIsOpen] = useState(depth === 0);

  const handleClick = useCallback(() => {
    if (entry.is_dir) {
      setIsOpen((value) => !value);
      onClickDirectory(entry.path);
      return;
    }
    onClickFile(entry.path);
  }, [entry, onClickDirectory, onClickFile]);

  const handleContextMenu = useCallback((event: React.MouseEvent) => {
    event.preventDefault();
    event.stopPropagation();
    onContextMenu(event, entry);
  }, [entry, onContextMenu]);

  return (
    <div>
      <div
        className={cn(
          "group relative flex min-w-full w-max items-center gap-1.25 rounded-xl px-2 py-1.25 text-left transition-colors",
          "hover:bg-(--surface-interactive-hover-background)",
          isSelected
            ? "bg-[color:color-mix(in_srgb,var(--foreground)_4%,transparent)] text-(--text-strong)"
            : "text-(--text-default)",
        )}
        style={{ paddingLeft: `${8 + depth * 12}px` }}
        onClick={handleClick}
        onContextMenu={handleContextMenu}
        onKeyDown={(event) => {
          if (event.key === "Enter" || event.key === " ") {
            event.preventDefault();
            handleClick();
          }
        }}
        role="button"
        tabIndex={0}
      >
        {isSelected ? (
          <span
            aria-hidden="true"
            className="absolute left-1 top-2 bottom-2 w-px rounded-full bg-[color:color-mix(in_srgb,var(--primary)_72%,white_28%)]"
          />
        ) : null}

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

        {entry.is_dir ? (
          isOpen ? (
            <FolderOpen
              className={cn(
                "h-3.5 w-3.5 shrink-0",
                isDirectoryTarget ? "text-[color:color-mix(in_srgb,var(--accent)_82%,var(--foreground)_18%)]" : "text-[var(--accent)]",
              )}
            />
          ) : (
            <Folder
              className={cn(
                "h-3.5 w-3.5 shrink-0",
                isDirectoryTarget ? "text-[color:color-mix(in_srgb,var(--accent)_82%,var(--foreground)_18%)]" : "text-[var(--accent)]",
              )}
            />
          )
        ) : (
          <FileIcon className={cn("h-3.5 w-3.5 shrink-0", iconClassName)} />
        )}

        <span
          className={cn(
            "shrink-0 whitespace-nowrap text-[13px] leading-[1.3rem]",
            entry.is_dir || isSelected ? "font-medium" : "font-normal",
          )}
        >
          {entry.name}
        </span>

        <div
          className={cn(
            "ml-auto flex shrink-0 items-center gap-0.5 pl-2 transition-opacity",
            isSelected ? "opacity-100" : "opacity-0 group-hover:opacity-100",
          )}
        >
          <button
            type="button"
            className="flex h-5.5 w-5.5 items-center justify-center rounded-md text-(--icon-muted) transition hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-default)"
            onClick={(event) => {
              event.stopPropagation();
              onRenameEntry(entry);
            }}
            title={t("home.rename")}
          >
            <Pencil className="h-3 w-3" />
          </button>

          <button
            type="button"
            className="flex h-5.5 w-5.5 items-center justify-center rounded-md text-(--icon-muted) transition hover:bg-[color:color-mix(in_srgb,var(--destructive)_8%,transparent)] hover:text-(--destructive)"
            onClick={(event) => {
              event.stopPropagation();
              onDeleteEntry(entry);
            }}
            title={t("common.delete")}
          >
            <Trash2 className="h-3 w-3" />
          </button>
        </div>
      </div>

      {entry.is_dir && isOpen && children.length > 0 ? (
        <div>
          {children.map((child) => (
            <WorkspaceFileTreeRow
              key={child.entry.path}
              node={child}
              activePath={activePath}
              focusedDirectoryPath={focusedDirectoryPath}
              depth={depth + 1}
              onClickFile={onClickFile}
              onClickDirectory={onClickDirectory}
              onRenameEntry={onRenameEntry}
              onDeleteEntry={onDeleteEntry}
              onContextMenu={onContextMenu}
            />
          ))}
        </div>
      ) : null}
    </div>
  );
});

export function WorkspaceFileTree({
  entries,
  activePath: activePath,
  focusedDirectoryPath: focusedDirectoryPath,
  onClickFile: onClickFile,
  onClickDirectory: onClickDirectory,
  onRenameEntry: onRenameEntry,
  onDeleteEntry: onDeleteEntry,
  onContextMenu: onContextMenu,
}: WorkspaceFileTreeProps) {
  const tree = useMemo(() => buildTree(entries), [entries]);

  return (
    <>
      {tree.map((node) => (
        <WorkspaceFileTreeRow
          key={node.entry.path}
          node={node}
          activePath={activePath}
          focusedDirectoryPath={focusedDirectoryPath}
          depth={0}
          onClickFile={onClickFile}
          onClickDirectory={onClickDirectory}
          onRenameEntry={onRenameEntry}
          onDeleteEntry={onDeleteEntry}
          onContextMenu={onContextMenu}
        />
      ))}
    </>
  );
}
