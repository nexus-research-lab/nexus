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

  const current_agent = useMemo(
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

  const current_room_conversations = useMemo(() => {
    if (!current_agent_id) {
      return [];
    }
    return sessionsByAgent.get(current_agent_id) ?? [];
  }, [current_agent_id, sessionsByAgent]);

  const current_conversation = useMemo(
    () => current_room_conversations.find((conversation) => conversation.session_key === current_conversation_id) ?? null,
    [current_room_conversations, current_conversation_id],
  );

  const recent_agents = useMemo(() => agents.slice(0, 4), [agents]);
  const editing_agent = useMemo(
    () => agents.find((agent) => agent.agent_id === editingAgentId),
    [agents, editingAgentId],
  );
  const dialog_initial_title = useMemo(
    () => (dialogMode === "edit" ? editing_agent?.name : undefined),
    [dialogMode, editing_agent],
  );
  const dialog_initial_options = useMemo(() => {
    if (dialogMode !== "edit") {
      return initialOptions;
    }

    return {
      model: editing_agent?.options?.model,
      permissionMode: editing_agent?.options?.permission_mode,
      allowedTools: editing_agent?.options?.allowed_tools,
      disallowedTools: editing_agent?.options?.disallowed_tools,
      maxTurns: editing_agent?.options?.max_turns,
      maxThinkingTokens: editing_agent?.options?.max_thinking_tokens,
      skillsEnabled: editing_agent?.options?.skills_enabled,
      settingSources: editing_agent?.options?.setting_sources,
    };
  }, [dialogMode, editing_agent]);

  useEffect(() => {
    if (!current_agent_id) {
      if (current_conversation_id !== null) {
        setCurrentConversation(null);
      }
      return;
    }

    const hasSelectedConversation = current_room_conversations.some(
      (conversation) => conversation.session_key === current_conversation_id,
    );
    if (!hasSelectedConversation) {
      setCurrentConversation(current_room_conversations[0]?.session_key ?? null);
    }
  }, [current_agent_id, current_conversation_id, current_room_conversations, setCurrentConversation]);

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
      const remaining = current_room_conversations.filter((conversation) => conversation.session_key !== sessionKey);
      setCurrentConversation(remaining[0]?.session_key ?? null);
    }
  }, [current_conversation_id, current_room_conversations, deleteConversation, setCurrentConversation]);

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
    current_agent,
    current_agent_id,
    current_room_conversations,
    current_conversation,
    current_conversation_id,
    conversations,
    recent_agents,
    isHydrated,
    isDialogOpen,
    dialogMode,
    dialog_initial_title,
    dialog_initial_options,
    setIsDialogOpen,
    handleOpenCreateAgent,
    handleEditAgent,
    handleAgentSelect,
    handleBackToDirectory,
    handleDeleteAgent,
    handleCreateConversation,
    handleSaveAgentOptions,
    handleValidateAgentName,
    handleConversationSelect,
    handleOpenConversationFromLauncher,
    handleConversationSnapshotChange,
    handleDeleteConversation,
  };
}
