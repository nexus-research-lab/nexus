"use client";

import { memo, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import {
  ChevronRight, File, FileText, Folder, FolderOpen, FolderTree,
  Image, FileArchive, FileSpreadsheet, FileType2, FileJson, FileCode2,
  Upload, LoaderCircle, Pencil, Trash2, FilePlus, FolderPlus,
  type LucideIcon,
} from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { WorkspaceSurfaceToolbarAction } from "@/shared/ui/workspace/workspace-surface-header";
import { WorkspaceSurfaceView } from "@/shared/ui/workspace/workspace-surface-view";
import { Agent, WorkspaceFileEntry } from "@/types/agent";
import { cn } from "@/lib/utils";
import { ConfirmDialog, PromptDialog } from "@/shared/ui/dialog/confirm-dialog";
import { DIALOG_POPOVER_CLASS_NAME } from "@/shared/ui/dialog/dialog-styles";
import {
  useRoomWorkspaceController,
} from "./use-room-workspace-controller";

interface RoomWorkspaceViewProps {
  active_workspace_path: string | null;
  agent_id: string;
  is_dm: boolean;
  room_members: Agent[];
  on_open_workspace_file: (path: string | null) => void;
}

// ── file icon ──────────────────────────────────────────────────────────────

const IMAGE_EXTENSIONS = new Set(["png", "jpg", "jpeg", "gif", "webp", "svg", "bmp", "ico", "avif"]);
const ARCHIVE_EXTENSIONS = new Set(["zip", "tar", "gz", "rar", "7z", "bz2", "xz"]);
const SPREADSHEET_EXTENSIONS = new Set(["xlsx", "xls", "csv", "ods"]);
const JSON_EXTENSIONS = new Set(["json", "jsonl"]);
const WEB_CODE_EXTENSIONS = new Set(["ts", "tsx", "js", "jsx", "mjs", "cjs", "html", "css", "scss", "less", "sass", "styl"]);
const SCRIPT_EXTENSIONS = new Set(["py", "go", "rs", "java", "c", "cpp", "h", "hpp", "cs", "swift", "kt", "dart", "php", "rb", "sh", "bash", "zsh", "sql", "r", "scala", "groovy", "lua", "pl", "perl"]);
const CONFIG_EXTENSIONS = new Set(["yaml", "yml", "toml", "ini", "conf", "env", "xml", "graphql", "proto"]);
const TEXT_EXTENSIONS = new Set(["md", "markdown", "txt", "log"]);
const DOCUMENT_EXTENSIONS = new Set(["pdf", "doc", "docx", "ppt", "pptx", "odt", "rtf"]);

interface FileVisual {
  Icon: LucideIcon;
  icon_class_name: string;
}

function get_file_extension(name: string): string | null {
  const lower_name = name.toLowerCase();
  if (lower_name === "dockerfile") return "docker";
  if (lower_name === "makefile") return "make";

  const last_dot_index = lower_name.lastIndexOf(".");
  if (last_dot_index <= 0 || last_dot_index === lower_name.length - 1) {
    return null;
  }
  return lower_name.slice(last_dot_index + 1);
}

function get_file_visual(name: string): FileVisual {
  const extension = get_file_extension(name);

  if (!extension) {
    return {
      Icon: FileText,
      icon_class_name: "text-slate-500",
    };
  }

  if (IMAGE_EXTENSIONS.has(extension)) {
    return {
      Icon: Image,
      icon_class_name: "text-fuchsia-500",
    };
  }

  if (ARCHIVE_EXTENSIONS.has(extension)) {
    return {
      Icon: FileArchive,
      icon_class_name: "text-violet-500",
    };
  }

  if (SPREADSHEET_EXTENSIONS.has(extension)) {
    return {
      Icon: FileSpreadsheet,
      icon_class_name: "text-emerald-600",
    };
  }

  if (JSON_EXTENSIONS.has(extension)) {
    return {
      Icon: FileJson,
      icon_class_name: "text-emerald-500",
    };
  }

  if (WEB_CODE_EXTENSIONS.has(extension)) {
    return {
      Icon: FileCode2,
      icon_class_name: "text-sky-500",
    };
  }

  if (SCRIPT_EXTENSIONS.has(extension)) {
    return {
      Icon: FileCode2,
      icon_class_name: extension === "py" ? "text-amber-500" : "text-blue-500",
    };
  }

  if (CONFIG_EXTENSIONS.has(extension)) {
    return {
      Icon: FileText,
      icon_class_name: "text-cyan-600",
    };
  }

  if (TEXT_EXTENSIONS.has(extension)) {
    return {
      Icon: FileText,
      icon_class_name: extension === "md" || extension === "markdown" ? "text-indigo-500" : "text-slate-500",
    };
  }

  if (DOCUMENT_EXTENSIONS.has(extension)) {
    return {
      Icon: FileType2,
      icon_class_name: extension === "pdf" ? "text-rose-500" : "text-orange-500",
    };
  }

  return {
    Icon: File,
    icon_class_name: "text-slate-500",
  };
}

// ── context menu ───────────────────────────────────────────────────────────

interface ContextMenuProps {
  position: { x: number; y: number } | null;
  entry: WorkspaceFileEntry | null;
  can_create_children: boolean;
  onUpload: () => void;
  onCreateFile: () => void;
  onCreateFolder: () => void;
  onRename: () => void;
  onDelete: () => void;
  onClose: () => void;
}

function ContextMenu({
  position,
  entry,
  can_create_children,
  onUpload,
  onCreateFile,
  onCreateFolder,
  onRename,
  onDelete,
  onClose,
}: ContextMenuProps) {
  const { t } = useI18n();
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
        {can_create_children ? (
          <>
            <button
              type="button"
              className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
              onClick={() => { onUpload(); onClose(); }}
            >
              <Upload className="h-4 w-4" />
              <span>{t("room.workspace_action_upload")}</span>
            </button>
            <button
              type="button"
              className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
              onClick={() => { onCreateFile(); onClose(); }}
            >
              <FilePlus className="h-4 w-4" />
              <span>{t("room.workspace_action_new_file")}</span>
            </button>
            <button
              type="button"
              className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
              onClick={() => { onCreateFolder(); onClose(); }}
            >
              <FolderPlus className="h-4 w-4" />
              <span>{t("room.workspace_action_new_folder")}</span>
            </button>
            {entry ? <div className="my-1 h-px bg-(--divider-subtle-color)" /> : null}
          </>
        ) : null}

        {entry && (
          <>
            <button
              type="button"
              className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
              onClick={() => { onRename(); onClose(); }}
            >
              <Pencil className="h-4 w-4" />
              <span>{t("home.rename")}</span>
            </button>
            <button
              type="button"
              className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-destructive"
              onClick={() => { onDelete(); onClose(); }}
            >
              <Trash2 className="h-4 w-4" />
              <span>{t("common.delete")}</span>
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
  focused_directory_path: string | null;
  depth: number;
  on_click_file: (path: string) => void;
  on_click_directory: (path: string) => void;
  on_rename_entry: (entry: WorkspaceFileEntry) => void;
  on_delete_entry: (entry: WorkspaceFileEntry) => void;
  on_context_menu: (e: React.MouseEvent, entry: WorkspaceFileEntry) => void;
}

const TreeRow = memo(function TreeRow({
  node,
  active_path,
  focused_directory_path,
  depth,
  on_click_file,
  on_click_directory,
  on_rename_entry,
  on_delete_entry,
  on_context_menu,
}: TreeRowProps) {
  const { t } = useI18n();
  const { entry, children } = node;
  const is_active = entry.path === active_path;
  const is_directory_target = entry.is_dir && entry.path === focused_directory_path;
  const { Icon: FileIcon, icon_class_name } = get_file_visual(entry.name);
  const [is_open, setIsOpen] = useState(depth === 0);

  const handle_click = useCallback(() => {
    if (entry.is_dir) {
      setIsOpen((value: boolean) => !value);
      on_click_directory(entry.path);
    } else {
      on_click_file(entry.path);
    }
  }, [entry, on_click_directory, on_click_file]);

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
          "hover:bg-(--surface-interactive-hover-background)",
          is_active || is_directory_target
            ? "bg-primary/10 text-primary"
            : "text-(--text-default)",
        )}
        style={{ paddingLeft: `${8 + depth * 14}px` }}
        onClick={handle_click}
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
          <FileIcon className={cn("h-4 w-4 shrink-0", is_active ? "text-primary" : icon_class_name)} />
        )}

        <span className={cn(
          "min-w-0 flex-1 truncate text-[14px] leading-6",
          entry.is_dir ? "font-medium" : "font-normal",
        )}>
          {entry.name}
        </span>

        <div className={cn(
          "ml-auto flex shrink-0 items-center gap-0.5 transition-opacity",
          is_active || is_directory_target ? "opacity-100" : "opacity-0 group-hover:opacity-100",
        )}>
          <button
            type="button"
            className="flex h-6 w-6 items-center justify-center rounded-md text-(--icon-muted) transition hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-default)"
            onClick={(e) => {
              e.stopPropagation();
              on_rename_entry(entry);
            }}
            title={t("home.rename")}
          >
            <Pencil className="h-3.5 w-3.5" />
          </button>

          <button
            type="button"
            className="flex h-6 w-6 items-center justify-center rounded-md text-(--icon-muted) transition hover:bg-[color:color-mix(in_srgb,var(--destructive)_8%,transparent)] hover:text-(--destructive)"
            onClick={(e) => {
              e.stopPropagation();
              on_delete_entry(entry);
            }}
            title={t("common.delete")}
          >
            <Trash2 className="h-3.5 w-3.5" />
          </button>
        </div>
      </div>

      {entry.is_dir && is_open && children.length > 0 && (
        <div>
          {children.map((child) => (
            <TreeRow
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
  const file_input_ref = useRef<HTMLInputElement>(null);
  const {
    files,
    selected_agent_id,
    set_selected_agent_id,
    is_uploading,
    is_loading_files,
    error_message,
    clear_error_message,
    context_menu,
    prompt_state,
    delete_target,
    focused_directory_path,
    current_directory_label,
    handle_click_file,
    handle_click_directory,
    handle_upload_click,
    handle_file_select,
    open_create_prompt,
    open_rename_prompt,
    handle_prompt_confirm,
    handle_confirm_delete,
    handle_context_menu,
    handle_root_context_menu,
    close_context_menu,
    set_delete_target,
    set_prompt_state,
  } = useRoomWorkspaceController({
    active_workspace_path,
    agent_id,
    is_dm,
    on_open_workspace_file,
    file_input_ref,
  });

  const tree = useMemo(() => buildTree(files), [files]);

  return (
    <>
      <input
        ref={file_input_ref}
        type="file"
        className="hidden"
        multiple
        onChange={handle_file_select}
      />

      <WorkspaceSurfaceView
        body_class_name="px-4 py-5 sm:px-5 xl:px-6"
        content_class_name="space-y-4"
        eyebrow={t("room.workspace")}
        max_width_class_name="max-w-[820px]"
        title={t("room.workspace_title")}
      >
        {!is_dm && room_members.length > 1 && (
          <MemberSwitcher
            members={room_members}
            selected_id={selected_agent_id}
            on_select={set_selected_agent_id}
          />
        )}

        <div className="mx-auto flex w-full max-w-[820px] items-center justify-between gap-3">
          <div className="inline-flex min-w-0 items-center gap-2 rounded-full border border-(--divider-subtle-color) px-3 py-1.5 text-[12px] text-(--text-default)">
            {focused_directory_path ? (
              <FolderOpen className="h-3.5 w-3.5 shrink-0 text-[var(--accent)]" />
            ) : (
              <FolderTree className="h-3.5 w-3.5 shrink-0 text-(--icon-muted)" />
            )}
            <span className="truncate font-medium text-(--text-strong)">{current_directory_label}</span>
          </div>

          <div className="flex shrink-0 items-center gap-3">
            <WorkspaceSurfaceToolbarAction onClick={() => handle_upload_click()} disabled={is_uploading} tone="primary">
              {is_uploading ? (
                <LoaderCircle className="h-3.5 w-3.5 animate-spin" />
              ) : (
                <Upload className="h-3.5 w-3.5" />
              )}
              {t(is_uploading ? "room.workspace_uploading" : "room.workspace_action_upload")}
            </WorkspaceSurfaceToolbarAction>

            <WorkspaceSurfaceToolbarAction onClick={() => open_create_prompt("directory")}>
              <FolderPlus className="h-3.5 w-3.5" />
              {t("room.workspace_action_new_folder")}
            </WorkspaceSurfaceToolbarAction>

            <WorkspaceSurfaceToolbarAction onClick={() => open_create_prompt("file")}>
              <FilePlus className="h-3.5 w-3.5" />
              {t("room.workspace_action_new_file")}
            </WorkspaceSurfaceToolbarAction>
          </div>
        </div>

        {error_message ? (
          <div className="mx-auto flex w-full max-w-[820px] items-center justify-between rounded-2xl border border-destructive/20 bg-destructive/6 px-4 py-3 text-sm text-destructive">
            <span className="min-w-0 flex-1 truncate">{error_message}</span>
            <button
              type="button"
              className="ml-3 shrink-0 rounded-md px-2 py-1 text-xs font-medium transition hover:bg-destructive/10"
              onClick={clear_error_message}
            >
              {t("common.close")}
            </button>
          </div>
        ) : null}

        {tree.length > 0 ? (
          <div className="mx-auto w-full max-w-[820px] py-1" onContextMenu={handle_root_context_menu}>
            {tree.map((node) => (
              <TreeRow
                key={node.entry.path}
                node={node}
                active_path={active_workspace_path}
                focused_directory_path={focused_directory_path}
                depth={0}
                on_click_file={handle_click_file}
                on_click_directory={handle_click_directory}
                on_rename_entry={open_rename_prompt}
                on_delete_entry={set_delete_target}
                on_context_menu={handle_context_menu}
              />
            ))}
          </div>
        ) : is_loading_files ? (
          <div className="mx-auto flex w-full max-w-[820px] items-center justify-center py-12 text-(--text-soft)">
            <LoaderCircle className="h-4 w-4 animate-spin" />
          </div>
        ) : (
          <div
            className="mx-auto w-full max-w-[820px] rounded-[24px] border border-(--divider-subtle-color) px-6 py-10 text-center"
            onContextMenu={handle_root_context_menu}
          >
            <div className="mx-auto flex h-11 w-11 items-center justify-center rounded-full border border-(--surface-avatar-border) bg-(--surface-avatar-background) text-(--icon-default) shadow-(--surface-avatar-shadow)">
              <FolderTree className="h-4 w-4" />
            </div>
            <p className="mt-4 text-[15px] font-semibold text-(--text-strong)">
              {t("room.no_files")}
            </p>
            <p className="mt-1 text-[12px] leading-6 text-(--text-soft)">
              {t("room.workspace_empty_description")}
            </p>
          </div>
        )}
      </WorkspaceSurfaceView>

      {/* 上下文菜单 */}
      <ContextMenu
        position={context_menu.position}
        entry={context_menu.entry}
        can_create_children={context_menu.entry === null || context_menu.entry.is_dir}
        onUpload={() => handle_upload_click(context_menu.entry?.is_dir ? context_menu.entry.path : null)}
        onCreateFile={() => open_create_prompt("file", context_menu.entry?.is_dir ? context_menu.entry.path : null)}
        onCreateFolder={() => open_create_prompt("directory", context_menu.entry?.is_dir ? context_menu.entry.path : null)}
        onRename={() => { if (context_menu.entry) open_rename_prompt(context_menu.entry); }}
        onDelete={() => { if (context_menu.entry) set_delete_target(context_menu.entry); }}
        onClose={close_context_menu}
      />

      <PromptDialog
        is_open={prompt_state !== null}
        title={
          prompt_state?.mode === "create-file"
            ? t("room.workspace_create_file_title")
            : prompt_state?.mode === "create-directory"
              ? t("room.workspace_create_folder_title")
              : t("room.workspace_rename_title")
        }
        placeholder={
          prompt_state?.mode === "create-file"
            ? t("room.workspace_create_file_placeholder")
            : prompt_state?.mode === "create-directory"
              ? t("room.workspace_create_folder_placeholder")
              : t("room.workspace_rename_placeholder")
        }
        default_value={prompt_state?.default_value ?? ""}
        on_confirm={handle_prompt_confirm}
        on_cancel={() => set_prompt_state(null)}
      />

      <ConfirmDialog
        is_open={delete_target !== null}
        title={t("room.workspace_delete_title")}
        message={t("room.workspace_delete_message", { name: delete_target?.name ?? "" })}
        confirm_text={t("common.delete")}
        cancel_text={t("common.cancel")}
        on_confirm={handle_confirm_delete}
        on_cancel={() => set_delete_target(null)}
        variant="danger"
      />
    </>
  );
}
