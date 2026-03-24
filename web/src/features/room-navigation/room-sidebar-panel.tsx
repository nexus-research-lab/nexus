"use client";

import { ReactNode, useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  ChevronDown,
  ChevronRight,
  File,
  FileCode2,
  FileImage,
  FileJson,
  FileSpreadsheet,
  FileText,
  FileType2,
  Folder,
  Pencil,
  Trash2,
} from "lucide-react";

import {
  createWorkspaceEntryApi,
  deleteWorkspaceEntryApi,
  getWorkspaceFilesApi,
  renameWorkspaceEntryApi,
} from "@/lib/agent-manage-api";
import { HOME_WORKSPACE_SIDEBAR_WIDTH_CLASS } from "@/lib/home-layout";
import { cn, formatRelativeTime, truncate } from "@/lib/utils";
import { ConfirmDialog, PromptDialog } from "@/shared/ui/confirm-dialog";
import { useWorkspaceFilesStore } from "@/store/workspace-files";
import { useWorkspaceLiveStore } from "@/store/workspace-live";
import { Agent, WorkspaceFileEntry } from "@/types/agent";
import { Conversation } from "@/types/conversation";

import { RoomContextSection } from "../room-context/room-context-section";
import { RoomMembersSection } from "../room-members/room-members-section";
import { RoomConversationsSection } from "./room-conversations-section";
import { RoomSidebarHeader } from "./room-sidebar-header";

interface FileTreeNode {
  entry: WorkspaceFileEntry;
  children: FileTreeNode[];
}

export interface RoomSidebarPanelProps {
  agents: Agent[];
  agent: Agent;
  current_agent_id: string | null;
  recent_agents: Agent[];
  conversations: Conversation[];
  current_conversation_id: string | null;
  active_workspace_path: string | null;
  on_select_agent: (agent_id: string) => void;
  on_open_directory: () => void;
  on_create_agent: () => void;
  on_select_conversation: (conversation_id: string) => void;
  on_create_conversation: () => void;
  on_delete_conversation: (conversation_id: string) => void;
  on_open_workspace_file: (path: string | null) => void;
}

function getFileIcon(name: string) {
  const extension = name.includes(".") ? name.split(".").pop()?.toLowerCase() ?? "" : "";

  if (["md", "mdx"].includes(extension)) {
    return FileText;
  }
  if (["txt", "log", "rtf"].includes(extension)) {
    return FileType2;
  }
  if (["pdf"].includes(extension)) {
    return File;
  }
  if (["doc", "docx", "odt", "pages"].includes(extension)) {
    return FileText;
  }
  if (["xls", "xlsx", "csv", "tsv"].includes(extension)) {
    return FileSpreadsheet;
  }
  if (["json", "jsonl"].includes(extension)) {
    return FileJson;
  }
  if (["png", "jpg", "jpeg", "gif", "webp", "svg", "ico"].includes(extension)) {
    return FileImage;
  }
  if (
    [
      "js",
      "jsx",
      "ts",
      "tsx",
      "py",
      "java",
      "go",
      "rs",
      "rb",
      "php",
      "c",
      "cc",
      "cpp",
      "h",
      "hpp",
      "css",
      "scss",
      "sass",
      "html",
      "vue",
      "sh",
      "bash",
      "zsh",
      "yml",
      "yaml",
      "toml",
      "ini",
      "env",
      "sql",
    ].includes(extension)
  ) {
    return FileCode2;
  }

  return File;
}

export function RoomSidebarPanel({
  agents,
  agent,
  current_agent_id,
  recent_agents,
  conversations,
  current_conversation_id,
  active_workspace_path,
  on_select_agent,
  on_open_directory,
  on_create_agent,
  on_select_conversation,
  on_create_conversation,
  on_delete_conversation,
  on_open_workspace_file,
}: RoomSidebarPanelProps) {
  const [files, setFiles] = useState<WorkspaceFileEntry[]>([]);
  const [is_loading_files, setIsLoadingFiles] = useState(false);
  const [filesystem_error, setFilesystemError] = useState<string | null>(null);
  const [expanded_directories, setExpandedDirectories] = useState<Record<string, boolean>>({});
  const [show_file_explorer, setShowFileExplorer] = useState(false);
  const [prompt_dialog, setPromptDialog] = useState<{
    is_open: boolean;
    type: "create" | "rename";
    entry_type: "file" | "directory";
    entry?: WorkspaceFileEntry;
  }>({ is_open: false, type: "create", entry_type: "file" });
  const [confirm_dialog, setConfirmDialog] = useState<{
    is_open: boolean;
    entry?: WorkspaceFileEntry;
  }>({ is_open: false });

  const file_states = useWorkspaceLiveStore((state) => state.fileStates);
  const recent_events = useWorkspaceLiveStore((state) => state.recentEvents);
  const mark_file_seen = useWorkspaceLiveStore((state) => state.markFileSeen);
  const set_workspace_files = useWorkspaceFilesStore((state) => state.setFiles);
  const row_refs = useRef<Record<string, HTMLDivElement | null>>({});

  const visible_files = useMemo(() => files.filter((file) => !file.is_dir), [files]);
  const memory_files = useMemo(
    () => visible_files.filter((file) => /memory|context|summary|skill/i.test(file.path)),
    [visible_files],
  );
  const contextual_files = useMemo(() => {
    const quick = [...memory_files];
    const active_file = active_workspace_path
      ? visible_files.find((file) => file.path === active_workspace_path)
      : null;

    if (active_file && !quick.some((file) => file.path === active_file.path)) {
      quick.unshift(active_file);
    }

    return quick.slice(0, 4);
  }, [active_workspace_path, memory_files, visible_files]);

  const directory_tree = useMemo(() => {
    const node_map = new Map<string, FileTreeNode>();
    const roots: FileTreeNode[] = [];

    [...files]
      .sort((left, right) => left.path.localeCompare(right.path, "zh-CN"))
      .forEach((entry) => {
        node_map.set(entry.path, { entry, children: [] });
      });

    [...files]
      .sort((left, right) => left.path.localeCompare(right.path, "zh-CN"))
      .forEach((entry) => {
        const node = node_map.get(entry.path);
        if (!node) {
          return;
        }

        const parent_path = entry.path.includes("/") ? entry.path.split("/").slice(0, -1).join("/") : null;
        const parent = parent_path ? node_map.get(parent_path) : null;

        if (parent) {
          parent.children.push(node);
        } else {
          roots.push(node);
        }
      });

    const sortNodes = (nodes: FileTreeNode[]) => {
      nodes.sort((left, right) => {
        if (left.entry.is_dir !== right.entry.is_dir) {
          return left.entry.is_dir ? -1 : 1;
        }
        return left.entry.name.localeCompare(right.entry.name, "zh-CN");
      });
      nodes.forEach((node) => sortNodes(node.children));
    };

    sortNodes(roots);
    return roots;
  }, [files]);

  const selected_conversation =
    conversations.find((conversation) => conversation.session_key === current_conversation_id) ?? null;
  const latest_conversation = selected_conversation ?? conversations[0] ?? null;
  const active_room_title =
    selected_conversation?.title?.trim() || latest_conversation?.title?.trim() || "未命名 room";

  const load_files = useCallback(async () => {
    setIsLoadingFiles(true);
    setFilesystemError(null);
    try {
      const next_files = await getWorkspaceFilesApi(agent.agent_id);
      setFiles(next_files);
      set_workspace_files(agent.agent_id, next_files);
    } catch (load_error) {
      setFilesystemError(load_error instanceof Error ? load_error.message : "加载 workspace 失败");
    } finally {
      setIsLoadingFiles(false);
    }
  }, [agent.agent_id, set_workspace_files]);

  useEffect(() => {
    void load_files();
  }, [load_files]);

  const latest_agent_event = useMemo(
    () => recent_events.find((item) => item.agent_id === agent.agent_id) ?? null,
    [agent.agent_id, recent_events],
  );
  const known_file_paths = useMemo(() => new Set(files.map((entry) => entry.path)), [files]);
  const visible_agents = useMemo(() => {
    const seen = new Set<string>();
    const merged = [agent, ...recent_agents, ...agents];
    return merged.filter((item) => {
      if (!item?.agent_id || seen.has(item.agent_id)) {
        return false;
      }
      seen.add(item.agent_id);
      return true;
    }).slice(0, 5);
  }, [agent, agents, recent_agents]);

  useEffect(() => {
    if (!latest_agent_event || latest_agent_event.event_type !== "file_write_end") {
      return;
    }

    const path_parts = latest_agent_event.path.split("/").slice(0, -1);
    if (path_parts.length > 0) {
      setExpandedDirectories((current) => {
        const next_state = { ...current };
        path_parts.forEach((_, index) => {
          const parent_path = path_parts.slice(0, index + 1).join("/");
          next_state[parent_path] = true;
        });
        return next_state;
      });
    }

    if (known_file_paths.has(latest_agent_event.path)) {
      return;
    }

    const timer = window.setTimeout(() => {
      void load_files();
    }, 240);

    return () => window.clearTimeout(timer);
  }, [known_file_paths, latest_agent_event?.id, latest_agent_event?.event_type, latest_agent_event?.path, load_files]);

  useEffect(() => {
    setExpandedDirectories((current) => {
      const next_state = { ...current };
      files
        .filter((entry) => entry.is_dir)
        .forEach((entry) => {
          if (next_state[entry.path] === undefined) {
            next_state[entry.path] = true;
          }
        });
      return next_state;
    });
  }, [files]);

  useEffect(() => {
    if (!active_workspace_path) {
      return;
    }
    mark_file_seen(agent.agent_id, active_workspace_path);
  }, [active_workspace_path, agent.agent_id, mark_file_seen]);

  useEffect(() => {
    if (!active_workspace_path) {
      return;
    }

    const row = row_refs.current[active_workspace_path];
    if (!row) {
      return;
    }

    row.scrollIntoView({
      behavior: "smooth",
      block: "nearest",
    });
  }, [active_workspace_path, files]);

  const handle_create_entry = (entry_type: "file" | "directory") => {
    setPromptDialog({ is_open: true, type: "create", entry_type });
  };

  const handle_create_entry_confirm = async (next_path: string) => {
    if (!next_path.trim()) {
      return;
    }

    const entry_type = prompt_dialog.entry_type;
    setPromptDialog({ is_open: false, type: "create", entry_type: "file" });

    try {
      const response = await createWorkspaceEntryApi(agent.agent_id, next_path, entry_type);
      await load_files();
      if (entry_type === "file") {
        on_open_workspace_file(response.path);
      }
    } catch (mutation_error) {
      setFilesystemError(mutation_error instanceof Error ? mutation_error.message : "创建条目失败");
    }
  };

  const handle_rename_entry = (entry: WorkspaceFileEntry) => {
    setPromptDialog({
      is_open: true,
      type: "rename",
      entry_type: entry.is_dir ? "directory" : "file",
      entry,
    });
  };

  const handle_rename_entry_confirm = async (next_path: string) => {
    const entry = prompt_dialog.entry;
    if (!entry || !next_path.trim() || next_path === entry.path) {
      setPromptDialog({ is_open: false, type: "create", entry_type: "file" });
      return;
    }

    setPromptDialog({ is_open: false, type: "create", entry_type: "file" });

    try {
      const response = await renameWorkspaceEntryApi(agent.agent_id, entry.path, next_path);
      await load_files();

      if (!entry.is_dir && active_workspace_path === entry.path) {
        on_open_workspace_file(response.new_path);
        return;
      }

      if (entry.is_dir && active_workspace_path?.startsWith(`${entry.path}/`)) {
        const renamed_active_path = active_workspace_path.replace(entry.path, response.new_path);
        on_open_workspace_file(renamed_active_path);
      }
    } catch (mutation_error) {
      setFilesystemError(mutation_error instanceof Error ? mutation_error.message : "重命名条目失败");
    }
  };

  const handle_delete_entry = (entry: WorkspaceFileEntry) => {
    setConfirmDialog({ is_open: true, entry });
  };

  const handle_delete_entry_confirm = async () => {
    const entry = confirm_dialog.entry;
    if (!entry) {
      setConfirmDialog({ is_open: false });
      return;
    }

    setConfirmDialog({ is_open: false });

    try {
      await deleteWorkspaceEntryApi(agent.agent_id, entry.path);
      await load_files();

      if (
        active_workspace_path === entry.path ||
        (entry.is_dir && active_workspace_path?.startsWith(`${entry.path}/`))
      ) {
        on_open_workspace_file(null);
      }
    } catch (mutation_error) {
      setFilesystemError(mutation_error instanceof Error ? mutation_error.message : "删除条目失败");
    }
  };

  const toggle_directory = (path: string) => {
    setExpandedDirectories((current) => ({
      ...current,
      [path]: !current[path],
    }));
  };

  const render_tree = (nodes: FileTreeNode[], depth = 0): ReactNode[] => {
    return nodes.flatMap((node) => {
      const is_directory = node.entry.is_dir;
      const is_expanded = expanded_directories[node.entry.path] ?? true;
      const is_active = !is_directory && active_workspace_path === node.entry.path;
      const FileIcon = is_directory ? Folder : getFileIcon(node.entry.name);
      const live_state = !is_directory ? file_states[`${agent.agent_id}:${node.entry.path}`] : undefined;
      const is_writing = live_state?.status === "writing";
      const is_updated = live_state?.status === "updated" && Date.now() - live_state.updated_at < 6000;

      const row = (
        <div
          key={node.entry.path}
          ref={(element) => {
            row_refs.current[node.entry.path] = element;
          }}
          className={cn(
            "group flex items-center gap-2 rounded-[18px] pr-2 transition-all duration-300",
            is_active
              ? "workspace-card-strong text-slate-950 shadow-[0_10px_20px_rgba(111,126,162,0.12)]"
              : is_writing
                ? "workspace-card text-slate-950"
                : is_updated
                  ? "workspace-card bg-[linear-gradient(135deg,rgba(166,255,194,0.26),rgba(242,250,245,0.24))] text-emerald-700"
                  : "text-slate-900/82 hover:bg-white/18",
          )}
          style={{ paddingLeft: `${depth * 16 + 8}px` }}
        >
          <button
            className="flex min-w-0 flex-1 items-center gap-2 py-2 text-left"
            onClick={() => {
              if (is_directory) {
                toggle_directory(node.entry.path);
                return;
              }
              on_open_workspace_file(is_active ? null : node.entry.path);
            }}
            type="button"
          >
            {is_directory ? (
              <>
                {is_expanded ? (
                  <ChevronDown className="h-3.5 w-3.5 shrink-0 text-slate-700/50" />
                ) : (
                  <ChevronRight className="h-3.5 w-3.5 shrink-0 text-slate-700/50" />
                )}
                <Folder className="h-4 w-4 shrink-0 text-slate-700/54" />
              </>
            ) : (
              <>
                <span className="w-3.5 shrink-0" />
                <FileIcon className="h-4 w-4 shrink-0" />
              </>
            )}

            <span className="truncate text-xs font-semibold" title={node.entry.path}>
              {node.entry.name}
            </span>

            {!is_directory && live_state ? (
              <span
                className={cn(
                  "ml-auto inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-[10px] font-semibold",
                  is_writing
                    ? "bg-[rgba(133,119,255,0.12)] text-primary"
                    : "bg-[rgba(102,217,143,0.16)] text-emerald-700 dark:text-emerald-300",
                )}
              >
                <span
                  className={cn(
                    "h-1.5 w-1.5 rounded-full",
                    is_writing ? "animate-pulse bg-primary" : "bg-emerald-500",
                  )}
                />
                {is_writing ? "writing" : "updated"}
              </span>
            ) : null}
          </button>

          <div className="flex items-center gap-1 opacity-0 transition-opacity group-hover:opacity-100">
            <button
              aria-label="重命名"
              className="workspace-chip rounded-xl p-1.5 text-slate-700/54 transition-colors hover:text-slate-950 focus-visible:ring-2 focus-visible:ring-primary/40 focus-visible:ring-offset-1"
              onClick={() => handle_rename_entry(node.entry)}
              type="button"
            >
              <Pencil className="h-3.5 w-3.5" />
            </button>
            <button
              aria-label="删除"
              className="workspace-chip rounded-xl p-1.5 text-slate-700/54 transition-colors hover:text-destructive focus-visible:ring-2 focus-visible:ring-primary/40 focus-visible:ring-offset-1"
              onClick={() => handle_delete_entry(node.entry)}
              type="button"
            >
              <Trash2 className="h-3.5 w-3.5" />
            </button>
          </div>
        </div>
      );

      if (!is_directory || !is_expanded) {
        return [row];
      }

      return [row, ...render_tree(node.children, depth + 1)];
    });
  };

  return (
    <aside className={`flex min-h-0 flex-col bg-transparent ${HOME_WORKSPACE_SIDEBAR_WIDTH_CLASS}`}>
      <RoomSidebarHeader
        active_room_title={active_room_title}
        current_agent_name={agent.name}
        is_refreshing={is_loading_files}
        on_open_directory={on_open_directory}
        on_refresh={() => void load_files()}
      />

      <div className="soft-scrollbar flex-1 overflow-y-auto">
        <RoomConversationsSection
          conversations={conversations}
          current_conversation_id={current_conversation_id}
          on_create_conversation={on_create_conversation}
          on_delete_conversation={on_delete_conversation}
          on_select_conversation={on_select_conversation}
        />

        <RoomMembersSection
          current_agent_id={current_agent_id}
          members={visible_agents}
          on_create_agent={on_create_agent}
          on_select_agent={on_select_agent}
        />

        <RoomContextSection
          active_workspace_path={active_workspace_path}
          contextualFiles={contextual_files}
          current_conversation={selected_conversation}
          file_explorer_content={
            directory_tree.length === 0 ? (
              <div className="workspace-card rounded-[22px] px-3 py-4 text-sm text-slate-700/58">
                还没有文件
              </div>
            ) : (
              render_tree(directory_tree)
            )
          }
          filesystem_error={filesystem_error}
          is_file_explorer_visible={show_file_explorer}
          memory_file_count={memory_files.length}
          on_create_directory={() => handle_create_entry("directory")}
          on_create_file={() => handle_create_entry("file")}
          on_open_workspace_file={on_open_workspace_file}
          on_toggle_file_explorer={() => setShowFileExplorer((current) => !current)}
          total_conversation_count={conversations.length}
        />
      </div>

      <PromptDialog
        default_value={prompt_dialog.type === "rename" && prompt_dialog.entry ? prompt_dialog.entry.path : ""}
        is_open={prompt_dialog.is_open}
        message={
          prompt_dialog.type === "create"
            ? prompt_dialog.entry_type === "file"
              ? "输入文件的路径和名称"
              : "输入目录的名称"
            : "输入新的名称"
        }
        on_cancel={() => setPromptDialog({ is_open: false, type: "create", entry_type: "file" })}
        on_confirm={prompt_dialog.type === "create" ? handle_create_entry_confirm : handle_rename_entry_confirm}
        placeholder={prompt_dialog.entry_type === "file" ? "notes/todo.md" : "notes"}
        title={
          prompt_dialog.type === "create"
            ? prompt_dialog.entry_type === "file"
              ? "创建新文件"
              : "创建新目录"
            : "重命名"
        }
      />

      <ConfirmDialog
        cancel_text="取消"
        confirm_text="删除"
        is_open={confirm_dialog.is_open}
        message={`确定要删除 ${confirm_dialog.entry?.path ?? ""} 吗？删除后无法恢复。`}
        on_cancel={() => setConfirmDialog({ is_open: false })}
        on_confirm={handle_delete_entry_confirm}
        title="确认删除"
        variant="danger"
      />
    </aside>
  );
}
