"use client";

import { useEffect } from "react";

import { useHomeAgentSessionController } from "@/hooks/use-home-agent-session-controller";
import { useHomeWorkspaceController } from "@/hooks/use-home-workspace-controller";

interface UseRoomPageControllerOptions {
  roomId?: string | null;
  conversationId?: string | null;
}

export function useRoomPageController({
  roomId,
  conversationId,
}: UseRoomPageControllerOptions) {
  const agentSession = useHomeAgentSessionController();
  const {
    agents,
    currentAgentId,
    currentConversation,
    currentConversationId,
    handleAgentSelect,
    handleConversationSelect,
    isHydrated,
    sessions,
  } = agentSession;
  const workspace = useHomeWorkspaceController({
    currentAgentId,
    currentSession: currentConversation,
  });

  useEffect(() => {
    if (!isHydrated) {
      return;
    }

    if (conversationId) {
      const targetSession = sessions.find((session) => session.session_key === conversationId);
      if (targetSession?.agent_id && targetSession.agent_id !== currentAgentId) {
        handleAgentSelect(targetSession.agent_id);
        return;
      }

      if (targetSession && targetSession.session_key !== currentConversationId) {
        handleConversationSelect(targetSession.session_key);
        return;
      }
    }

    if (!roomId) {
      return;
    }

    // 当前后端仍以 agent 维度承载 room 工作台，这里先做兼容路由映射。
    if (roomId !== currentAgentId) {
      const matchedAgent = agents.find((agent) => agent.agent_id === roomId);
      if (matchedAgent) {
        handleAgentSelect(matchedAgent.agent_id);
      }
    }
  }, [
    agents,
    conversationId,
    currentAgentId,
    currentConversationId,
    handleAgentSelect,
    handleConversationSelect,
    isHydrated,
    roomId,
    sessions,
  ]);

  return {
    ...agentSession,
    ...workspace,
    routeConversationId: conversationId ?? null,
    routeRoomId: roomId ?? null,
  };
}
