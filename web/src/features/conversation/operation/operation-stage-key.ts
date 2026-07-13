import type { AgentConversationIdentity } from "@/types/agent/agent-conversation";

export function buildOperationStageKey(
  identity: AgentConversationIdentity | null | undefined,
): string | null {
  if (!identity) {
    return null;
  }

  const room_session_id = identity.room_session_id?.trim();
  if (room_session_id) {
    return `room-session:${room_session_id}`;
  }

  const session_identity = stage_session_key_identity(identity.session_key);
  if (session_identity) {
    return `session:${session_identity}`;
  }

  const conversation_id = identity.conversation_id?.trim();
  if (conversation_id) {
    return `${identity.chat_type === "group" ? "room-conversation" : "dm-conversation"}:${conversation_id}`;
  }

  return null;
}

function stage_session_key_identity(session_key: string | null | undefined): string | null {
  const normalized_key = session_key?.trim();
  if (!normalized_key) {
    return null;
  }

  if (!normalized_key.startsWith("room:")) {
    return normalized_key;
  }

  const parts = normalized_key.split(":");
  const conversation_id = parts.slice(2).join(":").trim();
  return parts[1] === "group" && conversation_id
    ? `room:${conversation_id}`
    : normalized_key;
}
