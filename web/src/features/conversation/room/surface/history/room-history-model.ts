/**
 * INPUT: Room 会话快照、当前会话与管理能力。
 * OUTPUT: 排除内部草稿后，按活动时间排序、投影管理资格并可分成普通历史/IM 的条目。
 * POS: Room 历史菜单的纯协议到展示能力投影。
 */

import {
  getExternalSessionConversationLabel,
  isExternalSessionConversation,
  isExternalSessionConversationDeletable,
} from "@/lib/conversation/external-session";
import type { RoomConversationView } from "@/types/conversation/conversation";

export interface RoomHistoryEntry {
  conversation: RoomConversationView;
  externalSessionLabel: string | null;
  isActive: boolean;
  canBulkDelete: boolean;
  canDelete: boolean;
  canRename: boolean;
}

export interface RoomHistoryEntryGroups {
  history: RoomHistoryEntry[];
  im: RoomHistoryEntry[];
}

function compareByRecentActivity(
  left: RoomConversationView,
  right: RoomConversationView,
): number {
  return right.last_activity_at - left.last_activity_at
    || right.created_at - left.created_at
    || left.conversation_id.localeCompare(right.conversation_id);
}

export function filterRoomHistoryConversations(
  conversations: readonly RoomConversationView[],
): RoomConversationView[] {
  return conversations.filter((conversation) => (
    isExternalSessionConversation(conversation)
    || conversation.is_draft !== true
  ));
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
  const localConversationCount = conversations.filter(
    (conversation) => !isExternalSessionConversation(conversation),
  ).length;
  return filterRoomHistoryConversations(conversations)
    .sort(compareByRecentActivity)
    .map((conversation) => {
      const isExternalSession = isExternalSessionConversation(conversation);
      const isActive = conversation.conversation_id === currentConversationId;
      const canDelete = (
        canManageConversations
        && (isExternalSession
          ? isExternalSessionConversationDeletable(conversation)
          : localConversationCount > 1)
      );
      return {
        conversation,
        externalSessionLabel: getExternalSessionConversationLabel(conversation),
        isActive,
        canBulkDelete: !isExternalSession && canManageConversations,
        canDelete,
        canRename: (
          !isExternalSession
          && canManageConversations
          && canUpdateConversationTitle
        ),
      };
    });
}

export function groupRoomHistoryEntries(
  entries: readonly RoomHistoryEntry[],
): RoomHistoryEntryGroups {
  const groups: RoomHistoryEntryGroups = { history: [], im: [] };
  for (const entry of entries) {
    groups[entry.externalSessionLabel ? "im" : "history"].push(entry);
  }
  return groups;
}
