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

import { RoomContextSection } from "@/features/room-context/room-context-section";
import { RoomMembersSection } from "@/features/room-members/room-members-section";
import { RoomConversationsSection } from "@/features/room-navigation/room-conversations-section";
import { RoomSidebarHeader } from "@/features/room-navigation/room-sidebar-header";
import {
  createWorkspaceEntryApi,
  deleteWorkspaceEntryApi,
  getWorkspaceFilesApi,
  renameWorkspaceEntryApi,
} from "@/lib/agent-manage-api";
import { Agent, WorkspaceFileEntry } from "@/types/agent";
import { Conversation } from "@/types/conversation";
import { Session } from "@/types/session";
import { useWorkspaceFilesStore } from "@/store/workspace-files";
import { useWorkspaceLiveStore } from "@/store/workspace-live";
import { cn, formatRelativeTime, truncate } from "@/lib/utils";
import { ConfirmDialog, PromptDialog } from "@/components/dialog/confirm-dialog";
import { HOME_WORKSPACE_SIDEBAR_WIDTH_CLASS } from "@/lib/home-layout";

interface FileTreeNode {
  entry: WorkspaceFileEntry;
  children: FileTreeNode[];
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

interface WorkspaceSidebarProps {
  agents: Agent[];
  agent: Agent;
  currentAgentId: string | null;
  recentAgents: Agent[];
  sessions: Session[];
  currentSessionKey: string | null;
  activeWorkspacePath: string | null;
  onSelectAgent: (agentId: string) => void;
  onOpenDirectory: () => void;
  onCreateAgent: () => void;
  onSelectSession: (sessionKey: string) => void;
  onCreateSession: () => void;
  onDeleteSession: (sessionKey: string) => void;
  onOpenWorkspaceFile: (path: string | null) => void;
}

export function WorkspaceSidebar({
  agents,
  agent,
  currentAgentId,
  recentAgents,
  sessions,
  currentSessionKey,
  activeWorkspacePath,
  onSelectAgent,
  onOpenDirectory,
  onCreateAgent,
  onSelectSession,
  onCreateSession,
  onDeleteSession,
  onOpenWorkspaceFile,
}: WorkspaceSidebarProps) {
  const [files, setFiles] = useState<WorkspaceFileEntry[]>([]);
  const [isLoadingFiles, setIsLoadingFiles] = useState(false);
  const [filesystemError, setFilesystemError] = useState<string | null>(null);
  const [expandedDirectories, setExpandedDirectories] = useState<Record<string, boolean>>({});
  const [showFileExplorer, setShowFileExplorer] = useState(false);

  // 对话框状态
  const [promptDialog, setPromptDialog] = useState<{
    isOpen: boolean;
    type: "create" | "rename";
    entryType: "file" | "directory";
    entry?: WorkspaceFileEntry;
  }>({ isOpen: false, type: "create", entryType: "file" });
  const [confirmDialog, setConfirmDialog] = useState<{
    isOpen: boolean;
    entry?: WorkspaceFileEntry;
  }>({ isOpen: false });
  const fileStates = useWorkspaceLiveStore((state) => state.fileStates);
  const recentEvents = useWorkspaceLiveStore((state) => state.recentEvents);
  const markFileSeen = useWorkspaceLiveStore((state) => state.markFileSeen);
  const setWorkspaceFiles = useWorkspaceFilesStore((state) => state.setFiles);
  const rowRefs = useRef<Record<string, HTMLDivElement | null>>({});

  const visibleFiles = useMemo(() => files.filter((file) => !file.is_dir), [files]);
  const memoryFiles = useMemo(
    () => visibleFiles.filter((file) => /memory|context|summary|skill/i.test(file.path)),
    [visibleFiles],
  );
  const contextualFiles = useMemo(() => {
    const quick = [...memoryFiles];
    const activeFile = activeWorkspacePath
      ? visibleFiles.find((file) => file.path === activeWorkspacePath)
      : null;

    if (activeFile && !quick.some((file) => file.path === activeFile.path)) {
      quick.unshift(activeFile);
    }

    return quick.slice(0, 4);
  }, [activeWorkspacePath, memoryFiles, visibleFiles]);
  const directoryTree = useMemo(() => {
    const nodeMap = new Map<string, FileTreeNode>();
    const roots: FileTreeNode[] = [];

    [...files]
      .sort((left, right) => left.path.localeCompare(right.path, "zh-CN"))
      .forEach((entry) => {
        nodeMap.set(entry.path, { entry, children: [] });
      });

    [...files]
      .sort((left, right) => left.path.localeCompare(right.path, "zh-CN"))
      .forEach((entry) => {
        const node = nodeMap.get(entry.path);
        if (!node) {
          return;
        }

        const parentPath = entry.path.includes("/") ? entry.path.split("/").slice(0, -1).join("/") : null;
        const parent = parentPath ? nodeMap.get(parentPath) : null;

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
  const selectedSession = sessions.find((session) => session.session_key === currentSessionKey) ?? null;
  const latestSession = selectedSession ?? sessions[0] ?? null;
  const activeRoomTitle = selectedSession?.title?.trim() || latestSession?.title?.trim() || "未命名 room";
  const conversations = sessions as Conversation[];
  const currentConversation = selectedSession as Conversation | null;

  const loadFiles = useCallback(async () => {
    setIsLoadingFiles(true);
    setFilesystemError(null);
    try {
      const nextFiles = await getWorkspaceFilesApi(agent.agent_id);
      setFiles(nextFiles);
      setWorkspaceFiles(agent.agent_id, nextFiles);
    } catch (loadError) {
      setFilesystemError(loadError instanceof Error ? loadError.message : "加载 workspace 失败");
    } finally {
      setIsLoadingFiles(false);
    }
  }, [agent.agent_id, setWorkspaceFiles]);

  useEffect(() => {
    void loadFiles();
  }, [agent.agent_id]);

  const latestAgentEvent = useMemo(
    () => recentEvents.find((item) => item.agent_id === agent.agent_id) ?? null,
    [agent.agent_id, recentEvents],
  );
  const knownFilePaths = useMemo(() => new Set(files.map((entry) => entry.path)), [files]);
  const visibleAgents = useMemo(() => {
    const seen = new Set<string>();
    const merged = [agent, ...recentAgents, ...agents];
    return merged.filter((item) => {
      if (!item?.agent_id || seen.has(item.agent_id)) {
        return false;
      }
      seen.add(item.agent_id);
      return true;
    }).slice(0, 5);
  }, [agent, agents, recentAgents]);

  useEffect(() => {
    if (!latestAgentEvent || latestAgentEvent.event_type !== "file_write_end") {
      return;
    }

    const pathParts = latestAgentEvent.path.split("/").slice(0, -1);
    if (pathParts.length > 0) {
      setExpandedDirectories((current) => {
        const nextState = { ...current };
        pathParts.forEach((_, index) => {
          const parentPath = pathParts.slice(0, index + 1).join("/");
          nextState[parentPath] = true;
        });
        return nextState;
      });
    }

    if (knownFilePaths.has(latestAgentEvent.path)) {
      return;
    }

    const timer = window.setTimeout(() => {
      void loadFiles();
    }, 240);

    return () => window.clearTimeout(timer);
  }, [knownFilePaths, latestAgentEvent?.id, latestAgentEvent?.event_type, latestAgentEvent?.path, loadFiles]);

  useEffect(() => {
    setExpandedDirectories((current) => {
      const nextState = { ...current };
      files
        .filter((entry) => entry.is_dir)
        .forEach((entry) => {
          if (nextState[entry.path] === undefined) {
            nextState[entry.path] = true;
          }
        });
      return nextState;
    });
  }, [files]);

  useEffect(() => {
    if (!activeWorkspacePath) {
      return;
    }
    markFileSeen(agent.agent_id, activeWorkspacePath);
  }, [activeWorkspacePath, agent.agent_id, markFileSeen]);

  useEffect(() => {
    if (!activeWorkspacePath) {
      return;
    }

    const row = rowRefs.current[activeWorkspacePath];
    if (!row) {
      return;
    }

    row.scrollIntoView({
      behavior: "smooth",
      block: "nearest",
    });
  }, [activeWorkspacePath, files]);

  const handleCreateEntry = (entryType: "file" | "directory") => {
    setPromptDialog({ isOpen: true, type: "create", entryType });
  };

  const handleCreateEntryConfirm = async (nextPath: string) => {
    if (!nextPath.trim()) {
      return;
    }

    const entryType = promptDialog.entryType;
    setPromptDialog({ isOpen: false, type: "create", entryType: "file" });

    try {
      const response = await createWorkspaceEntryApi(agent.agent_id, nextPath, entryType);
      await loadFiles();
      if (entryType === "file") {
        onOpenWorkspaceFile(response.path);
      }
    } catch (mutationError) {
      setFilesystemError(mutationError instanceof Error ? mutationError.message : "创建条目失败");
    }
  };

  const handleRenameEntry = (entry: WorkspaceFileEntry) => {
    setPromptDialog({ isOpen: true, type: "rename", entryType: entry.is_dir ? "directory" : "file", entry });
  };

  const handleRenameEntryConfirm = async (nextPath: string) => {
    const entry = promptDialog.entry;
    if (!entry || !nextPath.trim() || nextPath === entry.path) {
      setPromptDialog({ isOpen: false, type: "create", entryType: "file" });
      return;
    }

    setPromptDialog({ isOpen: false, type: "create", entryType: "file" });

    try {
      const response = await renameWorkspaceEntryApi(agent.agent_id, entry.path, nextPath);
      await loadFiles();

      if (!entry.is_dir && activeWorkspacePath === entry.path) {
        onOpenWorkspaceFile(response.new_path);
        return;
      }

      if (entry.is_dir && activeWorkspacePath?.startsWith(`${entry.path}/`)) {
        const renamedActivePath = activeWorkspacePath.replace(entry.path, response.new_path);
        onOpenWorkspaceFile(renamedActivePath);
      }
    } catch (mutationError) {
      setFilesystemError(mutationError instanceof Error ? mutationError.message : "重命名条目失败");
    }
  };

  const handleDeleteEntry = (entry: WorkspaceFileEntry) => {
    setConfirmDialog({ isOpen: true, entry });
  };

  const handleDeleteEntryConfirm = async () => {
    const entry = confirmDialog.entry;
    if (!entry) {
      setConfirmDialog({ isOpen: false });
      return;
    }

    setConfirmDialog({ isOpen: false });

    try {
      await deleteWorkspaceEntryApi(agent.agent_id, entry.path);
      await loadFiles();

      if (
        activeWorkspacePath === entry.path ||
        (entry.is_dir && activeWorkspacePath?.startsWith(`${entry.path}/`))
      ) {
        onOpenWorkspaceFile(null);
      }
    } catch (mutationError) {
      setFilesystemError(mutationError instanceof Error ? mutationError.message : "删除条目失败");
    }
  };

  const toggleDirectory = (path: string) => {
    setExpandedDirectories((current) => ({
      ...current,
      [path]: !current[path],
    }));
  };

  const renderTree = (nodes: FileTreeNode[], depth = 0): ReactNode[] => {
    return nodes.flatMap((node) => {
      const isDirectory = node.entry.is_dir;
      const isExpanded = expandedDirectories[node.entry.path] ?? true;
      const isActive = !isDirectory && activeWorkspacePath === node.entry.path;
      const FileIcon = isDirectory ? Folder : getFileIcon(node.entry.name);
      const liveState = !isDirectory ? fileStates[`${agent.agent_id}:${node.entry.path}`] : undefined;
      const isWriting = liveState?.status === "writing";
      const isUpdated = liveState?.status === "updated" && Date.now() - liveState.updated_at < 6000;

      const row = (
        <div
          key={node.entry.path}
          ref={(element) => {
            rowRefs.current[node.entry.path] = element;
          }}
          className={cn(
            "group flex items-center gap-2 rounded-[18px] pr-2 transition-all duration-300",
            isActive
              ? "workspace-card-strong text-slate-950 shadow-[0_10px_20px_rgba(111,126,162,0.12)]"
              : isWriting
                ? "workspace-card text-slate-950"
                : isUpdated
                  ? "workspace-card bg-[linear-gradient(135deg,rgba(166,255,194,0.26),rgba(242,250,245,0.24))] text-emerald-700"
                  : "text-slate-900/82 hover:bg-white/18",
          )}
          style={{ paddingLeft: `${depth * 16 + 8}px` }}
        >
          <button
            className="flex min-w-0 flex-1 items-center gap-2 py-2 text-left"
            onClick={() => {
              if (isDirectory) {
                toggleDirectory(node.entry.path);
                return;
              }
              onOpenWorkspaceFile(isActive ? null : node.entry.path);
            }}
            type="button"
          >
            {isDirectory ? (
              <>
                {isExpanded ? (
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

            <span
              className="truncate text-xs font-semibold"
              title={node.entry.path}
            >
              {node.entry.name}
            </span>

            {!isDirectory && liveState && (
              <span
                className={cn(
                  "ml-auto inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-[10px] font-semibold",
                  isWriting
                    ? "bg-[rgba(133,119,255,0.12)] text-primary"
                    : "bg-[rgba(102,217,143,0.16)] text-emerald-700 dark:text-emerald-300",
                )}
              >
                <span
                  className={cn(
                    "h-1.5 w-1.5 rounded-full",
                    isWriting ? "animate-pulse bg-primary" : "bg-emerald-500",
                  )}
                />
                {isWriting ? "writing" : "updated"}
              </span>
            )}
          </button>

          <div className="flex items-center gap-1 opacity-0 transition-opacity group-hover:opacity-100">
            <button
              aria-label="重命名"
              className="workspace-chip rounded-xl p-1.5 text-slate-700/54 transition-colors hover:text-slate-950 focus-visible:ring-2 focus-visible:ring-primary/40 focus-visible:ring-offset-1"
              onClick={() => handleRenameEntry(node.entry)}
              type="button"
            >
              <Pencil className="h-3.5 w-3.5" />
            </button>
            <button
              aria-label="删除"
              className="workspace-chip rounded-xl p-1.5 text-slate-700/54 transition-colors hover:text-destructive focus-visible:ring-2 focus-visible:ring-primary/40 focus-visible:ring-offset-1"
              onClick={() => handleDeleteEntry(node.entry)}
              type="button"
            >
              <Trash2 className="h-3.5 w-3.5" />
            </button>
          </div>
        </div>
      );

      if (!isDirectory || !isExpanded) {
        return [row];
      }

      return [row, ...renderTree(node.children, depth + 1)];
    });
  };

  return (
    <aside
      className={`flex min-h-0 flex-col bg-transparent ${HOME_WORKSPACE_SIDEBAR_WIDTH_CLASS}`}
    >
      <RoomSidebarHeader
        activeRoomTitle={activeRoomTitle}
        currentAgentName={agent.name}
        isRefreshing={isLoadingFiles}
        onOpenDirectory={onOpenDirectory}
        onRefresh={() => void loadFiles()}
      />

      <div className="soft-scrollbar flex-1 overflow-y-auto">
        <RoomConversationsSection
          conversations={conversations}
          currentConversationId={currentSessionKey}
          onCreateConversation={onCreateSession}
          onDeleteConversation={onDeleteSession}
          onSelectConversation={onSelectSession}
        />

        <RoomMembersSection
          currentAgentId={currentAgentId}
          members={visibleAgents}
          onCreateAgent={onCreateAgent}
          onSelectAgent={onSelectAgent}
        />

        <RoomContextSection
          activeWorkspacePath={activeWorkspacePath}
          contextualFiles={contextualFiles}
          currentConversation={currentConversation}
          fileExplorerContent={
            directoryTree.length === 0 ? (
              <div className="workspace-card rounded-[22px] px-3 py-4 text-sm text-slate-700/58">
                还没有文件
              </div>
            ) : (
              renderTree(directoryTree)
            )
          }
          filesystemError={filesystemError}
          isFileExplorerVisible={showFileExplorer}
          memoryFileCount={memoryFiles.length}
          onCreateDirectory={() => handleCreateEntry("directory")}
          onCreateFile={() => handleCreateEntry("file")}
          onOpenWorkspaceFile={onOpenWorkspaceFile}
          onToggleFileExplorer={() => setShowFileExplorer((current) => !current)}
          totalConversationCount={conversations.length}
        />
      </div>

      {/* 创建/重命名对话框 */}
      <PromptDialog
        isOpen={promptDialog.isOpen}
        title={
          promptDialog.type === "create"
            ? promptDialog.entryType === "file"
              ? "创建新文件"
              : "创建新目录"
            : "重命名"
        }
        message={
          promptDialog.type === "create"
            ? promptDialog.entryType === "file"
              ? "输入文件的路径和名称"
              : "输入目录的名称"
            : "输入新的名称"
        }
        placeholder={promptDialog.entryType === "file" ? "notes/todo.md" : "notes"}
        defaultValue={promptDialog.type === "rename" && promptDialog.entry ? promptDialog.entry.path : ""}
        onConfirm={promptDialog.type === "create" ? handleCreateEntryConfirm : handleRenameEntryConfirm}
        onCancel={() => setPromptDialog({ isOpen: false, type: "create", entryType: "file" })}
      />

      {/* 删除确认对话框 */}
      <ConfirmDialog
        isOpen={confirmDialog.isOpen}
        title="确认删除"
        message={`确定要删除 ${confirmDialog.entry?.path ?? ""} 吗？删除后无法恢复。`}
        confirmText="删除"
        cancelText="取消"
        variant="danger"
        onConfirm={handleDeleteEntryConfirm}
        onCancel={() => setConfirmDialog({ isOpen: false })}
      />
    </aside>
  );
}
