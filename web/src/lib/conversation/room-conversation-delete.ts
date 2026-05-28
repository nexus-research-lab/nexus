import { I18nContextValue } from "@/shared/i18n/i18n-context";
import { RoomConversationView } from "@/types/conversation/conversation";

export interface ConversationDeleteState {
  enabled: boolean;
  reason: string | null;
}

export function resolve_room_conversation_delete_state(
  conversation: RoomConversationView,
  can_manage_conversations: boolean,
  t: I18nContextValue["t"],
): ConversationDeleteState {
  if (!can_manage_conversations) {
    return { enabled: false, reason: t("room.delete_no_permission") };
  }

  if (conversation.conversation_type !== "topic") {
    return { enabled: false, reason: t("room.delete_main_locked") };
  }

  return { enabled: true, reason: null };
}
