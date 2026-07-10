import type { AgentConversationIdentity } from "@/types/agent/agent-conversation";
import type { SubagentTaskSource } from "@/types/conversation/subagent-task";

export function resolveRoomSubagentTaskSource({
  conversationId,
  isDm,
  roomId,
  sessionIdentity,
}: {
  conversationId: string | null;
  isDm: boolean;
  roomId: string | null;
  sessionIdentity: AgentConversationIdentity | null;
}): SubagentTaskSource | null {
  if (isDm) {
    const sessionKey = sessionIdentity?.session_key?.trim();
    return sessionKey ? { kind: "session", session_key: sessionKey } : null;
  }
  if (!roomId || !conversationId) {
    return null;
  }
  return {
    kind: "room",
    room_id: roomId,
    conversation_id: conversationId,
  };
}
