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
  const agent_session = useHomeAgentSessionController();
  const {
    agents,
    current_agent,
    current_agent_id,
    current_conversation,
    current_conversation_id,
    handle_select_agent,
    handle_select_conversation,
    isHydrated,
    conversations,
    recent_agents,
    dialog_initial_options,
    dialog_initial_title,
  } = agent_session;
  const workspace = useHomeWorkspaceController({
    current_agent_id,
    current_conversation,
  });

  useEffect(() => {
    if (!isHydrated) {
      return;
    }

    if (conversationId) {
      const target_conversation = conversations.find((conversation) => conversation.session_key === conversationId);
      if (target_conversation?.agent_id && target_conversation.agent_id !== current_agent_id) {
        handle_select_agent(target_conversation.agent_id);
        return;
      }

      if (target_conversation && target_conversation.session_key !== current_conversation_id) {
        handle_select_conversation(target_conversation.session_key);
        return;
      }
    }

    if (!roomId) {
      return;
    }

    // 当前后端仍以 agent 维度承载 room 工作台，这里先做兼容路由映射。
    if (roomId !== current_agent_id) {
      const matchedAgent = agents.find((agent) => agent.agent_id === roomId);
      if (matchedAgent) {
        handle_select_agent(matchedAgent.agent_id);
      }
    }
  }, [
    agents,
    conversationId,
    conversations,
    current_agent_id,
    current_conversation_id,
    handle_select_agent,
    handle_select_conversation,
    isHydrated,
    roomId,
  ]);

  return {
    ...agent_session,
    ...workspace,
    current_agent,
    current_agent_id,
    current_conversation,
    current_conversation_id,
    recent_agents,
    dialog_initial_options,
    dialog_initial_title,
    route_conversation_id: conversationId ?? null,
    route_room_id: roomId ?? null,
  };
}
