/* @refresh reset */
// Room 页面入口聚合多个 Hook，热更新时重挂页面可避免 Hook 签名迁移打断当前对话。
"use client";

import { useCallback } from "react";

import { useHomeWorkspaceController } from "@/hooks/home/use-home-workspace-controller";
import { useAgentStore } from "@/store/agent";
import { useConversationStore } from "@/store/conversation";
import type { RoomPageControllerOptions } from "@/types/app/route";

import { useRoomAgentOptions } from "./use-room-agent-options";
import { useRoomConversationSnapshot } from "./use-room-conversation-snapshot";
import { useRoomPageCommands } from "./use-room-page-commands";
import { useRoomPageData } from "./use-room-page-data";
import { useRoomPageModel } from "./use-room-page-model";

export function useRoomPageController({
  roomId,
  conversationId,
  sessionKey,
}: RoomPageControllerOptions) {
  // 页面只订阅实际消费的 Store 字段，避免无关 Agent 状态使 Room 整页重渲染。
  const agents = useAgentStore((state) => state.agents);
  const updateAgent = useAgentStore((state) => state.update_agent);
  const loadAgentsFromServer = useAgentStore((state) => state.load_agents_from_server);
  const syncConversationSnapshot = useConversationStore(
    (state) => state.sync_conversation_snapshot,
  );
  const data = useRoomPageData({roomId});
  const model = useRoomPageModel({
    agents,
    conversationId,
    roomContexts: data.roomContexts,
    roomId,
    sessionKey,
  });
  const agentOptions = useRoomAgentOptions({updateAgent});
  const commands = useRoomPageCommands({
    roomId,
    refreshRoomContexts: data.refreshRoomContexts,
    saveExistingAgentOptions: agentOptions.saveExistingAgentOptions,
  });
  const handleConversationSnapshotChange = useRoomConversationSnapshot({
    activeRoomSessionId: model.activeRoomSession?.id ?? null,
    currentConversationId: model.currentRoomContext?.conversation.id ?? null,
    currentIdentity: model.currentAgentSessionIdentity,
    setRoomContexts: data.setRoomContexts,
    syncConversationSnapshot,
  });
  const workspace = useHomeWorkspaceController({
    currentAgentId: model.currentAgent?.agent_id ?? null,
    workspaceAgentIds: model.workspaceAgentIds,
  });
  const handlePrepareRoomAgentCatalog = useCallback(async () => {
    await loadAgentsFromServer();
  }, [loadAgentsFromServer]);

  return {
    roomError: data.roomError,
    currentRoom: model.currentRoom,
    currentRoomType: model.currentRoom?.room_type ?? "room",
    currentRoomTitle: model.currentRoom?.name?.trim()
      || model.currentAgent?.name
      || "未命名 room",
    currentRoomSkillNames: model.currentRoom?.skill_names ?? [],
    roomMembers: model.roomMemberAgents,
    availableRoomAgents: model.availableRoomAgents,
    currentAgent: model.currentAgent,
    currentRoomConversations: model.currentRoomConversations,
    currentRoomConversation: model.currentRoomConversation,
    currentAgentSessionIdentity: model.currentAgentSessionIdentity,
    conversationId: model.selectedConversationId,
    isHydrated: data.isBootstrapped && !data.isRoomLoading,
    handlePrepareRoomAgentCatalog,
    handleSaveExistingAgentOptions: commands.handleSaveExistingRoomMemberOptions,
    handleValidateAgentNameForAgent: agentOptions.validateAgentName,
    handleConversationSnapshotChange,
    handleRefreshRoomState: commands.handleRefreshRoomState,
    handleCloseConversation: commands.handleCloseConversation,
    handleDeleteConversation: commands.handleDeleteConversation,
    handleCreateConversation: commands.handleCreateConversation,
    handleUpdateConversationTitle: commands.handleUpdateConversationTitle,
    handleUpdateRoom: commands.handleUpdateRoom,
    handleAddRoomMember: commands.handleAddRoomMember,
    handleRemoveRoomMember: commands.handleRemoveRoomMember,
    routeRoomId: roomId ?? null,
    ...workspace,
  };
}
