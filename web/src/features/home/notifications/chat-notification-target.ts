/**
 * INPUT: 聊天完成事件的 Room/Conversation/Session 身份与当前路由。
 * OUTPUT: 稳定通知 key、Room shared-session 判型、路由目标和只匹配精确 Conversation 的活动态判断。
 * POS: Home 通知与侧栏共用的目标身份纯模型。
 */
export interface ChatNotificationTargetInput {
  conversation_id?: string | null;
  room_id?: string | null;
  session_key?: string | null;
}

export interface ActiveChatNotificationTarget {
  conversation_id?: string | null;
  key: string;
  room_id?: string | null;
  session_key?: string | null;
}

export interface ChatNotificationTargetMatcher {
  key?: string | null;
  room_id?: string | null;
}

interface NormalizedChatNotificationTarget {
  conversationId: string;
  roomId: string;
  sessionKey: string;
}

type ChatRoomType = "dm" | "room" | null | undefined;

type ChatNotificationTargetKeyResolver = (
  target: NormalizedChatNotificationTarget,
) => string | null;

type ActiveTargetRouteResolver = (
  parts: string[],
  roomId: string,
) => ActiveChatNotificationTarget | null;

const CHAT_NOTIFICATION_TARGET_KEY_RESOLVERS: readonly ChatNotificationTargetKeyResolver[] = [
  ({conversationId, roomId}) => roomId && conversationId
    ? `room:${roomId}:conversation:${conversationId}`
    : null,
  ({roomId}) => roomId ? `room:${roomId}` : null,
  ({sessionKey}) => sessionKey ? `session:${sessionKey}` : null,
];

const ACTIVE_TARGET_ROUTE_RESOLVERS: Record<
  string,
  ActiveTargetRouteResolver
> = {
  conversations: resolveConversationRouteTarget,
  sessions: resolveSessionRouteTarget,
};

const ROOM_GROUP_SESSION_KEY_PREFIX = "room:group:";

export function isGroupRoomNotificationTarget(
  target: ChatNotificationTargetInput,
  roomType: ChatRoomType,
): boolean {
  if (roomType) {
    return roomType === "room";
  }
  return Boolean(
    target.room_id?.trim()
    && target.session_key?.trim().startsWith(ROOM_GROUP_SESSION_KEY_PREFIX),
  );
}

export function buildChatNotificationTargetKey({
  conversation_id: conversationId,
  room_id: roomId,
  session_key: sessionKey,
}: ChatNotificationTargetInput): string | null {
  const target = {
    conversationId: conversationId?.trim() ?? "",
    roomId: roomId?.trim() ?? "",
    sessionKey: sessionKey?.trim() ?? "",
  };
  for (const resolveKey of CHAT_NOTIFICATION_TARGET_KEY_RESOLVERS) {
    const key = resolveKey(target);
    if (key) {
      return key;
    }
  }
  return null;
}

export function getActiveChatTargetFromPath(
  pathname: string,
): ActiveChatNotificationTarget | null {
  const parts = pathname.split("/").filter(Boolean);
  if (parts[0] !== "rooms") {
    return null;
  }

  const roomId = decodeRouteSegment(parts[1]);
  const resolveTarget = ACTIVE_TARGET_ROUTE_RESOLVERS[parts[2] ?? ""]
    ?? resolveRoomRouteTarget;
  return resolveTarget(parts, roomId);
}

function resolveConversationRouteTarget(
  parts: string[],
  roomId: string,
): ActiveChatNotificationTarget | null {
  const conversationId = decodeRouteSegment(parts[3]);
  return createActiveChatTarget({
    conversation_id: conversationId,
    room_id: roomId,
  });
}

function resolveSessionRouteTarget(
  parts: string[],
  roomId: string,
): ActiveChatNotificationTarget | null {
  const sessionKey = decodeRouteSegment(parts[3]);
  const key = buildChatNotificationTargetKey({session_key: sessionKey});
  return key ? {key, room_id: roomId, session_key: sessionKey} : null;
}

function resolveRoomRouteTarget(
  _parts: string[],
  roomId: string,
): ActiveChatNotificationTarget | null {
  return createActiveChatTarget({conversation_id: "", room_id: roomId});
}

function createActiveChatTarget(
  target: Omit<ActiveChatNotificationTarget, "key">,
): ActiveChatNotificationTarget | null {
  const key = buildChatNotificationTargetKey(target);
  return key ? {...target, key} : null;
}

export function isChatNotificationTargetActive(
  activeTarget: ActiveChatNotificationTarget | null,
  target: ChatNotificationTargetMatcher,
): boolean {
  if (!activeTarget) {
    return false;
  }
  const exactKeyMatches = Boolean(
    target.key && target.key === activeTarget.key,
  );
  const roomFallbackMatches = Boolean(
    !activeTarget.session_key
      && !activeTarget.conversation_id
      && activeTarget.room_id
      && target.room_id === activeTarget.room_id,
  );
  return exactKeyMatches || roomFallbackMatches;
}

function decodeRouteSegment(value: string | undefined): string {
  if (!value) {
    return "";
  }
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}
