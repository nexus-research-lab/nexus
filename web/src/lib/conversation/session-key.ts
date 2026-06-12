import { resolve_agent_id } from "@/config/options";

const AGENT_SESSION_PREFIX = "agent";
const ROOM_SESSION_PREFIX = "room";
const ROOM_SHARED_SESSION_PREFIX = "room:group:";
const TOPIC_SEGMENT = "topic";
const ACCOUNT_SEGMENT = "acct";

export interface BuildSessionKeyOptions {
  channel: string;
  chat_type: string;
  ref: string;
  agent_id?: string | null;
  account_id?: string | null;
  thread_id?: string | null;
}

export type SessionKeyKind = "agent" | "room" | "unknown";

export interface ParsedSessionKey {
  raw: string;
  kind: SessionKeyKind;
  is_structured: boolean;
  is_shared: boolean;
  agent_id: string | null;
  channel: string | null;
  chat_type: string | null;
  account_id: string | null;
  ref: string | null;
  thread_id: string | null;
  conversation_id: string | null;
}

function find_topic_index(parts: string[], min_index: number): number {
  return parts.findIndex((part, index) => part === TOPIC_SEGMENT && index >= min_index);
}

function agent_session_key_shape_error(): string {
  return "session_key must match agent:<agent_id>:<channel>:<chat_type>[:acct:<account_id>]:<ref>[:topic:<thread_id>]";
}

function split_agent_ref_parts(parts: string[]): {
  account_id: string | null;
  ref_start: number;
  error: string | null;
} {
  if (parts[4] === ACCOUNT_SEGMENT) {
    if (parts.length < 7) {
      return { account_id: null, ref_start: 0, error: agent_session_key_shape_error() };
    }
    const account_id = parts[5]?.trim() ?? "";
    if (!account_id) {
      return { account_id: null, ref_start: 0, error: "session_key account_id is required after acct segment" };
    }
    return { account_id, ref_start: 6, error: null };
  }
  return { account_id: null, ref_start: 4, error: null };
}

/**
 * 中文注释：前后端共享同一套 session_key 语义，前端不要再散落手拼字符串。
 */
export function build_session_key({
  channel,
  chat_type,
  ref,
  agent_id,
  account_id,
  thread_id,
}: BuildSessionKeyOptions): string {
  const resolved_agent_id = resolve_agent_id(agent_id);
  const resolved_channel = channel.trim();
  const resolved_chat_type = chat_type.trim();
  const resolved_ref = ref.trim();
  const resolved_account_id = account_id?.trim() ?? "";
  let key = resolved_account_id
    ? `${AGENT_SESSION_PREFIX}:${resolved_agent_id}:${resolved_channel}:${resolved_chat_type}:${ACCOUNT_SEGMENT}:${resolved_account_id}:${resolved_ref}`
    : `${AGENT_SESSION_PREFIX}:${resolved_agent_id}:${resolved_channel}:${resolved_chat_type}:${resolved_ref}`;
  if (thread_id?.trim()) {
    key += `:${TOPIC_SEGMENT}:${thread_id.trim()}`;
  }
  return key;
}

export function build_room_shared_session_key(conversation_id: string): string {
  return `${ROOM_SHARED_SESSION_PREFIX}${conversation_id}`;
}

export function build_room_agent_session_key(
  conversation_id: string,
  agent_id: string,
  room_type: "dm" | "room" = "room",
): string {
  return build_session_key({
    channel: "ws",
    chat_type: room_type === "dm" ? "dm" : "group",
    ref: conversation_id,
    agent_id,
  });
}

export function get_session_key_validation_error(session_key: string | null | undefined): string | null {
  const normalized_key = (session_key ?? "").trim();
  if (!normalized_key) {
    return "session_key is required";
  }

  if (normalized_key.startsWith(`${AGENT_SESSION_PREFIX}:`)) {
    const parts = normalized_key.split(":");
    if (parts.length < 5 || !parts[1] || !parts[2] || !parts[3]) {
      return agent_session_key_shape_error();
    }

    const split = split_agent_ref_parts(parts);
    if (split.error) {
      return split.error;
    }
    const topic_index = find_topic_index(parts, split.ref_start);
    if (topic_index >= 0) {
      const ref = parts.slice(split.ref_start, topic_index).join(":").trim();
      const thread_id = parts.slice(topic_index + 1).join(":").trim();
      return ref && thread_id ? null : agent_session_key_shape_error();
    }

    return parts.slice(split.ref_start).join(":").trim() ? null : agent_session_key_shape_error();
  }

  if (normalized_key.startsWith(`${ROOM_SESSION_PREFIX}:`)) {
    const parts = normalized_key.split(":");
    const conversation_id = parts.slice(2).join(":").trim();
    return parts.length >= 3 && parts[1] === "group" && conversation_id
      ? null
      : "session_key must match room:group:<conversation_id>";
  }

  return "session_key must use structured gateway format";
}

export function is_structured_session_key(session_key: string): boolean {
  return get_session_key_validation_error(session_key) === null;
}

export function assert_structured_session_key(session_key: string | null | undefined): string {
  const error_message = get_session_key_validation_error(session_key);
  if (error_message) {
    throw new Error(error_message);
  }
  return (session_key ?? "").trim();
}

export function is_room_shared_session_key(session_key: string): boolean {
  const parsed = parse_session_key(session_key);
  return parsed.kind === "room" && parsed.is_structured && Boolean(parsed.conversation_id);
}

export function parse_session_key(session_key: string | null | undefined): ParsedSessionKey {
  const normalized_key = (session_key ?? "").trim();
  const validation_error = get_session_key_validation_error(normalized_key);
  const result: ParsedSessionKey = {
    raw: normalized_key,
    kind: "unknown",
    is_structured: false,
    is_shared: false,
    agent_id: null,
    channel: null,
    chat_type: null,
    account_id: null,
    ref: null,
    thread_id: null,
    conversation_id: null,
  };

  if (normalized_key.startsWith(`${AGENT_SESSION_PREFIX}:`)) {
    const parts = normalized_key.split(":");
    result.kind = "agent";
    result.is_structured = validation_error === null;
    result.agent_id = resolve_agent_id(parts[1]);
    result.channel = parts[2] || null;
    result.chat_type = parts[3] || "dm";

    // `:topic:` 是协议保留边界，ref 中允许冒号，但不能跨过该边界。
    const split = split_agent_ref_parts(parts);
    if (split.error) {
      return result;
    }
    result.account_id = split.account_id;
    const topic_index = find_topic_index(parts, split.ref_start);
    if (topic_index >= 0) {
      result.ref = parts.slice(split.ref_start, topic_index).join(":") || null;
      result.thread_id = parts.slice(topic_index + 1).join(":") || null;
    } else {
      result.ref = parts.slice(split.ref_start).join(":") || null;
    }
    return result;
  }

  if (normalized_key.startsWith(`${ROOM_SESSION_PREFIX}:`)) {
    const parts = normalized_key.split(":");
    const conversation_id = parts.slice(2).join(":").trim();
    result.kind = "room";
    result.is_structured = validation_error === null;
    result.is_shared = validation_error === null;
    result.chat_type = parts[1] || "group";
    result.ref = conversation_id || null;
    result.conversation_id = conversation_id || null;
  }

  return result;
}

export function get_session_key_identity(session_key: string | null | undefined): string | null {
  const parsed = parse_session_key(session_key);
  if (!parsed.raw) {
    return null;
  }

  // Room 键比较时只认 conversation_id，避免未来 alias 演进时前端错判。
  if (parsed.kind === "room" && parsed.conversation_id) {
    return `${ROOM_SESSION_PREFIX}:${parsed.conversation_id}`;
  }

  return parsed.raw;
}

export function are_equivalent_session_keys(
  left: string | null | undefined,
  right: string | null | undefined,
): boolean {
  const left_identity = get_session_key_identity(left);
  const right_identity = get_session_key_identity(right);
  return Boolean(left_identity && right_identity && left_identity === right_identity);
}
