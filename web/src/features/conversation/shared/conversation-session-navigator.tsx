import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { RefObject } from "react";
import { ChevronRight, MessageSquareText } from "lucide-react";

import { cn, formatRelativeTime } from "@/lib/utils";
import {
  UiDialogBody,
  UiDialogHeader,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import {
  DIALOG_HEADER_ICON_CLASS_NAME,
  DIALOG_HEADER_LEADING_CLASS_NAME,
} from "@/shared/ui/dialog/dialog-styles";
import type { AssistantMessage, Message, UserMessage } from "@/types/conversation/message";
import type { SessionRoundIndexItem } from "@/types/conversation/room";
import type {
  ConversationRoundScrollHandleRef,
  ConversationRoundScrollOptions,
} from "./conversation-round-scroll";
import {
  clearConversationRoundNavigationTarget,
  CONVERSATION_ROUND_SELECTOR,
  findConversationRoundElement,
  getConversationRoundFocusOffset,
  getConversationRoundNavigationTarget,
  isConversationRoundScrollTargetVisible,
  scrollToConversationRoundElement,
  setConversationRoundNavigationTarget,
} from "./conversation-round-scroll";

import {
  extractTextFromContentBlocks,
  formatMessageTime,
  stripRoomControlMarkers,
} from "./message/item/message-item-support";

interface ConversationSessionNavigatorProps {
  agentNameMap?: Record<string, string>;
  className?: string;
  liveRoundIds: string[];
  messageGroups: Map<string, Message[]>;
  onLoadRoundWindow?: (roundId: string) => Promise<boolean>;
  onNavigateStart?: () => void;
  roundScrollRef?: ConversationRoundScrollHandleRef;
  roundIndexItems?: SessionRoundIndexItem[];
  scrollRef: RefObject<HTMLDivElement | null>;
}

interface SessionNavigationItem {
  duration: string | null;
  agentIds: string[];
  hasUserMessage: boolean;
  index: number;
  inputRoundId: string;
  isLive: boolean;
  meta: string;
  roundId: string;
  status: string;
  summary: string;
  time: string;
  title: string;
}

const PENDING_SCROLL_MAX_FRAMES = 30;
const RULER_TICK_SPACING_PX = 14;
const SCROLL_BOUNDARY_EPSILON_PX = 2;
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

function getRulerTrackHeight(itemCount: number): number {
  return Math.max(RULER_TICK_SPACING_PX, itemCount * RULER_TICK_SPACING_PX);
}

function getTickDisplayPercent(index: number, total: number): number {
  if (total <= 0) {
    return 50;
  }
  return ((index + 0.5) / total) * 100;
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

function uniqueAgentIds(messages: AssistantMessage[]): string[] {
  const seen = new Set<string>();
  const agentIds: string[] = [];
  for (const message of messages) {
    const agentId = message.agent_id?.trim();
    if (!agentId || seen.has(agentId)) {
      continue;
    }
    seen.add(agentId);
    agentIds.push(agentId);
  }
  return agentIds;
}

function normalizeAgentIds(agentIds: string[]): string[] {
  const seen = new Set<string>();
  const normalizedAgentIds: string[] = [];
  for (const agentId of agentIds) {
    const normalizedAgentId = agentId.trim();
    if (!normalizedAgentId || seen.has(normalizedAgentId)) {
      continue;
    }
    seen.add(normalizedAgentId);
    normalizedAgentIds.push(normalizedAgentId);
  }
  return normalizedAgentIds;
}

function buildTickSegments(item: SessionNavigationItem): string[] {
  const agentIds = normalizeAgentIds(item.agentIds).slice(0, 4);
  const segments: string[] = [];
  if (item.hasUserMessage) {
    segments.push(USER_TICK_COLOR);
  }
  for (const agentId of agentIds) {
    segments.push(getAgentTickColor(agentId));
  }
  if (segments.length === 0 && item.isLive) {
    segments.push(LIVE_TICK_COLOR);
  }
  if (segments.length === 0) {
    segments.push(NEUTRAL_TICK_COLOR);
  }
  return segments;
}

function buildTickBackground(item: SessionNavigationItem): string {
  const segments = buildTickSegments(item);
  if (segments.length === 1) {
    return segments[0];
  }

  const step = 100 / segments.length;
  const stops = segments.flatMap((color, index) => {
    const start = Math.max(0, index * step);
    const end = Math.min(100, (index + 1) * step);
    return [`${color} ${start}%`, `${color} ${end}%`];
  });
  return `linear-gradient(90deg, ${stops.join(", ")})`;
}

function formatAgentDisplayName(
  agentId: string,
  agentNameMap?: Record<string, string>,
): string {
  return agentNameMap?.[agentId] || `Agent ${agentId.slice(0, 6)}`;
}

function formatSpeakerSummary(
  item: SessionNavigationItem,
  agentNameMap?: Record<string, string>,
): string {
  const parts: string[] = [];
  if (item.hasUserMessage) {
    parts.push("用户");
  }
  const agentNames = normalizeAgentIds(item.agentIds)
    .map((agentId) => formatAgentDisplayName(agentId, agentNameMap));
  parts.push(...agentNames);
  return parts.join(" · ") || "未加载";
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
  if (isLive) {
    return "处理中";
  }
  switch (status) {
    case "error":
      return "失败";
    case "interrupted":
      return "已中断";
    case "success":
      return "已处理";
    default:
      return "已处理";
  }
}

function buildSessionNavigationItem(
  roundId: string,
  index: number,
  messages: Message[],
  liveRoundIds: Set<string>,
): SessionNavigationItem {
  const userMessage = messages.find(isUserMessage);
  const assistantMessages = messages.filter(isAssistantMessage);
  const firstAssistant = assistantMessages[0];
  const lastAssistant = assistantMessages[assistantMessages.length - 1];
  const title = compactText(userMessage?.content ?? "", `第 ${index + 1} 轮`);
  const resultSummary = lastAssistant?.result_summary;
  const assistantText = firstAssistant
    ? extractTextFromContentBlocks(firstAssistant.content)
    : "";
  const isLive = liveRoundIds.has(roundId);
  const summary = isLive
    ? "正在处理当前轮次"
    : compactText(resultSummary?.result ?? assistantText, "尚无回复内容");
  const timestamp = userMessage?.timestamp ?? firstAssistant?.timestamp ?? resultSummary?.timestamp ?? null;
  const duration = formatDuration(resultSummary?.duration_ms);
  const status = isLive
    ? "处理中"
    : resultSummary?.subtype === "error"
      ? "失败"
      : "已处理";
  const metaParts = [status, duration].filter(Boolean);
  return {
    duration,
    agentIds: uniqueAgentIds(assistantMessages),
    hasUserMessage: Boolean(userMessage),
    index,
    inputRoundId: roundId,
    isLive,
    meta: metaParts.join(" "),
    roundId,
    status,
    summary,
    time: timestamp ? formatRelativeTime(timestamp) : formatMessageTime(null),
    title,
  };
}

function buildIndexedSessionNavigationItem(
  item: SessionRoundIndexItem,
  index: number,
  messages: Message[],
  liveRoundIds: Set<string>,
): SessionNavigationItem {
  if (messages.length > 0) {
    return buildSessionNavigationItem(item.roundId, index, messages, liveRoundIds);
  }
  const isLive = item.isLive || liveRoundIds.has(item.roundId);
  const duration = formatDuration(item.durationMs);
  const status = formatIndexedStatus(item.status, isLive);
  const metaParts = [status, duration].filter(Boolean);
  return {
    duration,
    agentIds: normalizeAgentIds(item.agentIds),
    hasUserMessage: item.hasUserMessage,
    index,
    inputRoundId: item.roundId,
    isLive,
    meta: metaParts.join(" "),
    roundId: item.roundId,
    status,
    summary: isLive ? "正在处理当前轮次" : "滚动加载后可查看详情",
    time: item.timestamp ? formatRelativeTime(item.timestamp) : formatMessageTime(null),
    title: compactText(item.title, `第 ${index + 1} 轮`),
  };
}

function estimateRoundIndex(
  scrollElement: HTMLDivElement,
  roundIds: string[],
): number {
  if (roundIds.length <= 1) {
    return 0;
  }
  const maxScroll = Math.max(1, scrollElement.scrollHeight - scrollElement.clientHeight);
  const ratio = Math.min(1, Math.max(0, scrollElement.scrollTop / maxScroll));
  return Math.min(roundIds.length - 1, Math.max(0, Math.round(ratio * (roundIds.length - 1))));
}

function resolveVisibleRoundId(
  scrollElement: HTMLDivElement,
  roundIds: string[],
  roundIdSet: Set<string>,
): string | null {
  if (roundIds.length === 0) {
    return null;
  }
  if (scrollElement.scrollTop <= SCROLL_BOUNDARY_EPSILON_PX) {
    return roundIds[0] ?? null;
  }
  const maxScroll = Math.max(0, scrollElement.scrollHeight - scrollElement.clientHeight);
  if (scrollElement.scrollTop >= maxScroll - SCROLL_BOUNDARY_EPSILON_PX) {
    return roundIds[roundIds.length - 1] ?? null;
  }
  const elements = Array.from(
    scrollElement.querySelectorAll<HTMLElement>(CONVERSATION_ROUND_SELECTOR),
  );
  if (elements.length === 0) {
    return roundIds[estimateRoundIndex(scrollElement, roundIds)] ?? null;
  }

  const containerRect = scrollElement.getBoundingClientRect();
  const focusY = containerRect.top + getConversationRoundFocusOffset(scrollElement);
  let bestRoundId: string | null = null;
  let bestDistance = Number.POSITIVE_INFINITY;
  let containingRoundId: string | null = null;
  let containingTop = Number.NEGATIVE_INFINITY;

  for (const element of elements) {
    const directRoundId = element.dataset.conversationRoundId;
    const candidateRoundId = directRoundId && roundIdSet.has(directRoundId)
      ? directRoundId
      : null;
    if (!candidateRoundId) {
      continue;
    }
    const rect = element.getBoundingClientRect();
    if (rect.bottom < containerRect.top || rect.top > containerRect.bottom) {
      continue;
    }
    if (rect.top <= focusY && rect.bottom >= focusY && rect.top > containingTop) {
      containingTop = rect.top;
      containingRoundId = candidateRoundId;
    }
    const distance = Math.abs(rect.top - focusY);
    if (distance < bestDistance) {
      bestDistance = distance;
      bestRoundId = candidateRoundId;
    }
  }

  if (containingRoundId) {
    return containingRoundId;
  }
  if (bestRoundId) {
    return bestRoundId;
  }
  return roundIds[estimateRoundIndex(scrollElement, roundIds)] ?? null;
}

export function ConversationSessionNavigator({
  agentNameMap,
  className,
  liveRoundIds,
  messageGroups,
  onLoadRoundWindow,
  onNavigateStart,
  roundScrollRef,
  roundIndexItems = [],
  scrollRef,
}: ConversationSessionNavigatorProps) {
  const [activeRoundId, setActiveRoundId] = useState<string | null>(null);
  const [pendingScrollRoundId, setPendingScrollRoundId] = useState<string | null>(null);
  const loadingRoundIdRef = useRef<string | null>(null);
  const navigationTargetRoundIdRef = useRef<string | null>(null);
  const previewClickItemRef = useRef<SessionNavigationItem | null>(null);
  const queuedLoadRoundIdRef = useRef<string | null>(null);
  const [previewIndex, setPreviewIndex] = useState<number | null>(null);

  const items = useMemo(() => {
    const live = new Set(liveRoundIds);
    const indexedRoundIds = new Set(roundIndexItems.map((item) => item.roundId));
    const missingLiveRoundIds = liveRoundIds.filter((roundId, index) =>
      roundId.trim() !== "" &&
      !indexedRoundIds.has(roundId) &&
      liveRoundIds.indexOf(roundId) === index,
    );
    const total = roundIndexItems.length + missingLiveRoundIds.length;
    if (total === 0) {
      return [];
    }

    const indexedItems = roundIndexItems.map((item, index) =>
      buildIndexedSessionNavigationItem(
        item,
        index,
        messageGroups.get(item.roundId) ?? [],
        live,
      ),
    );
    const liveItems = missingLiveRoundIds.map((roundId, offset) =>
      buildSessionNavigationItem(
        roundId,
        roundIndexItems.length + offset,
        messageGroups.get(roundId) ?? [],
        live,
      ),
    );
    const builtItems = [...indexedItems, ...liveItems];
    let currentInputRoundId: string | null = null;
    return builtItems.map((item) => {
      if (item.hasUserMessage) {
        currentInputRoundId = item.roundId;
      }
      return {
        ...item,
        inputRoundId: currentInputRoundId ?? item.roundId,
      };
    });
  }, [liveRoundIds, messageGroups, roundIndexItems]);

  const navigationRoundIds = useMemo(
    () => items.map((item) => item.roundId),
    [items],
  );

  useEffect(() => {
    if (navigationRoundIds.length === 0) {
      setActiveRoundId(null);
      return;
    }
    if (!activeRoundId || !navigationRoundIds.includes(activeRoundId)) {
      setActiveRoundId(navigationRoundIds[navigationRoundIds.length - 1] ?? null);
    }
  }, [activeRoundId, navigationRoundIds]);

  useEffect(() => {
    const scrollElement = scrollRef.current;
    if (!scrollElement || navigationRoundIds.length === 0) {
      return;
    }

    let frame = 0;
    const roundIdSet = new Set(navigationRoundIds);
    const syncActiveRound = () => {
      frame = 0;
      const navigationTargetRoundId = getConversationRoundNavigationTarget(scrollElement);
      if (navigationTargetRoundId && roundIdSet.has(navigationTargetRoundId)) {
        setActiveRoundId((current) =>
          current === navigationTargetRoundId ? current : navigationTargetRoundId,
        );
        return;
      }
      const nextRoundId = resolveVisibleRoundId(scrollElement, navigationRoundIds, roundIdSet);
      if (nextRoundId) {
        setActiveRoundId((current) => current === nextRoundId ? current : nextRoundId);
      }
    };
    const scheduleSync = () => {
      if (frame) {
        return;
      }
      frame = window.requestAnimationFrame(syncActiveRound);
    };

    syncActiveRound();
    scrollElement.addEventListener("scroll", scheduleSync, { passive: true });
    window.addEventListener("resize", scheduleSync);
    return () => {
      if (frame) {
        window.cancelAnimationFrame(frame);
      }
      scrollElement.removeEventListener("scroll", scheduleSync);
      window.removeEventListener("resize", scheduleSync);
    };
  }, [navigationRoundIds, scrollRef]);

  useEffect(() => {
    const scrollElement = scrollRef.current;
    if (!scrollElement) {
      return;
    }

    const clearNavigationTarget = () => {
      navigationTargetRoundIdRef.current = null;
      clearConversationRoundNavigationTarget(scrollElement);
    };
    const handleKeyDown = (event: KeyboardEvent) => {
      const scrollKeys = new Set([
        "ArrowDown",
        "ArrowUp",
        "End",
        "Home",
        "PageDown",
        "PageUp",
        " ",
      ]);
      if (scrollKeys.has(event.key)) {
        clearNavigationTarget();
      }
    };

    scrollElement.addEventListener("wheel", clearNavigationTarget, { passive: true });
    scrollElement.addEventListener("touchstart", clearNavigationTarget, { passive: true });
    window.addEventListener("keydown", handleKeyDown);
    return () => {
      clearNavigationTarget();
      scrollElement.removeEventListener("wheel", clearNavigationTarget);
      scrollElement.removeEventListener("touchstart", clearNavigationTarget);
      window.removeEventListener("keydown", handleKeyDown);
    };
  }, [scrollRef]);

  const scrollToTimelineRound = useCallback((
    roundId: string,
    options?: ConversationRoundScrollOptions,
  ): boolean => {
    if (roundScrollRef?.current?.scrollToRoundId(roundId, options)) {
      return true;
    }
    const scrollElement = scrollRef.current;
    if (!scrollElement) {
      return false;
    }
    const target = findConversationRoundElement(scrollElement, roundId);
    if (!target) {
      return false;
    }
    scrollToConversationRoundElement(scrollElement, target, options);
    return true;
  }, [roundScrollRef, scrollRef]);

  const drainRoundWindowLoadQueue = useCallback(async () => {
    if (!onLoadRoundWindow || loadingRoundIdRef.current) {
      return;
    }

    while (queuedLoadRoundIdRef.current) {
      const roundId = queuedLoadRoundIdRef.current;
      queuedLoadRoundIdRef.current = null;
      loadingRoundIdRef.current = roundId;
      try {
        const loaded = await onLoadRoundWindow(roundId);
        if (!loaded) {
          setPendingScrollRoundId((current) =>
            current === roundId ? null : current,
          );
        }
      } finally {
        if (loadingRoundIdRef.current === roundId) {
          loadingRoundIdRef.current = null;
        }
      }
    }
  }, [onLoadRoundWindow]);

  useEffect(() => {
    if (!pendingScrollRoundId) {
      return;
    }
    let frame = 0;
    let attemptCount = 0;

    const tryScroll = () => {
      frame = 0;
      const isTargetLoaded =
        (messageGroups.get(pendingScrollRoundId)?.length ?? 0) > 0 ||
        liveRoundIds.includes(pendingScrollRoundId);
      const didScroll = scrollToTimelineRound(pendingScrollRoundId, {
        align: "focus",
        behavior: isTargetLoaded ? "auto" : "smooth",
      });
      if (didScroll && isTargetLoaded) {
        const scrollElement = scrollRef.current;
        const target = scrollElement
          ? findConversationRoundElement(scrollElement, pendingScrollRoundId)
          : null;
        const isTargetVisible = scrollElement && target
          ? isConversationRoundScrollTargetVisible(scrollElement, target)
          : false;
        if (!isTargetVisible) {
          if (attemptCount >= PENDING_SCROLL_MAX_FRAMES) {
            return;
          }
          attemptCount += 1;
          frame = window.requestAnimationFrame(tryScroll);
          return;
        }
        const activeTargetRoundId =
          navigationTargetRoundIdRef.current ||
          (scrollElement ? getConversationRoundNavigationTarget(scrollElement) : null) ||
          pendingScrollRoundId;
        setActiveRoundId(activeTargetRoundId);
        setPendingScrollRoundId((current) =>
          current === pendingScrollRoundId ? null : current,
        );
        return;
      }
      if (didScroll && !isTargetLoaded) {
        return;
      }
      if (attemptCount >= PENDING_SCROLL_MAX_FRAMES) {
        return;
      }
      attemptCount += 1;
      frame = window.requestAnimationFrame(tryScroll);
    };

    frame = window.requestAnimationFrame(tryScroll);
    return () => {
      if (frame) {
        window.cancelAnimationFrame(frame);
      }
    };
  }, [liveRoundIds, messageGroups, pendingScrollRoundId, scrollRef, scrollToTimelineRound]);

  const jumpToRound = useCallback((item: SessionNavigationItem) => {
    const scrollElement = scrollRef.current;
    if (!scrollElement) {
      return;
    }
    const scrollRoundId = item.inputRoundId || item.roundId;
    const isScrollRoundLoaded =
      (messageGroups.get(scrollRoundId)?.length ?? 0) > 0 ||
      liveRoundIds.includes(scrollRoundId);

    onNavigateStart?.();
    navigationTargetRoundIdRef.current = item.roundId;
    setConversationRoundNavigationTarget(scrollElement, item.roundId);
    const didScroll = scrollToTimelineRound(scrollRoundId, {
      align: "focus",
      behavior: "smooth",
    });
    setActiveRoundId(item.roundId);
    setPendingScrollRoundId(scrollRoundId);

    if (!isScrollRoundLoaded) {
      if (!onLoadRoundWindow || loadingRoundIdRef.current === scrollRoundId) {
        return;
      }
      queuedLoadRoundIdRef.current = scrollRoundId;
      void drainRoundWindowLoadQueue();
      return;
    }

    if (!didScroll) {
      setPendingScrollRoundId(scrollRoundId);
    }
  }, [
    drainRoundWindowLoadQueue,
    liveRoundIds,
    messageGroups,
    onNavigateStart,
    onLoadRoundWindow,
    scrollRef,
    scrollToTimelineRound,
  ]);

  const jumpToPreviewClickTarget = useCallback((item: SessionNavigationItem) => {
    const targetItem = previewClickItemRef.current ?? item;
    previewClickItemRef.current = null;
    void jumpToRound(targetItem);
  }, [jumpToRound]);

  if (items.length <= 1) {
    return null;
  }

  const previewItem = previewIndex === null ? null : items[previewIndex] ?? null;
  const activeItem = items.find((item) => item.roundId === activeRoundId) ?? items[0];
  const trackHeight = getRulerTrackHeight(items.length);

  return (
    <nav
      aria-label="会话导航"
      className={cn(
        "pointer-events-none hidden h-auto w-11 select-none xl:block",
        className,
      )}
      onMouseLeave={() => {
        setPreviewIndex(null);
      }}
    >
      <div className="relative h-full min-h-[220px] w-full">
        <div
          className="pointer-events-auto absolute left-0 top-1/2 flex w-12 -translate-y-1/2 flex-col overflow-visible"
          style={{ height: `min(100%, ${trackHeight}px)` }}
          onPointerLeave={() => {
            setPreviewIndex(null);
          }}
        >
          {items.map((item) => {
            const isActive = item.roundId === activeItem?.roundId;
            const isPreviewed = previewItem?.roundId === item.roundId;
            const wave = previewIndex === null
              ? 0
              : smoothWave(Math.abs(item.index - previewIndex));
            const opacity = previewIndex === null
              ? isActive ? 0.9 : 0.58
              : tickOpacity(wave);
            const width = previewIndex === null ? 5 : tickWidth(wave);
            const tickBackground = isPreviewed
              ? buildTickBackground(item)
              : previewIndex === null && isActive
                ? ACTIVE_NEUTRAL_TICK_COLOR
                : NEUTRAL_TICK_COLOR;
            return (
              <button
                key={item.roundId}
                type="button"
                aria-current={isActive ? "true" : undefined}
                aria-label={`跳转到${item.title}`}
                className="flex min-h-0 w-12 flex-1 items-center justify-start rounded-sm outline-none focus-visible:ring-2 focus-visible:ring-primary/35"
                onClick={() => {
                  void jumpToRound(item);
                }}
                onFocus={() => {
                  previewClickItemRef.current = item;
                  setPreviewIndex(item.index);
                }}
                onPointerEnter={() => {
                  previewClickItemRef.current = item;
                  setPreviewIndex(item.index);
                }}
              >
                <span
                  className="block h-[2px] rounded-full transition-[width,opacity,filter] duration-[90ms] ease-out"
                  style={{
                    background: tickBackground,
                    filter: isPreviewed ? "saturate(1.18)" : undefined,
                    opacity,
                    width,
                  }}
                />
              </button>
            );
          })}

          <div
            className="pointer-events-none absolute left-0 top-[calc(100%+12px)] hidden w-32 items-center gap-1 text-[12px] font-medium text-(--text-muted) 2xl:flex"
            aria-hidden
          >
            <span className="truncate">
              {activeItem?.duration ? `已处理 ${activeItem.duration}` : activeItem?.status}
            </span>
            <span className="text-(--icon-muted)">›</span>
          </div>

          {previewItem ? (
            <UiDialogShell
              className="pointer-events-auto absolute left-12 z-[60] w-[min(332px,calc(100vw-96px))] max-w-none -translate-y-1/2 outline-none"
              data-session-navigator-preview="true"
              size="sm"
              style={{ top: `${getTickDisplayPercent(previewItem.index, items.length)}%` }}
              onPointerDown={(event) => {
                event.stopPropagation();
              }}
              onPointerEnter={(event) => {
                event.stopPropagation();
              }}
              onPointerMove={(event) => {
                event.stopPropagation();
              }}
            >
              <UiDialogHeader
                className="cursor-pointer gap-2 px-3 py-2.5"
                onClick={() => {
                  jumpToPreviewClickTarget(previewItem);
                }}
              >
                <div className={cn(DIALOG_HEADER_LEADING_CLASS_NAME, "min-w-0 flex-1 items-center")}>
                  <div
                    className={cn(
                      DIALOG_HEADER_ICON_CLASS_NAME,
                      "h-7 w-7 rounded-[10px] bg-primary/10 text-primary",
                    )}
                  >
                    <MessageSquareText className="h-3.5 w-3.5" />
                  </div>
                  <div className="min-w-0 flex-1">
                    <h3 className="truncate text-[13px] font-semibold leading-[18px] text-(--text-strong)">
                      {previewItem.title}
                    </h3>
                    <p className="mt-0.5 truncate text-[11px] leading-4 text-(--text-muted)">
                      {previewItem.time}
                    </p>
                  </div>
                  <ChevronRight className="h-3.5 w-3.5 shrink-0 text-(--icon-muted)" />
                </div>
              </UiDialogHeader>
              <UiDialogBody
                className="cursor-pointer px-3 py-2.5"
                onClick={() => {
                  jumpToPreviewClickTarget(previewItem);
                }}
              >
                <p className="line-clamp-2 text-[11px] leading-[18px] text-(--text-default)">
                  {previewItem.summary}
                </p>
                <div className="mt-2 flex min-w-0 items-center gap-1.5 text-[10px] font-medium leading-4 text-(--text-soft)">
                  <span
                    className={cn(
                      "h-1.5 w-1.5 shrink-0 rounded-full",
                      previewItem.isLive ? "bg-primary" : "bg-(--icon-muted)",
                    )}
                    style={{ background: buildTickBackground(previewItem) }}
                  />
                  <span className="truncate">{formatSpeakerSummary(previewItem, agentNameMap)}</span>
                  <span className="text-(--text-soft)">·</span>
                  <span className="truncate">{previewItem.meta}</span>
                </div>
              </UiDialogBody>
            </UiDialogShell>
          ) : null}
        </div>
      </div>
    </nav>
  );
}
