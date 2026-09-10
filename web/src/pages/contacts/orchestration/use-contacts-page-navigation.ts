/**
 * INPUT: 联系人目录、URL 查询参数与 Agent/Room 命令。
 * OUTPUT: 目录、详情、私聊和群聊之间的稳定路由动作。
 * POS: Contacts 页面唯一的路由协调入口。
 */
import { useCallback, useEffect, useRef } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";

import { AppRouteBuilders } from "@/shared/navigation/route-paths";
import { resolveDirectRoomNavigationTarget } from "@/features/navigation/direct-room/direct-room-navigation";
import { createRoom } from "@/lib/api/conversation/room-command-api";
import type { Agent } from "@/types/agent/agent";

interface UseContactsPageNavigationOptions {
  agents: Agent[];
  loading: boolean;
  confirmDeleteAgent: () => Promise<string | null>;
  closeAgentEditor: () => void;
  openCreateAgent: () => void;
}

export function useContactsPageNavigation({
  agents,
  loading,
  confirmDeleteAgent,
  closeAgentEditor,
  openCreateAgent,
}: UseContactsPageNavigationOptions) {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const selectedAgentId = searchParams.get("agent");
  const createAgentRequested = searchParams.get("view") === "create";
  const openedCreateAgentFromRoute = useRef(false);
  const selectedAgent = selectedAgentId
    ? agents.find((agent) => agent.agent_id === selectedAgentId) ?? null
    : null;

  useEffect(() => {
    if (selectedAgentId && !loading && !selectedAgent) {
      navigate(AppRouteBuilders.contacts(), {replace: true});
    }
  }, [loading, navigate, selectedAgent, selectedAgentId]);

  useEffect(() => {
    if (createAgentRequested) {
      openedCreateAgentFromRoute.current = true;
      openCreateAgent();
      return;
    }
    if (openedCreateAgentFromRoute.current) {
      openedCreateAgentFromRoute.current = false;
      closeAgentEditor();
    }
  }, [closeAgentEditor, createAgentRequested, openCreateAgent]);

  const openDirectRoom = useCallback((agentId: string) => {
    void resolveDirectRoomNavigationTarget(agentId).then(({route}) => {
      navigate(route);
    });
  }, [navigate]);
  const openAgent = useCallback((agentId: string) => {
    navigate(AppRouteBuilders.contactAgent(agentId));
  }, [navigate]);
  const openDirectory = useCallback(() => {
    navigate(AppRouteBuilders.contacts());
  }, [navigate]);

  const createTeam = useCallback((agentId: string) => {
    void createRoom({agent_ids: [agentId]}).then((context) => {
      navigate(AppRouteBuilders.roomConversation(
        context.room.id,
        context.conversation.id,
      ));
    });
  }, [navigate]);

  const confirmDelete = useCallback(async () => {
    const deletedAgentId = await confirmDeleteAgent();
    if (deletedAgentId && selectedAgentId === deletedAgentId) {
      navigate(AppRouteBuilders.contacts(), {replace: true});
    }
  }, [confirmDeleteAgent, navigate, selectedAgentId]);
  const closeEditor = useCallback(() => {
    closeAgentEditor();
    if (createAgentRequested) {
      navigate(AppRouteBuilders.contacts(), {replace: true});
    }
  }, [closeAgentEditor, createAgentRequested, navigate]);

  return {
    selectedAgent,
    closeEditor,
    openAgent,
    openDirectory,
    openDirectRoom,
    createTeam,
    confirmDelete,
  };
}
