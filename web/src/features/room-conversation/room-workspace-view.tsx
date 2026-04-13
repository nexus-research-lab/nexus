"use client";

import { ChevronRight, File, FileCode, FileText, Folder, FolderOpen, FolderTree } from "lucide-react";
import { memo, useCallback, useMemo, useState } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { WorkspaceSurfaceView } from "@/shared/ui/workspace/workspace-surface-view";
import { useWorkspaceFilesStore } from "@/store/workspace-files";
import { Agent, WorkspaceFileEntry } from "@/types/agent";
import { cn } from "@/lib/utils";

interface RoomWorkspaceViewProps {
  active_workspace_path: string | null;
  agent_id: string;
  is_dm: boolean;
  room_members: Agent[];
  on_open_workspace_file: (path: string | null) => void;
}

// ── file icon ──────────────────────────────────────────────────────────────

function getFileIcon(name: string) {
  const ext = name.split(".").pop()?.toLowerCase();
  if (!ext) return FileText;
  if (["ts", "tsx", "js", "jsx", "py", "go", "rs", "java", "c", "cpp", "h"].includes(ext)) return FileCode;
  if (["md", "txt", "log", "json", "yaml", "yml", "toml", "xml", "csv"].includes(ext)) return FileText;
  return File;
}

// ── tree ───────────────────────────────────────────────────────────────────

interface TreeNode {
  entry: WorkspaceFileEntry;
  children: TreeNode[];
}

function buildTree(entries: WorkspaceFileEntry[]): TreeNode[] {
  const sorted = [...entries].sort((a, b) => {
    if (a.is_dir !== b.is_dir) return a.is_dir ? -1 : 1;
    return a.path.localeCompare(b.path);
  });

  const roots: TreeNode[] = [];
  const map = new Map<string, TreeNode>();

  for (const entry of sorted) {
    const node: TreeNode = { entry, children: [] };
    map.set(entry.path, node);
    const parent_path = entry.path.substring(0, entry.path.lastIndexOf("/"));
    const parent = map.get(parent_path);
    if (parent) {
      parent.children.push(node);
    } else {
      roots.push(node);
    }
  }

  return roots;
}

// ── tree row ───────────────────────────────────────────────────────────────

const TreeRow = memo(function TreeRow({
  node,
  active_path,
  depth,
  on_click_file,
}: {
  node: TreeNode;
  active_path: string | null;
  depth: number;
  on_click_file: (path: string) => void;
}) {
  const [open, set_open] = useState(depth === 0);
  const { entry, children } = node;
  const is_active = entry.path === active_path;
  const FileIcon = getFileIcon(entry.name);

  const handle_click = useCallback(() => {
    if (entry.is_dir) set_open((v) => !v);
    else on_click_file(entry.path);
  }, [entry, on_click_file]);

  return (
    <div>
      <button
        type="button"
        onClick={handle_click}
        className={cn(
          "group flex w-full items-center gap-1.5 rounded-lg px-2 py-[5px] text-left transition-colors duration-[var(--motion-duration-fast)]",
          is_active
            ? "bg-primary/10 text-primary"
            : "text-[color:var(--text-default)] hover:bg-[var(--surface-interactive-hover-background)] hover:text-[color:var(--text-strong)]",
        )}
        style={{ paddingLeft: `${8 + depth * 16}px` }}
      >
        {entry.is_dir ? (
          <ChevronRight className={cn("h-3.5 w-3.5 shrink-0 text-[color:var(--icon-muted)] transition-transform duration-[var(--motion-duration-fast)]", open && "rotate-90")} />
        ) : (
          <span className="w-3.5 shrink-0" />
        )}

        {entry.is_dir ? (
          open
            ? <FolderOpen className="h-4 w-4 shrink-0 text-[color:var(--warning)]" />
            : <Folder className="h-4 w-4 shrink-0 text-[color:var(--warning)]" />
        ) : (
          <FileIcon className={cn("h-4 w-4 shrink-0", is_active ? "text-primary" : "text-[color:var(--icon-muted)] group-hover:text-[color:var(--icon-default)]")} />
        )}

        <span className={cn("min-w-0 flex-1 truncate text-[12.5px]", entry.is_dir ? "font-medium" : "font-normal")}>
          {entry.name}
        </span>
      </button>

      {entry.is_dir && open && children.length > 0 && (
        <div>
          {children.map((child) => (
            <TreeRow
              key={child.entry.path}
              node={child}
              active_path={active_path}
              depth={depth + 1}
              on_click_file={on_click_file}
            />
          ))}
        </div>
      )}
    </div>
  );
});

// ── member switcher (room only) ────────────────────────────────────────────

function MemberSwitcher({
  members,
  selected_id,
  on_select,
}: {
  members: Agent[];
  selected_id: string;
  on_select: (id: string) => void;
}) {
  return (
    <div className="mb-4 flex flex-wrap gap-1.5">
      {members.map((m) => {
        const is_active = m.agent_id === selected_id;
        return (
          <button
            key={m.agent_id}
            type="button"
            onClick={() => on_select(m.agent_id)}
            className={cn(
              "inline-flex h-7 items-center gap-1.5 rounded-full border px-2.5 text-[11.5px] font-medium transition-all",
              is_active
                ? "border-primary/30 bg-primary/10 text-primary"
                : "text-[color:var(--text-default)] hover:text-[color:var(--text-strong)]",
            )}
            style={!is_active ? {
              background: "var(--card-default-background)",
              borderColor: "var(--card-default-border)",
            } : undefined}
          >
            <span className={cn(
              "flex h-4 w-4 shrink-0 items-center justify-center rounded-full text-[9px] font-bold",
              is_active ? "bg-primary/20 text-primary" : "bg-[var(--surface-interactive-hover-background)] text-[color:var(--text-default)]",
            )}>
              {m.name.slice(0, 1).toUpperCase()}
            </span>
            {m.name}
          </button>
        );
      })}
    </div>
  );
}

// ── main view ──────────────────────────────────────────────────────────────

export function RoomWorkspaceView({
  active_workspace_path,
  agent_id,
  is_dm,
  room_members,
  on_open_workspace_file,
}: RoomWorkspaceViewProps) {
  const { t } = useI18n();
  const [selected_agent_id, set_selected_agent_id] = useState(agent_id);
  const files_by_agent = useWorkspaceFilesStore((state) => state.files_by_agent);

  const view_agent_id = is_dm ? agent_id : selected_agent_id;
  const tree = useMemo(() => {
    const all_files = files_by_agent[view_agent_id];
    return buildTree(all_files ?? []);
  }, [files_by_agent, view_agent_id]);

  const handle_click_file = useCallback(
    (path: string) => on_open_workspace_file(path),
    [on_open_workspace_file],
  );

  return (
    <WorkspaceSurfaceView eyebrow={t("room.workspace")} title={t("room.workspace_title")}>
      {!is_dm && room_members.length > 1 && (
        <MemberSwitcher
          members={room_members}
          selected_id={selected_agent_id}
          on_select={set_selected_agent_id}
        />
      )}

      {tree.length > 0 ? (
        <div
          className="rounded-xl border py-1.5"
          style={{
            background: "var(--surface-panel-subtle-background)",
            borderColor: "var(--surface-panel-subtle-border)",
          }}
        >
          {tree.map((node) => (
            <TreeRow
              key={node.entry.path}
              node={node}
              active_path={active_workspace_path}
              depth={0}
              on_click_file={handle_click_file}
            />
          ))}
        </div>
      ) : (
        <div
          className="rounded-2xl border px-5 py-5 text-sm leading-7 text-[color:var(--text-muted)]"
          style={{
            background: "var(--surface-panel-subtle-background)",
            borderColor: "var(--surface-panel-subtle-border)",
          }}
        >
          <div className="mb-2 flex items-center gap-2 font-medium text-[color:var(--text-strong)]">
            <FolderTree className="h-4 w-4" />
            {t("room.no_files")}
          </div>
          {is_dm
            ? t("room.no_files_dm_hint")
            : t("room.no_files_room_hint")}
        </div>
      )}
    </WorkspaceSurfaceView>
  );
}
