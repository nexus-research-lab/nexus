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
  CONVERSATION_ROUND_SELECTOR,
  findConversationRoundElement,
  scrollToConversationRoundElement,
} from "./conversation-round-scroll";

import {
  extractTextFromContentBlocks,
  formatMessageTime,
  stripRoomControlMarkers,
} from "./message/item/message-item-support";

interface ConversationSessionNavigatorProps {
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
  index: number;
  isLive: boolean;
  isLoaded: boolean;
  meta: string;
  percent: number;
  roundId: string;
  status: string;
  summary: string;
  time: string;
  title: string;
}

const PENDING_SCROLL_MAX_FRAMES = 12;
const RULER_TICK_SPACING_PX = 14;
const WAVE_RADIUS_TICKS = 4.5;

function clampPercent(percent: number): number {
  return Math.min(88, Math.max(12, percent));
}

function clampIndex(index: number, total: number): number {
  return Math.min(total - 1, Math.max(0, index));
}

function itemIndexForPercent(percent: number, itemCount: number): number {
  if (itemCount <= 1) {
    return 0;
  }
  return clampIndex(Math.round((percent / 100) * (itemCount - 1)), itemCount);
}

function getRulerTrackHeight(itemCount: number): number {
  return Math.max(RULER_TICK_SPACING_PX, (itemCount - 1) * RULER_TICK_SPACING_PX);
}

function smoothWave(distanceTicks: number): number {
  const normalized = Math.max(0, 1 - distanceTicks / WAVE_RADIUS_TICKS);
  return normalized * normalized * (3 - 2 * normalized);
}

function tickWidth(wave: number): number {
  return Math.round(5 + wave * 10);
}

function tickOpacity(wave: number): number {
  return Math.min(1, 0.34 + wave * 0.5);
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
  total: number,
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
    index,
    isLive,
    isLoaded: messages.length > 0,
    meta: metaParts.join(" "),
    percent: total <= 1 ? 50 : (index / (total - 1)) * 100,
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
  total: number,
  messages: Message[],
  liveRoundIds: Set<string>,
): SessionNavigationItem {
  if (messages.length > 0) {
    return buildSessionNavigationItem(item.roundId, index, total, messages, liveRoundIds);
  }
  const isLive = item.isLive || liveRoundIds.has(item.roundId);
  const duration = formatDuration(item.durationMs);
  const status = formatIndexedStatus(item.status, isLive);
  const metaParts = [status, duration].filter(Boolean);
  return {
    duration,
    index,
    isLive,
    isLoaded: false,
    meta: metaParts.join(" "),
    percent: total <= 1 ? 50 : (index / (total - 1)) * 100,
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
  const elements = Array.from(
    scrollElement.querySelectorAll<HTMLElement>(CONVERSATION_ROUND_SELECTOR),
  );
  if (elements.length === 0) {
    return roundIds[estimateRoundIndex(scrollElement, roundIds)] ?? null;
  }

  const containerRect = scrollElement.getBoundingClientRect();
  const focusY = containerRect.top + Math.min(180, scrollElement.clientHeight * 0.34);
  let bestRoundId: string | null = null;
  let bestDistance = Number.POSITIVE_INFINITY;

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
    const distance = Math.abs(rect.top - focusY);
    if (distance < bestDistance) {
      bestDistance = distance;
      bestRoundId = candidateRoundId;
    }
  }

  if (bestRoundId) {
    return bestRoundId;
  }
  return roundIds[estimateRoundIndex(scrollElement, roundIds)] ?? null;
}

export function ConversationSessionNavigator({
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
  const queuedLoadRoundIdRef = useRef<string | null>(null);
  const [previewIndex, setPreviewIndex] = useState<number | null>(null);
  const [hoverPercent, setHoverPercent] = useState<number | null>(null);

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
        total,
        messageGroups.get(item.roundId) ?? [],
        live,
      ),
    );
    const liveItems = missingLiveRoundIds.map((roundId, offset) =>
      buildSessionNavigationItem(
        roundId,
        roundIndexItems.length + offset,
        total,
        messageGroups.get(roundId) ?? [],
        live,
      ),
    );
    return [...indexedItems, ...liveItems];
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
        behavior: isTargetLoaded ? "auto" : "smooth",
      });
      if (didScroll && isTargetLoaded) {
        setActiveRoundId(pendingScrollRoundId);
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
  }, [liveRoundIds, messageGroups, pendingScrollRoundId, scrollToTimelineRound]);

  const jumpToRound = useCallback((item: SessionNavigationItem) => {
    const scrollElement = scrollRef.current;
    if (!scrollElement) {
      return;
    }

    onNavigateStart?.();
    const didScroll = scrollToTimelineRound(item.roundId, {
      behavior: "smooth",
    });
    setActiveRoundId(item.roundId);

    if (!item.isLoaded) {
      setPendingScrollRoundId(item.roundId);
      if (!onLoadRoundWindow || loadingRoundIdRef.current === item.roundId) {
        return;
      }
      queuedLoadRoundIdRef.current = item.roundId;
      void drainRoundWindowLoadQueue();
      return;
    }

    if (!didScroll) {
      setPendingScrollRoundId(item.roundId);
    }
  }, [
    drainRoundWindowLoadQueue,
    onNavigateStart,
    onLoadRoundWindow,
    scrollRef,
    scrollToTimelineRound,
  ]);

  if (items.length <= 1) {
    return null;
  }

  const handlePointerMove = (event: React.PointerEvent<HTMLDivElement>) => {
    const rect = event.currentTarget.getBoundingClientRect();
    const percent = Math.min(100, Math.max(0, ((event.clientY - rect.top) / rect.height) * 100));
    setHoverPercent(percent);
    setPreviewIndex(itemIndexForPercent(percent, items.length));
  };

  const previewItem = previewIndex === null ? null : items[previewIndex] ?? null;
  const activeItem = items.find((item) => item.roundId === activeRoundId) ?? items[0];
  const hoverIndex = hoverPercent === null ? null : (hoverPercent / 100) * (items.length - 1);
  const trackHeight = getRulerTrackHeight(items.length);

  return (
    <nav
      aria-label="会话导航"
      className={cn(
        "pointer-events-none hidden h-auto w-11 select-none xl:block",
        className,
      )}
      onMouseLeave={() => {
        setHoverPercent(null);
        setPreviewIndex(null);
      }}
    >
      <div className="relative h-full min-h-[220px] w-full">
        <div
          className="pointer-events-auto absolute left-0 top-1/2 w-12 -translate-y-1/2 overflow-visible"
          style={{ height: `min(100%, ${trackHeight}px)` }}
          onPointerMove={handlePointerMove}
          onPointerLeave={() => {
            setHoverPercent(null);
            setPreviewIndex(null);
          }}
        >
          {items.map((item) => {
            const percent = item.percent;
            const isActive = item.roundId === activeItem?.roundId;
            const isLiveTick = item.isLive;
            const hoverDistance = hoverIndex === null
              ? Number.POSITIVE_INFINITY
              : Math.abs(item.index - hoverIndex);
            const wave = smoothWave(hoverDistance);
            const width = tickWidth(wave);
            const isHoverTinted = wave > 0.16;
            const isActiveTinted = hoverPercent === null && isActive;
            const opacity = isActiveTinted
              ? 0.92
              : tickOpacity(wave);
            return (
              <button
                key={item.roundId}
                type="button"
                aria-current={isActive ? "true" : undefined}
                aria-label={`跳转到${item.title}`}
                className="absolute left-0 flex h-3.5 w-12 -translate-y-1/2 items-center justify-start rounded-sm outline-none focus-visible:ring-2 focus-visible:ring-primary/35"
                style={{ top: `${percent}%` }}
                onClick={() => {
                  void jumpToRound(item);
                }}
                onFocus={() => setPreviewIndex(item.index)}
              >
                <span
                  className={cn(
                    "block h-[2px] rounded-full transition-[width,background-color,opacity] duration-[90ms] ease-out",
                    isActiveTinted || isHoverTinted
                      ? "bg-(--text-strong)"
                      : isLiveTick
                        ? "bg-primary/70"
                        : "bg-(--text-muted)",
                  )}
                  style={{
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
              size="sm"
              style={{ top: `${clampPercent(previewItem.percent)}%` }}
            >
              <UiDialogHeader
                className="cursor-pointer gap-2 px-3 py-2.5"
                onClick={() => {
                  void jumpToRound(previewItem);
                }}
              >
                <div className={cn(DIALOG_HEADER_LEADING_CLASS_NAME, "min-w-0 flex-1 items-center")}>
                  <div className={cn(DIALOG_HEADER_ICON_CLASS_NAME, "h-7 w-7 rounded-[10px] text-primary")}>
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
                  void jumpToRound(previewItem);
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
                  />
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
