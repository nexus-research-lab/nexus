"use client";

import { useEffect } from "react";

import { useHomeAgentConversationController } from "@/hooks/use-home-agent-conversation-controller";
import { useOpenRoomController } from "@/hooks/use-open-room-controller";
import { useHomeWorkspaceController } from "@/hooks/use-home-workspace-controller";
import { useProtocolRoomController } from "@/hooks/use-protocol-room-controller";
import { RoomPageControllerOptions } from "@/types/route";

export function useRoomPageController({
  room_id,
  conversation_id,
}: RoomPageControllerOptions) {
  const protocol_room = useProtocolRoomController({ room_id });
  const open_room = useOpenRoomController({ room_id });
  const agent_conversation = useHomeAgentConversationController();
  const {
    agents,
    current_agent,
    current_agent_id,
    current_conversation,
    current_conversation_id,
    handle_select_agent,
    handle_select_conversation,
    is_hydrated,
    conversations,
    recent_agents,
    dialog_initial_options,
    dialog_initial_title,
  } = agent_conversation;
  const workspace = useHomeWorkspaceController({
    current_agent_id,
    current_conversation,
  });

  useEffect(() => {
    if (!is_hydrated) {
      return;
    }

    if (room_id && (!protocol_room.is_checked || !open_room.is_open_room_checked)) {
      return;
    }

    if (protocol_room.is_protocol_room || open_room.is_open_room) {
      return;
    }

    if (conversation_id) {
      const target_conversation = conversations.find((conversation) => conversation.session_key === conversation_id);
      if (target_conversation?.agent_id && target_conversation.agent_id !== current_agent_id) {
        handle_select_agent(target_conversation.agent_id);
        return;
      }

      if (target_conversation && target_conversation.session_key !== current_conversation_id) {
        handle_select_conversation(target_conversation.session_key);
        return;
      }
    }

    if (!room_id) {
      return;
    }

    // 当前后端仍以 agent 维度承载 room 工作台，这里先做兼容路由映射。
    if (room_id !== current_agent_id) {
      const matched_agent = agents.find((agent) => agent.agent_id === room_id);
      if (matched_agent) {
        handle_select_agent(matched_agent.agent_id);
      }
    }
  }, [
    agents,
    conversation_id,
    conversations,
    current_agent_id,
    current_conversation_id,
    handle_select_agent,
    handle_select_conversation,
    is_hydrated,
    open_room.is_open_room_checked,
    open_room.is_open_room,
    protocol_room.is_checked,
    protocol_room.is_protocol_room,
    room_id,
  ]);

  return {
    ...protocol_room,
    ...open_room,
    ...agent_conversation,
    ...workspace,
    current_agent,
    current_agent_id,
    current_conversation,
    current_conversation_id,
    recent_agents,
    dialog_initial_options,
    dialog_initial_title,
    route_conversation_id: conversation_id ?? null,
    route_room_id: room_id ?? null,
  };
}
