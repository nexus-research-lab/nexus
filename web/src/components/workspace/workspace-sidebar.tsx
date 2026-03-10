"use client";

import { useEffect, useMemo, useState } from "react";
import {
  BrainCircuit,
  Clock3,
  FileCode2,
  FileText,
  FolderTree,
  MessageSquarePlus,
  RefreshCw,
  Trash2,
} from "lucide-react";

import { getWorkspaceFilesApi } from "@/lib/agent-manage-api";
import { Agent, WorkspaceFileEntry } from "@/types/agent";
import { Session } from "@/types/session";
import { cn, formatRelativeTime, truncate } from "@/lib/utils";

interface WorkspaceSidebarProps {
  agent: Agent;
  sessions: Session[];
  currentSessionKey: string | null;
  activeWorkspacePath: string | null;
  onSelectSession: (sessionKey: string) => void;
  onCreateSession: () => void;
  onDeleteSession: (sessionKey: string) => void;
  onOpenWorkspaceFile: (path: string) => void;
}

export function WorkspaceSidebar({
  agent,
  sessions,
  currentSessionKey,
  activeWorkspacePath,
  onSelectSession,
  onCreateSession,
  onDeleteSession,
  onOpenWorkspaceFile,
}: WorkspaceSidebarProps) {
  const [files, setFiles] = useState<WorkspaceFileEntry[]>([]);
  const [isLoadingFiles, setIsLoadingFiles] = useState(false);

  const visibleFiles = useMemo(
    () => files.filter((file) => !file.is_dir),
    [files],
  );
  const memoryFiles = useMemo(
    () => visibleFiles.filter((file) => /memory|context|summary|skill/i.test(file.path)),
    [visibleFiles],
  );
  const selectedSession = sessions.find((session) => session.session_key === currentSessionKey) ?? null;
  const latestSession = selectedSession ?? sessions[0] ?? null;

  const loadFiles = async () => {
    setIsLoadingFiles(true);
    try {
      const nextFiles = await getWorkspaceFilesApi(agent.agent_id);
      setFiles(nextFiles);
    } finally {
      setIsLoadingFiles(false);
    }
  };

  useEffect(() => {
    void loadFiles();
  }, [agent.agent_id]);

  return (
    <aside className="flex min-h-0 w-[300px] flex-col rounded-[20px] panel-surface">
      <div className="border-b border-border/80 px-4 py-3">
        <div className="flex items-center justify-between gap-3">
          <div className="min-w-0">
            <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-muted-foreground">
              Workspace
            </p>
            <p className="mt-1 truncate text-sm font-medium text-foreground">{agent.name}</p>
          </div>
          <button
            className="rounded-xl border border-border/80 bg-secondary/80 p-2 text-muted-foreground transition-colors hover:border-primary/20 hover:text-primary"
            onClick={() => void loadFiles()}
            type="button"
          >
            <RefreshCw className={cn("h-4 w-4", isLoadingFiles && "animate-spin")} />
          </button>
        </div>
      </div>

      <div className="soft-scrollbar flex-1 overflow-y-auto">
        <section className="border-b border-border/80 px-3 py-3">
          <div className="mb-3 flex items-center justify-between">
            <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.16em] text-muted-foreground">
              <FolderTree className="h-3.5 w-3.5" />
              Virtual Filesystem
            </div>
          </div>

          <div className="space-y-1.5">
            {visibleFiles.length === 0 ? (
              <div className="rounded-xl border border-dashed border-border/80 bg-secondary/60 px-3 py-4 text-sm text-muted-foreground">
                当前 workspace 还没有可展示的文件。
              </div>
            ) : (
              visibleFiles.map((file) => (
                <button
                  key={file.path}
                  className={cn(
                    "w-full rounded-xl border px-3 py-2.5 text-left transition-colors",
                    activeWorkspacePath === file.path
                      ? "border-primary/20 bg-primary/8 text-primary"
                      : "border-transparent bg-secondary/40 text-foreground hover:border-border/80 hover:bg-secondary/80",
                  )}
                  onClick={() => onOpenWorkspaceFile(file.path)}
                  type="button"
                >
                  <div className="flex items-start gap-2">
                    <FileCode2 className="mt-0.5 h-4 w-4 shrink-0" />
                    <div className="min-w-0">
                      <p className="truncate text-sm font-medium">{file.name}</p>
                      <p className="mt-1 text-[11px] text-muted-foreground">{truncate(file.path, 28)}</p>
                    </div>
                  </div>
                </button>
              ))
            )}
          </div>
        </section>

        <section className="px-3 py-3">
          <div className="mb-3 flex items-center justify-between gap-2">
            <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.16em] text-muted-foreground">
              <BrainCircuit className="h-3.5 w-3.5" />
              Context
            </div>
            <button
              className="inline-flex items-center gap-1 rounded-lg bg-primary px-2.5 py-1.5 text-xs font-medium text-primary-foreground"
              onClick={onCreateSession}
              type="button"
            >
              <MessageSquarePlus className="h-3.5 w-3.5" />
              新会话
            </button>
          </div>

          <div className="space-y-3">
            <div className="rounded-2xl bg-secondary/80 px-3 py-3">
              <div className="flex items-center justify-between gap-3">
                <div className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.16em] text-muted-foreground">
                  <Clock3 className="h-3.5 w-3.5" />
                  Current Session
                </div>
                <span className="rounded-full border border-border/70 px-2 py-1 text-[11px] text-muted-foreground">
                  {latestSession ? `${latestSession.message_count ?? 0} msgs` : "idle"}
                </span>
              </div>
              <p className="mt-3 truncate text-sm font-medium text-foreground">
                {latestSession?.title || "暂无活动会话"}
              </p>
              <p className="mt-2 text-xs text-muted-foreground">
                {latestSession ? formatRelativeTime(latestSession.last_activity_at) : "创建新会话后会出现在这里"}
              </p>
            </div>

            <div className="rounded-2xl bg-secondary/80 px-3 py-3">
              <div className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.16em] text-muted-foreground">
                <FileText className="h-3.5 w-3.5" />
                Memory / Context
              </div>
              <div className="mt-3 space-y-2 text-sm">
                <div className="flex justify-between gap-3">
                  <span className="text-muted-foreground">Context Threads</span>
                  <span className="font-medium text-foreground">{sessions.length}</span>
                </div>
                <div className="flex justify-between gap-3">
                  <span className="text-muted-foreground">Memory Files</span>
                  <span className="font-medium text-foreground">{memoryFiles.length}</span>
                </div>
              </div>
              <p className="mt-3 text-xs text-muted-foreground">
                Memory 文件统一映射到 filesystem。
              </p>
            </div>

            <div className="space-y-2">
              {sessions.map((session) => {
                const isActive = session.session_key === currentSessionKey;
                return (
                  <button
                    key={session.session_key}
                    className={cn(
                      "group w-full rounded-xl border px-3 py-3 text-left transition-all",
                      isActive
                        ? "border-primary/30 bg-primary/8 shadow-sm"
                        : "border-transparent bg-secondary/55 hover:border-border/90 hover:bg-secondary/90",
                    )}
                    onClick={() => onSelectSession(session.session_key)}
                    type="button"
                  >
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0">
                        <p className="text-sm font-medium text-foreground">
                          {truncate(session.title || "Untitled Session", 22)}
                        </p>
                        <p className="mt-2 text-[11px] text-muted-foreground">
                          {formatRelativeTime(session.last_activity_at)} / {session.message_count ?? 0} 条消息
                        </p>
                      </div>

                      <button
                        className="rounded-lg border border-border/80 p-1.5 text-muted-foreground opacity-0 transition-all group-hover:opacity-100 hover:border-destructive/20 hover:text-destructive"
                        onClick={(event) => {
                          event.stopPropagation();
                          onDeleteSession(session.session_key);
                        }}
                        type="button"
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </button>
                    </div>
                  </button>
                );
              })}
            </div>
          </div>
        </section>
      </div>
    </aside>
  );
}
