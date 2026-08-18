/**
 * INPUT: 已去重的历史/实时消息和当前加载窗口的根 round 身份。
 * OUTPUT: 保留活跃尾部且受 round/估算内存双预算约束的消息窗口。
 * POS: 浏览器消息内存边界；只淘汰完整根 round，canonical 历史与导航索引不受影响。
 */
import type { Message } from "@/types/conversation/message/entity";

export const MAX_LOADED_MESSAGE_ROUNDS = 320;
export const MAX_LOADED_MESSAGE_BYTES = 24 * 1024 * 1024;
const MIN_LATEST_MESSAGE_ROUNDS = 8;

interface MessageWindowGroup {
  bytes: number;
  key: string;
  messages: Message[];
  pinned: boolean;
}

export interface MessageWindowOptions {
  anchorRoundIds?: Iterable<string>;
  maxBytes?: number;
  maxRounds?: number;
  preference?: "anchor" | "latest";
}

/** 超出预算时优先保留当前加载窗口、实时尾部和未落库消息。 */
export function boundLoadedMessages(
  messages: Message[],
  options: MessageWindowOptions = {},
): Message[] {
  const maxBytes = Math.max(options.maxBytes ?? MAX_LOADED_MESSAGE_BYTES, 1);
  const maxRounds = Math.max(options.maxRounds ?? MAX_LOADED_MESSAGE_ROUNDS, 1);
  const groups = groupMessagesForWindow(messages);
  const totalBytes = groups.reduce((total, group) => total + group.bytes, 0);
  if (groups.length <= maxRounds && totalBytes <= maxBytes) {
    return messages;
  }

  const anchors = new Set(
    [...(options.anchorRoundIds ?? [])]
      .map((roundId) => roundId.trim())
      .filter(Boolean),
  );
  const selected = new Set<string>();
  let selectedBytes = 0;
  const select = (group: MessageWindowGroup, force = false): boolean => {
    if (selected.has(group.key)) {
      return true;
    }
    if (!force && (
      selected.size >= maxRounds || selectedBytes + group.bytes > maxBytes
    )) {
      return false;
    }
    selected.add(group.key);
    selectedBytes += group.bytes;
    return true;
  };

  // 至少保留最新完整 round；单个 round 自身可能超过字节预算，但不会因此
  // 拆开。其余尾部 round 只有在预算内才进入窗口。
  const latestGroup = groups.at(-1);
  if (latestGroup) {
    select(latestGroup, true);
  }
  groups.forEach((group) => {
    if (group.pinned || anchors.has(group.key)) {
      select(group, true);
    }
  });

  groups
    .slice(-Math.min(MIN_LATEST_MESSAGE_ROUNDS, maxRounds))
    .reverse()
    .forEach((group) => select(group));

  const candidates = options.preference === "anchor" && anchors.size > 0
    ? groupsByDistanceFromAnchors(groups, anchors)
    : [...groups].reverse();
  for (const group of candidates) {
    select(group);
  }
  if (selected.size === groups.length) {
    return messages;
  }
  return groups.flatMap((group) => selected.has(group.key) ? group.messages : []);
}

export function loadedMessageRoundIds(messages: Message[]): string[] {
  return [...new Set(messages.map(messageWindowKey).filter(Boolean))];
}

function groupMessagesForWindow(messages: Message[]): MessageWindowGroup[] {
  const groups: MessageWindowGroup[] = [];
  const byKey = new Map<string, MessageWindowGroup>();
  for (const message of messages) {
    const key = messageWindowKey(message);
    let group = byKey.get(key);
    if (!group) {
      group = { bytes: 0, key, messages: [], pinned: false };
      byKey.set(key, group);
      groups.push(group);
    }
    group.messages.push(message);
    group.bytes += estimateMessageBytes(message);
    group.pinned ||= isPinnedWindowMessage(message);
  }
  return groups;
}

function groupsByDistanceFromAnchors(
  groups: MessageWindowGroup[],
  anchors: Set<string>,
): MessageWindowGroup[] {
  const anchorIndexes = groups.flatMap((group, index) => (
    anchors.has(group.key) ? [index] : []
  ));
  if (anchorIndexes.length === 0) {
    return [...groups].reverse();
  }
  return groups
    .map((group, index) => ({
      distance: Math.min(...anchorIndexes.map((anchor) => Math.abs(anchor - index))),
      group,
      index,
    }))
    .sort((left, right) => (
      left.distance - right.distance || right.index - left.index
    ))
    .map(({ group }) => group);
}

function messageWindowKey(message: Message): string {
  return message.round_id?.trim() || `message:${message.message_id}`;
}

function isPinnedWindowMessage(message: Message): boolean {
  if (message.delivery_mode && message.delivery_mode !== "durable") {
    return true;
  }
  if (message.role === "user") {
    return Boolean(
      message.client_message_id &&
      message.client_message_id === message.message_id,
    );
  }
  return message.role === "assistant" && (
    message.is_complete === false ||
    message.stream_status === "pending" ||
    message.stream_status === "streaming"
  );
}

function estimateMessageBytes(message: Message): number {
  try {
    // JS 字符串通常按 UTF-16 驻留；乘二比网络 JSON 大小更接近内存预算。
    return Math.max(JSON.stringify(message).length * 2, 256);
  } catch {
    return 4 * 1024;
  }
}
