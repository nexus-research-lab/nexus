import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type RefObject,
} from "react";

import {
  clearConversationRoundNavigationTarget,
  CONVERSATION_ROUND_SELECTOR,
  findConversationRoundElement,
  getConversationRoundFocusOffset,
  getConversationRoundNavigationTarget,
  isConversationRoundScrollTargetVisible,
  scrollToConversationRoundElement,
  setConversationRoundNavigationTarget,
  type ConversationRoundScrollHandleRef,
  type ConversationRoundScrollOptions,
} from "../timeline/round-scroll";
import type { ConversationTimeline } from "../timeline/timeline-model";
import {
  buildSessionNavigationItems,
  type SessionNavigationItem,
} from "./session-navigator-model";

interface UseConversationSessionNavigationParams {
  timeline: ConversationTimeline;
  onLoadRoundWindow?: (roundId: string) => Promise<boolean>;
  onNavigateStart?: () => void;
  roundScrollRef?: ConversationRoundScrollHandleRef;
  scrollRef: RefObject<HTMLDivElement | null>;
}

const PENDING_SCROLL_MAX_FRAMES = 30;
const SCROLL_BOUNDARY_EPSILON_PX = 2;
const SCROLL_NAVIGATION_KEYS = new Set([
  "ArrowDown",
  "ArrowUp",
  "End",
  "Home",
  "PageDown",
  "PageUp",
  " ",
]);

function estimateRoundIndex(
  scrollElement: HTMLDivElement,
  roundIds: string[],
): number {
  if (roundIds.length <= 1) {
    return 0;
  }
  const maxScroll = Math.max(
    1,
    scrollElement.scrollHeight - scrollElement.clientHeight,
  );
  const ratio = Math.min(
    1,
    Math.max(0, scrollElement.scrollTop / maxScroll),
  );
  return Math.min(
    roundIds.length - 1,
    Math.max(0, Math.round(ratio * (roundIds.length - 1))),
  );
}

function resolveBoundaryRoundId(
  scrollElement: HTMLDivElement,
  roundIds: string[],
): string | undefined {
  if (scrollElement.scrollTop <= SCROLL_BOUNDARY_EPSILON_PX) {
    return roundIds[0];
  }
  const maxScroll = Math.max(
    0,
    scrollElement.scrollHeight - scrollElement.clientHeight,
  );
  if (scrollElement.scrollTop >= maxScroll - SCROLL_BOUNDARY_EPSILON_PX) {
    return roundIds[roundIds.length - 1];
  }
  return undefined;
}

function findFocusedVisibleRoundId(
  scrollElement: HTMLDivElement,
  roundIdSet: Set<string>,
): string | null {
  const elements = Array.from(
    scrollElement.querySelectorAll<HTMLElement>(CONVERSATION_ROUND_SELECTOR),
  );
  if (elements.length === 0) {
    return null;
  }
  const containerRect = scrollElement.getBoundingClientRect();
  const focusY = containerRect.top
    + getConversationRoundFocusOffset(scrollElement);
  let closestRoundId: string | null = null;
  let closestDistance = Number.POSITIVE_INFINITY;
  let containingRoundId: string | null = null;
  let containingTop = Number.NEGATIVE_INFINITY;

  for (const element of elements) {
    const roundId = element.dataset.conversationRoundId;
    if (!roundId || !roundIdSet.has(roundId)) {
      continue;
    }
    const rect = element.getBoundingClientRect();
    if (rect.bottom < containerRect.top || rect.top > containerRect.bottom) {
      continue;
    }
    if (rect.top <= focusY && rect.bottom >= focusY && rect.top > containingTop) {
      containingTop = rect.top;
      containingRoundId = roundId;
    }
    const distance = Math.abs(rect.top - focusY);
    if (distance < closestDistance) {
      closestDistance = distance;
      closestRoundId = roundId;
    }
  }
  return containingRoundId ?? closestRoundId;
}

function resolveVisibleRoundId(
  scrollElement: HTMLDivElement,
  roundIds: string[],
  roundIdSet: Set<string>,
): string | null {
  if (roundIds.length === 0) {
    return null;
  }
  const boundaryRoundId = resolveBoundaryRoundId(scrollElement, roundIds);
  if (boundaryRoundId) {
    return boundaryRoundId;
  }
  return findFocusedVisibleRoundId(scrollElement, roundIdSet)
    ?? roundIds[estimateRoundIndex(scrollElement, roundIds)]
    ?? null;
}

/**
 * 管理导航条与滚动容器之间的同步。
 * 导航目标只存于滚动容器，避免 ref 与 data 属性形成两份状态真相。
 */
export function useConversationSessionNavigation({
  timeline,
  onLoadRoundWindow,
  onNavigateStart,
  roundScrollRef,
  scrollRef,
}: UseConversationSessionNavigationParams) {
  const items = useMemo(
    () => buildSessionNavigationItems(timeline),
    [timeline],
  );
  const navigationRoundIds = useMemo(
    () => items.map((item) => item.roundId),
    [items],
  );
  const [activeRoundId, setActiveRoundId] = useState<string | null>(null);
  const [pendingScrollRoundId, setPendingScrollRoundId] = useState<
    string | null
  >(null);
  const [previewIndex, setPreviewIndex] = useState<number | null>(null);
  const loadingRoundIdRef = useRef<string | null>(null);
  const queuedLoadRoundIdRef = useRef<string | null>(null);

  useEffect(() => {
    if (navigationRoundIds.length === 0) {
      setActiveRoundId(null);
      return;
    }
    if (!activeRoundId || !navigationRoundIds.includes(activeRoundId)) {
      setActiveRoundId(navigationRoundIds[navigationRoundIds.length - 1]);
    }
  }, [activeRoundId, navigationRoundIds]);

  useEffect(() => {
    const scrollElement = scrollRef.current;
    if (!scrollElement || navigationRoundIds.length === 0) {
      return;
    }
    let frame = 0;
    const roundIdSet = new Set(navigationRoundIds);
    const syncActiveRound = (): void => {
      frame = 0;
      const navigationTarget = getConversationRoundNavigationTarget(
        scrollElement,
      );
      const nextRoundId = navigationTarget && roundIdSet.has(navigationTarget)
        ? navigationTarget
        : resolveVisibleRoundId(
            scrollElement,
            navigationRoundIds,
            roundIdSet,
          );
      if (nextRoundId) {
        setActiveRoundId((current) => (
          current === nextRoundId ? current : nextRoundId
        ));
      }
    };
    const scheduleSync = (): void => {
      if (!frame) {
        frame = window.requestAnimationFrame(syncActiveRound);
      }
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
    const clearNavigationTarget = (): void => {
      clearConversationRoundNavigationTarget(scrollElement);
    };
    const handleKeyDown = (event: KeyboardEvent): void => {
      if (SCROLL_NAVIGATION_KEYS.has(event.key)) {
        clearNavigationTarget();
      }
    };

    scrollElement.addEventListener("wheel", clearNavigationTarget, {
      passive: true,
    });
    scrollElement.addEventListener("touchstart", clearNavigationTarget, {
      passive: true,
    });
    window.addEventListener("keydown", handleKeyDown);
    return () => {
      clearNavigationTarget();
      scrollElement.removeEventListener("wheel", clearNavigationTarget);
      scrollElement.removeEventListener("touchstart", clearNavigationTarget);
      window.removeEventListener("keydown", handleKeyDown);
    };
  }, [scrollRef]);

  const scrollToTimelineRound = useCallback(
    (
      roundId: string,
      options?: ConversationRoundScrollOptions,
    ): boolean => {
      if (roundScrollRef?.current?.scrollToRoundId(roundId, options)) {
        return true;
      }
      const scrollElement = scrollRef.current;
      const target = scrollElement
        ? findConversationRoundElement(scrollElement, roundId)
        : null;
      if (!scrollElement || !target) {
        return false;
      }
      scrollToConversationRoundElement(scrollElement, target, options);
      return true;
    },
    [roundScrollRef, scrollRef],
  );

  const drainRoundWindowLoadQueue = useCallback(async (): Promise<void> => {
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
          setPendingScrollRoundId((current) => (
            current === roundId ? null : current
          ));
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
    const targetRoundId = pendingScrollRoundId;
    let frame = 0;
    let attemptCount = 0;

    function scheduleRetry(): void {
      if (attemptCount >= PENDING_SCROLL_MAX_FRAMES) {
        return;
      }
      attemptCount += 1;
      frame = window.requestAnimationFrame(tryScroll);
    }

    function tryScroll(): void {
      frame = 0;
      const isTargetLoaded =
        (timeline.message_groups.get(targetRoundId)?.length ?? 0) > 0
        || timeline.live_round_ids.includes(targetRoundId);
      const didScroll = scrollToTimelineRound(targetRoundId, {
        align: "focus",
        behavior: isTargetLoaded ? "auto" : "smooth",
      });
      if (!didScroll) {
        scheduleRetry();
        return;
      }
      if (!isTargetLoaded) {
        return;
      }
      const scrollElement = scrollRef.current;
      const target = scrollElement
        ? findConversationRoundElement(scrollElement, targetRoundId)
        : null;
      const isTargetVisible = Boolean(
        scrollElement
        && target
        && isConversationRoundScrollTargetVisible(scrollElement, target),
      );
      if (!isTargetVisible) {
        scheduleRetry();
        return;
      }
      setActiveRoundId(
        (scrollElement
          ? getConversationRoundNavigationTarget(scrollElement)
          : null) ?? targetRoundId,
      );
      setPendingScrollRoundId((current) => (
        current === targetRoundId ? null : current
      ));
    }

    frame = window.requestAnimationFrame(tryScroll);
    return () => {
      if (frame) {
        window.cancelAnimationFrame(frame);
      }
    };
  }, [pendingScrollRoundId, scrollRef, scrollToTimelineRound, timeline]);

  const jumpToRound = useCallback(
    (item: SessionNavigationItem): void => {
      const scrollElement = scrollRef.current;
      if (!scrollElement) {
        return;
      }
      const scrollRoundId = item.inputRoundId;
      const isScrollRoundLoaded =
        (timeline.message_groups.get(scrollRoundId)?.length ?? 0) > 0
        || timeline.live_round_ids.includes(scrollRoundId);

      onNavigateStart?.();
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
    },
    [
      drainRoundWindowLoadQueue,
      onLoadRoundWindow,
      onNavigateStart,
      scrollRef,
      scrollToTimelineRound,
      timeline,
    ],
  );

  const clearPreview = useCallback((): void => {
    setPreviewIndex(null);
  }, []);

  const previewItemAt = useCallback((item: SessionNavigationItem): void => {
    setPreviewIndex(item.index);
  }, []);

  const previewItem = previewIndex === null
    ? null
    : items[previewIndex] ?? null;
  const activeItem = items.find((item) => item.roundId === activeRoundId)
    ?? items[0]
    ?? null;

  return {
    activeItem,
    clearPreview,
    items,
    jumpToRound,
    previewIndex,
    previewItem,
    previewItemAt,
  };
}
