import { parse_session_key } from "@/lib/conversation/session-key";

const CHANNEL_LABELS: Record<string, string> = {
  dingtalk: "钉钉",
  dt: "钉钉",
  discord: "Discord",
  dg: "Discord",
  feishu: "飞书",
  fs: "飞书",
  telegram: "Telegram",
  tg: "Telegram",
  wechat: "微信",
  wx: "微信",
  "weixin-personal": "微信",
};

const INTERNAL_CHANNELS = new Set(["", "websocket", "ws", "internal"]);
const SUPPORTED_EXTERNAL_CHANNELS = new Set(Object.keys(CHANNEL_LABELS));
const EXTERNAL_SESSION_CONVERSATION_PREFIX = "external-session:";

function normalize_channel(channel_type?: string | null, session_key?: string | null): string {
  const parsed = parse_session_key(session_key);
  return (channel_type || parsed.channel || "").trim();
}

export function is_external_session_channel(channel_type?: string | null, session_key?: string | null): boolean {
  const channel = normalize_channel(channel_type, session_key);
  return !INTERNAL_CHANNELS.has(channel) && SUPPORTED_EXTERNAL_CHANNELS.has(channel);
}

export function get_session_channel_label(channel_type?: string | null, session_key?: string | null): string {
  const channel = normalize_channel(channel_type, session_key);
  return CHANNEL_LABELS[channel] ?? (channel || "外部通道");
}

export function format_external_session_title({
  title,
}: {
  title?: string | null;
}): string {
  return (title ?? "").trim() || "New Chat";
}

function format_external_session_summary({
  channel_type,
  chat_type,
  session_key,
}: {
  agent_name?: string | null;
  channel_type?: string | null;
  chat_type?: string | null;
  session_key?: string | null;
}): string {
  const parsed = parse_session_key(session_key);
  const channel_label = get_session_channel_label(channel_type, session_key);
  const chat_label = (chat_type || parsed.chat_type) === "group" ? "群聊" : "私聊";
  return `${channel_label}${chat_label}`;
}

export function build_external_session_conversation_id(session_key: string): string {
  return `${EXTERNAL_SESSION_CONVERSATION_PREFIX}${session_key.trim()}`;
}

export function get_external_session_key_from_conversation_id(conversation_id?: string | null): string | null {
  const normalized = (conversation_id ?? "").trim();
  if (!normalized.startsWith(EXTERNAL_SESSION_CONVERSATION_PREFIX)) {
    return null;
  }
  return normalized.slice(EXTERNAL_SESSION_CONVERSATION_PREFIX.length).trim() || null;
}

export function is_external_session_conversation_id(conversation_id?: string | null): boolean {
  return get_external_session_key_from_conversation_id(conversation_id) !== null;
}
