"use client";

import { memo, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import {
  ChevronRight, File, FileCode, FileText, Folder, FolderOpen, FolderTree,
  Image, FileArchive, FileSpreadsheet, FileType2, FileJson, FileCode2,
  Upload, LoaderCircle, Pencil, Trash2, FilePlus, FolderPlus,
} from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { WorkspaceSurfaceHeader, WorkspaceSurfaceToolbarAction } from "@/shared/ui/workspace/workspace-surface-header";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/workspace-surface-scaffold";
import { useWorkspaceFilesStore } from "@/store/workspace-files";
import { Agent, WorkspaceFileEntry } from "@/types/agent";
import { cn } from "@/lib/utils";
import { uploadWorkspaceFileApi, createWorkspaceEntryApi, deleteWorkspaceEntryApi, renameWorkspaceEntryApi } from "@/lib/agent-manage-api";
import { DIALOG_POPOVER_CLASS_NAME } from "@/shared/ui/dialog/dialog-styles";

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

  if (["png", "jpg", "jpeg", "gif", "webp", "svg", "bmp", "ico", "avif"].includes(ext)) return Image;
  if (["zip", "tar", "gz", "rar", "7z", "bz2", "xz"].includes(ext)) return FileArchive;
  if (["xlsx", "xls", "csv", "ods"].includes(ext)) return FileSpreadsheet;
  if (["json", "jsonl"].includes(ext)) return FileJson;
  if (["ts", "tsx", "js", "jsx", "mjs", "cjs", "py", "go", "rs", "java", "c", "cpp", "h", "hpp", "cs", "swift", "kt", "dart", "php", "rb", "sh", "bash", "zsh", "sql", "r", "scala", "groovy", "lua", "pl", "perl"].includes(ext)) return FileCode2;
  if (["md", "markdown", "txt", "log", "yaml", "yml", "toml", "ini", "conf", "env", "xml", "html", "css", "scss", "less", "sass", "styl", "graphql", "proto", "dockerfile", "makefile", "cmake", "gradle", "pom", "manifest"].includes(ext)) return FileText;
  if (["pdf", "doc", "docx", "ppt", "pptx", "odt", "rtf"].includes(ext)) return FileType2;

  return File;
}

// ── context menu ───────────────────────────────────────────────────────────

interface ContextMenuProps {
  position: { x: number; y: number } | null;
  entry: WorkspaceFileEntry | null;
  onCreateFile: () => void;
  onCreateFolder: () => void;
  onRename: () => void;
  onDelete: () => void;
  onClose: () => void;
}

function ContextMenu({
  position,
  entry,
  onCreateFile,
  onCreateFolder,
  onRename,
  onDelete,
  onClose,
}: ContextMenuProps) {
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        onClose();
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
    };
  }, [onClose]);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        onClose();
      }
    };
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [onClose]);

  if (!position) return null;

  const menu = (
    <div
      ref={menuRef}
      className={DIALOG_POPOVER_CLASS_NAME}
      style={{
        top: `${position.y}px`,
        left: `${position.x}px`,
        minWidth: "180px",
      }}
    >
      <div className="py-1">
        {entry && (
          <>
            <button
              type="button"
              className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
              onClick={() => { onRename(); onClose(); }}
            >
              <Pencil className="h-4 w-4" />
              <span>重命名</span>
            </button>
            <button
              type="button"
              className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-destructive"
              onClick={() => { onDelete(); onClose(); }}
            >
              <Trash2 className="h-4 w-4" />
              <span>删除</span>
            </button>
          </>
        )}
        {!entry && (
          <>
            <button
              type="button"
              className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
              onClick={() => { onCreateFile(); onClose(); }}
            >
              <FilePlus className="h-4 w-4" />
              <span>新建文件</span>
            </button>
            <button
              type="button"
              className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
              onClick={() => { onCreateFolder(); onClose(); }}
            >
              <FolderPlus className="h-4 w-4" />
              <span>新建文件夹</span>
            </button>
          </>
        )}
      </div>
    </div>
  );

  return createPortal(menu, document.body);
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

interface TreeRowProps {
  node: TreeNode;
  active_path: string | null;
  depth: number;
  on_click_file: (path: string) => void;
  on_context_menu: (e: React.MouseEvent, entry: WorkspaceFileEntry) => void;
  on_start_rename: (entry: WorkspaceFileEntry) => void;
  renaming_entry: WorkspaceFileEntry | null;
  on_rename_complete: (entry: WorkspaceFileEntry, new_name: string) => void;
}

const TreeRow = memo(function TreeRow({
  node,
  active_path,
  depth,
  on_click_file,
  on_context_menu,
  on_start_rename,
  renaming_entry,
  on_rename_complete,
}: TreeRowProps) {
  const { entry, children } = node;
  const is_active = entry.path === active_path;
  const is_renaming = renaming_entry?.path === entry.path;
  const FileIcon = getFileIcon(entry.name);
  const inputRef = useRef<HTMLInputElement>(null);
  const [is_open, setIsOpen] = useState(depth === 0 && !is_renaming);

  useEffect(() => {
    if (is_renaming && inputRef.current) {
      inputRef.current.focus();
      inputRef.current.select();
    }
  }, [is_renaming]);

  const handle_click = useCallback(() => {
    if (is_renaming) return;
    if (entry.is_dir) {
      setIsOpen((v) => !v);
    } else {
      on_click_file(entry.path);
    }
  }, [entry, is_renaming, on_click_file]);

  const handle_double_click = useCallback((e: React.MouseEvent) => {
    if (!entry.is_dir) {
      on_start_rename(entry);
      e.preventDefault();
      e.stopPropagation();
    }
  }, [entry, on_start_rename]);

  const handle_rename_key_down = useCallback((e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      on_rename_complete(entry, e.currentTarget.value);
      e.preventDefault();
    } else if (e.key === "Escape") {
      on_rename_complete(entry, entry.name);
      e.preventDefault();
    }
  }, [entry, on_rename_complete]);

  const handle_context_menu = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    on_context_menu(e, entry);
  }, [entry, on_context_menu]);

  return (
    <div>
      <div
        className={cn(
          "group flex w-full items-center gap-1.5 rounded-lg px-2 py-[5px] text-left transition-colors",
          is_renaming ? "bg-primary/10" : "hover:bg-(--surface-interactive-hover-background)",
          is_active
            ? "bg-primary/10 text-primary"
            : "text-(--text-default)",
        )}
        style={{ paddingLeft: `${8 + depth * 14}px` }}
        onClick={handle_click}
        onDoubleClick={handle_double_click}
        onContextMenu={handle_context_menu}
      >
        {entry.is_dir ? (
          <ChevronRight className={cn("h-3.5 w-3.5 shrink-0 text-(--icon-muted) transition-transform", is_open && "rotate-90")} />
        ) : (
          <span className="w-3.5 shrink-0" />
        )}

        {entry.is_dir ? (
          is_open ? (
            <FolderOpen className="h-4 w-4 shrink-0 text-[var(--accent)]" />
          ) : (
            <Folder className="h-4 w-4 shrink-0 text-[var(--accent)]" />
          )
        ) : (
          <FileIcon className={cn("h-4 w-4 shrink-0", is_active ? "text-primary" : "text-(--icon-muted)")} />
        )}

        {is_renaming ? (
          <input
            ref={inputRef}
            type="text"
            defaultValue={entry.name}
            className="flex-1 bg-transparent text-sm text-(--text-default) outline-none"
            onKeyDown={handle_rename_key_down}
            onBlur={(e) => on_rename_complete(entry, e.currentTarget.value)}
            onClick={(e) => e.stopPropagation()}
          />
        ) : (
          <span className={cn(
            "min-w-0 flex-1 truncate text-[13px]",
            entry.is_dir ? "font-medium" : "font-normal",
          )}>
            {entry.name}
          </span>
        )}
      </div>

      {entry.is_dir && is_open && children.length > 0 && (
        <div>
          {children.map((child) => (
            <TreeRow
              key={child.entry.path}
              node={child}
              active_path={active_path}
              depth={depth + 1}
              on_click_file={on_click_file}
              on_context_menu={on_context_menu}
              on_start_rename={on_start_rename}
              renaming_entry={renaming_entry}
              on_rename_complete={on_rename_complete}
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
                : "text-(--text-default) hover:text-(--text-strong)",
            )}
            style={!is_active ? {
              background: "var(--card-default-background)",
              borderColor: "var(--card-default-border)",
            } : undefined}
          >
            <span className={cn(
              "flex h-4 w-4 shrink-0 items-center justify-center rounded-full text-[9px] font-bold",
              is_active ? "bg-primary/20 text-primary" : "bg-(--surface-interactive-hover-background) text-(--text-default)",
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
  const [is_uploading, setIsUploading] = useState(false);
  const [context_menu, setContextMenu] = useState<{ position: { x: number; y: number } | null; entry: WorkspaceFileEntry | null }>({
    position: null,
    entry: null,
  });

  const file_input_ref = useRef<HTMLInputElement>(null);
  const files_by_agent = useWorkspaceFilesStore((state) => state.files_by_agent);
  const refresh_files = useWorkspaceFilesStore((state) => state.refresh_files);

  const view_agent_id = is_dm ? agent_id : selected_agent_id;
  const tree = useMemo(() => {
    const all_files = files_by_agent[view_agent_id] || [];
    return buildTree(all_files);
  }, [files_by_agent, view_agent_id]);

  const [renaming_entry, setRenamingEntry] = useState<WorkspaceFileEntry | null>(null);

  const handle_click_file = useCallback(
    (path: string) => {
      on_open_workspace_file(path);
    },
    [on_open_workspace_file],
  );

  const handle_upload_click = useCallback(() => {
    file_input_ref.current?.click();
  }, []);

  const handle_file_select = useCallback(async (event: React.ChangeEvent<HTMLInputElement>) => {
    const files = event.target.files;
    if (!files || files.length === 0) return;

    setIsUploading(true);
    try {
      for (const file of Array.from(files)) {
        await uploadWorkspaceFileApi(view_agent_id, file);
      }
      await refresh_files(view_agent_id);
    } catch (error) {
      console.error("上传文件失败:", error);
      alert(error instanceof Error ? error.message : "上传文件失败");
    } finally {
      setIsUploading(false);
      if (file_input_ref.current) {
        file_input_ref.current.value = "";
      }
    }
  }, [view_agent_id, refresh_files]);

  const handle_create_entry = useCallback(async (type: "file" | "directory") => {
    const placeholder = type === "file" ? "untitled.txt" : "new-folder";
    const new_name = window.prompt(
      type === "file" ? "输入新文件名" : "输入新文件夹名",
      placeholder,
    );
    if (!new_name) return;

    try {
      await createWorkspaceEntryApi(view_agent_id, new_name, type);
      await refresh_files(view_agent_id);
    } catch (error) {
      console.error("创建失败:", error);
      alert(error instanceof Error ? error.message : "创建失败");
    }
  }, [view_agent_id, refresh_files]);

  const handle_rename_entry = useCallback(async (entry: WorkspaceFileEntry, new_name: string) => {
    if (!new_name || new_name === entry.name) {
      setRenamingEntry(null);
      return;
    }

    try {
      await renameWorkspaceEntryApi(view_agent_id, entry.path, new_name);
      await refresh_files(view_agent_id);
      if (active_workspace_path === entry.path) {
        on_open_workspace_file(new_name);
      }
    } catch (error) {
      console.error("重命名失败:", error);
      alert(error instanceof Error ? error.message : "重命名失败");
    }
    setRenamingEntry(null);
  }, [active_workspace_path, view_agent_id, refresh_files, on_open_workspace_file]);

  const handle_delete_entry = useCallback(async (entry: WorkspaceFileEntry) => {
    const confirmed = window.confirm(`确认删除 "${entry.name}" 吗？`);
    if (!confirmed) return;

    try {
      await deleteWorkspaceEntryApi(view_agent_id, entry.path);
      await refresh_files(view_agent_id);
      if (active_workspace_path?.startsWith(entry.path)) {
        on_open_workspace_file(null);
      }
    } catch (error) {
      console.error("删除失败:", error);
      alert(error instanceof Error ? error.message : "删除失败");
    }
  }, [active_workspace_path, view_agent_id, refresh_files, on_open_workspace_file]);

  const handle_context_menu = useCallback((
    e: React.MouseEvent,
    entry: WorkspaceFileEntry,
  ) => {
    const menuWidth = 180;
    const menuHeight = entry.is_dir ? 80 : 130;
    const x = Math.min(e.clientX, window.innerWidth - menuWidth);
    const y = Math.min(e.clientY, window.innerHeight - menuHeight);

    setContextMenu({
      position: { x, y },
      entry,
    });
  }, []);

  const close_context_menu = useCallback(() => {
    setContextMenu({ position: null, entry: null });
  }, []);

  const upload_button = (
    <>
      <input
        ref={file_input_ref}
        type="file"
        className="hidden"
        multiple
        onChange={handle_file_select}
      />
      <WorkspaceSurfaceToolbarAction onClick={handle_upload_click} disabled={is_uploading}>
        {is_uploading ? (
          <LoaderCircle className="h-3.5 w-3.5 animate-spin" />
        ) : (
          <Upload className="h-3.5 w-3.5" />
        )}
      </WorkspaceSurfaceToolbarAction>

      <WorkspaceSurfaceToolbarAction onClick={() => handle_create_entry("directory")}>
        <FolderPlus className="h-3.5 w-3.5" />
      </WorkspaceSurfaceToolbarAction>

      <WorkspaceSurfaceToolbarAction onClick={() => handle_create_entry("file")}>
        <FilePlus className="h-3.5 w-3.5" />
      </WorkspaceSurfaceToolbarAction>
    </>
  );

  return (
    <>
      <WorkspaceSurfaceScaffold
        header={(
          <WorkspaceSurfaceHeader
            density="compact"
            leading={<FolderTree className="h-4 w-4" />}
            title={t("room.workspace_title")}
            trailing={upload_button}
          />
        )}
        body_scrollable
        stable_gutter
      >
        {!is_dm && room_members.length > 1 && (
          <MemberSwitcher
            members={room_members}
            selected_id={selected_agent_id}
            on_select={set_selected_agent_id}
          />
        )}

        {tree.length > 0 ? (
          <div className="rounded-xl border py-1" style={{
            background: "var(--surface-panel-subtle-background)",
            borderColor: "var(--surface-panel-subtle-border)",
          }}>
            {tree.map((node) => (
              <TreeRow
                key={node.entry.path}
                node={node}
                active_path={active_workspace_path}
                depth={0}
                on_click_file={handle_click_file}
                on_context_menu={handle_context_menu}
                on_start_rename={setRenamingEntry}
                renaming_entry={renaming_entry}
                on_rename_complete={handle_rename_entry}
              />
            ))}
          </div>
        ) : (
          <div
            className="rounded-2xl border px-5 py-6 text-sm leading-7 text-(--text-muted)"
            style={{
              background: "var(--surface-panel-subtle-background)",
              borderColor: "var(--surface-panel-subtle-border)",
            }}
          >
            <div className="mb-2 flex items-center gap-2 font-medium text-(--text-strong)">
              <FolderTree className="h-4 w-4" />
              {t("room.no_files")}
            </div>
            <div className="text-[13px] text-(--text-soft)">
              <p className="mb-1">点击上方按钮创建内容：</p>
              <ul className="ml-4 space-y-1 text-(--text-muted)">
                <li>• 新建文件夹</li>
                <li>• 新建文件</li>
                <li>• 上传文件</li>
              </ul>
            </div>
          </div>
        )}
      </WorkspaceSurfaceScaffold>

      {/* 上下文菜单 */}
      <ContextMenu
        position={context_menu.position}
        entry={context_menu.entry}
        onCreateFile={() => handle_create_entry("file")}
        onCreateFolder={() => handle_create_entry("directory")}
        onRename={() => { if (context_menu.entry) setRenamingEntry(context_menu.entry); }}
        onDelete={() => { if (context_menu.entry) handle_delete_entry(context_menu.entry); }}
        onClose={close_context_menu}
      />
    </>
  );
}
