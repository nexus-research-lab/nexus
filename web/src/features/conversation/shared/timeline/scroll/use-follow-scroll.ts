import { useCallback, useEffect, useLayoutEffect, useRef } from "react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";

import { isNearScrollBottom } from "./follow-scroll-model";
import { HistoryPrependAnchor } from "./history-prepend-anchor";
import { BottomScrollAnimator } from "./scroll-animation";
import { useFollowScrollInteractions } from "./use-follow-scroll-interactions";

interface UseFollowScrollOptions {
  messageCount: number;
  auxiliaryBlockCount?: number;
  auxiliaryBlockKey?: string | null;
  contentKey?: string | null;
  isLoading: boolean;
  sessionKey: string | null;
  historyPrependToken?: number;
}

interface UseFollowScrollReturn {
  scrollRef: React.RefObject<HTMLDivElement | null>;
  feedRef: React.RefObject<HTMLDivElement | null>;
  bottomAnchorRef: React.RefObject<HTMLDivElement | null>;
  showScrollToBottom: boolean;
  scrollToBottom: (behavior?: ScrollBehavior) => void;
  pauseFollowLatest: () => void;
  prepareHistoryPrependRestore: () => void;
  cancelHistoryPrependRestore: () => void;
  onScroll: () => void;
  onPointerDown: (event: React.PointerEvent<HTMLDivElement>) => void;
  onWheel: (event: React.WheelEvent<HTMLDivElement>) => void;
  onTouchStart: (event: React.TouchEvent<HTMLDivElement>) => void;
  onTouchMove: (event: React.TouchEvent<HTMLDivElement>) => void;
  onTouchEnd: () => void;
}

export function useFollowScroll({
  messageCount,
  auxiliaryBlockCount = 0,
  auxiliaryBlockKey = null,
  contentKey = null,
  isLoading,
  sessionKey,
  historyPrependToken = 0,
}: UseFollowScrollOptions): UseFollowScrollReturn {
  const scrollRef = useRef<HTMLDivElement>(null);
  const feedRef = useRef<HTMLDivElement>(null);
  const bottomAnchorRef = useRef<HTMLDivElement>(null);
  const shouldFollowLatestRef = useRef(true);
  const lastScrollTopRef = useRef(0);
  const visibilityRef = useRef(false);
  const historyAnchorRef = useRef(new HistoryPrependAnchor());
  const animatorRef = useRef<BottomScrollAnimator | null>(null);
  const [showScrollToBottom, setShowScrollToBottom] = useResettableState(
    false,
    sessionKey ?? "",
  );

  if (!animatorRef.current) {
    animatorRef.current = new BottomScrollAnimator(
      () => scrollRef.current,
      (scrollTop) => {
        lastScrollTopRef.current = scrollTop;
      },
    );
  }

  const setScrollToBottomVisibility = useCallback(
    (visible: boolean) => {
      if (visibilityRef.current === visible) {
        return;
      }
      visibilityRef.current = visible;
      setShowScrollToBottom(visible);
    },
    [setShowScrollToBottom],
  );

  const updateFollowState = useCallback(() => {
    const container = scrollRef.current;
    if (!container) {
      return;
    }
    const shouldFollow = isNearScrollBottom(container);
    shouldFollowLatestRef.current = shouldFollow;
    setScrollToBottomVisibility(!shouldFollow);
  }, [setScrollToBottomVisibility]);

  const cancelAnimation = useCallback(() => {
    animatorRef.current?.cancel();
  }, []);

  const scheduleScrollToBottom = useCallback(
    (behavior: ScrollBehavior = "smooth") => {
      animatorRef.current?.scroll(behavior);
    },
    [],
  );

  const scrollToBottom = useCallback(
    (behavior: ScrollBehavior = "smooth") => {
      shouldFollowLatestRef.current = true;
      setScrollToBottomVisibility(false);
      scheduleScrollToBottom(behavior);
    },
    [scheduleScrollToBottom, setScrollToBottomVisibility],
  );

  const pauseFollowLatest = useCallback(() => {
    cancelAnimation();
    shouldFollowLatestRef.current = false;
    setScrollToBottomVisibility(true);
  }, [cancelAnimation, setScrollToBottomVisibility]);

  const prepareHistoryPrependRestore = useCallback(() => {
    const container = scrollRef.current;
    if (!container) {
      return;
    }
    cancelAnimation();
    shouldFollowLatestRef.current = false;
    historyAnchorRef.current.prepare(container);
  }, [cancelAnimation]);

  const cancelHistoryPrependRestore = useCallback(() => {
    historyAnchorRef.current.cancel();
  }, []);

  useLayoutEffect(() => {
    if (!shouldFollowLatestRef.current) {
      setScrollToBottomVisibility(true);
      return;
    }
    scheduleScrollToBottom(isLoading ? "auto" : "smooth");
  }, [
    auxiliaryBlockCount,
    auxiliaryBlockKey,
    contentKey,
    isLoading,
    messageCount,
    scheduleScrollToBottom,
    setScrollToBottomVisibility,
  ]);

  useLayoutEffect(() => {
    const container = scrollRef.current;
    if (!container) {
      return;
    }
    const restoredScrollTop = historyAnchorRef.current.restore(container);
    if (restoredScrollTop === null) {
      return;
    }
    lastScrollTopRef.current = restoredScrollTop;
    setScrollToBottomVisibility(true);
  }, [historyPrependToken, setScrollToBottomVisibility]);

  useEffect(() => {
    const feed = feedRef.current;
    if (!feed || typeof ResizeObserver === "undefined") {
      return;
    }

    const observer = new ResizeObserver(() => {
      if (!shouldFollowLatestRef.current) {
        setScrollToBottomVisibility(true);
        return;
      }
      scheduleScrollToBottom("auto");
    });
    observer.observe(feed);
    return () => observer.disconnect();
  }, [scheduleScrollToBottom, setScrollToBottomVisibility]);

  useLayoutEffect(() => {
    shouldFollowLatestRef.current = true;
    historyAnchorRef.current.cancel();
    setScrollToBottomVisibility(false);
    scheduleScrollToBottom("auto");
  }, [scheduleScrollToBottom, sessionKey, setScrollToBottomVisibility]);

  useEffect(() => cancelAnimation, [cancelAnimation]);

  const interactions = useFollowScrollInteractions({
    lastScrollTopRef,
    pauseFollowLatest,
    scrollRef,
    updateFollowState,
  });

  return {
    scrollRef,
    feedRef,
    bottomAnchorRef,
    showScrollToBottom,
    scrollToBottom,
    pauseFollowLatest,
    prepareHistoryPrependRestore,
    cancelHistoryPrependRestore,
    ...interactions,
  };
}
