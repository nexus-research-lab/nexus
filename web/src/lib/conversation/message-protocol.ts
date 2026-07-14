/**
 * INPUT: WebSocket message / stream 的未知协议载荷与信封回退字段。
 * OUTPUT: 校验后的 Conversation Message 或 Stream Message。
 * POS: 前端消息协议入口；Room user 可无 agent_id，assistant/stream 必须有 agent_id。
 */
import {
  asUnknownRecord,
  hasFiniteNumberFields,
  hasNonEmptyStringFields,
  readString,
  readStringFromSet,
  type UnknownRecord,
} from "@/lib/unknown-value";
import type { Message } from "@/types/conversation/message/entity";
import type { StreamMessage } from "@/types/conversation/message/event";

const MESSAGE_ROLES = new Set(["assistant", "system", "user"]);
const STREAM_MESSAGE_TYPES = new Set([
  "message_start",
  "content_block_start",
  "content_block_delta",
  "message_delta",
  "message_stop",
]);
const MESSAGE_IDENTITY_STRING_FIELDS = [
  "message_id",
  "round_id",
] as const;
const MESSAGE_TIMESTAMP_FIELDS = ["timestamp"] as const;

interface MessageEnvelopeProjection {
  deliveryMode?: string;
  sessionKey?: string;
}

function hasMessageContent(role: string, content: unknown): boolean {
  return role === "assistant" ? Array.isArray(content) : typeof content === "string";
}

function hasMessageIdentity(record: UnknownRecord, role: string): boolean {
  return (
    hasNonEmptyStringFields(record, MESSAGE_IDENTITY_STRING_FIELDS) &&
    hasFiniteNumberFields(record, MESSAGE_TIMESTAMP_FIELDS) &&
    typeof record.agent_id === "string" &&
    (role !== "assistant" || record.agent_id.length > 0)
  );
}

function readDeliveryMode(
  record: Record<string, unknown>,
  envelopeMode?: string,
): Message["delivery_mode"] {
  const mode = envelopeMode ?? readString(record, "delivery_mode");
  return mode === "durable" || mode === "ephemeral" ? mode : undefined;
}

export function parseConversationMessage(
  value: unknown,
  envelope: MessageEnvelopeProjection = {},
): Message | null {
  const record = asUnknownRecord(value);
  if (!record) {
    return null;
  }
  const role = readStringFromSet(record, "role", MESSAGE_ROLES);
  const sessionKey = readString(record, "session_key") ?? envelope.sessionKey ?? null;
  if (
    !role
    || !sessionKey
    || !hasMessageIdentity(record, role)
    || !hasMessageContent(role, record.content)
  ) {
    return null;
  }

  const { delivery_mode: _ignoredDeliveryMode, ...messageFields } = record;
  const deliveryMode = readDeliveryMode(record, envelope.deliveryMode);
  return {
    ...messageFields,
    session_key: sessionKey,
    ...(deliveryMode ? { delivery_mode: deliveryMode } : {}),
  } as unknown as Message;
}

export function parseStreamMessage(
  value: unknown,
  fallbackSessionKey?: string,
): StreamMessage | null {
  const record = asUnknownRecord(value);
  if (!record) {
    return null;
  }
  const sessionKey = readString(record, "session_key") ?? fallbackSessionKey ?? null;
  const type = readStringFromSet(record, "type", STREAM_MESSAGE_TYPES);
  if (
    !sessionKey
    || !type
    || !hasMessageIdentity(record, "assistant")
  ) {
    return null;
  }
  return { ...record, session_key: sessionKey } as unknown as StreamMessage;
}
