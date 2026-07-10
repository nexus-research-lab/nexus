import { formatRelativeTime } from "@/lib/utils";
import type {
  AssistantMessage,
  Message,
  UserMessage,
} from "@/types/conversation/message";
import type { SessionRoundIndexItem } from "@/types/conversation/room";

import type { ConversationTimeline } from "../use-conversation-timeline";
import {
  extractTextFromContentBlocks,
  formatMessageTime,
  stripRoomControlMarkers,
} from "../message/item/message-item-support";

export interface SessionNavigationItem {
  agentIds: string[];
  hasUserMessage: boolean;
  index: number;
  inputRoundId: string;
  isLive: boolean;
  meta: string;
  roundId: string;
  summary: string;
  time: string;
  title: string;
}

export interface SessionNavigatorTickVisual {
  background: string;
  filter: string | undefined;
  opacity: number;
  width: number;
}

export const RULER_TRACK_TOP_SAFE_INSET_PX = 56;
export const RULER_TRACK_BOTTOM_SAFE_INSET_PX = 24;

const RULER_TICK_SPACING_PX = 14;
const WAVE_RADIUS_TICKS = 4;
const USER_TICK_COLOR = "#5b7cfa";
const LIVE_TICK_COLOR = "#7c8cff";
const NEUTRAL_TICK_COLOR = "var(--text-muted)";
const ACTIVE_NEUTRAL_TICK_COLOR = "var(--text-strong)";
const AGENT_TICK_COLORS = [
  "#10b981",
  "#f59e0b",
  "#ec4899",
  "#06b6d4",
  "#8b5cf6",
  "#ef4444",
  "#84cc16",
  "#14b8a6",
];
const INDEXED_STATUS_LABELS: Readonly<Record<string, string>> = {
  error: "失败",
  interrupted: "已中断",
};

export function getRulerTrackHeight(itemCount: number): number {
  return Math.max(RULER_TICK_SPACING_PX, itemCount * RULER_TICK_SPACING_PX);
}

export function getTickDisplayPercent(index: number, total: number): number {
  return total > 0 ? ((index + 0.5) / total) * 100 : 50;
}

function smoothWave(distanceTicks: number): number {
  const normalized = Math.max(0, 1 - distanceTicks / WAVE_RADIUS_TICKS);
  return normalized * normalized * (3 - 2 * normalized);
}

function tickWidth(wave: number): number {
  return Math.round(5 + wave * 11);
}

function tickOpacity(wave: number): number {
  return Math.min(1, 0.48 + wave * 0.42);
}

function hashText(value: string): number {
  let hash = 0;
  for (let index = 0; index < value.length; index += 1) {
    hash = (hash * 31 + value.charCodeAt(index)) >>> 0;
  }
  return hash;
}

function getAgentTickColor(agentId: string): string {
  return AGENT_TICK_COLORS[hashText(agentId) % AGENT_TICK_COLORS.length];
}

function normalizeAgentIds(agentIds: string[]): string[] {
  const normalizedAgentIds = agentIds
    .map((agentId) => agentId.trim())
    .filter(Boolean);
  return Array.from(new Set(normalizedAgentIds));
}

function buildTickSegments(item: SessionNavigationItem): string[] {
  const segments = item.hasUserMessage ? [USER_TICK_COLOR] : [];
  segments.push(
    ...normalizeAgentIds(item.agentIds)
      .slice(0, 4)
      .map(getAgentTickColor),
  );
  if (segments.length === 0) {
    segments.push(item.isLive ? LIVE_TICK_COLOR : NEUTRAL_TICK_COLOR);
  }
  return segments;
}

export function buildTickBackground(item: SessionNavigationItem): string {
  const segments = buildTickSegments(item);
  if (segments.length === 1) {
    return segments[0];
  }
  const step = 100 / segments.length;
  const stops = segments.flatMap((color, index) => [
    `${color} ${index * step}%`,
    `${color} ${(index + 1) * step}%`,
  ]);
  return `linear-gradient(90deg, ${stops.join(", ")})`;
}

export function buildTickVisual(
  item: SessionNavigationItem,
  activeRoundId: string | null,
  previewIndex: number | null,
  previewRoundId: string | null,
): SessionNavigatorTickVisual {
  const hasPreview = previewIndex !== null;
  const isActive = item.roundId === activeRoundId;
  const isPreviewed = item.roundId === previewRoundId;
  const wave = hasPreview
    ? smoothWave(Math.abs(item.index - previewIndex))
    : 0;
  let background = NEUTRAL_TICK_COLOR;
  if (isPreviewed) {
    background = buildTickBackground(item);
  } else if (!hasPreview && isActive) {
    background = ACTIVE_NEUTRAL_TICK_COLOR;
  }
  let opacity = 0.58;
  if (hasPreview) {
    opacity = tickOpacity(wave);
  } else if (isActive) {
    opacity = 0.9;
  }
  return {
    background,
    filter: isPreviewed ? "saturate(1.18)" : undefined,
    opacity,
    width: hasPreview ? tickWidth(wave) : 5,
  };
}

function formatAgentDisplayName(
  agentId: string,
  agentNameMap?: Record<string, string>,
): string {
  return agentNameMap?.[agentId] || `Agent ${agentId.slice(0, 6)}`;
}

export function formatSpeakerSummary(
  item: SessionNavigationItem,
  agentNameMap?: Record<string, string>,
): string {
  const speakers = item.hasUserMessage ? ["用户"] : [];
  speakers.push(
    ...normalizeAgentIds(item.agentIds)
      .map((agentId) => formatAgentDisplayName(agentId, agentNameMap)),
  );
  return speakers.join(" · ") || "未加载";
}

function compactText(text: string, fallback: string): string {
  const normalized = stripRoomControlMarkers(text)
    .replace(/\s+/g, " ")
    .trim();
  return normalized || fallback;
}

function isUserMessage(message: Message): message is UserMessage {
  return message.role === "user";
}

function isAssistantMessage(message: Message): message is AssistantMessage {
  return message.role === "assistant";
}

function formatDuration(durationMs: number | null | undefined): string | null {
  if (!durationMs || durationMs <= 0) {
    return null;
  }
  const totalSeconds = Math.max(1, Math.round(durationMs / 1000));
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  if (minutes <= 0) {
    return `${seconds}s`;
  }
  if (minutes < 60) {
    return `${minutes}m ${seconds}s`;
  }
  const hours = Math.floor(minutes / 60);
  const restMinutes = minutes % 60;
  return restMinutes > 0 ? `${hours}h ${restMinutes}m` : `${hours}h`;
}

function formatIndexedStatus(status: string | null, isLive: boolean): string {
  return isLive ? "处理中" : INDEXED_STATUS_LABELS[status ?? ""] ?? "已处理";
}

function formatLoadedStatus(isLive: boolean, isError: boolean): string {
  if (isLive) {
    return "处理中";
  }
  return isError ? "失败" : "已处理";
}

function buildLoadedNavigationItem(
  roundId: string,
  index: number,
  messages: Message[],
  liveRoundIds: Set<string>,
): SessionNavigationItem {
  const userMessage = messages.find(isUserMessage);
  const assistantMessages = messages.filter(isAssistantMessage);
  const firstAssistant = assistantMessages[0];
  const lastAssistant = assistantMessages[assistantMessages.length - 1];
  const resultSummary = lastAssistant?.result_summary;
  const isLive = liveRoundIds.has(roundId);
  const assistantText = firstAssistant
    ? extractTextFromContentBlocks(firstAssistant.content)
    : "";
  const status = formatLoadedStatus(
    isLive,
    resultSummary?.subtype === "error",
  );
  const duration = formatDuration(resultSummary?.duration_ms);
  const timestamp = userMessage?.timestamp
    ?? firstAssistant?.timestamp
    ?? resultSummary?.timestamp
    ?? null;
  return {
    agentIds: normalizeAgentIds(
      assistantMessages.map((message) => message.agent_id ?? ""),
    ),
    hasUserMessage: Boolean(userMessage),
    index,
    inputRoundId: roundId,
    isLive,
    meta: [status, duration].filter(Boolean).join(" "),
    roundId,
    summary: isLive
      ? "正在处理当前轮次"
      : compactText(resultSummary?.result ?? assistantText, "尚无回复内容"),
    time: timestamp ? formatRelativeTime(timestamp) : formatMessageTime(null),
    title: compactText(userMessage?.content ?? "", `第 ${index + 1} 轮`),
  };
}

function buildIndexedNavigationItem(
  item: SessionRoundIndexItem,
  index: number,
  messages: Message[],
  liveRoundIds: Set<string>,
): SessionNavigationItem {
  if (messages.length > 0) {
    return buildLoadedNavigationItem(
      item.roundId,
      index,
      messages,
      liveRoundIds,
    );
  }
  const isLive = item.isLive || liveRoundIds.has(item.roundId);
  const duration = formatDuration(item.durationMs);
  return {
    agentIds: normalizeAgentIds(item.agentIds),
    hasUserMessage: item.hasUserMessage,
    index,
    inputRoundId: item.roundId,
    isLive,
    meta: [formatIndexedStatus(item.status, isLive), duration]
      .filter(Boolean)
      .join(" "),
    roundId: item.roundId,
    summary: isLive ? "正在处理当前轮次" : "滚动加载后可查看详情",
    time: item.timestamp
      ? formatRelativeTime(item.timestamp)
      : formatMessageTime(null),
    title: compactText(item.title, `第 ${index + 1} 轮`),
  };
}

function bindInputRoundIds(
  items: SessionNavigationItem[],
): SessionNavigationItem[] {
  let currentInputRoundId: string | null = null;
  return items.map((item) => {
    if (item.hasUserMessage) {
      currentInputRoundId = item.roundId;
    }
    return {
      ...item,
      inputRoundId: currentInputRoundId ?? item.roundId,
    };
  });
}

/** 将唯一时间线投影转换为导航条展示模型，不在组件中重新分组消息。 */
export function buildSessionNavigationItems(
  timeline: ConversationTimeline,
): SessionNavigationItem[] {
  const {
    live_round_ids: liveRoundIds,
    message_groups: messageGroups,
    round_index_items: roundIndexItems,
  } = timeline;
  const liveRoundIdSet = new Set(liveRoundIds);
  const indexedRoundIds = new Set(
    roundIndexItems.map((item) => item.roundId),
  );
  const missingLiveRoundIds = Array.from(new Set(liveRoundIds))
    .filter((roundId) => roundId.trim() && !indexedRoundIds.has(roundId));
  const indexedItems = roundIndexItems.map((item, index) => (
    buildIndexedNavigationItem(
      item,
      index,
      messageGroups.get(item.roundId) ?? [],
      liveRoundIdSet,
    )
  ));
  const liveItems = missingLiveRoundIds.map((roundId, offset) => (
    buildLoadedNavigationItem(
      roundId,
      roundIndexItems.length + offset,
      messageGroups.get(roundId) ?? [],
      liveRoundIdSet,
    )
  ));
  return bindInputRoundIds([...indexedItems, ...liveItems]);
}
