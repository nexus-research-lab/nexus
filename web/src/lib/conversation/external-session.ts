import { parseSessionKey } from "@/lib/conversation/session-key";
import type { BaseConversation } from "@/types/conversation/conversation";

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

function normalizeChannel(
  channelType?: string | null,
  sessionKey?: string | null,
): string {
  const parsed = parseSessionKey(sessionKey);
  return (channelType || parsed.channel || "").trim();
}

export function isExternalSessionChannel(
  channelType?: string | null,
  sessionKey?: string | null,
): boolean {
  const channel = normalizeChannel(channelType, sessionKey);
  return !INTERNAL_CHANNELS.has(channel) && SUPPORTED_EXTERNAL_CHANNELS.has(channel);
}

export function getSessionChannelLabel(
  channelType?: string | null,
  sessionKey?: string | null,
): string {
  const channel = normalizeChannel(channelType, sessionKey);
  return CHANNEL_LABELS[channel] ?? (channel || "外部通道");
}

export function isExternalSessionConversation(
  conversation?: Pick<BaseConversation, "options">,
): boolean {
  return conversation?.options.external_session === true;
}

export function getExternalSessionConversationLabel(
  conversation: Pick<BaseConversation, "options" | "session_key">,
): string | null {
  if (!isExternalSessionConversation(conversation)) {
    return null;
  }
  return getSessionChannelLabel(
    readStringOption(conversation.options, "channel_type"),
    conversation.session_key,
  );
}

export function formatExternalSessionTitle({
  title,
}: {
  title?: string | null;
}): string {
  return (title ?? "").trim() || "New Chat";
}

export function buildExternalSessionConversationId(sessionKey: string): string {
  return `${EXTERNAL_SESSION_CONVERSATION_PREFIX}${sessionKey.trim()}`;
}

export function getExternalSessionKeyFromConversationId(
  conversationId?: string | null,
): string | null {
  const normalized = (conversationId ?? "").trim();
  if (!normalized.startsWith(EXTERNAL_SESSION_CONVERSATION_PREFIX)) {
    return null;
  }
  return normalized.slice(EXTERNAL_SESSION_CONVERSATION_PREFIX.length).trim() || null;
}

export function isExternalSessionConversationId(
  conversationId?: string | null,
): boolean {
  return getExternalSessionKeyFromConversationId(conversationId) !== null;
}

function readStringOption(
  options: Record<string, unknown>,
  key: string,
): string | null {
  const value = options[key];
  return typeof value === "string" ? value : null;
}
