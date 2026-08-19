import { getSessionKeyIdentity } from "@/lib/conversation/session-key";

interface AgentConversationIdentitySource {
  chat_type: string;
  conversation_id?: string | null;
  room_session_id?: string | null;
  session_key: string | null;
}

export function getAgentConversationIdentityKey(
  identity: AgentConversationIdentitySource | null | undefined,
): string | null {
  if (!identity) {
    return null;
  }

  const scopedIdentity = resolveScopedConversationIdentity(identity);
  if (scopedIdentity) {
    return scopedIdentity;
  }

  const sessionIdentity = getSessionKeyIdentity(identity.session_key);
  return sessionIdentity ? `session:${sessionIdentity}` : null;
}

function resolveScopedConversationIdentity(
  identity: AgentConversationIdentitySource,
): string | null {
  // A Room conversation is one shared logical timeline. Its physical
  // room_session_id may change when another member starts, reconnects, or
  // becomes the first projected session; using that process identity here
  // would clear live Room state even though the user never changed chats.
  if (identity.chat_type === "group" && identity.conversation_id) {
    return `room-conversation:${identity.conversation_id}`;
  }
  if (identity.room_session_id) {
    return `room-session:${identity.room_session_id}`;
  }
  return null;
}
