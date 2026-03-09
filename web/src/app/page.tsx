/**
 * 主页面 — Agent 管理 + 对话视图
 *
 * [INPUT]: 依赖 SessionStore, AgentStore, ChatInterface, Sidebar
 * [OUTPUT]: 对外提供主页面
 * [POS]: app 的根页面，编排 Agent 管理和对话交互
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Sidebar } from "@/components/sidebar";
import { ChatInterface } from "@/components/chat-interface";
import { AgentOptions } from "@/components/option/agent-options";
import { useSessionStore } from "@/store/session";
import { useAgentStore } from "@/store/agent";
import { useInitializeSessions } from "@/hooks/use-initialize-sessions";
import { SessionOptions } from "@/types/session";
import { validateAgentNameApi } from "@/lib/agent-manage-api";
import { initialOptions } from "@/config/options";

export default function Home() {
  // Agent Store
  const {
    agents,
    current_agent_id,
    create_agent,
    update_agent,
    delete_agent,
    set_current_agent,
    load_agents_from_server,
  } = useAgentStore();

  // Session Store
  const {
    sessions,
    current_session_key,
    createSession,
    setCurrentSession,
    deleteSession,
    updateSession,
    loadSessionsFromServer,
  } = useSessionStore();

  // 初始化
  useEffect(() => { load_agents_from_server(); }, [load_agents_from_server]);

  const isHydrated = useInitializeSessions({
    loadSessionsFromServer,
    setCurrentSession,
    autoSelectFirst: true,
    debugName: "Page",
  });

  // UI 状态
  const [isSidebarCollapsed, setIsSidebarCollapsed] = useState(false);
  const [isDialogOpen, setIsDialogOpen] = useState(false);
  const [dialogMode, setDialogMode] = useState<"create" | "edit">("create");
  const [editingAgentId, setEditingAgentId] = useState<string | null>(null);

  const editingAgent = useMemo(
    () => agents.find(a => a.agent_id === editingAgentId),
    [agents, editingAgentId]
  );
  const dialogInitialTitle = useMemo(
    () => (dialogMode === "edit" ? editingAgent?.name : undefined),
    [dialogMode, editingAgent]
  );
  const dialogInitialOptions = useMemo(() => {
    if (dialogMode !== "edit") return initialOptions;
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

  // ==================== Agent 操作 ====================

  const handleNewAgent = useCallback(() => {
    setDialogMode("create");
    setEditingAgentId(null);
    setIsDialogOpen(true);
  }, []);

  const handleAgentSelect = useCallback((agent_id: string) => {
    set_current_agent(agent_id);
  }, [set_current_agent]);

  const handleDeleteAgent = useCallback(async (agent_id: string) => {
    await delete_agent(agent_id);
  }, [delete_agent]);

  // ==================== Session 操作 ====================

  const handleNewSession = useCallback(async () => {
    if (!current_agent_id) return;
    const key = await createSession({
      title: "New Chat",
      agent_id: current_agent_id,
    });
    setCurrentSession(key);
  }, [current_agent_id, createSession, setCurrentSession]);

  const handleSaveAgentOptions = useCallback(async (title: string, options: SessionOptions) => {
    const agent_options = {
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
      const agent_id = await create_agent({
        name: title,
        options: agent_options,
      });
      set_current_agent(agent_id);
    } else if (dialogMode === "edit" && editingAgentId) {
      await update_agent(editingAgentId, {
        name: title,
        options: agent_options,
      });
    }
  }, [dialogMode, editingAgentId, create_agent, set_current_agent, update_agent]);

  const handleEditAgent = useCallback((agent_id: string) => {
    setDialogMode("edit");
    setEditingAgentId(agent_id);
    setIsDialogOpen(true);
  }, []);

  const handleValidateAgentName = useCallback(async (name: string) => {
    const exclude_agent_id = dialogMode === "edit" ? (editingAgentId || undefined) : undefined;
    return validateAgentNameApi(name, exclude_agent_id);
  }, [dialogMode, editingAgentId]);

  const handleSessionSelect = useCallback((key: string) => {
    setCurrentSession(key);
  }, [setCurrentSession]);

  const handleDeleteSession = useCallback(async (key: string) => {
    await deleteSession(key);
    if (current_session_key === key) {
      const remaining = sessions.filter(s => s.session_key !== key);
      setCurrentSession(remaining.length > 0 ? remaining[0].session_key : null);
    }
  }, [deleteSession, current_session_key, sessions, setCurrentSession]);

  const handleEditTitle = useCallback((key: string, title: string) => {
    updateSession(key, { title });
  }, [updateSession]);

  if (!isHydrated) {
    return (
      <main className="flex h-screen w-full bg-background text-foreground items-center justify-center">
        <div className="flex flex-col items-center gap-4">
          <div className="w-8 h-8 border-4 border-primary border-t-transparent rounded-full animate-spin" />
          <p className="text-sm text-muted-foreground">加载中...</p>
        </div>
      </main>
    );
  }

  return (
    <main className="flex h-screen w-full bg-background text-foreground overflow-hidden">
      <Sidebar
        agents={agents}
        sessions={sessions}
        current_agent_id={current_agent_id}
        current_session_key={current_session_key}
        on_new_agent={handleNewAgent}
        on_agent_select={handleAgentSelect}
        on_delete_agent={handleDeleteAgent}
        on_new_session={handleNewSession}
        on_session_select={handleSessionSelect}
        on_delete_session={handleDeleteSession}
        on_edit_title={handleEditTitle}
        on_edit_agent={handleEditAgent}
        is_collapsed={isSidebarCollapsed}
        on_toggle_collapse={() => setIsSidebarCollapsed(!isSidebarCollapsed)}
      />

      <ChatInterface
        sessionKey={current_session_key}
        onNewSession={handleNewSession}
      />

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
