/**
 * INPUT: 会话内容版本、历史前插令牌与滚动容器尺寸变化。
 * OUTPUT: DM、Room、Thread 共用的跟随状态、定位入口与用户滚动处理器。
 * POS: FOLLOW 同步贴底、READING 锚定、历史恢复和资源清理的 React 编排层。
 */
import { useCallback, useEffect, useLayoutEffect, useRef } from "react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";

import {
  getConversationViewportSize,
  hasScrollableOverflow,
  isAtScrollBottom,
  resolveConversationViewportResizeState,
  resolveConversationViewportSizeRevision,
} from "./follow-scroll-model";
import { ConversationViewportAnchor } from "./conversation-viewport-anchor";
import { HistoryPrependAnchor } from "./history-prepend-anchor";
import { BottomScrollAnimator } from "./scroll-animation";
import { useFollowScrollInteractions } from "./use-follow-scroll-interactions";

interface UseFollowScrollOptions {
  messageCount: number;
  atomicLayoutKey?: string | null;
  contentKey?: string | null;
  historyPrependToken?: number;
  sessionKey: string | null;
  topologyKey?: string | null;
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
  atomicLayoutKey = null,
  contentKey = null,
  historyPrependToken = 0,
  sessionKey,
  topologyKey = null,
}: UseFollowScrollOptions): UseFollowScrollReturn {
  const scrollRef = useRef<HTMLDivElement>(null);
  const feedRef = useRef<HTMLDivElement>(null);
  const bottomAnchorRef = useRef<HTMLDivElement>(null);
  const shouldFollowLatestRef = useRef(true);
  const lastScrollTopRef = useRef(0);
  const revisionSessionKeyRef = useRef<string | null>(null);
  const previousTopologyKeyRef = useRef<string | null>(null);
  const viewportSizeRef = useRef<ReturnType<
    typeof getConversationViewportSize
  > | null>(null);
  const visibilityRef = useRef(false);
  const historyAnchorRef = useRef(new HistoryPrependAnchor());
  const viewportAnchorRef = useRef(new ConversationViewportAnchor());
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
    const shouldFollow = isAtScrollBottom(container);
    shouldFollowLatestRef.current = shouldFollow;
    setScrollToBottomVisibility(
      hasScrollableOverflow(container) && !shouldFollow,
    );
  }, [setScrollToBottomVisibility]);

  const cancelAnimation = useCallback(() => {
    animatorRef.current?.cancel();
  }, []);

  const retainPositionForViewportResize = useCallback((
    container: HTMLDivElement,
  ): boolean => {
    const nextSize = getConversationViewportSize(container);
    const sizeRevision = resolveConversationViewportSizeRevision(
      viewportSizeRef.current,
      nextSize,
    );
    viewportSizeRef.current = sizeRevision.baseline;
    if (!sizeRevision.changed) {
      return false;
    }
    const resizeState = resolveConversationViewportResizeState(
      container,
      lastScrollTopRef.current,
      shouldFollowLatestRef.current,
    );
    cancelAnimation();
    container.scrollTop = resizeState.scrollTop;
    lastScrollTopRef.current = container.scrollTop;
    shouldFollowLatestRef.current = resizeState.shouldFollow;
    if (resizeState.shouldFollow) {
      viewportAnchorRef.current.reset();
    } else {
      viewportAnchorRef.current.capture(container, feedRef.current);
    }
    setScrollToBottomVisibility(resizeState.showScrollToBottom);
    return true;
  }, [cancelAnimation, setScrollToBottomVisibility]);

  const scheduleScrollToBottom = useCallback(
    (behavior: ScrollBehavior = "smooth") => {
      animatorRef.current?.scroll(behavior);
    },
    [],
  );

  const scheduleFollowLatest = useCallback(() => {
    const container = scrollRef.current;
    if (
      container
      && retainPositionForViewportResize(container)
    ) {
      // 该尺寸变化属于 Composer/虚拟键盘/App viewport，不属于正文增长。
      // resize 所有者已按当前 FOLLOW/READING 意图完成同步写入。
      return;
    }
    animatorRef.current?.follow();
  }, [retainPositionForViewportResize]);

  const scrollToBottom = useCallback(
    (behavior: ScrollBehavior = "smooth") => {
      shouldFollowLatestRef.current = true;
      viewportAnchorRef.current.reset();
      setScrollToBottomVisibility(false);
      scheduleScrollToBottom(behavior);
    },
    [scheduleScrollToBottom, setScrollToBottomVisibility],
  );

  const pauseFollowLatest = useCallback(() => {
    const container = scrollRef.current;
    if (!container || !hasScrollableOverflow(container)) {
      shouldFollowLatestRef.current = true;
      setScrollToBottomVisibility(false);
      return;
    }
    cancelAnimation();
    shouldFollowLatestRef.current = false;
    viewportAnchorRef.current.capture(container, feedRef.current);
    setScrollToBottomVisibility(!isAtScrollBottom(container));
  }, [cancelAnimation, setScrollToBottomVisibility]);

  const prepareHistoryPrependRestore = useCallback(() => {
    const container = scrollRef.current;
    if (!container) {
      return;
    }
    cancelAnimation();
    shouldFollowLatestRef.current = false;
    viewportAnchorRef.current.reset();
    historyAnchorRef.current.prepare(container);
  }, [cancelAnimation]);

  const cancelHistoryPrependRestore = useCallback(() => {
    historyAnchorRef.current.cancel();
  }, []);

  useLayoutEffect(() => {
    const container = scrollRef.current;
    const isNewSession = revisionSessionKeyRef.current !== sessionKey;
    const topologyChanged = (
      !isNewSession
      && previousTopologyKeyRef.current !== topologyKey
    );
    revisionSessionKeyRef.current = sessionKey;
    previousTopologyKeyRef.current = topologyKey;
    if (!container) {
      return;
    }

    if (shouldFollowLatestRef.current) {
      // FOLLOW 独占真实 bottom；内容、终态或权限布局提交都必须在 paint 前
      // 同步收口，不能先恢复可见锚点再用动画追赶。
      viewportAnchorRef.current.reset();
      setScrollToBottomVisibility(false);
      scheduleFollowLatest();
      return;
    }

    // READING 独占可见锚点；任何内容版本都只恢复阅读位置，绝不追底。
    cancelAnimation();
    const restoredScrollTop = viewportAnchorRef.current.restore(
      container,
      feedRef.current,
      { allowVirtualFeed: topologyChanged },
    );
    if (restoredScrollTop !== null) {
      lastScrollTopRef.current = restoredScrollTop;
    }
    setScrollToBottomVisibility(
      hasScrollableOverflow(container) && !isAtScrollBottom(container),
    );
  }, [
    atomicLayoutKey,
    cancelAnimation,
    contentKey,
    messageCount,
    scheduleFollowLatest,
    sessionKey,
    setScrollToBottomVisibility,
    topologyKey,
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
    viewportAnchorRef.current.capture(container, feedRef.current);
    setScrollToBottomVisibility(
      hasScrollableOverflow(container) && !isAtScrollBottom(container),
    );
  }, [historyPrependToken, setScrollToBottomVisibility]);

  // 只观察内容轨道增长；FOLLOW 同步贴底，READING 只恢复可见锚点。
  useEffect(() => {
    if (typeof ResizeObserver === "undefined") {
      return;
    }

    const observer = new ResizeObserver(() => {
      const currentContainer = scrollRef.current;
      if (!currentContainer) {
        return;
      }

      if (shouldFollowLatestRef.current) {
        viewportAnchorRef.current.reset();
        setScrollToBottomVisibility(false);
        scheduleFollowLatest();
        return;
      }

      const restoredScrollTop = viewportAnchorRef.current.restore(
        currentContainer,
        feedRef.current,
      );
      if (restoredScrollTop !== null) {
        lastScrollTopRef.current = restoredScrollTop;
      }
      setScrollToBottomVisibility(
        hasScrollableOverflow(currentContainer)
          && !isAtScrollBottom(currentContainer),
      );
    });
    const feed = feedRef.current;
    if (feed) {
      observer.observe(feed);
    }
    return () => observer.disconnect();
  }, [
    messageCount,
    scheduleFollowLatest,
    sessionKey,
    setScrollToBottomVisibility,
    topologyKey,
  ]);

  // 视口尺寸变化来自 Composer、虚拟键盘或 App/browser 窗口，不是模型正文。
  // FOLLOW 保留贴底意图，READING 保留可见锚点；尺寸变化不能替用户切换。
  useEffect(() => {
    if (typeof ResizeObserver === "undefined") {
      return;
    }
    const container = scrollRef.current;
    if (!container) {
      return;
    }
    const observer = new ResizeObserver(() => {
      retainPositionForViewportResize(container);
    });
    observer.observe(container);
    return () => observer.disconnect();
  }, [
    retainPositionForViewportResize,
    sessionKey,
  ]);

  useLayoutEffect(() => {
    shouldFollowLatestRef.current = true;
    historyAnchorRef.current.cancel();
    viewportAnchorRef.current.reset();
    const container = scrollRef.current;
    viewportSizeRef.current = container
      ? getConversationViewportSize(container)
      : null;
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
  const interactionOnScroll = interactions.onScroll;
  const onScroll = useCallback(() => {
    interactionOnScroll();
    const container = scrollRef.current;
    if (container && !shouldFollowLatestRef.current) {
      viewportAnchorRef.current.capture(container, feedRef.current);
    }
  }, [interactionOnScroll]);

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
    onScroll,
  };
}
