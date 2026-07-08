/* @refresh reset */
// 中文注释：Room 控制器聚合多个 hook，开发热更新时直接重挂页面，避免 hook 签名迁移触发错误边界。
"use client";

import { useCallback, useEffect, useMemo, useState } from "react";

import { isMainAgent } from "@/config/options";
import {
  addRoomMember,
  closeRoomConversationRuntime,
  createRoomConversation,
  deleteRoom,
  deleteRoomConversation,
  notifyRoomDirectoryUpdated,
  removeRoomMember,
  updateRoom,
  updateRoomConversation,
} from "@/lib/api/room-api";
import {
  buildExternalSessionConversationId,
  isExternalSessionChannel,
} from "@/features/conversation/external-session-labels";
import { useHomeWorkspaceController } from "@/hooks/home/use-home-workspace-controller";
import {
  applyConversationSnapshotToRoomContexts,
  buildRoomConversationViews,
  resolveCurrentAgentSessionIdentity,
  resolveCurrentRoomContext,
  resolveRoomMemberAgents,
  resolveSelectedConversationId,
  resolveSelectedMemberAgentId,
} from "@/hooks/room-page-controller/room-page-controller-core";
import { useRoomPageAgentDialog } from "@/hooks/room-page-controller/use-room-page-agent-dialog";
import { useRoomPageData } from "@/hooks/room-page-controller/use-room-page-data";
import { useRoomExternalSessions } from "@/hooks/room-page-controller/use-room-external-sessions";
import { useAgentStore } from "@/store/agent";
import { useConversationStore } from "@/store/conversation";
import { AgentIdentityDraft, AgentOptions } from "@/types/agent/agent";
import { AgentConversationIdentity } from "@/types/agent/agent-conversation";
import { ConversationSnapshotPayload, RoomConversationView } from "@/types/conversation/conversation";
import { UpdateRoomParams } from "@/types/conversation/room";
import { RoomPageControllerOptions } from "@/types/app/route";

export function useRoomPageController({
  roomId: roomId,
  conversationId: conversationId,
  sessionKey: sessionKey,
}: RoomPageControllerOptions) {
  // 这里坚持使用细粒度 selector，避免 Room 页面因为 store
  // 里无关字段变动而整页重渲染。
  const agents = useAgentStore((s) => s.agents);
  const createAgent = useAgentStore((s) => s.create_agent);
  const updateAgent = useAgentStore((s) => s.update_agent);
  const deleteAgent = useAgentStore((s) => s.delete_agent);
  const loadAgentsFromServer = useAgentStore((s) => s.load_agents_from_server);

  const syncConversationSnapshot = useConversationStore((s) => s.sync_conversation_snapshot);

  const [selectedMemberAgentId, setSelectedMemberAgentId] = useState<string | null>(null);
  const {
    isBootstrapped: isBootstrapped,
    roomContexts: roomContexts,
    setRoomContexts: setRoomContexts,
    roomError: roomError,
    isRoomLoading: isRoomLoading,
    refreshRoomContexts: refreshRoomContexts,
  } = useRoomPageData({
    roomId: roomId,
  });
  const {
    isDialogOpen: isDialogOpen,
    dialogMode: dialogMode,
    editingAgentId: editingAgentId,
    dialogInitialTitle: dialogInitialTitle,
    dialogInitialAvatar: dialogInitialAvatar,
    dialogInitialDescription: dialogInitialDescription,
    dialogInitialOptions: dialogInitialOptions,
    dialogInitialVibeTags: dialogInitialVibeTags,
    setIsDialogOpen: setIsDialogOpen,
    handleOpenCreateAgent: handleOpenCreateAgent,
    handleEditAgent: handleEditAgent,
    handleSaveAgentOptions: handleSaveAgentOptions,
    handleSaveExistingAgentOptions: handleSaveExistingAgentOptions,
    handleValidateAgentName: handleValidateAgentName,
    handleValidateAgentNameForAgent: handleValidateAgentNameForAgent,
  } = useRoomPageAgentDialog({
    agents,
    createAgent: createAgent,
    updateAgent: updateAgent,
  });

  const scopedRoomContexts = useMemo(
    () => roomContexts.filter((context) => context.room.id === roomId),
    [roomContexts, roomId],
  );

  const currentRoom = useMemo(
    () => scopedRoomContexts[0]?.room ?? null,
    [scopedRoomContexts],
  );

  const roomMemberAgents = useMemo(() => {
    return resolveRoomMemberAgents(scopedRoomContexts);
  }, [scopedRoomContexts]);

  const workspaceAgentIds = useMemo(() => {
    return roomMemberAgents.map((agent) => agent.agent_id);
  }, [roomMemberAgents]);

  const baseRoomConversations = useMemo<RoomConversationView[]>(() => {
    return buildRoomConversationViews(scopedRoomContexts);
  }, [scopedRoomContexts]);
  const routeSessionKey = useMemo(
    () => sessionKey?.trim() || null,
    [sessionKey],
  );

  const selectedBaseConversationId = useMemo(() => {
    return resolveSelectedConversationId(conversationId, baseRoomConversations);
  }, [baseRoomConversations, conversationId]);

  const currentRoomContext = useMemo(
    () => resolveCurrentRoomContext(scopedRoomContexts, selectedBaseConversationId),
    [scopedRoomContexts, selectedBaseConversationId],
  );

  const activeRoomSession = useMemo(
    () =>
      currentRoomContext?.sessions.find(
        (session) => session.agent_id === selectedMemberAgentId,
      ) ??
      currentRoomContext?.sessions[0] ??
      null,
    [currentRoomContext, selectedMemberAgentId],
  );

  const currentAgent = useMemo(
    () =>
      roomMemberAgents.find(
        (agent) => agent.agent_id === activeRoomSession?.agent_id,
      ) ?? null,
    [activeRoomSession?.agent_id, roomMemberAgents],
  );

  const {
    externalAgentSessions: externalAgentSessions,
    externalRoomConversations: externalRoomConversations,
  } = useRoomExternalSessions({
    agentId: currentAgent?.agent_id ?? null,
    roomId: currentRoom?.id ?? null,
    roomType: currentRoom?.room_type ?? null,
  });

  const currentRoomConversations = useMemo(
    () => [...baseRoomConversations, ...externalRoomConversations]
      .sort((left, right) => right.last_activity_at - left.last_activity_at),
    [baseRoomConversations, externalRoomConversations],
  );

  const selectedConversationId = useMemo(() => {
    if (routeSessionKey) {
      return buildExternalSessionConversationId(routeSessionKey);
    }
    return selectedBaseConversationId;
  }, [routeSessionKey, selectedBaseConversationId]);

  const currentRoomConversation = useMemo(
    () =>
      currentRoomConversations.find(
        (conversation) => conversation.conversation_id === selectedConversationId,
      ) ?? null,
    [currentRoomConversations, selectedConversationId],
  );

  useEffect(() => {
    const nextSelectedMemberAgentId = resolveSelectedMemberAgentId(
      currentRoomContext,
      selectedMemberAgentId,
    );

    if (selectedMemberAgentId !== nextSelectedMemberAgentId) {
      setSelectedMemberAgentId(nextSelectedMemberAgentId);
    }
  }, [currentRoomContext, selectedMemberAgentId]);

  // Room 详情页现在直接基于当前 room context 解析 session 身份；
  // 外部 IM 会话则以 route sessionKey 作为同一 Agent 下的独立会话。
  const currentAgentSessionIdentity = useMemo<AgentConversationIdentity | null>(() => {
    if (routeSessionKey && currentAgent?.agent_id) {
      const externalSession = externalAgentSessions.find((item) => item.session_key === routeSessionKey);
      const externalChatType: AgentConversationIdentity["chat_type"] =
        externalSession?.chat_type === "group" ? "group" : "dm";
      return {
        session_key: routeSessionKey,
        agent_id: externalSession?.agent_id ?? currentAgent.agent_id,
        chat_type: externalChatType,
      };
    }

    return resolveCurrentAgentSessionIdentity({
      currentRoomId: currentRoom?.id ?? null,
      currentConversationId: currentRoomContext?.conversation.id ?? null,
      activeRoomSession: activeRoomSession,
      currentRoomType: currentRoom?.room_type ?? "dm",
    });
  }, [
    activeRoomSession,
    currentAgent?.agent_id,
    currentRoom?.id,
    currentRoom?.room_type,
    currentRoomContext?.conversation.id,
    externalAgentSessions,
    routeSessionKey,
  ]);
  const availableRoomAgents = useMemo(() => {
    const joinedAgentIds = new Set(roomMemberAgents.map((agent) => agent.agent_id));
    return agents.filter((agent) => (
      !joinedAgentIds.has(agent.agent_id) &&
      !isMainAgent(agent.agent_id)
    ));
  }, [agents, roomMemberAgents]);

  const handlePrepareRoomAgentCatalog = useCallback(async () => {
    await loadAgentsFromServer();
  }, [loadAgentsFromServer]);

  const workspace = useHomeWorkspaceController({
    currentAgentId: currentAgent?.agent_id ?? null,
    workspaceAgentIds: workspaceAgentIds,
  });

  const handleSelectAgent = useCallback((agentId: string) => {
    setSelectedMemberAgentId(agentId);
  }, []);

  const handleSelectConversation = useCallback((_nextConversationId: string) => {
    // 路由层负责切换当前 room conversation。
  }, []);

  const handleBackToDirectory = useCallback(() => {
    setSelectedMemberAgentId(null);
  }, []);

  const handleDeleteAgent = useCallback(async (agentId: string) => {
    await deleteAgent(agentId);
  }, [deleteAgent]);

  const handleConversationSnapshotChange = useCallback((snapshot: ConversationSnapshotPayload) => {
    const snapshotConversationId = "conversation_id" in snapshot
      ? snapshot.conversation_id ?? null
      : currentRoomContext?.conversation.id ?? null;
    const snapshotRoomSessionId = "room_session_id" in snapshot
      ? snapshot.room_session_id ?? null
      : activeRoomSession?.id ?? null;

    const nextSnapshot = {
      ...(snapshot.last_activity_at ? { last_activity_at: snapshot.last_activity_at } : {}),
      session_id: snapshot.session_id,
    };

    setRoomContexts((prev) => {
      return applyConversationSnapshotToRoomContexts(prev, {
        conversation_id: snapshotConversationId,
        room_session_id: snapshotRoomSessionId,
        session_id: snapshot.session_id ?? null,
        last_activity_at: snapshot.last_activity_at,
      });
    });

    const snapshotSessionKey = "session_key" in snapshot
      ? snapshot.session_key
      : currentAgentSessionIdentity?.session_key ?? null;

    if (!snapshotSessionKey) {
      return;
    }

    syncConversationSnapshot(snapshotSessionKey, nextSnapshot);
    if (isExternalSessionChannel(null, snapshotSessionKey)) {
      notifyRoomDirectoryUpdated();
    }
  }, [
    activeRoomSession?.id,
    currentRoomContext?.conversation.id,
    currentAgentSessionIdentity?.session_key,
    setRoomContexts,
    syncConversationSnapshot,
  ]);

  const handleUpdateRoom = useCallback(async (params: UpdateRoomParams) => {
    if (!roomId) {
      return;
    }
    await updateRoom(roomId, params);
    await refreshRoomContexts(roomId);
  }, [refreshRoomContexts, roomId]);

  const handleDeleteRoom = useCallback(async () => {
    if (!roomId) {
      return;
    }
    await deleteRoom(roomId);
  }, [roomId]);

  const handleCreateConversation = useCallback(async (title?: string) => {
    if (!roomId) {
      return null;
    }
    const context = await createRoomConversation(roomId, {title});
    await refreshRoomContexts(roomId);
    return context.conversation.id;
  }, [refreshRoomContexts, roomId]);

  const handleDeleteConversation = useCallback(async (conversationId: string) => {
    if (!roomId) {
      return null;
    }
    const fallbackContext = await deleteRoomConversation(roomId, conversationId);
    await refreshRoomContexts(roomId);
    return fallbackContext.conversation.id;
  }, [refreshRoomContexts, roomId]);

  const handleCloseConversation = useCallback(async (conversationId: string) => {
    if (!roomId) {
      return;
    }
    await closeRoomConversationRuntime(roomId, conversationId);
  }, [roomId]);

  const handleUpdateConversationTitle = useCallback(async (conversationId: string, title: string) => {
    if (!roomId) return;
    await updateRoomConversation(roomId, conversationId, { title });
    await refreshRoomContexts(roomId);
  }, [refreshRoomContexts, roomId]);

  const handleAddRoomMember = useCallback(async (agentId: string) => {
    if (!roomId) {
      return;
    }
    await addRoomMember(roomId, agentId);
    await refreshRoomContexts(roomId);
  }, [refreshRoomContexts, roomId]);

  const handleSaveExistingRoomMemberOptions = useCallback(async (
    agentId: string,
    title: string,
    options: AgentOptions,
    identity: AgentIdentityDraft,
  ) => {
    await handleSaveExistingAgentOptions(agentId, title, options, identity);
    if (!roomId) {
      return;
    }
    await refreshRoomContexts(roomId);
  }, [handleSaveExistingAgentOptions, refreshRoomContexts, roomId]);

  const handleRemoveRoomMember = useCallback(async (agentId: string) => {
    if (!roomId) {
      return;
    }
    await removeRoomMember(roomId, agentId);
    await refreshRoomContexts(roomId);
  }, [refreshRoomContexts, roomId]);

  const handleOpenConversationFromLauncher = useCallback((conversationId: string, agentId?: string) => {
    // Launcher 打开 Room 时只认 conversationId，不再接受其他回退标识。
    const targetConversation = currentRoomConversations.find(
      (conversation) => conversation.conversation_id === conversationId,
    );

    if (!targetConversation) {
      return;
    }

    // 如果指定了 agentId，优先使用
    // 否则使用 conversation 的 agentId
    const targetAgentId = agentId ?? targetConversation.agent_id ?? null;

    if (targetAgentId && roomMemberAgents.some((agent) => agent.agent_id === targetAgentId)) {
      setSelectedMemberAgentId(targetAgentId);
    } else if (roomMemberAgents.length > 0) {
      // 如果指定的 agent 不在当前 room 中，默认选择第一个
      setSelectedMemberAgentId(roomMemberAgents[0].agent_id);
    }
  }, [currentRoomConversations, roomMemberAgents]);

  const handleRefreshRoomState = useCallback(async () => {
    if (!roomId) {
      return;
    }

    await refreshRoomContexts(roomId);
    notifyRoomDirectoryUpdated();
  }, [refreshRoomContexts, roomId]);

  const isHydrated = isBootstrapped && !isRoomLoading;

  // 对外 controller 对象本身保持稳定，避免消费端因为对象引用变化
  // 产生无意义重渲染。
  return useMemo(() => ({
    agents,
    roomError: roomError,
    currentRoom: currentRoom,
    currentRoomType: currentRoom?.room_type ?? "room",
    currentRoomTitle: currentRoom?.name?.trim() || currentAgent?.name || "未命名 room",
    currentRoomDescription: currentRoom?.description ?? "",
    currentRoomSkillNames: currentRoom?.skill_names ?? [],
    roomMembers: roomMemberAgents,
    availableRoomAgents: availableRoomAgents,
    handlePrepareRoomAgentCatalog: handlePrepareRoomAgentCatalog,
    currentAgent: currentAgent,
    currentAgentId: currentAgent?.agent_id ?? null,
    currentRoomConversations: currentRoomConversations,
    currentRoomConversation: currentRoomConversation,
    currentAgentSessionIdentity: currentAgentSessionIdentity,
    conversationId: selectedConversationId,
    recentAgents: roomMemberAgents,
    isHydrated: isHydrated,
    isDialogOpen: isDialogOpen,
    dialogMode: dialogMode,
    editingAgentId: editingAgentId,
    dialogInitialTitle: dialogInitialTitle,
    dialogInitialAvatar: dialogInitialAvatar,
    dialogInitialDescription: dialogInitialDescription,
    dialogInitialOptions: dialogInitialOptions,
    dialogInitialVibeTags: dialogInitialVibeTags,
    setIsDialogOpen: setIsDialogOpen,
    handleOpenCreateAgent: handleOpenCreateAgent,
    handleEditAgent: handleEditAgent,
    handleSelectAgent: handleSelectAgent,
    handleSelectConversation: handleSelectConversation,
    handleBackToDirectory: handleBackToDirectory,
    handleDeleteAgent: handleDeleteAgent,
    handleCreateConversation: handleCreateConversation,
    handleSaveAgentOptions: handleSaveAgentOptions,
    handleSaveExistingAgentOptions: handleSaveExistingRoomMemberOptions,
    handleValidateAgentName: handleValidateAgentName,
    handleValidateAgentNameForAgent: handleValidateAgentNameForAgent,
    handleOpenConversationFromLauncher: handleOpenConversationFromLauncher,
    handleRefreshRoomState: handleRefreshRoomState,
    handleConversationSnapshotChange: handleConversationSnapshotChange,
    handleCloseConversation: handleCloseConversation,
    handleDeleteConversation: handleDeleteConversation,
    handleUpdateConversationTitle: handleUpdateConversationTitle,
    handleUpdateRoom: handleUpdateRoom,
    handleDeleteRoom: handleDeleteRoom,
    handleAddRoomMember: handleAddRoomMember,
    handleRemoveRoomMember: handleRemoveRoomMember,
    routeRoomId: roomId ?? null,
    ...workspace,
  }), [
    agents, roomError, currentRoom, currentAgent,
    roomMemberAgents, availableRoomAgents, currentRoomConversations, currentRoomConversation,
    currentAgentSessionIdentity, selectedConversationId, isHydrated, isDialogOpen, dialogMode,
    editingAgentId, dialogInitialTitle, dialogInitialAvatar, dialogInitialDescription, dialogInitialOptions, dialogInitialVibeTags, setIsDialogOpen,
    handleOpenCreateAgent, handleEditAgent, handleSelectAgent,
    handleSelectConversation, handleBackToDirectory, handleDeleteAgent,
    handleCreateConversation, handleSaveAgentOptions, handleSaveExistingRoomMemberOptions, handleValidateAgentName, handleValidateAgentNameForAgent,
    handleOpenConversationFromLauncher, handleRefreshRoomState, handleConversationSnapshotChange,
    handleCloseConversation, handleDeleteConversation, handleUpdateConversationTitle, handleUpdateRoom, handleDeleteRoom,
    handleAddRoomMember, handleRemoveRoomMember, handlePrepareRoomAgentCatalog, roomId, workspace,
  ]);
}
