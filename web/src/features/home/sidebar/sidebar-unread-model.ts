import {
  buildChatNotificationTargetKey,
  isChatNotificationTargetActive,
  type ActiveChatNotificationTarget,
} from "@/features/home/notifications/chat-notification-target";
import type { ChatNotificationTargetState } from "@/store/sidebar";

import type { SidebarConversationItem } from "./sidebar-conversation-model";

interface UnreadCandidate {
  count: number;
  target: ChatNotificationTargetState;
  timestamp: number;
}

interface UnreadProjectionInput {
  chatUnreadCounts: Record<string, number>;
  chatUnreadTargets: Record<string, ChatNotificationTargetState>;
  chatUnreadTimestamps: Record<string, number>;
  notificationKey?: string | null;
  roomId?: string | null;
  sessionKey?: string | null;
}

interface SidebarUnreadProjectionInput {
  activeTarget: ActiveChatNotificationTarget | null;
  chatUnreadCounts: Record<string, number>;
  chatUnreadTargets: Record<string, ChatNotificationTargetState>;
  chatUnreadTimestamps: Record<string, number>;
  items: SidebarConversationItem[];
}

const EMPTY_UNREAD_STATE = {
  unreadConversationId: null,
  unreadCount: 0,
  unreadTargetKey: null,
} as const;

export function projectSidebarUnreadItems({
  activeTarget,
  chatUnreadCounts,
  chatUnreadTargets,
  chatUnreadTimestamps,
  items,
}: SidebarUnreadProjectionInput): SidebarConversationItem[] {
  return items.map((item) => {
    const notificationKey = buildSidebarItemNotificationKey(item);
    const projectedItem = { ...item, notificationKey };
    const unreadState = getSidebarItemUnreadState({
      chatUnreadCounts,
      chatUnreadTargets,
      chatUnreadTimestamps,
      notificationKey,
      roomId: item.roomId,
      sessionKey: item.sessionKey,
    });
    return {
      ...projectedItem,
      ...(isActiveSidebarChatItem(projectedItem, activeTarget)
        ? EMPTY_UNREAD_STATE
        : unreadState),
    };
  });
}

function isActiveSidebarChatItem(
  item: SidebarConversationItem,
  activeTarget: ActiveChatNotificationTarget | null,
): boolean {
  return isChatNotificationTargetActive(activeTarget, {
    key: item.notificationKey,
    room_id: item.roomId,
  });
}

function buildSidebarItemNotificationKey(
  item: SidebarConversationItem,
): string | null {
  return buildChatNotificationTargetKey({
    conversation_id: item.conversationId,
    room_id: item.roomId,
    session_key: item.sessionKey,
  });
}

function getSidebarItemUnreadState(
  input: UnreadProjectionInput,
): {
  unreadConversationId: string | null;
  unreadCount: number;
  unreadTargetKey: string | null;
} {
  const candidates = collectUnreadCandidates(input);
  let unreadCount = 0;
  let newestCandidate: UnreadCandidate | null = null;

  for (const candidate of candidates.values()) {
    unreadCount += candidate.count;
    if (!newestCandidate || candidate.timestamp >= newestCandidate.timestamp) {
      newestCandidate = candidate;
    }
  }

  return {
    unreadConversationId: newestCandidate?.target.conversation_id ?? null,
    unreadCount,
    unreadTargetKey: newestCandidate?.target.key ?? null,
  };
}

function collectUnreadCandidates(
  input: UnreadProjectionInput,
): Map<string, UnreadCandidate> {
  const candidates = new Map<string, UnreadCandidate>();
  const roomId = input.roomId?.trim();

  if (roomId) {
    collectRoomCandidates(candidates, roomId, input);
  } else if (input.notificationKey) {
    addCandidate(candidates, input.notificationKey, input, {
      key: input.notificationKey,
      room_id: input.roomId,
    });
  }

  const sessionKey = buildChatNotificationTargetKey({ session_key: input.sessionKey });
  if (sessionKey) {
    addCandidate(candidates, sessionKey, input, {
      conversation_id: null,
      key: sessionKey,
      room_id: input.roomId,
      session_key: input.sessionKey,
    });
  }
  return candidates;
}

function collectRoomCandidates(
  candidates: Map<string, UnreadCandidate>,
  roomId: string,
  input: UnreadProjectionInput,
): void {
  for (const [key, target] of Object.entries(input.chatUnreadTargets)) {
    if (target.room_id === roomId) {
      addCandidate(candidates, key, input, target);
    }
  }

  const roomKey = `room:${roomId}`;
  const conversationPrefix = `${roomKey}:conversation:`;
  for (const key of Object.keys(input.chatUnreadCounts)) {
    if (key !== roomKey && !key.startsWith(conversationPrefix)) {
      continue;
    }
    addCandidate(candidates, key, input, {
      conversation_id: key.startsWith(conversationPrefix)
        ? key.slice(conversationPrefix.length)
        : null,
      key,
      room_id: roomId,
    });
  }
}

function addCandidate(
  candidates: Map<string, UnreadCandidate>,
  key: string,
  input: UnreadProjectionInput,
  fallbackTarget: ChatNotificationTargetState,
): void {
  if (candidates.has(key)) {
    return;
  }
  const count = input.chatUnreadCounts[key] ?? 0;
  if (count <= 0) {
    return;
  }
  candidates.set(key, {
    count,
    target: input.chatUnreadTargets[key] ?? fallbackTarget,
    timestamp: input.chatUnreadTimestamps[key] ?? 0,
  });
}
