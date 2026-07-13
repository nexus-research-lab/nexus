import { formatRelativeTime } from "@/lib/format/relative-time";
import type {
  AssistantMessage,
  Message,
  ResultSummary,
  UserMessage,
} from "@/types/conversation/message/entity";
import type { SessionRoundIndexItem } from "@/types/conversation/history";

import type { ConversationTimeline } from "../timeline/timeline-model";
import {
  extractTextFromContentBlocks,
  stripRoomControlMarkers,
} from "../message/message-content-model";
import { formatMessageTime } from "../message/message-time";

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

const INDEXED_STATUS_LABELS: Readonly<Record<string, string>> = {
  error: "失败",
  interrupted: "已中断",
};

interface NavigationItemSource {
  agentIds: string[];
  durationMs: number | null | undefined;
  hasUserMessage: boolean;
  isLive: boolean;
  roundId: string;
  status: string;
  summary: string;
  summaryFallback: string;
  timestamp: number | null | undefined;
  title: string;
}

interface UserRoundSnapshot {
  hasUserMessage: boolean;
  timestamp: number | null;
  title: string;
}

interface AssistantRoundSnapshot {
  agentIds: string[];
  durationMs: number | null;
  firstText: string;
  result: string;
  status: ResultSummary["subtype"] | null;
  timestamp: number | null;
}

interface ResultSummarySnapshot {
  durationMs: number | null;
  result: string;
  status: ResultSummary["subtype"] | null;
  timestamp: number | null;
}

const EMPTY_USER_ROUND_SNAPSHOT: UserRoundSnapshot = {
  hasUserMessage: false,
  timestamp: null,
  title: "",
};

const EMPTY_RESULT_SUMMARY_SNAPSHOT: ResultSummarySnapshot = {
  durationMs: null,
  result: "",
  status: null,
  timestamp: null,
};

const DURATION_FORMAT_RULES = [
  {
    matches: (totalSeconds: number) => totalSeconds < 60,
    format: (totalSeconds: number) => `${totalSeconds}s`,
  },
  {
    matches: (totalSeconds: number) => totalSeconds < 3600,
    format: (totalSeconds: number) => (
      `${Math.floor(totalSeconds / 60)}m ${totalSeconds % 60}s`
    ),
  },
];

function normalizeAgentIds(agentIds: string[]): string[] {
  const normalizedAgentIds = agentIds
    .map((agentId) => agentId.trim())
    .filter(Boolean);
  return Array.from(new Set(normalizedAgentIds));
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

function projectUserRoundSnapshot(
  message: UserMessage | undefined,
): UserRoundSnapshot {
  if (!message) {
    return EMPTY_USER_ROUND_SNAPSHOT;
  }
  return {
    hasUserMessage: true,
    timestamp: message.timestamp,
    title: message.content,
  };
}

function projectResultSummary(
  summary: ResultSummary | undefined,
): ResultSummarySnapshot {
  if (!summary) {
    return EMPTY_RESULT_SUMMARY_SNAPSHOT;
  }
  return {
    durationMs: summary.duration_ms,
    result: summary.result ?? "",
    status: summary.subtype,
    timestamp: summary.timestamp ?? null,
  };
}

function projectAssistantRoundSnapshot(
  messages: AssistantMessage[],
): AssistantRoundSnapshot {
  const firstAssistant = messages[0];
  const lastAssistant = messages.at(-1);
  const resultSummary = projectResultSummary(lastAssistant?.result_summary);
  return {
    agentIds: messages.map((message) => message.agent_id),
    durationMs: resultSummary.durationMs,
    firstText: extractTextFromContentBlocks(firstAssistant?.content),
    result: resultSummary.result,
    status: resultSummary.status,
    timestamp: firstAssistant?.timestamp ?? resultSummary.timestamp,
  };
}

function formatDuration(durationMs: number | null | undefined): string | null {
  if (!durationMs || durationMs <= 0) {
    return null;
  }
  const totalSeconds = Math.max(1, Math.round(durationMs / 1000));
  const rule = DURATION_FORMAT_RULES.find((candidate) => (
    candidate.matches(totalSeconds)
  ));
  if (rule) {
    return rule.format(totalSeconds);
  }
  const minutes = Math.floor(totalSeconds / 60);
  const hours = Math.floor(minutes / 60);
  const restMinutes = minutes % 60;
  return restMinutes > 0 ? `${hours}h ${restMinutes}m` : `${hours}h`;
}

function formatStatus(status: string | null, isLive: boolean): string {
  return isLive ? "处理中" : INDEXED_STATUS_LABELS[status ?? ""] ?? "已处理";
}

function resolveLoadedNavigationSource(
  roundId: string,
  messages: Message[],
  liveRoundIds: Set<string>,
): NavigationItemSource {
  const user = projectUserRoundSnapshot(messages.find(isUserMessage));
  const assistant = projectAssistantRoundSnapshot(
    messages.filter(isAssistantMessage),
  );
  const isLive = liveRoundIds.has(roundId);
  return {
    agentIds: assistant.agentIds,
    durationMs: assistant.durationMs,
    hasUserMessage: user.hasUserMessage,
    isLive,
    roundId,
    status: formatStatus(assistant.status, isLive),
    summary: assistant.result || assistant.firstText,
    summaryFallback: "尚无回复内容",
    timestamp: user.timestamp ?? assistant.timestamp,
    title: user.title,
  };
}

function resolveIndexedNavigationSource(
  item: SessionRoundIndexItem,
  liveRoundIds: Set<string>,
): NavigationItemSource {
  const isLive = item.isLive || liveRoundIds.has(item.roundId);
  return {
    agentIds: item.agentIds,
    durationMs: item.durationMs,
    hasUserMessage: item.hasUserMessage,
    isLive,
    roundId: item.roundId,
    status: formatStatus(item.status, isLive),
    summary: "",
    summaryFallback: "滚动加载后可查看详情",
    timestamp: item.timestamp,
    title: item.title,
  };
}

function projectNavigationItem(
  source: NavigationItemSource,
  index: number,
): SessionNavigationItem {
  const duration = formatDuration(source.durationMs);
  return {
    agentIds: normalizeAgentIds(source.agentIds),
    hasUserMessage: source.hasUserMessage,
    index,
    inputRoundId: source.roundId,
    isLive: source.isLive,
    meta: [source.status, duration].filter(Boolean).join(" "),
    roundId: source.roundId,
    summary: source.isLive
      ? "正在处理当前轮次"
      : compactText(source.summary, source.summaryFallback),
    time: source.timestamp
      ? formatRelativeTime(source.timestamp)
      : formatMessageTime(null),
    title: compactText(source.title, `第 ${index + 1} 轮`),
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
  const indexedItems = roundIndexItems.map((item, index) => {
    const messages = messageGroups.get(item.roundId) ?? [];
    const source = messages.length > 0
      ? resolveLoadedNavigationSource(item.roundId, messages, liveRoundIdSet)
      : resolveIndexedNavigationSource(item, liveRoundIdSet);
    return projectNavigationItem(source, index);
  });
  const liveItems = missingLiveRoundIds.map((roundId, offset) => (
    projectNavigationItem(
      resolveLoadedNavigationSource(
        roundId,
        messageGroups.get(roundId) ?? [],
        liveRoundIdSet,
      ),
      roundIndexItems.length + offset,
    )
  ));
  return bindInputRoundIds([...indexedItems, ...liveItems]);
}
