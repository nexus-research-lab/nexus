/**
 * INPUT: 已去重的历史/实时消息、窗口断点和需要让出预算的窗口端。
 * OUTPUT: 受 round/估算内存双预算约束、只从一端整轮丢弃的连续消息窗口及其断点。
 * POS: 浏览器消息内存边界；只淘汰完整根 round，canonical 历史与导航索引不受影响。
 */
import type { Message } from "@/types/conversation/message/entity";

export const MAX_LOADED_MESSAGE_ROUNDS = 320;
export const MAX_LOADED_MESSAGE_BYTES = 24 * 1024 * 1024;

interface MessageWindowGroup {
  bytes: number;
  key: string;
  messages: Message[];
  pinned: boolean;
}

/** 超出预算时被整轮丢弃的窗口端：head 为最旧一端，tail 为最新一端。 */
export type MessageWindowEdge = "head" | "tail";

/**
 * 浏览器驻留的消息窗口：一段连续历史，加上可选的一处断点。
 * gapAfterRoundId 指向断点前最后一个驻留 round；其后的 round（实时尾部）
 * 与它之间存在尚未加载的历史。null 表示窗口一直连到最新。
 */
export interface LoadedMessageWindow {
  gapAfterRoundId: string | null;
  messages: Message[];
}

export interface MessageWindowOptions {
  evict?: MessageWindowEdge;
  maxBytes?: number;
  maxRounds?: number;
}

/** 只需要消息数组的写入方（实时快照、会话恢复）使用的预算入口。 */
export function boundLoadedMessages(
  messages: Message[],
  options: MessageWindowOptions = {},
): Message[] {
  return boundMessageWindow(
    { gapAfterRoundId: null, messages },
    options,
  ).messages;
}

/**
 * 超出 round/字节预算时从指定窗口端整轮丢弃，未落库与流式中的 round 永不丢弃。
 * 结果始终是「一段连续窗口 + 至多一处断点 + 连到最新的尾部」：
 * - 丢弃 tail：保留窗口起点开始的连续前缀，断点移到前缀末尾；断点后的尾部
 *   只在预算有余时保留其最新的连续后缀。
 * - 丢弃 head：保留整体最新的连续后缀；后缀没有覆盖到断点时断点随之消失。
 */
export function boundMessageWindow(
  window: LoadedMessageWindow,
  options: MessageWindowOptions = {},
): LoadedMessageWindow {
  const maxBytes = Math.max(options.maxBytes ?? MAX_LOADED_MESSAGE_BYTES, 1);
  const maxRounds = Math.max(options.maxRounds ?? MAX_LOADED_MESSAGE_ROUNDS, 1);
  const evict = options.evict ?? "head";
  const groups = groupMessagesForWindow(window.messages);
  const gapIndex = window.gapAfterRoundId === null
    ? -1
    : groups.findIndex((group) => group.key === window.gapAfterRoundId);
  const gapAfterRoundId = gapIndex < 0 ? null : window.gapAfterRoundId;
  const totalBytes = groups.reduce((total, group) => total + group.bytes, 0);
  if (groups.length <= maxRounds && totalBytes <= maxBytes) {
    return gapAfterRoundId === window.gapAfterRoundId
      ? window
      : { gapAfterRoundId, messages: window.messages };
  }

  const selected = new Set<MessageWindowGroup>();
  let selectedBytes = 0;
  const select = (group: MessageWindowGroup): void => {
    if (!selected.has(group)) {
      selected.add(group);
      selectedBytes += group.bytes;
    }
  };
  /** 按给定顺序连续保留，遇到第一个放不下的 round 即停止；返回保留数量。 */
  const selectRun = (run: MessageWindowGroup[], forceFirst: boolean): number => {
    let count = 0;
    for (const group of run) {
      const fits = selected.has(group) || (
        selected.size < maxRounds && selectedBytes + group.bytes <= maxBytes
      );
      // 单个 round 自身可能超过字节预算，但不会因此拆开或让窗口变空。
      if (!fits && !(forceFirst && count === 0)) {
        break;
      }
      select(group);
      count += 1;
    }
    return count;
  };

  groups.filter((group) => group.pinned).forEach(select);
  const windowGroups = gapIndex < 0 ? groups : groups.slice(0, gapIndex + 1);
  const tailGroups = gapIndex < 0 ? [] : groups.slice(gapIndex + 1).reverse();

  let nextGapAfterRoundId: string | null;
  if (evict === "tail") {
    const kept = selectRun(windowGroups, true);
    selectRun(tailGroups, false);
    nextGapAfterRoundId = kept < windowGroups.length
      ? windowGroups[kept - 1]?.key ?? null
      : gapAfterRoundId;
  } else {
    const keptTail = selectRun(tailGroups, true);
    const keptWindow = keptTail < tailGroups.length
      ? 0
      : selectRun([...windowGroups].reverse(), tailGroups.length === 0);
    nextGapAfterRoundId = keptWindow > 0 ? gapAfterRoundId : null;
  }

  return {
    gapAfterRoundId: nextGapAfterRoundId,
    messages: groups
      .filter((group) => selected.has(group))
      .flatMap((group) => group.messages),
  };
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

export function messageWindowKey(message: Message): string {
  return message.round_id?.trim() || `message:${message.message_id}`;
}

export function isPinnedWindowMessage(message: Message): boolean {
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

// 消息对象不可变（流式更新产生新对象），按身份缓存估算值；否则每个流式帧都要
// 对整个加载窗口逐条 JSON.stringify。WeakMap 随消息被回收自动释放。
const messageBytesCache = new WeakMap<Message, number>();

function estimateMessageBytes(message: Message): number {
  const cached = messageBytesCache.get(message);
  if (cached !== undefined) {
    return cached;
  }
  const bytes = measureMessageBytes(message);
  messageBytesCache.set(message, bytes);
  return bytes;
}

function measureMessageBytes(message: Message): number {
  try {
    // JS 字符串通常按 UTF-16 驻留；乘二比网络 JSON 大小更接近内存预算。
    return Math.max(JSON.stringify(message).length * 2, 256);
  } catch {
    return 4 * 1024;
  }
}
