import { useCallback, useEffect } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { resolveDirectRoomNavigationTarget } from "@/features/navigation/direct-room/direct-room-navigation";
import { createRoom } from "@/lib/api/conversation/room-command-api";
import type { Agent } from "@/types/agent/agent";

interface UseContactsPageNavigationOptions {
  agents: Agent[];
  loading: boolean;
  confirmDeleteAgent: () => Promise<string | null>;
}

export function useContactsPageNavigation({
  agents,
  loading,
  confirmDeleteAgent,
}: UseContactsPageNavigationOptions) {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const selectedAgentId = searchParams.get("agent");
  const selectedAgent = selectedAgentId
    ? agents.find((agent) => agent.agent_id === selectedAgentId) ?? null
    : null;

  useEffect(() => {
    if (selectedAgentId && !loading && !selectedAgent) {
      navigate(AppRouteBuilders.contacts(), {replace: true});
    }
  }, [loading, navigate, selectedAgent, selectedAgentId]);

  const openDirectRoom = useCallback((agentId: string) => {
    void resolveDirectRoomNavigationTarget(agentId).then(({route}) => {
      navigate(route);
    });
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
  const backToDirectory = useCallback(() => {
    navigate(AppRouteBuilders.contacts());
  }, [navigate]);

  return {
    selectedAgent,
    backToDirectory,
    openDirectRoom,
    createTeam,
    confirmDelete,
  };
}
