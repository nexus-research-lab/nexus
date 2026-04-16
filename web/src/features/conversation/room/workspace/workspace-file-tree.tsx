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

import { get_workspace_file_visual } from "./workspace-file-visuals";

interface TreeNode {
  entry: WorkspaceFileEntry;
  children: TreeNode[];
}

interface WorkspaceFileTreeProps {
  entries: WorkspaceFileEntry[];
  active_path: string | null;
  focused_directory_path: string | null;
  on_click_file: (path: string) => void;
  on_click_directory: (path: string) => void;
  on_rename_entry: (entry: WorkspaceFileEntry) => void;
  on_delete_entry: (entry: WorkspaceFileEntry) => void;
  on_context_menu: (event: React.MouseEvent, entry: WorkspaceFileEntry) => void;
}

function build_tree(entries: WorkspaceFileEntry[]): TreeNode[] {
  const sorted_entries = [...entries].sort((left, right) => {
    if (left.is_dir !== right.is_dir) {
      return left.is_dir ? -1 : 1;
    }
    return left.path.localeCompare(right.path);
  });

  const roots: TreeNode[] = [];
  const node_map = new Map<string, TreeNode>();

  for (const entry of sorted_entries) {
    const node: TreeNode = { entry, children: [] };
    node_map.set(entry.path, node);

    const parent_path = entry.path.substring(0, entry.path.lastIndexOf("/"));
    const parent = node_map.get(parent_path);
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
  active_path: string | null;
  focused_directory_path: string | null;
  depth: number;
  on_click_file: (path: string) => void;
  on_click_directory: (path: string) => void;
  on_rename_entry: (entry: WorkspaceFileEntry) => void;
  on_delete_entry: (entry: WorkspaceFileEntry) => void;
  on_context_menu: (event: React.MouseEvent, entry: WorkspaceFileEntry) => void;
}

const WorkspaceFileTreeRow = memo(function WorkspaceFileTreeRow({
  node,
  active_path,
  focused_directory_path,
  depth,
  on_click_file,
  on_click_directory,
  on_rename_entry,
  on_delete_entry,
  on_context_menu,
}: WorkspaceFileTreeRowProps) {
  const { t } = useI18n();
  const { entry, children } = node;
  const is_active = entry.path === active_path;
  const is_directory_target = entry.is_dir && entry.path === focused_directory_path;
  const is_selected = is_active || is_directory_target;
  const { Icon: FileIcon, icon_class_name } = get_workspace_file_visual(entry.name);
  const [is_open, set_is_open] = useState(depth === 0);

  const handle_click = useCallback(() => {
    if (entry.is_dir) {
      set_is_open((value) => !value);
      on_click_directory(entry.path);
      return;
    }
    on_click_file(entry.path);
  }, [entry, on_click_directory, on_click_file]);

  const handle_context_menu = useCallback((event: React.MouseEvent) => {
    event.preventDefault();
    event.stopPropagation();
    on_context_menu(event, entry);
  }, [entry, on_context_menu]);

  return (
    <div>
      <div
        className={cn(
          "group relative flex w-full items-center gap-1.5 rounded-xl px-2.5 py-1.5 text-left transition-colors",
          "hover:bg-(--surface-interactive-hover-background)",
          is_selected
            ? "bg-[color:color-mix(in_srgb,var(--foreground)_4%,transparent)] text-(--text-strong)"
            : "text-(--text-default)",
        )}
        style={{ paddingLeft: `${8 + depth * 14}px` }}
        onClick={handle_click}
        onContextMenu={handle_context_menu}
      >
        {is_selected ? (
          <span
            aria-hidden="true"
            className="absolute left-1 top-2 bottom-2 w-px rounded-full bg-[color:color-mix(in_srgb,var(--primary)_72%,white_28%)]"
          />
        ) : null}

        {entry.is_dir ? (
          <ChevronRight
            className={cn(
              "h-3.5 w-3.5 shrink-0 transition-transform",
              is_selected ? "text-(--icon-default)" : "text-(--icon-muted)",
              is_open && "rotate-90",
            )}
          />
        ) : (
          <span className="w-3.5 shrink-0" />
        )}

        {entry.is_dir ? (
          is_open ? (
            <FolderOpen
              className={cn(
                "h-4 w-4 shrink-0",
                is_directory_target ? "text-[color:color-mix(in_srgb,var(--accent)_82%,var(--foreground)_18%)]" : "text-[var(--accent)]",
              )}
            />
          ) : (
            <Folder
              className={cn(
                "h-4 w-4 shrink-0",
                is_directory_target ? "text-[color:color-mix(in_srgb,var(--accent)_82%,var(--foreground)_18%)]" : "text-[var(--accent)]",
              )}
            />
          )
        ) : (
          <FileIcon className={cn("h-4 w-4 shrink-0", icon_class_name)} />
        )}

        <span
          className={cn(
            "min-w-0 flex-1 truncate text-[14px] leading-[1.45rem]",
            entry.is_dir || is_selected ? "font-medium" : "font-normal",
          )}
        >
          {entry.name}
        </span>

        <div
          className={cn(
            "ml-auto flex shrink-0 items-center gap-0.5 transition-opacity",
            is_selected ? "opacity-100" : "opacity-0 group-hover:opacity-100",
          )}
        >
          <button
            type="button"
            className="flex h-6 w-6 items-center justify-center rounded-md text-(--icon-muted) transition hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-default)"
            onClick={(event) => {
              event.stopPropagation();
              on_rename_entry(entry);
            }}
            title={t("home.rename")}
          >
            <Pencil className="h-3.5 w-3.5" />
          </button>

          <button
            type="button"
            className="flex h-6 w-6 items-center justify-center rounded-md text-(--icon-muted) transition hover:bg-[color:color-mix(in_srgb,var(--destructive)_8%,transparent)] hover:text-(--destructive)"
            onClick={(event) => {
              event.stopPropagation();
              on_delete_entry(entry);
            }}
            title={t("common.delete")}
          >
            <Trash2 className="h-3.5 w-3.5" />
          </button>
        </div>
      </div>

      {entry.is_dir && is_open && children.length > 0 ? (
        <div>
          {children.map((child) => (
            <WorkspaceFileTreeRow
              key={child.entry.path}
              node={child}
              active_path={active_path}
              focused_directory_path={focused_directory_path}
              depth={depth + 1}
              on_click_file={on_click_file}
              on_click_directory={on_click_directory}
              on_rename_entry={on_rename_entry}
              on_delete_entry={on_delete_entry}
              on_context_menu={on_context_menu}
            />
          ))}
        </div>
      ) : null}
    </div>
  );
});

export function WorkspaceFileTree({
  entries,
  active_path,
  focused_directory_path,
  on_click_file,
  on_click_directory,
  on_rename_entry,
  on_delete_entry,
  on_context_menu,
}: WorkspaceFileTreeProps) {
  const tree = useMemo(() => build_tree(entries), [entries]);

  return (
    <>
      {tree.map((node) => (
        <WorkspaceFileTreeRow
          key={node.entry.path}
          node={node}
          active_path={active_path}
          focused_directory_path={focused_directory_path}
          depth={0}
          on_click_file={on_click_file}
          on_click_directory={on_click_directory}
          on_rename_entry={on_rename_entry}
          on_delete_entry={on_delete_entry}
          on_context_menu={on_context_menu}
        />
      ))}
    </>
  );
}
