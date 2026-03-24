"use client";

import { useCallback, useEffect, useMemo, useState } from "react";

import { initialOptions } from "@/config/options";
import { useInitializeConversations } from "@/hooks/use-initialize-conversations";
import { validateAgentNameApi } from "@/lib/agent-manage-api";
import { useConversationStore } from "@/store/conversation";
import { useAgentStore } from "@/store/agent";
import { ConversationSnapshotPayload } from "@/types/conversation";
import { SessionOptions } from "@/types/session";

interface SessionSnapshotPayload {
  session_key: string;
  message_count: number;
  last_activity_at: number;
  session_id: string | null;
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
    conversations,
    current_conversation_id,
    createConversation,
    setCurrentConversation,
    syncConversationSnapshot,
    deleteConversation,
    loadConversationsFromServer,
  } = useConversationStore();

  const [isDialogOpen, setIsDialogOpen] = useState(false);
  const [dialogMode, setDialogMode] = useState<"create" | "edit">("create");
  const [editingAgentId, setEditingAgentId] = useState<string | null>(null);

  useEffect(() => {
    load_agents_from_server();
  }, [load_agents_from_server]);

  const isHydrated = useInitializeConversations({
    loadConversationsFromServer,
    setCurrentConversation,
    autoSelectFirst: false,
    debugName: "Page",
  });

  const currentAgent = useMemo(
    () => agents.find((agent) => agent.agent_id === current_agent_id) ?? null,
    [agents, current_agent_id],
  );

  const sessionsByAgent = useMemo(() => {
    const grouped = new Map<string, typeof conversations>();
    conversations.forEach((conversation) => {
      const owner = conversation.agent_id ?? "main";
      const currentList = grouped.get(owner) ?? [];
      currentList.push(conversation);
      grouped.set(owner, currentList);
    });

    grouped.forEach((groupedSessions) => {
      groupedSessions.sort((left, right) => right.last_activity_at - left.last_activity_at);
    });

    return grouped;
  }, [conversations]);

  const currentAgentSessions = useMemo(() => {
    if (!current_agent_id) {
      return [];
    }
    return sessionsByAgent.get(current_agent_id) ?? [];
  }, [current_agent_id, sessionsByAgent]);

  const currentSession = useMemo(
    () => currentAgentSessions.find((session) => session.session_key === current_conversation_id) ?? null,
    [currentAgentSessions, current_conversation_id],
  );
  const currentConversation = currentSession;
  const currentConversationId = current_conversation_id;
  const currentRoomConversations = currentAgentSessions;

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
      if (current_conversation_id !== null) {
        setCurrentConversation(null);
      }
      return;
    }

    const hasSelectedSession = currentAgentSessions.some(
      (conversation) => conversation.session_key === current_conversation_id,
    );
    if (!hasSelectedSession) {
      setCurrentConversation(currentAgentSessions[0]?.session_key ?? null);
    }
  }, [current_agent_id, current_conversation_id, currentAgentSessions, setCurrentConversation]);

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

    const key = await createConversation({
      title: "New Chat",
      agent_id: current_agent_id,
    });
    setCurrentConversation(key);
  }, [createConversation, current_agent_id, setCurrentConversation]);

  const handleSaveAgentOptions = useCallback(async (title: string, options: SessionOptions) => {
    const agentOptions = {
      model: options.model,
      permission_mode: options.permission_mode,
      allowed_tools: options.allowed_tools,
      disallowed_tools: options.disallowed_tools,
      skills_enabled: options.skills_enabled,
      setting_sources: options.setting_sources,
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
    setCurrentConversation(sessionKey);
  }, [setCurrentConversation]);

  const handleOpenSessionFromDirectory = useCallback((sessionKey: string, agentId?: string) => {
    if (agentId) {
      set_current_agent(agentId);
    }
    setCurrentConversation(sessionKey);
  }, [setCurrentConversation, set_current_agent]);

  const handleSessionSnapshotChange = useCallback((snapshot: SessionSnapshotPayload) => {
    if (!snapshot.session_key) {
      return;
    }

    syncConversationSnapshot(snapshot.session_key, {
      message_count: snapshot.message_count,
      last_activity_at: snapshot.last_activity_at,
      session_id: snapshot.session_id,
    });
  }, [syncConversationSnapshot]);

  const handleDeleteSession = useCallback(async (sessionKey: string) => {
    await deleteConversation(sessionKey);
    if (current_conversation_id === sessionKey) {
      const remaining = currentAgentSessions.filter((conversation) => conversation.session_key !== sessionKey);
      setCurrentConversation(remaining[0]?.session_key ?? null);
    }
  }, [current_conversation_id, currentAgentSessions, deleteConversation, setCurrentConversation]);

  const handleCreateConversation = handleNewSession;
  const handleConversationSelect = handleSessionSelect;
  const handleOpenConversationFromLauncher = handleOpenSessionFromDirectory;
  const handleConversationSnapshotChange = useCallback((snapshot: ConversationSnapshotPayload) => {
    handleSessionSnapshotChange({
      session_key: snapshot.conversation_id,
      message_count: snapshot.message_count,
      last_activity_at: snapshot.last_activity_at,
      session_id: snapshot.session_id,
    });
  }, [handleSessionSnapshotChange]);
  const handleDeleteConversation = handleDeleteSession;

  return {
    agents,
    currentAgent,
    currentAgentId: current_agent_id,
    currentAgentSessions,
    currentRoomConversations,
    currentSession,
    currentSessionKey: current_conversation_id,
    currentConversation,
    currentConversationId,
    conversations,
    sessions: conversations,
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
    handleCreateConversation,
    handleSaveAgentOptions,
    handleValidateAgentName,
    handleSessionSelect,
    handleConversationSelect,
    handleOpenSessionFromDirectory,
    handleOpenConversationFromLauncher,
    handleSessionSnapshotChange,
    handleConversationSnapshotChange,
    handleDeleteSession,
    handleDeleteConversation,
  };
}
