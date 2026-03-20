"use client";

import { useCallback, useEffect, useMemo, useState } from "react";

import { initialOptions } from "@/config/options";
import { useInitializeSessions } from "@/hooks/use-initialize-sessions";
import { validateAgentNameApi } from "@/lib/agent-manage-api";
import { useAgentStore } from "@/store/agent";
import { useSessionStore } from "@/store/session";
import { SessionOptions } from "@/types/session";

interface SessionSnapshotPayload {
  sessionKey: string;
  messageCount: number;
  lastActivityAt: number;
  sessionId: string | null;
}

export function useHomeAgentSessionController() {
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

  const [isDialogOpen, setIsDialogOpen] = useState(false);
  const [dialogMode, setDialogMode] = useState<"create" | "edit">("create");
  const [editingAgentId, setEditingAgentId] = useState<string | null>(null);

  useEffect(() => {
    load_agents_from_server();
  }, [load_agents_from_server]);

  const isHydrated = useInitializeSessions({
    loadSessionsFromServer,
    setCurrentSession,
    autoSelectFirst: false,
    debugName: "Page",
  });

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
      return;
    }

    const hasSelectedSession = currentAgentSessions.some(
      (session) => session.session_key === current_session_key,
    );
    if (!hasSelectedSession) {
      setCurrentSession(currentAgentSessions[0]?.session_key ?? null);
    }
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

  const handleSessionSnapshotChange = useCallback((snapshot: SessionSnapshotPayload) => {
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

  return {
    agents,
    currentAgent,
    currentAgentId: current_agent_id,
    currentAgentSessions,
    currentSession,
    currentSessionKey: current_session_key,
    sessions,
    recentAgents,
    isHydrated,
    isDialogOpen,
    dialogMode,
    dialogInitialTitle,
    dialogInitialOptions,
    setIsDialogOpen,
    handleOpenCreateAgent,
    handleEditAgent,
    handleAgentSelect,
    handleBackToDirectory,
    handleDeleteAgent,
    handleNewSession,
    handleSaveAgentOptions,
    handleValidateAgentName,
    handleSessionSelect,
    handleOpenSessionFromDirectory,
    handleSessionSnapshotChange,
    handleDeleteSession,
  };
}
