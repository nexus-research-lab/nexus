import { asUnknownRecord, readNumber, readString } from "@/lib/unknown-value";
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

interface MessageEnvelopeProjection {
  deliveryMode?: string;
  sessionKey?: string;
}

function hasMessageContent(role: string, content: unknown): boolean {
  return role === "assistant" ? Array.isArray(content) : typeof content === "string";
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
  const role = readString(record, "role");
  const sessionKey = readString(record, "session_key") ?? envelope.sessionKey ?? null;
  if (
    !role
    || !MESSAGE_ROLES.has(role)
    || !sessionKey
    || !readString(record, "message_id")
    || !readString(record, "agent_id")
    || !readString(record, "round_id")
    || readNumber(record, "timestamp") === null
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
  const type = readString(record, "type");
  if (
    !sessionKey
    || !type
    || !STREAM_MESSAGE_TYPES.has(type)
    || !readString(record, "message_id")
    || !readString(record, "agent_id")
    || !readString(record, "round_id")
    || readNumber(record, "timestamp") === null
  ) {
    return null;
  }
  return { ...record, session_key: sessionKey } as unknown as StreamMessage;
}
