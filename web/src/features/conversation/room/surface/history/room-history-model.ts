import {
  getExternalSessionConversationLabel,
  isExternalSessionConversation,
} from "@/lib/conversation/external-session";
import type { RoomConversationView } from "@/types/conversation/conversation";

export interface RoomHistoryEntry {
  conversation: RoomConversationView;
  externalSessionLabel: string | null;
  isActive: boolean;
  canDelete: boolean;
  canRename: boolean;
}

function compareByRecentActivity(
  left: RoomConversationView,
  right: RoomConversationView,
): number {
  return right.last_activity_at - left.last_activity_at
    || right.created_at - left.created_at
    || left.conversation_id.localeCompare(right.conversation_id);
}

export function buildRoomHistoryEntries({
  conversations,
  currentConversationId,
  canManageConversations,
  canUpdateConversationTitle,
}: {
  conversations: RoomConversationView[];
  currentConversationId: string | null;
  canManageConversations: boolean;
  canUpdateConversationTitle: boolean;
}): RoomHistoryEntry[] {
  const conversationCount = conversations.length;
  return [...conversations]
    .sort(compareByRecentActivity)
    .map((conversation) => {
      const isExternalSession = isExternalSessionConversation(conversation);
      return {
        conversation,
        externalSessionLabel: getExternalSessionConversationLabel(conversation),
        isActive: conversation.conversation_id === currentConversationId,
        canDelete: (
          !isExternalSession
          && canManageConversations
          && conversation.conversation_type === "topic"
          && conversationCount > 1
        ),
        canRename: (
          !isExternalSession
          && canManageConversations
          && canUpdateConversationTitle
        ),
      };
    });
}
