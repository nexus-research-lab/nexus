import { useMemo } from "react";

import { isMainAgent } from "@/config/runtime-options";
import { buildExternalSessionConversationId } from "@/lib/conversation/external-session";
import type { Agent, AgentSession } from "@/types/agent/agent";
import type { AgentConversationIdentity } from "@/types/agent/agent-conversation";
import type { RoomConversationView } from "@/types/conversation/conversation";
import type { RoomContextAggregate } from "@/types/conversation/room";

import {
  buildRoomConversationViews,
  resolveCurrentRoomContext,
  resolveSelectedConversationId,
} from "./room-conversation-model";
import { resolveRoomMemberAgents } from "./room-member-model";
import { resolveCurrentAgentSessionIdentity } from "./room-session-model";
import { useRoomExternalSessions } from "./use-room-external-sessions";

interface UseRoomPageModelOptions {
  agents: Agent[];
  conversationId?: string | null;
  roomContexts: RoomContextAggregate[];
  roomId?: string | null;
  sessionKey?: string | null;
}

function resolveExternalSessionIdentity(
  routeSessionKey: string,
  currentAgent: Agent,
  externalAgentSessions: AgentSession[],
): AgentConversationIdentity {
  const externalSession = externalAgentSessions.find(
    (session) => session.session_key === routeSessionKey,
  );
  return {
    session_key: routeSessionKey,
    agent_id: externalSession?.agent_id ?? currentAgent.agent_id,
    chat_type: externalSession?.chat_type === "group" ? "group" : "dm",
  };
}

export function useRoomPageModel({
  agents,
  conversationId,
  roomContexts,
  roomId,
  sessionKey,
}: UseRoomPageModelOptions) {
  const scopedRoomContexts = useMemo(
    () => roomContexts.filter((context) => context.room.id === roomId),
    [roomContexts, roomId],
  );
  const currentRoom = scopedRoomContexts[0]?.room ?? null;
  const roomMemberAgents = useMemo(
    () => resolveRoomMemberAgents(scopedRoomContexts),
    [scopedRoomContexts],
  );
  const workspaceAgentIds = useMemo(
    () => roomMemberAgents.map((agent) => agent.agent_id),
    [roomMemberAgents],
  );
  const baseRoomConversations = useMemo<RoomConversationView[]>(
    () => buildRoomConversationViews(scopedRoomContexts),
    [scopedRoomContexts],
  );
  const selectedBaseConversationId = useMemo(
    () => resolveSelectedConversationId(conversationId, baseRoomConversations),
    [baseRoomConversations, conversationId],
  );
  const currentRoomContext = useMemo(
    () => resolveCurrentRoomContext(scopedRoomContexts, selectedBaseConversationId),
    [scopedRoomContexts, selectedBaseConversationId],
  );
  const activeRoomSession = currentRoomContext?.sessions[0] ?? null;
  const currentAgent = useMemo(
    () => roomMemberAgents.find(
      (agent) => agent.agent_id === activeRoomSession?.agent_id,
    ) ?? null,
    [activeRoomSession?.agent_id, roomMemberAgents],
  );
  const routeSessionKey = sessionKey?.trim() || null;
  const externalSessions = useRoomExternalSessions({
    agentId: currentAgent?.agent_id ?? null,
    roomId: currentRoom?.id ?? null,
    roomType: currentRoom?.room_type ?? null,
  });
  const currentRoomConversations = useMemo(
    () => [...baseRoomConversations, ...externalSessions.externalRoomConversations]
      .sort((left, right) => right.last_activity_at - left.last_activity_at),
    [baseRoomConversations, externalSessions.externalRoomConversations],
  );
  const selectedConversationId = routeSessionKey
    ? buildExternalSessionConversationId(routeSessionKey)
    : selectedBaseConversationId;
  const currentRoomConversation = useMemo(
    () => currentRoomConversations.find(
      (conversation) => conversation.conversation_id === selectedConversationId,
    ) ?? null,
    [currentRoomConversations, selectedConversationId],
  );
  const currentAgentSessionIdentity = useMemo<AgentConversationIdentity | null>(() => {
    if (routeSessionKey && currentAgent) {
      return resolveExternalSessionIdentity(
        routeSessionKey,
        currentAgent,
        externalSessions.externalAgentSessions,
      );
    }
    return resolveCurrentAgentSessionIdentity({
      currentRoomId: currentRoom?.id ?? null,
      currentConversationId: currentRoomContext?.conversation.id ?? null,
      activeRoomSession,
      currentRoomType: currentRoom?.room_type ?? "dm",
    });
  }, [
    activeRoomSession,
    currentAgent,
    currentRoom?.id,
    currentRoom?.room_type,
    currentRoomContext?.conversation.id,
    externalSessions.externalAgentSessions,
    routeSessionKey,
  ]);
  const availableRoomAgents = useMemo(() => {
    const joinedAgentIds = new Set(roomMemberAgents.map((agent) => agent.agent_id));
    return agents.filter((agent) => (
      !joinedAgentIds.has(agent.agent_id) && !isMainAgent(agent.agent_id)
    ));
  }, [agents, roomMemberAgents]);

  return {
    currentRoom,
    roomMemberAgents,
    workspaceAgentIds,
    currentRoomContext,
    activeRoomSession,
    currentAgent,
    currentRoomConversations,
    selectedConversationId,
    currentRoomConversation,
    currentAgentSessionIdentity,
    availableRoomAgents,
  };
}
