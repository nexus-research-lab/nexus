"use client";

import { useEffect, useMemo, useState } from "react";
import { Bot, FileCode2, FolderTree, RefreshCw, Save, Settings2, Workflow } from "lucide-react";

import { getWorkspaceFileContentApi, getWorkspaceFilesApi, updateWorkspaceFileContentApi } from "@/lib/agent-manage-api";
import { Agent, WorkspaceFileEntry } from "@/types/agent";
import { Session } from "@/types/session";
import { cn, formatRelativeTime, truncate } from "@/lib/utils";

interface AgentInspectorProps {
  agent: Agent;
  sessions: Session[];
  activeSession: Session | null;
  onEditAgent: (agentId: string) => void;
}

export function AgentInspector({
  agent,
  sessions,
  activeSession,
  onEditAgent,
}: AgentInspectorProps) {
  const [files, setFiles] = useState<WorkspaceFileEntry[]>([]);
  const [selectedPath, setSelectedPath] = useState<string | null>(null);
  const [draftContent, setDraftContent] = useState("");
  const [savedContent, setSavedContent] = useState("");
  const [isLoadingFiles, setIsLoadingFiles] = useState(false);
  const [isLoadingContent, setIsLoadingContent] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [syncedAt, setSyncedAt] = useState<number | null>(null);

  const permissionMode = agent.options.permission_mode || "default";

  const visibleFiles = useMemo(
    () => files.filter((file) => !file.is_dir),
    [files],
  );

  const selectedFile = useMemo(
    () => visibleFiles.find((file) => file.path === selectedPath) ?? null,
    [selectedPath, visibleFiles],
  );

  const isDirty = draftContent !== savedContent;

  const loadFiles = async (preferredPath?: string | null) => {
    setIsLoadingFiles(true);
    setError(null);

    try {
      const nextFiles = await getWorkspaceFilesApi(agent.agent_id);
      setFiles(nextFiles);
      setSyncedAt(Date.now());

      const textFiles = nextFiles.filter((item) => !item.is_dir);
      if (textFiles.length === 0) {
        setSelectedPath(null);
        setDraftContent("");
        setSavedContent("");
        return;
      }

      const nextPath = preferredPath && textFiles.some((file) => file.path === preferredPath)
        ? preferredPath
        : textFiles[0].path;
      setSelectedPath(nextPath);
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : "加载 Workspace 文件失败");
    } finally {
      setIsLoadingFiles(false);
    }
  };

  useEffect(() => {
    void loadFiles();
  }, [agent.agent_id]);

  useEffect(() => {
    if (!selectedPath) {
      return;
    }

    let cancelled = false;
    const loadContent = async () => {
      setIsLoadingContent(true);
      setError(null);
      try {
        const response = await getWorkspaceFileContentApi(agent.agent_id, selectedPath);
        if (cancelled) {
          return;
        }
        setDraftContent(response.content);
        setSavedContent(response.content);
      } catch (loadError) {
        if (!cancelled) {
          setError(loadError instanceof Error ? loadError.message : "读取 Workspace 文件失败");
        }
      } finally {
        if (!cancelled) {
          setIsLoadingContent(false);
        }
      }
    };

    void loadContent();
    return () => {
      cancelled = true;
    };
  }, [agent.agent_id, selectedPath]);

  const handleSave = async () => {
    if (!selectedPath || !isDirty || isSaving) {
      return;
    }

    setIsSaving(true);
    setError(null);
    try {
      const response = await updateWorkspaceFileContentApi(agent.agent_id, selectedPath, draftContent);
      setSavedContent(response.content);
      setDraftContent(response.content);
      await loadFiles(response.path);
    } catch (saveError) {
      setError(saveError instanceof Error ? saveError.message : "保存 Workspace 文件失败");
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <aside className="flex min-h-0 w-[520px] flex-col rounded-[20px] panel-surface">
      <div className="border-b border-border/80 px-4 py-4">
        <div className="flex items-center justify-between gap-3">
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-muted-foreground">Workspace</p>
            <h2 className="mt-1 text-base font-semibold text-foreground">{agent.name}</h2>
          </div>
          <button
            className="rounded-xl border border-border/80 bg-white px-3 py-2 text-sm font-medium text-foreground transition-colors hover:border-primary/20 hover:text-primary"
            onClick={() => onEditAgent(agent.agent_id)}
            type="button"
          >
            设置
          </button>
        </div>

        <div className="mt-4 grid grid-cols-3 gap-2 text-sm">
          <div className="rounded-xl bg-muted/70 px-3 py-3">
            <div className="flex items-center gap-2 text-xs uppercase tracking-[0.16em] text-muted-foreground">
              <Workflow className="h-3.5 w-3.5" />
              Sessions
            </div>
            <p className="mt-2 font-medium text-foreground">{sessions.length}</p>
          </div>
          <div className="rounded-xl bg-muted/70 px-3 py-3">
            <div className="flex items-center gap-2 text-xs uppercase tracking-[0.16em] text-muted-foreground">
              <Bot className="h-3.5 w-3.5" />
              Model
            </div>
            <p className="mt-2 font-medium text-foreground">{agent.options.model || "inherit"}</p>
          </div>
          <div className="rounded-xl bg-muted/70 px-3 py-3">
            <div className="flex items-center gap-2 text-xs uppercase tracking-[0.16em] text-muted-foreground">
              <Settings2 className="h-3.5 w-3.5" />
              Permission
            </div>
            <p className="mt-2 font-medium text-foreground">{permissionMode}</p>
          </div>
        </div>
      </div>

      <div className="border-b border-border/80 px-4 py-3">
        <div className="flex items-center justify-between gap-3">
          <div>
            <p className="text-xs font-semibold uppercase tracking-[0.16em] text-muted-foreground">
              本地文件
            </p>
            <p className="mt-1 text-xs text-muted-foreground">
              {truncate(agent.workspace_path, 48)}
            </p>
          </div>
          <button
            className="rounded-xl border border-border/80 bg-white p-2 text-muted-foreground transition-colors hover:border-primary/20 hover:text-primary"
            onClick={() => void loadFiles(selectedPath)}
            type="button"
          >
            <RefreshCw className={cn("h-4 w-4", isLoadingFiles && "animate-spin")} />
          </button>
        </div>
      </div>

      <div className="flex min-h-0 flex-1">
        <div className="soft-scrollbar w-[192px] overflow-y-auto border-r border-border/80 px-2 py-2">
          {visibleFiles.map((file) => (
            <button
              key={file.path}
              className={cn(
                "mb-1 w-full rounded-xl px-3 py-2 text-left transition-colors",
                selectedPath === file.path
                  ? "bg-primary/10 text-primary"
                  : "text-foreground hover:bg-muted/80",
              )}
              onClick={() => setSelectedPath(file.path)}
              type="button"
            >
              <div className="flex items-start gap-2">
                <FileCode2 className="mt-0.5 h-4 w-4 shrink-0" />
                <div className="min-w-0">
                  <p className="truncate text-sm font-medium">{file.name}</p>
                  <p className="mt-1 text-[11px] text-muted-foreground">
                    {file.path}
                  </p>
                </div>
              </div>
            </button>
          ))}

          {!isLoadingFiles && visibleFiles.length === 0 && (
            <div className="rounded-xl border border-dashed border-border/80 px-3 py-4 text-xs leading-5 text-muted-foreground">
              暂无可编辑文件
            </div>
          )}
        </div>

        <div className="flex min-h-0 flex-1 flex-col">
          <div className="flex items-center justify-between gap-3 border-b border-border/80 px-4 py-3">
            <div className="min-w-0">
              <div className="flex items-center gap-2 text-xs uppercase tracking-[0.16em] text-muted-foreground">
                <FolderTree className="h-3.5 w-3.5" />
                <span>{selectedFile?.path || "选择文件"}</span>
              </div>
              {syncedAt && (
                <p className="mt-1 text-[11px] text-muted-foreground">
                  当前 Session：{activeSession?.title || "未选择"} / 最近同步 {formatRelativeTime(syncedAt)}
                </p>
              )}
            </div>

            <button
              className={cn(
                "inline-flex items-center gap-2 rounded-xl px-3 py-2 text-sm font-medium transition-colors",
                isDirty
                  ? "bg-primary text-primary-foreground"
                  : "bg-muted/80 text-muted-foreground",
              )}
              disabled={!selectedPath || !isDirty || isSaving}
              onClick={() => void handleSave()}
              type="button"
            >
              <Save className="h-4 w-4" />
              {isSaving ? "保存中" : "保存"}
            </button>
          </div>

          {error && (
            <div className="px-4 py-3 text-sm text-destructive">{error}</div>
          )}

          <div className="flex-1 p-4">
            <textarea
              className="soft-scrollbar h-full w-full resize-none rounded-2xl border border-border/80 bg-secondary/90 p-4 font-mono text-sm leading-6 text-foreground outline-none focus:border-primary/20 disabled:opacity-70"
              disabled={!selectedPath || isLoadingContent}
              onChange={(event) => setDraftContent(event.target.value)}
              placeholder={selectedPath ? "读取中..." : "选择左侧文件开始编辑"}
              value={isLoadingContent ? "加载中..." : draftContent}
            />
          </div>
        </div>
      </div>
    </aside>
  );
}
