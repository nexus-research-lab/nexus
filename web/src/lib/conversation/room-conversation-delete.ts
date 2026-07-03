import { I18nContextValue } from "@/shared/i18n/i18n-context";
import { RoomConversationView } from "@/types/conversation/conversation";

export interface ConversationDeleteState {
  enabled: boolean;
  reason: string | null;
}

export function resolveRoomConversationDeleteState(
  conversation: RoomConversationView,
  conversationCount: number,
  canManageConversations: boolean,
  t: I18nContextValue["t"],
): ConversationDeleteState {
  if (!canManageConversations) {
    return { enabled: false, reason: t("room.delete_no_permission") };
  }

  if (conversation.conversation_type !== "topic") {
    return { enabled: false, reason: t("room.delete_main_locked") };
  }

  if (conversationCount <= 1) {
    return { enabled: false, reason: t("room.delete_keep_one") };
  }

  return { enabled: true, reason: null };
}
