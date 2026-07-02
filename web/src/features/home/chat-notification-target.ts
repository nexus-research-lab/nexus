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

export function build_chat_notification_target_key({
  conversation_id: conversationId,
  room_id: roomId,
  session_key: sessionKey,
}: ChatNotificationTargetInput): string | null {
  const normalizedRoomId = roomId?.trim() ?? "";
  const normalizedConversationId = conversationId?.trim() ?? "";
  const normalizedSessionKey = sessionKey?.trim() ?? "";

  if (normalizedRoomId && normalizedConversationId) {
    return `room:${normalizedRoomId}:conversation:${normalizedConversationId}`;
  }
  if (normalizedRoomId) {
    return `room:${normalizedRoomId}`;
  }
  if (normalizedSessionKey) {
    return `session:${normalizedSessionKey}`;
  }
  return null;
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

export function get_active_chat_target_from_path(
  pathname: string,
): ActiveChatNotificationTarget | null {
  const parts = pathname.split("/").filter(Boolean);
  if (parts[0] !== "rooms") {
    return null;
  }

  const roomId = decodeRouteSegment(parts[1]);
  if (parts[2] === "sessions") {
    const sessionKey = decodeRouteSegment(parts[3]);
    const key = build_chat_notification_target_key({ session_key: sessionKey });
    return key
      ? {
          key,
          room_id: roomId,
          session_key: sessionKey,
        }
      : null;
  }

  const conversationId = parts[2] === "conversations"
    ? decodeRouteSegment(parts[3])
    : "";
  const key = build_chat_notification_target_key({ conversation_id: conversationId, room_id: roomId });
  return key
    ? {
        conversation_id: conversationId,
        key,
        room_id: roomId,
      }
    : null;
}

export function is_chat_notification_target_active(
  activeTarget: ActiveChatNotificationTarget | null,
  target: ChatNotificationTargetMatcher,
): boolean {
  if (!activeTarget) {
    return false;
  }
  if (target.key && target.key === activeTarget.key) {
    return true;
  }
  if (activeTarget.session_key) {
    return false;
  }
  if (activeTarget.room_id && target.room_id === activeTarget.room_id) {
    return true;
  }
  return false;
}
