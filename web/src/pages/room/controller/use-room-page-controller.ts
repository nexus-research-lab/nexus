/* @refresh reset */
// Room 页面入口聚合多个 Hook，热更新时重挂页面可避免 Hook 签名迁移打断当前对话。
"use client";

import { useExistingAgentOptionsCommands } from "@/features/agents/options/use-existing-agent-options-commands";
import { useHomeWorkspaceController } from "@/hooks/home/use-home-workspace-controller";
import { useAgentStore } from "@/store/agent";
import { useConversationStore } from "@/store/conversation";
import type { RoomPageControllerOptions } from "@/types/app/route";

import { useRoomPageCommands } from "./commands/use-room-page-commands";
import { useRoomConversationSnapshot } from "./model/use-room-conversation-snapshot";
import { useRoomPageModel } from "./model/page/use-room-page-model";
import { useRoomPageData } from "./use-room-page-data";

export function useRoomPageController({
  roomId,
  conversationId,
  sessionKey,
}: RoomPageControllerOptions) {
  // 页面只订阅实际消费的 Store 字段，避免无关 Agent 状态使 Room 整页重渲染。
  const agents = useAgentStore((state) => state.agents);
  const updateAgent = useAgentStore((state) => state.update_agent);
  const loadAgentsFromServer = useAgentStore(
    (state) => state.load_agents_from_server,
  );
  const syncConversationSnapshot = useConversationStore(
    (state) => state.sync_conversation_snapshot,
  );
  const data = useRoomPageData({ roomId });
  const model = useRoomPageModel({
    agents,
    conversationId,
    roomContexts: data.roomContexts,
    roomId,
    sessionKey,
  });
  const agentOptions = useExistingAgentOptionsCommands({ updateAgent });
  const commands = useRoomPageCommands({
    roomId,
    roomMembers: model.room.members,
    refreshRoomContexts: data.refreshRoomContexts,
    saveExistingAgentOptions: agentOptions.saveAgentOptions,
  });
  const handleConversationSnapshotChange = useRoomConversationSnapshot({
    activeRoomSessionId: model.conversation.activeSession?.id ?? null,
    currentConversationId:
      model.conversation.currentContext?.conversation.id ?? null,
    currentIdentity: model.agent.sessionIdentity,
    setRoomContexts: data.setRoomContexts,
    syncConversationSnapshot,
  });
  const workspace = useHomeWorkspaceController({
    currentAgentId: model.agent.current?.agent_id ?? null,
    workspaceAgentIds: model.agent.workspaceIds,
  });

  return {
    actions: {
      closeConversation: commands.handleCloseConversation,
      createConversation: commands.handleCreateConversation,
      deleteConversation: commands.handleDeleteConversation,
      manageRoom: commands.handleManageRoom,
      prepareAgentCatalog: loadAgentsFromServer,
      refreshRoomState: commands.handleRefreshRoomState,
      saveAgentOptions: commands.handleSaveExistingRoomMemberOptions,
      updateConversationTitle: commands.handleUpdateConversationTitle,
      validateAgentName: agentOptions.validateAgentName,
    },
    agent: model.agent,
    conversation: {
      ...model.conversation,
      handleSnapshotChange: handleConversationSnapshotChange,
    },
    room: model.room,
    status: {
      isHydrated: !data.isRoomLoading,
    },
    workspace,
  };
}
