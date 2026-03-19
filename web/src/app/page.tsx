/**
 * 主页面 — Agent Directory + Agent Space
 *
 * [INPUT]: 依赖 SessionStore, AgentStore, Agent Directory/Space 组件
 * [OUTPUT]: 对外提供 B 端控制台首页
 * [POS]: app 根页面，负责编排 Agent 目录、Agent 工作台与对话视图
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { ArrowLeft } from "lucide-react";

import { ChatInterface } from "@/components/chat/chat-interface";
import { AgentOptions } from "@/components/dialog/agent-options";
import { Console } from "@/components/home/console";
import { AgentInspector } from "@/components/workspace/agent-inspector";
import { AgentSwitcher } from "@/components/workspace/agent-switcher";
import { WorkspaceEditorPane } from "@/components/workspace/workspace-editor-pane";
import { WorkspaceSidebar } from "@/components/workspace/workspace-sidebar";
import { useAgentStore } from "@/store/agent";
import { useSessionStore } from "@/store/session";
import { useInitializeSessions } from "@/hooks/use-initialize-sessions";
import { getSessionCostSummary } from "@/lib/agent-api";
import { getAgentCostSummaryApi, validateAgentNameApi } from "@/lib/agent-manage-api";
import { initialOptions } from "@/config/options";
import { SessionOptions } from "@/types/session";
import { TodoItem } from "@/components/workspace/agent-task-widget";
import { cn } from "@/lib/utils";
import { AgentCostSummary, SessionCostSummary } from "@/types/cost";

const EMPTY_SESSION_COST_SUMMARY: SessionCostSummary = {
  agent_id: "",
  session_key: "",
  session_id: "",
  total_input_tokens: 0,
  total_output_tokens: 0,
  total_tokens: 0,
  total_cache_creation_input_tokens: 0,
  total_cache_read_input_tokens: 0,
  total_cost_usd: 0,
  completed_rounds: 0,
  error_rounds: 0,
  last_round_id: null,
  last_run_duration_ms: null,
  last_run_cost_usd: null,
};

const EMPTY_AGENT_COST_SUMMARY: AgentCostSummary = {
  agent_id: "",
  total_input_tokens: 0,
  total_output_tokens: 0,
  total_tokens: 0,
  total_cache_creation_input_tokens: 0,
  total_cache_read_input_tokens: 0,
  total_cost_usd: 0,
  completed_rounds: 0,
  error_rounds: 0,
  cost_sessions: 0,
};

export default function Home() {
  const {
    agents,
    current_agent_id,
    create_agent,
    update_agent,
    delete_agent,
    set_current_agent,
    load_agents_from_server,
  } = useAgentStore();

  const {
    sessions,
    current_session_key,
    createSession,
    setCurrentSession,
    syncSessionSnapshot,
    deleteSession,
    loadSessionsFromServer,
  } = useSessionStore();

  useEffect(() => {
    load_agents_from_server();
  }, [load_agents_from_server]);

  const isHydrated = useInitializeSessions({
    loadSessionsFromServer,
    setCurrentSession,
    autoSelectFirst: false,
    debugName: "Page",
  });

  const [isDialogOpen, setIsDialogOpen] = useState(false);
  const [dialogMode, setDialogMode] = useState<"create" | "edit">("create");
  const [editingAgentId, setEditingAgentId] = useState<string | null>(null);
  const [activeWorkspacePath, setActiveWorkspacePath] = useState<string | null>(null);
  const [isEditorOpen, setIsEditorOpen] = useState(false);
  const [editorWidthPercent, setEditorWidthPercent] = useState(42);
  const [isResizingEditor, setIsResizingEditor] = useState(false);
  const [currentTodos, setCurrentTodos] = useState<TodoItem[]>([]);
  const [isSessionBusy, setIsSessionBusy] = useState(false);
  const [sessionCostSummary, setSessionCostSummary] = useState<SessionCostSummary>(
    EMPTY_SESSION_COST_SUMMARY,
  );
  const [agentCostSummary, setAgentCostSummary] = useState<AgentCostSummary>(
    EMPTY_AGENT_COST_SUMMARY,
  );
  const workspaceSplitRef = useRef<HTMLElement | null>(null);

  const currentAgent = useMemo(
    () => agents.find((agent) => agent.agent_id === current_agent_id) ?? null,
    [agents, current_agent_id],
  );

  const sessionsByAgent = useMemo(() => {
    const grouped = new Map<string, typeof sessions>();
    sessions.forEach((session) => {
      const owner = session.agent_id ?? "main";
      const currentList = grouped.get(owner) ?? [];
      currentList.push(session);
      grouped.set(owner, currentList);
    });

    grouped.forEach((groupedSessions) => {
      groupedSessions.sort((left, right) => right.last_activity_at - left.last_activity_at);
    });

    return grouped;
  }, [sessions]);

  const currentAgentSessions = useMemo(() => {
    if (!current_agent_id) {
      return [];
    }
    return sessionsByAgent.get(current_agent_id) ?? [];
  }, [current_agent_id, sessionsByAgent]);

  const currentSession = useMemo(
    () => currentAgentSessions.find((session) => session.session_key === current_session_key) ?? null,
    [currentAgentSessions, current_session_key],
  );

  const recentAgents = useMemo(() => agents.slice(0, 4), [agents]);

  const editingAgent = useMemo(
    () => agents.find((agent) => agent.agent_id === editingAgentId),
    [agents, editingAgentId],
  );

  const dialogInitialTitle = useMemo(
    () => (dialogMode === "edit" ? editingAgent?.name : undefined),
    [dialogMode, editingAgent],
  );

  const dialogInitialOptions = useMemo(() => {
    if (dialogMode !== "edit") {
      return initialOptions;
    }

    return {
      model: editingAgent?.options?.model,
      permissionMode: editingAgent?.options?.permission_mode,
      allowedTools: editingAgent?.options?.allowed_tools,
      disallowedTools: editingAgent?.options?.disallowed_tools,
      maxTurns: editingAgent?.options?.max_turns,
      maxThinkingTokens: editingAgent?.options?.max_thinking_tokens,
      skillsEnabled: editingAgent?.options?.skills_enabled,
      settingSources: editingAgent?.options?.setting_sources,
    };
  }, [dialogMode, editingAgent]);

  useEffect(() => {
    if (!current_agent_id) {
      if (current_session_key !== null) {
        setCurrentSession(null);
      }
      setIsEditorOpen(false);
      setCurrentTodos([]);
      setIsSessionBusy(false);
      setSessionCostSummary(EMPTY_SESSION_COST_SUMMARY);
      setAgentCostSummary(EMPTY_AGENT_COST_SUMMARY);
      return;
    }

    const hasSelectedSession = currentAgentSessions.some(
      (session) => session.session_key === current_session_key,
    );
    if (!hasSelectedSession) {
      setCurrentSession(currentAgentSessions[0]?.session_key ?? null);
    }
  }, [current_agent_id, current_session_key, currentAgentSessions, setCurrentSession]);

  useEffect(() => {
    if (!current_agent_id || isSessionBusy) {
      return;
    }

    let ignore = false;

    const loadAgentCostSummary = async () => {
      try {
        const nextSummary = await getAgentCostSummaryApi(current_agent_id);
        if (!ignore) {
          setAgentCostSummary(nextSummary);
        }
      } catch (error) {
        console.error("Failed to load agent cost summary:", error);
        if (!ignore) {
          setAgentCostSummary({
            ...EMPTY_AGENT_COST_SUMMARY,
            agent_id: current_agent_id,
          });
        }
      }
    };

    void loadAgentCostSummary();

    return () => {
      ignore = true;
    };
  }, [current_agent_id, isSessionBusy]);

  useEffect(() => {
    if (!currentSession?.session_key) {
      setSessionCostSummary({
        ...EMPTY_SESSION_COST_SUMMARY,
        agent_id: current_agent_id ?? "",
      });
      return;
    }
    if (isSessionBusy) {
      return;
    }

    let ignore = false;

    const loadSessionCostSummary = async () => {
      try {
        const nextSummary = await getSessionCostSummary(currentSession.session_key);
        if (!ignore) {
          setSessionCostSummary(nextSummary);
        }
      } catch (error) {
        console.error("Failed to load session cost summary:", error);
        if (!ignore) {
          setSessionCostSummary({
            ...EMPTY_SESSION_COST_SUMMARY,
            agent_id: current_agent_id ?? "",
            session_key: currentSession.session_key,
            session_id: currentSession.session_id ?? "",
          });
        }
      }
    };

    void loadSessionCostSummary();

    return () => {
      ignore = true;
    };
  }, [currentSession?.session_id, currentSession?.session_key, current_agent_id, isSessionBusy]);

  const handleOpenCreateAgent = useCallback(() => {
    setDialogMode("create");
    setEditingAgentId(null);
    setIsDialogOpen(true);
  }, []);

  const handleEditAgent = useCallback((agentId: string) => {
    setDialogMode("edit");
    setEditingAgentId(agentId);
    setIsDialogOpen(true);
  }, []);

  const handleAgentSelect = useCallback((agentId: string) => {
    set_current_agent(agentId);
  }, [set_current_agent]);

  const handleBackToDirectory = useCallback(() => {
    set_current_agent(null);
  }, [set_current_agent]);

  const handleDeleteAgent = useCallback(async (agentId: string) => {
    await delete_agent(agentId);
  }, [delete_agent]);

  const handleNewSession = useCallback(async () => {
    if (!current_agent_id) {
      return;
    }

    const key = await createSession({
      title: "New Chat",
      agent_id: current_agent_id,
    });
    setCurrentSession(key);
  }, [createSession, current_agent_id, setCurrentSession]);

  const handleSaveAgentOptions = useCallback(async (title: string, options: SessionOptions) => {
    const agentOptions = {
      model: options.model,
      permission_mode: options.permissionMode,
      allowed_tools: options.allowedTools,
      disallowed_tools: options.disallowedTools,
      skills_enabled: options.skillsEnabled,
      setting_sources: options.settingSources,
    };

    if (dialogMode === "create") {
      const agentId = await create_agent({
        name: title,
        options: agentOptions,
      });
      set_current_agent(agentId);
      return;
    }

    if (dialogMode === "edit" && editingAgentId) {
      await update_agent(editingAgentId, {
        name: title,
        options: agentOptions,
      });
    }
  }, [create_agent, dialogMode, editingAgentId, set_current_agent, update_agent]);

  const handleValidateAgentName = useCallback(async (name: string) => {
    const excludeAgentId = dialogMode === "edit" ? (editingAgentId ?? undefined) : undefined;
    return validateAgentNameApi(name, excludeAgentId);
  }, [dialogMode, editingAgentId]);

  const handleSessionSelect = useCallback((sessionKey: string) => {
    setCurrentSession(sessionKey);
  }, [setCurrentSession]);

  const handleOpenSessionFromDirectory = useCallback((sessionKey: string, agentId?: string) => {
    if (agentId) {
      set_current_agent(agentId);
    }
    setCurrentSession(sessionKey);
  }, [setCurrentSession, set_current_agent]);

  const handleSessionSnapshotChange = useCallback((snapshot: {
    sessionKey: string;
    messageCount: number;
    lastActivityAt: number;
    sessionId: string | null;
  }) => {
    if (!snapshot.sessionKey) {
      return;
    }

    syncSessionSnapshot(snapshot.sessionKey, {
      message_count: snapshot.messageCount,
      last_activity_at: snapshot.lastActivityAt,
      session_id: snapshot.sessionId,
    });
  }, [syncSessionSnapshot]);

  const handleDeleteSession = useCallback(async (sessionKey: string) => {
    await deleteSession(sessionKey);
    if (current_session_key === sessionKey) {
      const remaining = currentAgentSessions.filter((session) => session.session_key !== sessionKey);
      setCurrentSession(remaining[0]?.session_key ?? null);
    }
  }, [current_session_key, currentAgentSessions, deleteSession, setCurrentSession]);

  const handleOpenWorkspaceFile = useCallback((path: string | null) => {
    setActiveWorkspacePath((currentPath) => {
      if (path && currentPath === path && isEditorOpen) {
        setIsEditorOpen(false);
        return null;
      }

      setIsEditorOpen(Boolean(path));
      return path;
    });
  }, [isEditorOpen]);

  const handleStartEditorResize = useCallback(() => {
    setIsResizingEditor(true);
  }, []);

  const handleCloseWorkspacePane = useCallback(() => {
    setIsEditorOpen(false);
  }, []);

  useEffect(() => {
    if (!isResizingEditor) {
      return;
    }

    const handleMouseMove = (event: MouseEvent) => {
      const container = workspaceSplitRef.current;
      if (!container) {
        return;
      }

      const bounds = container.getBoundingClientRect();
      const nextPercent = ((event.clientX - bounds.left) / bounds.width) * 100;
      const clamped = Math.min(Math.max(nextPercent, 28), 58);
      setEditorWidthPercent(clamped);
    };

    const handleMouseUp = () => {
      setIsResizingEditor(false);
    };

    window.addEventListener("mousemove", handleMouseMove);
    window.addEventListener("mouseup", handleMouseUp);

    return () => {
      window.removeEventListener("mousemove", handleMouseMove);
      window.removeEventListener("mouseup", handleMouseUp);
    };
  }, [isResizingEditor]);

  if (!isHydrated) {
    return (
      <main className="relative flex h-screen w-full items-center justify-center overflow-hidden bg-background px-6 text-foreground">
        <div className="pointer-events-none absolute left-[10%] top-[12%] h-56 w-56 rounded-full glow-lilac opacity-70" />
        <div className="pointer-events-none absolute bottom-[10%] right-[12%] h-64 w-64 rounded-full glow-green opacity-60" />
        <div className="soft-ring radius-shell-lg panel-surface px-10 py-9 text-center">
          <div className="mx-auto h-10 w-10 animate-spin rounded-full border-4 border-primary/20 border-t-primary" />
          <p className="mt-4 text-sm text-muted-foreground">正在加载...</p>
        </div>
      </main>
    );
  }

  return (
    <main className="relative flex h-screen w-full overflow-hidden bg-background text-foreground">
      <div className="pointer-events-none absolute left-[5%] top-[8%] h-72 w-72 rounded-full glow-lilac opacity-55" />
      <div className="pointer-events-none absolute bottom-[6%] left-[22%] h-80 w-80 rounded-full bg-white/40 blur-3xl" />
      <div className="pointer-events-none absolute right-[8%] top-[18%] h-72 w-72 rounded-full glow-peach opacity-35" />
      <div className="pointer-events-none absolute right-[12%] bottom-[8%] h-80 w-80 rounded-full glow-green opacity-40" />

      <div className="relative flex min-h-0 flex-1 flex-col p-3 sm:p-4 xl:p-5 2xl:p-6">
        {!currentAgent ? (
          <Console
            agents={agents}
            sessions={sessions}
            currentAgentId={current_agent_id}
            onSelectAgent={handleAgentSelect}
            onOpenSession={handleOpenSessionFromDirectory}
            onCreateAgent={handleOpenCreateAgent}
            onEditAgent={handleEditAgent}
            onDeleteAgent={handleDeleteAgent}
          />
        ) : (
          <section className="flex min-h-0 flex-1 flex-col gap-2 xl:gap-3">
            <div className="soft-ring radius-shell-lg panel-surface px-4 py-3">
              <div className="flex flex-wrap items-center justify-between gap-4">
                <AgentSwitcher
                  agents={agents}
                  currentAgentId={current_agent_id}
                  recentAgents={recentAgents}
                  onSelectAgent={handleAgentSelect}
                  onOpenDirectory={handleBackToDirectory}
                  onCreateAgent={handleOpenCreateAgent}
                />

                <div className="flex flex-wrap items-center gap-2">
                  <button
                    className="neo-pill inline-flex items-center gap-2 rounded-full px-4 py-2 text-xs font-semibold text-foreground transition-transform duration-300 hover:-translate-y-0.5 hover:text-primary"
                    onClick={handleBackToDirectory}
                    type="button"
                  >
                    <ArrowLeft className="h-3.5 w-3.5" />
                    返回目录
                  </button>
                </div>
              </div>
            </div>

            <div className="flex min-h-0 flex-1 gap-2 xl:gap-4">
              <WorkspaceSidebar
                activeWorkspacePath={activeWorkspacePath}
                agent={currentAgent}
                currentSessionKey={current_session_key}
                onCreateSession={handleNewSession}
                onDeleteSession={handleDeleteSession}
                onOpenWorkspaceFile={handleOpenWorkspaceFile}
                onSelectSession={handleSessionSelect}
                sessions={currentAgentSessions}
              />

              <section
                ref={workspaceSplitRef}
                className={cn(
                  "flex min-h-0 min-w-0 flex-1",
                  isEditorOpen ? "gap-4" : "gap-0",
                  isResizingEditor && "select-none cursor-col-resize",
                )}
              >
                <WorkspaceEditorPane
                  agentId={currentAgent.agent_id}
                  isOpen={isEditorOpen}
                  onClose={handleCloseWorkspacePane}
                  onResizeStart={handleStartEditorResize}
                  path={activeWorkspacePath}
                  widthPercent={editorWidthPercent}
                />

                <div className="soft-ring radius-shell-lg flex min-h-0 min-w-0 flex-1 flex-col panel-surface overflow-hidden">
                  <div className="min-h-0 flex-1">
                    <ChatInterface
                      agentId={currentAgent.agent_id}
                      onOpenWorkspaceFile={handleOpenWorkspaceFile}
                      onLoadingChange={setIsSessionBusy}
                      onSessionSnapshotChange={handleSessionSnapshotChange}
                      onTodosChange={setCurrentTodos}
                      sessionKey={currentSession?.session_key ?? null}
                      onNewSession={handleNewSession}
                    />
                  </div>
                </div>
              </section>

              {!isEditorOpen && (
                <div className="hidden min-[1280px]:flex min-[1280px]:min-h-0 min-[1280px]:shrink-0">
                  <AgentInspector
                    activeSession={currentSession}
                    agent={currentAgent}
                    agentCostSummary={agentCostSummary}
                    isSessionBusy={isSessionBusy}
                    onEditAgent={handleEditAgent}
                    sessionCostSummary={sessionCostSummary}
                    sessions={currentAgentSessions}
                    todos={currentTodos}
                  />
                </div>
              )}
            </div>
          </section>
        )}
      </div>

      <AgentOptions
        mode={dialogMode}
        isOpen={isDialogOpen}
        onClose={() => setIsDialogOpen(false)}
        onSave={handleSaveAgentOptions}
        onValidateName={handleValidateAgentName}
        initialTitle={dialogInitialTitle}
        initialOptions={dialogInitialOptions}
      />
    </main>
  );
}
