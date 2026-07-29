/**
 * INPUT: 侧栏聊天条目、各目标未读计数、精确消息锚点与当前活动 Conversation。
 * OUTPUT: Room 聚合未读数及按最早 room_seq 选择的真实跳转 Conversation。
 * POS: Home 聊天侧栏未读纯投影；不清除 Store 或执行导航。
 */
import {
  buildChatNotificationTargetKey,
  isChatNotificationTargetActive,
  type ActiveChatNotificationTarget,
} from "@/features/home/notifications/chat-notification-target";
import type {
  ChatNotificationTargetState,
  ChatUnreadAnchorState,
  ChatUnreadMessageAnchor,
} from "@/store/sidebar";

import type { SidebarConversationItem } from "./sidebar-conversation-model";

interface UnreadCandidate {
  count: number;
  firstMessage: ChatUnreadMessageAnchor | null;
  target: ChatNotificationTargetState;
  timestamp: number;
}

interface UnreadProjectionInput {
  activeTarget: ActiveChatNotificationTarget | null;
  chatUnreadAnchors: Record<string, ChatUnreadAnchorState>;
  chatUnreadCounts: Record<string, number>;
  chatUnreadTargets: Record<string, ChatNotificationTargetState>;
  chatUnreadTimestamps: Record<string, number>;
  notificationKey?: string | null;
  roomId?: string | null;
  sessionKey?: string | null;
}

interface SidebarUnreadProjectionInput {
  activeTarget: ActiveChatNotificationTarget | null;
  chatUnreadAnchors: Record<string, ChatUnreadAnchorState>;
  chatUnreadCounts: Record<string, number>;
  chatUnreadTargets: Record<string, ChatNotificationTargetState>;
  chatUnreadTimestamps: Record<string, number>;
  items: SidebarConversationItem[];
}

export function projectSidebarUnreadItems({
  activeTarget,
  chatUnreadAnchors,
  chatUnreadCounts,
  chatUnreadTargets,
  chatUnreadTimestamps,
  items,
}: SidebarUnreadProjectionInput): SidebarConversationItem[] {
  return items.map((item) => {
    const notificationKey = buildSidebarItemNotificationKey(item);
    const projectedItem = { ...item, notificationKey };
    const unreadState = getSidebarItemUnreadState({
      activeTarget,
      chatUnreadAnchors,
      chatUnreadCounts,
      chatUnreadTargets,
      chatUnreadTimestamps,
      notificationKey,
      roomId: item.roomId,
      sessionKey: item.sessionKey,
    });
    return {
      ...projectedItem,
      ...unreadState,
    };
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
  let oldestCandidate: UnreadCandidate | null = null;

  for (const candidate of candidates.values()) {
    unreadCount += candidate.count;
    if (!oldestCandidate || compareUnreadCandidates(candidate, oldestCandidate) < 0) {
      oldestCandidate = candidate;
    }
  }

  return {
    unreadConversationId: oldestCandidate?.target.conversation_id ?? null,
    unreadCount,
    unreadTargetKey: oldestCandidate?.target.key ?? null,
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
  const target = input.chatUnreadTargets[key] ?? fallbackTarget;
  if (isChatNotificationTargetActive(input.activeTarget, target)) {
    return;
  }
  candidates.set(key, {
    count,
    firstMessage: input.chatUnreadAnchors[key]?.messages[0] ?? null,
    target,
    timestamp: input.chatUnreadTimestamps[key] ?? 0,
  });
}

function compareUnreadCandidates(
  left: UnreadCandidate,
  right: UnreadCandidate,
): number {
  const leftSequence = Number.isFinite(left.firstMessage?.room_seq)
    ? Number(left.firstMessage?.room_seq)
    : Number.POSITIVE_INFINITY;
  const rightSequence = Number.isFinite(right.firstMessage?.room_seq)
    ? Number(right.firstMessage?.room_seq)
    : Number.POSITIVE_INFINITY;
  return leftSequence - rightSequence
    || (left.firstMessage?.timestamp ?? left.timestamp)
      - (right.firstMessage?.timestamp ?? right.timestamp)
    || left.target.key.localeCompare(right.target.key);
}
