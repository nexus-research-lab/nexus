/* @refresh reset */
// 中文注释：Room 控制器聚合多个 hook，开发热更新时直接重挂页面，避免 hook 签名迁移触发错误边界。
"use client";

import { useCallback, useEffect, useMemo, useState } from "react";

import { is_main_agent } from "@/config/options";
import {
  add_room_member,
  close_room_conversation_runtime,
  create_room_conversation,
  delete_room,
  delete_room_conversation,
  notify_room_directory_updated,
  remove_room_member,
  update_room,
  update_room_conversation,
} from "@/lib/api/room-api";
import {
  build_external_session_conversation_id,
  is_external_session_channel,
} from "@/features/conversation/external-session-labels";
import { useHomeWorkspaceController } from "@/hooks/home/use-home-workspace-controller";
import {
  apply_conversation_snapshot_to_room_contexts,
  build_room_conversation_views,
  resolve_current_agent_session_identity,
  resolve_current_room_context,
  resolve_room_member_agents,
  resolve_selected_conversation_id,
  resolve_selected_member_agent_id,
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
  room_id: roomId,
  conversation_id: conversationId,
  session_key: sessionKey,
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
    is_bootstrapped: isBootstrapped,
    room_contexts: roomContexts,
    set_room_contexts: setRoomContexts,
    room_error: roomError,
    is_room_loading: isRoomLoading,
    refresh_room_contexts: refreshRoomContexts,
  } = useRoomPageData({
    room_id: roomId,
  });
  const {
    is_dialog_open: isDialogOpen,
    dialog_mode: dialogMode,
    editing_agent_id: editingAgentId,
    dialog_initial_title: dialogInitialTitle,
    dialog_initial_avatar: dialogInitialAvatar,
    dialog_initial_description: dialogInitialDescription,
    dialog_initial_options: dialogInitialOptions,
    dialog_initial_vibe_tags: dialogInitialVibeTags,
    set_is_dialog_open: setIsDialogOpen,
    handle_open_create_agent: handleOpenCreateAgent,
    handle_edit_agent: handleEditAgent,
    handle_save_agent_options: handleSaveAgentOptions,
    handle_save_existing_agent_options: handleSaveExistingAgentOptions,
    handle_validate_agent_name: handleValidateAgentName,
    handle_validate_agent_name_for_agent: handleValidateAgentNameForAgent,
  } = useRoomPageAgentDialog({
    agents,
    create_agent: createAgent,
    update_agent: updateAgent,
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
    return resolve_room_member_agents(scopedRoomContexts);
  }, [scopedRoomContexts]);

  const workspaceAgentIds = useMemo(() => {
    return roomMemberAgents.map((agent) => agent.agent_id);
  }, [roomMemberAgents]);

  const baseRoomConversations = useMemo<RoomConversationView[]>(() => {
    return build_room_conversation_views(scopedRoomContexts);
  }, [scopedRoomContexts]);
  const routeSessionKey = useMemo(
    () => sessionKey?.trim() || null,
    [sessionKey],
  );

  const selectedBaseConversationId = useMemo(() => {
    return resolve_selected_conversation_id(conversationId, baseRoomConversations);
  }, [baseRoomConversations, conversationId]);

  const currentRoomContext = useMemo(
    () => resolve_current_room_context(scopedRoomContexts, selectedBaseConversationId),
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
    external_agent_sessions: externalAgentSessions,
    external_room_conversations: externalRoomConversations,
  } = useRoomExternalSessions({
    agent_id: currentAgent?.agent_id ?? null,
    room_id: currentRoom?.id ?? null,
    room_type: currentRoom?.room_type ?? null,
  });

  const currentRoomConversations = useMemo(
    () => [...baseRoomConversations, ...externalRoomConversations]
      .sort((left, right) => right.last_activity_at - left.last_activity_at),
    [baseRoomConversations, externalRoomConversations],
  );

  const selectedConversationId = useMemo(() => {
    if (routeSessionKey) {
      return build_external_session_conversation_id(routeSessionKey);
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
    const nextSelectedMemberAgentId = resolve_selected_member_agent_id(
      currentRoomContext,
      selectedMemberAgentId,
    );

    if (selectedMemberAgentId !== nextSelectedMemberAgentId) {
      setSelectedMemberAgentId(nextSelectedMemberAgentId);
    }
  }, [currentRoomContext, selectedMemberAgentId]);

  // Room 详情页现在直接基于当前 room context 解析 session 身份；
  // 外部 IM 会话则以 route session_key 作为同一 Agent 下的独立会话。
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

    return resolve_current_agent_session_identity({
      current_room_id: currentRoom?.id ?? null,
      current_conversation_id: currentRoomContext?.conversation.id ?? null,
      active_room_session: activeRoomSession,
      current_room_type: currentRoom?.room_type ?? "dm",
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
      !is_main_agent(agent.agent_id)
    ));
  }, [agents, roomMemberAgents]);

  const handlePrepareRoomAgentCatalog = useCallback(async () => {
    await loadAgentsFromServer();
  }, [loadAgentsFromServer]);

  const workspace = useHomeWorkspaceController({
    current_agent_id: currentAgent?.agent_id ?? null,
    workspace_agent_ids: workspaceAgentIds,
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
      return apply_conversation_snapshot_to_room_contexts(prev, {
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
    if (is_external_session_channel(null, snapshotSessionKey)) {
      notify_room_directory_updated();
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
    await update_room(roomId, params);
    await refreshRoomContexts(roomId);
  }, [refreshRoomContexts, roomId]);

  const handleDeleteRoom = useCallback(async () => {
    if (!roomId) {
      return;
    }
    await delete_room(roomId);
  }, [roomId]);

  const handleCreateConversation = useCallback(async (title?: string) => {
    if (!roomId) {
      return null;
    }
    const context = await create_room_conversation(roomId, {title});
    await refreshRoomContexts(roomId);
    return context.conversation.id;
  }, [refreshRoomContexts, roomId]);

  const handleDeleteConversation = useCallback(async (conversationId: string) => {
    if (!roomId) {
      return null;
    }
    const fallbackContext = await delete_room_conversation(roomId, conversationId);
    await refreshRoomContexts(roomId);
    return fallbackContext.conversation.id;
  }, [refreshRoomContexts, roomId]);

  const handleCloseConversation = useCallback(async (conversationId: string) => {
    if (!roomId) {
      return;
    }
    await close_room_conversation_runtime(roomId, conversationId);
  }, [roomId]);

  const handleUpdateConversationTitle = useCallback(async (conversationId: string, title: string) => {
    if (!roomId) return;
    await update_room_conversation(roomId, conversationId, { title });
    await refreshRoomContexts(roomId);
  }, [refreshRoomContexts, roomId]);

  const handleAddRoomMember = useCallback(async (agentId: string) => {
    if (!roomId) {
      return;
    }
    await add_room_member(roomId, agentId);
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
    await remove_room_member(roomId, agentId);
    await refreshRoomContexts(roomId);
  }, [refreshRoomContexts, roomId]);

  const handleOpenConversationFromLauncher = useCallback((conversationId: string, agentId?: string) => {
    // Launcher 打开 Room 时只认 conversation_id，不再接受其他回退标识。
    const targetConversation = currentRoomConversations.find(
      (conversation) => conversation.conversation_id === conversationId,
    );

    if (!targetConversation) {
      return;
    }

    // 如果指定了 agent_id，优先使用
    // 否则使用 conversation 的 agent_id
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
    notify_room_directory_updated();
  }, [refreshRoomContexts, roomId]);

  const isHydrated = isBootstrapped && !isRoomLoading;

  // 对外 controller 对象本身保持稳定，避免消费端因为对象引用变化
  // 产生无意义重渲染。
  return useMemo(() => ({
    agents,
    room_error: roomError,
    current_room: currentRoom,
    current_room_type: currentRoom?.room_type ?? "room",
    current_room_title: currentRoom?.name?.trim() || currentAgent?.name || "未命名 room",
    current_room_description: currentRoom?.description ?? "",
    current_room_skill_names: currentRoom?.skill_names ?? [],
    room_members: roomMemberAgents,
    available_room_agents: availableRoomAgents,
    handle_prepare_room_agent_catalog: handlePrepareRoomAgentCatalog,
    current_agent: currentAgent,
    current_agent_id: currentAgent?.agent_id ?? null,
    current_room_conversations: currentRoomConversations,
    current_room_conversation: currentRoomConversation,
    current_agent_session_identity: currentAgentSessionIdentity,
    conversation_id: selectedConversationId,
    recent_agents: roomMemberAgents,
    is_hydrated: isHydrated,
    is_dialog_open: isDialogOpen,
    dialog_mode: dialogMode,
    editing_agent_id: editingAgentId,
    dialog_initial_title: dialogInitialTitle,
    dialog_initial_avatar: dialogInitialAvatar,
    dialog_initial_description: dialogInitialDescription,
    dialog_initial_options: dialogInitialOptions,
    dialog_initial_vibe_tags: dialogInitialVibeTags,
    set_is_dialog_open: setIsDialogOpen,
    handle_open_create_agent: handleOpenCreateAgent,
    handle_edit_agent: handleEditAgent,
    handle_select_agent: handleSelectAgent,
    handle_select_conversation: handleSelectConversation,
    handle_back_to_directory: handleBackToDirectory,
    handle_delete_agent: handleDeleteAgent,
    handle_create_conversation: handleCreateConversation,
    handle_save_agent_options: handleSaveAgentOptions,
    handle_save_existing_agent_options: handleSaveExistingRoomMemberOptions,
    handle_validate_agent_name: handleValidateAgentName,
    handle_validate_agent_name_for_agent: handleValidateAgentNameForAgent,
    handle_open_conversation_from_launcher: handleOpenConversationFromLauncher,
    handle_refresh_room_state: handleRefreshRoomState,
    handle_conversation_snapshot_change: handleConversationSnapshotChange,
    handle_close_conversation: handleCloseConversation,
    handle_delete_conversation: handleDeleteConversation,
    handle_update_conversation_title: handleUpdateConversationTitle,
    handle_update_room: handleUpdateRoom,
    handle_delete_room: handleDeleteRoom,
    handle_add_room_member: handleAddRoomMember,
    handle_remove_room_member: handleRemoveRoomMember,
    route_room_id: roomId ?? null,
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
