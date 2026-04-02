import type { Conversation } from "@/types/conversation";

import { parseRoomConversationId } from "@/lib/session-key";

export function getConversationRouteId(
  conversation: Pick<Conversation, "conversation_id" | "session_key">,
): string {
  return (
    conversation.conversation_id ||
    parseRoomConversationId(conversation.session_key) ||
    conversation.session_key
  );
}
