/**
 * 主页面 — Agent Directory + Agent Space
 *
 * [INPUT]: 依赖 SessionStore, AgentStore, Agent Directory/Space 组件
 * [OUTPUT]: 对外提供 B 端控制台首页
 * [POS]: app 根页面，负责编排 Agent 目录、Agent 工作台与对话视图
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { ArrowLeft, Command, MessageSquarePlus, PanelRightOpen, Settings2 } from "lucide-react";

import { ChatInterface } from "@/components/chat-interface";
import { AgentOptions } from "@/components/option/agent-options";
import { AgentDirectory } from "@/components/workspace/agent-directory";
import { AgentInspector } from "@/components/workspace/agent-inspector";
import { AgentSwitcher } from "@/components/workspace/agent-switcher";
import { SessionRail } from "@/components/workspace/session-rail";
import { useAgentStore } from "@/store/agent";
import { useSessionStore } from "@/store/session";
import { useInitializeSessions } from "@/hooks/use-initialize-sessions";
import { validateAgentNameApi } from "@/lib/agent-manage-api";
import { initialOptions } from "@/config/options";
import { SessionOptions } from "@/types/session";

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
    deleteSession,
    updateSession,
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

  const currentSession = useMemo(() => {
    return currentAgentSessions.find((session) => session.session_key === current_session_key) ?? null;
  }, [currentAgentSessions, current_session_key]);

  const recentAgents = useMemo(() => {
    return agents.slice(0, 4);
  }, [agents]);

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

  // 保证进入 Agent Space 后，当前 session 一直属于当前 agent。
  useEffect(() => {
    if (!current_agent_id) {
      if (current_session_key !== null) {
        setCurrentSession(null);
      }
      return;
    }

    const hasSelectedSession = currentAgentSessions.some(
      (session) => session.session_key === current_session_key,
    );

    if (hasSelectedSession) {
      return;
    }

    setCurrentSession(currentAgentSessions[0]?.session_key ?? null);
  }, [current_agent_id, current_session_key, currentAgentSessions, setCurrentSession]);

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
      max_turns: options.maxTurns,
      max_thinking_tokens: options.maxThinkingTokens,
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

  const handleDeleteSession = useCallback(async (sessionKey: string) => {
    await deleteSession(sessionKey);
    if (current_session_key === sessionKey) {
      const remaining = currentAgentSessions.filter((session) => session.session_key !== sessionKey);
      setCurrentSession(remaining[0]?.session_key ?? null);
    }
  }, [current_session_key, currentAgentSessions, deleteSession, setCurrentSession]);

  const handleEditTitle = useCallback((sessionKey: string, title: string) => {
    updateSession(sessionKey, { title });
  }, [updateSession]);

  if (!isHydrated) {
    return (
      <main className="flex h-screen w-full items-center justify-center bg-background text-foreground">
        <div className="rounded-[28px] panel-surface px-8 py-7 text-center">
          <div className="mx-auto h-10 w-10 animate-spin rounded-full border-4 border-primary/20 border-t-primary" />
          <p className="mt-4 text-sm text-muted-foreground">正在加载 Agent Console...</p>
        </div>
      </main>
    );
  }

  return (
    <main className="flex h-screen w-full overflow-hidden bg-background text-foreground">
      <div className="flex min-h-0 flex-1 flex-col p-4">
        <header className="mb-4 rounded-[20px] panel-surface px-5 py-4">
          <div className="flex flex-wrap items-start justify-between gap-6">
            <div className="space-y-2">
              <p className="text-xs font-semibold uppercase tracking-[0.22em] text-primary">
                Nexus Core Console
              </p>
              <div>
                <h1 className="text-2xl font-semibold tracking-tight text-foreground">
                  Agent 工作台
                </h1>
              </div>
            </div>

            <div className="flex flex-wrap items-center gap-3 text-sm text-muted-foreground">
              <div className="rounded-full border border-border/80 bg-white/80 px-3 py-1.5">
                {agents.length} Agents
              </div>
              <div className="rounded-full border border-border/80 bg-white/80 px-3 py-1.5">
                {sessions.length} Sessions
              </div>
              <div className="rounded-full border border-border/80 bg-white/80 px-3 py-1.5">
                <span className="font-mono">Cmd/Ctrl + K</span> 预留快速操作入口
              </div>
            </div>
          </div>
        </header>

        {!currentAgent ? (
          <AgentDirectory
            agents={agents}
            sessions={sessions}
            currentAgentId={current_agent_id}
            onSelectAgent={handleAgentSelect}
            onCreateAgent={handleOpenCreateAgent}
            onEditAgent={handleEditAgent}
            onDeleteAgent={handleDeleteAgent}
          />
        ) : (
          <section className="flex min-h-0 flex-1 flex-col gap-4">
            <div className="rounded-[20px] panel-surface px-5 py-4">
              <div className="flex flex-wrap items-start justify-between gap-5">
                <div className="space-y-3">
                  <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.18em] text-muted-foreground">
                    <button
                      className="inline-flex items-center gap-2 rounded-full border border-border/80 bg-white/80 px-3 py-1.5 text-xs font-medium text-foreground transition-colors hover:border-primary/20 hover:text-primary"
                      onClick={handleBackToDirectory}
                      type="button"
                    >
                      <ArrowLeft className="h-3.5 w-3.5" />
                      Agent Directory
                    </button>
                    <span>/</span>
                    <span>{currentAgent.name}</span>
                  </div>

                <div>
                  <h2 className="text-3xl font-semibold tracking-tight text-foreground">
                      {currentAgent.name}
                  </h2>
                </div>
              </div>

                <div className="flex flex-wrap items-center gap-3">
                  <button
                    className="inline-flex items-center gap-2 rounded-xl border border-border/80 bg-white/80 px-4 py-2 text-sm font-medium text-foreground transition-colors hover:border-primary/20 hover:text-primary"
                    onClick={() => handleEditAgent(currentAgent.agent_id)}
                    type="button"
                  >
                    <Settings2 className="h-4 w-4" />
                    Agent 设置
                  </button>
                  <button
                    className="inline-flex items-center gap-2 rounded-xl bg-primary px-4 py-2 text-sm font-medium text-primary-foreground"
                    onClick={handleNewSession}
                    type="button"
                  >
                    <MessageSquarePlus className="h-4 w-4" />
                    新建 Session
                  </button>
                </div>
              </div>

              <div className="mt-4 flex flex-wrap items-center justify-between gap-4 border-t border-border/80 pt-4">
                <AgentSwitcher
                  agents={agents}
                  currentAgentId={current_agent_id}
                  recentAgents={recentAgents}
                  onSelectAgent={handleAgentSelect}
                  onOpenDirectory={handleBackToDirectory}
                  onCreateAgent={handleOpenCreateAgent}
                />

                <div className="flex items-center gap-2 rounded-full border border-border/80 bg-white/80 px-3 py-1.5 text-sm text-muted-foreground">
                  <Command className="h-4 w-4" />
                  快速切换入口预留
                </div>
              </div>
            </div>

            <div className="flex min-h-0 flex-1 gap-4">
              <SessionRail
                sessions={currentAgentSessions}
                currentSessionKey={current_session_key}
                onSelectSession={handleSessionSelect}
                onCreateSession={handleNewSession}
                onDeleteSession={handleDeleteSession}
              />

              <section className="flex min-h-0 min-w-0 flex-1 flex-col rounded-[20px] panel-surface">
                <div className="flex items-center justify-between border-b border-border/80 px-5 py-3">
                  <div>
                    <p className="text-xs font-semibold uppercase tracking-[0.18em] text-muted-foreground">
                      Session Workspace
                    </p>
                    <p className="mt-1 text-sm text-foreground">
                      {currentSession?.title || "选择 Session"}
                    </p>
                  </div>

                  <div className="inline-flex items-center gap-2 rounded-full border border-border/80 bg-white/80 px-3 py-2 text-xs text-muted-foreground">
                    <PanelRightOpen className="h-3.5 w-3.5" />
                    Timeline + Inspector
                  </div>
                </div>

                <div className="min-h-0 flex-1">
                  <ChatInterface
                    sessionKey={currentSession?.session_key ?? null}
                    onNewSession={handleNewSession}
                  />
                </div>
              </section>

              <AgentInspector
                activeSession={currentSession}
                agent={currentAgent}
                onEditAgent={handleEditAgent}
                sessions={currentAgentSessions}
              />
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
