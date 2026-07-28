/**
 * INPUT: 会话内容版本、历史前插令牌与滚动容器尺寸变化。
 * OUTPUT: DM、Room、Thread 共用的跟随状态、定位入口与用户滚动处理器。
 * POS: 会话贴底、脱离跟随、历史恢复和资源清理的 React 编排层。
 */
import { useCallback, useEffect, useLayoutEffect, useRef } from "react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";

import {
  getConversationViewportSize,
  hasScrollableOverflow,
  isNearScrollBottom,
  resolveConversationViewportResizeState,
  resolveConversationViewportSizeRevision,
  shouldDetachFollowForAtomicGrowth,
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
  const lastScrollHeightRef = useRef(0);
  const revisionSessionKeyRef = useRef<string | null>(null);
  const previousAtomicLayoutKeyRef = useRef<string | null>(null);
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
    const shouldFollow = isNearScrollBottom(container);
    shouldFollowLatestRef.current = shouldFollow;
    setScrollToBottomVisibility(!shouldFollow);
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
    lastScrollHeightRef.current = container.scrollHeight;
    shouldFollowLatestRef.current = resizeState.shouldFollow;
    viewportAnchorRef.current.capture(container, feedRef.current);
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
      // 在 reduced-motion 的同步贴底分支之前也必须先解除追随。
      return;
    }
    animatorRef.current?.follow();
  }, [retainPositionForViewportResize]);

  const scrollToBottom = useCallback(
    (behavior: ScrollBehavior = "smooth") => {
      shouldFollowLatestRef.current = true;
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
    setScrollToBottomVisibility(true);
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
    const atomicLayoutChanged = (
      !isNewSession
      && previousAtomicLayoutKeyRef.current !== atomicLayoutKey
    );
    revisionSessionKeyRef.current = sessionKey;
    previousTopologyKeyRef.current = topologyKey;
    previousAtomicLayoutKeyRef.current = atomicLayoutKey;
    const shouldDetachForGrowth = Boolean(
      container
      && shouldFollowLatestRef.current
      && shouldDetachFollowForAtomicGrowth(
        container,
        lastScrollHeightRef.current,
      ),
    );
    let restoredScrollTop: number | null = null;
    if (container) {
      lastScrollHeightRef.current = container.scrollHeight;
      restoredScrollTop = viewportAnchorRef.current.restore(
        container,
        feedRef.current,
        { allowVirtualFeed: topologyChanged },
      );
      if (restoredScrollTop !== null) {
        lastScrollTopRef.current = restoredScrollTop;
      }
    }
    const shouldDetachForLayout = Boolean(
      container
      && shouldFollowLatestRef.current
      && (
        restoredScrollTop !== null
        || (atomicLayoutChanged && hasScrollableOverflow(container))
      ),
    );
    if (shouldDetachForGrowth || shouldDetachForLayout) {
      cancelAnimation();
      shouldFollowLatestRef.current = false;
      setScrollToBottomVisibility(
        Boolean(container && hasScrollableOverflow(container)),
      );
      return;
    }
    if (!shouldFollowLatestRef.current) {
      setScrollToBottomVisibility(
        Boolean(container && hasScrollableOverflow(container)),
      );
      return;
    }
    scheduleFollowLatest();
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
      hasScrollableOverflow(container) && !isNearScrollBottom(container),
    );
  }, [historyPrependToken, setScrollToBottomVisibility]);

  // 只观察内容轨道增长。Composer 撑高或 App 窗口缩放会改变 viewport，
  // 但不应借此重新追底并推走用户当前看到的内容。
  useEffect(() => {
    if (typeof ResizeObserver === "undefined") {
      return;
    }

    const observer = new ResizeObserver(() => {
      const currentContainer = scrollRef.current;
      if (currentContainer) {
        lastScrollHeightRef.current = currentContainer.scrollHeight;
        const restoredScrollTop = viewportAnchorRef.current.restore(
          currentContainer,
          feedRef.current,
        );
        if (restoredScrollTop !== null) {
          lastScrollTopRef.current = restoredScrollTop;
        }
      }
      if (!shouldFollowLatestRef.current) {
        setScrollToBottomVisibility(
          Boolean(
            currentContainer && hasScrollableOverflow(currentContainer),
          ),
        );
        return;
      }
      scheduleFollowLatest();
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
  // 即使底部弹簧正在运行也要先停止；下一条 token 不能借机重新拉走画面。
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
    lastScrollHeightRef.current = container?.scrollHeight ?? 0;
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
    if (container) {
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
