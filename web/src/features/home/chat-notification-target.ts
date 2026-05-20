export interface ChatNotificationTargetInput {
  conversation_id?: string | null;
  room_id?: string | null;
  session_key?: string | null;
}

export interface ActiveChatNotificationTarget {
  conversation_id?: string | null;
  key: string;
  room_id?: string | null;
}

export interface ChatNotificationTargetMatcher {
  key?: string | null;
  room_id?: string | null;
}

export function build_chat_notification_target_key({
  conversation_id,
  room_id,
  session_key,
}: ChatNotificationTargetInput): string | null {
  const normalized_room_id = room_id?.trim() ?? "";
  const normalized_conversation_id = conversation_id?.trim() ?? "";
  const normalized_session_key = session_key?.trim() ?? "";

  if (normalized_room_id && normalized_conversation_id) {
    return `room:${normalized_room_id}:conversation:${normalized_conversation_id}`;
  }
  if (normalized_room_id) {
    return `room:${normalized_room_id}`;
  }
  if (normalized_session_key) {
    return `session:${normalized_session_key}`;
  }
  return null;
}

function decode_route_segment(value: string | undefined): string {
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

  const room_id = decode_route_segment(parts[1]);
  const conversation_id = parts[2] === "conversations"
    ? decode_route_segment(parts[3])
    : "";
  const key = build_chat_notification_target_key({ conversation_id, room_id });
  return key
    ? {
        conversation_id,
        key,
        room_id,
      }
    : null;
}

export function is_chat_notification_target_active(
  active_target: ActiveChatNotificationTarget | null,
  target: ChatNotificationTargetMatcher,
): boolean {
  if (!active_target) {
    return false;
  }
  if (active_target.room_id && target.room_id === active_target.room_id) {
    return true;
  }
  return Boolean(target.key && target.key === active_target.key);
}
