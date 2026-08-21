import { formatRelativeTime } from "@/lib/format/relative-time";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
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
  isLive: boolean;
  meta: string;
  roundId: string;
  summary: string;
  time: string;
  title: string;
}

const INDEXED_STATUS_LABEL_KEYS: Readonly<Record<string, TranslationKey>> = {
  error: "room.session_navigator_status_error",
  interrupted: "room.session_navigator_status_interrupted",
};

type SessionNavigatorLocalization = Pick<I18nContextValue, "locale" | "t">;

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
  const firstText = messages
    .map((message) => extractTextFromContentBlocks(message.content).trim())
    .find(Boolean) ?? "";
  const resultSummary = projectResultSummary(
    findLastResultSummary(messages),
  );
  return {
    agentIds: messages.map((message) => message.agent_id),
    durationMs: resultSummary.durationMs,
    firstText,
    result: resultSummary.result,
    status: resultSummary.status,
    timestamp: firstAssistant?.timestamp ?? resultSummary.timestamp,
  };
}

function findLastResultSummary(
  messages: readonly AssistantMessage[],
): ResultSummary | undefined {
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const summary = messages[index]?.result_summary;
    if (summary) {
      return summary;
    }
  }
  return undefined;
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

function formatStatus(
  status: string | null,
  isLive: boolean,
  { t }: SessionNavigatorLocalization,
): string {
  if (isLive) {
    return t("room.session_navigator_status_processing");
  }
  const statusKey = INDEXED_STATUS_LABEL_KEYS[status ?? ""];
  return statusKey
    ? t(statusKey)
    : t("room.session_navigator_status_processed");
}

function resolveLoadedNavigationSource(
  roundId: string,
  messages: Message[],
  liveRoundIds: Set<string>,
  localization: SessionNavigatorLocalization,
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
    status: formatStatus(assistant.status, isLive, localization),
    summary: assistant.result || assistant.firstText,
    summaryFallback: localization.t("room.session_navigator_empty_reply"),
    timestamp: user.timestamp ?? assistant.timestamp,
    title: user.title,
  };
}

function resolveIndexedNavigationSource(
  item: SessionRoundIndexItem,
  liveRoundIds: Set<string>,
  localization: SessionNavigatorLocalization,
): NavigationItemSource {
  const isLive = item.isLive || liveRoundIds.has(item.roundId);
  return {
    agentIds: item.agentIds,
    durationMs: item.durationMs,
    hasUserMessage: item.hasUserMessage,
    isLive,
    roundId: item.roundId,
    status: formatStatus(item.status, isLive, localization),
    summary: "",
    summaryFallback: localization.t("room.session_navigator_load_detail"),
    timestamp: item.timestamp,
    title: item.title,
  };
}

function projectNavigationItem(
  source: NavigationItemSource,
  index: number,
  localization: SessionNavigatorLocalization,
): SessionNavigationItem {
  const duration = formatDuration(source.durationMs);
  return {
    agentIds: normalizeAgentIds(source.agentIds),
    hasUserMessage: source.hasUserMessage,
    index,
    isLive: source.isLive,
    meta: [source.status, duration].filter(Boolean).join(" "),
    roundId: source.roundId,
    summary: source.isLive
      ? localization.t("room.session_navigator_current_processing")
      : compactText(source.summary, source.summaryFallback),
    time: source.timestamp
      ? formatRelativeTime(source.timestamp, localization.locale)
      : formatMessageTime(null),
    title: compactText(
      source.title,
      localization.t("room.session_navigator_round", { count: index + 1 }),
    ),
  };
}

/** 将唯一时间线投影转换为导航条展示模型，不在组件中重新分组消息。 */
export function buildSessionNavigationItems(
  timeline: ConversationTimeline,
  localization: SessionNavigatorLocalization,
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
      ? resolveLoadedNavigationSource(
        item.roundId,
        messages,
        liveRoundIdSet,
        localization,
      )
      : resolveIndexedNavigationSource(item, liveRoundIdSet, localization);
    return projectNavigationItem(source, index, localization);
  });
  const liveItems = missingLiveRoundIds.map((roundId, offset) => (
    projectNavigationItem(
      resolveLoadedNavigationSource(
        roundId,
        messageGroups.get(roundId) ?? [],
        liveRoundIdSet,
        localization,
      ),
      roundIndexItems.length + offset,
      localization,
    )
  ));
  return [...indexedItems, ...liveItems];
}
