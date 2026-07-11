import { buildRoomAgentSessionKey, buildRoomSharedSessionKey } from "@/lib/conversation/session-key";
import type { AgentConversationIdentity } from "@/types/agent/agent-conversation";
import type { RoomContextAggregate } from "@/types/conversation/room";

interface ResolveRoomSessionIdentityOptions {
  currentRoomId: string | null;
  currentConversationId: string | null;
  activeRoomSession: RoomContextAggregate["sessions"][number] | null;
  currentRoomType: string;
}

export function resolveCurrentAgentSessionIdentity({
  currentRoomId,
  currentConversationId,
  activeRoomSession,
  currentRoomType,
}: ResolveRoomSessionIdentityOptions): AgentConversationIdentity | null {
  const agentId = activeRoomSession?.agent_id ?? null;
  const conversationId = currentConversationId ?? activeRoomSession?.conversation_id ?? null;
  if (!conversationId) {
    return null;
  }

  return {
    session_key: currentRoomType === "dm" && agentId
      ? buildRoomAgentSessionKey(conversationId, agentId, "dm")
      : buildRoomSharedSessionKey(conversationId),
    agent_id: agentId,
    room_id: currentRoomId,
    conversation_id: conversationId,
    room_session_id: activeRoomSession?.id ?? null,
    chat_type: currentRoomType === "dm" ? "dm" : "group",
  };
}
