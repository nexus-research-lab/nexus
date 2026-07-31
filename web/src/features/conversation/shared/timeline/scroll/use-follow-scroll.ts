/**
 * INPUT: 会话内容版本、历史前插令牌与滚动容器尺寸变化。
 * OUTPUT: DM、Room、Thread 共用的跟随状态、上方增长锚定、定位入口与用户滚动处理器。
 * POS: FOLLOW 尾部贴底、Virtualizer 测高委托、READING 锚定和资源清理的 React 编排层。
 */
import { useCallback, useEffect, useLayoutEffect, useRef } from "react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";

import {
  getConversationViewportSize,
  hasScrollableOverflow,
  isAtScrollBottom,
  resolveConversationFollowCommitOwner,
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

  if (
    !animatorRef.current
    || typeof animatorRef.current.isActive !== "function"
  ) {
    // Vite HMR 会保留 Hook ref；执行器协议升级时先终止旧实例，不能让旧
    // prototype 留在 Room 内直到整页刷新。
    animatorRef.current?.cancel();
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
      // resize 所有者已按当前 FOLLOW/READING 意图完成同步写入。
      return;
    }
    animatorRef.current?.follow();
    const nextContainer = scrollRef.current;
    if (nextContainer) {
      viewportAnchorRef.current.capture(nextContainer, feedRef.current);
    }
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
      if (isNewSession) {
        viewportAnchorRef.current.reset();
      }
      const feed = feedRef.current;
      const restoredScrollTop = viewportAnchorRef.current.restore(
        container,
        feed,
        { allowVirtualFeed: topologyChanged },
      );
      if (restoredScrollTop !== null) {
        lastScrollTopRef.current = restoredScrollTop;
      }
      const owner = resolveConversationFollowCommitOwner({
        bottomScrollActive: animatorRef.current?.isActive?.() ?? false,
        isNewSession,
        isVirtualFeed: feed?.dataset.conversationVirtualFeed === "true",
        topologyChanged,
        viewportAnchorRestored: restoredScrollTop !== null,
      });
      setScrollToBottomVisibility(
        owner === "viewport-anchor"
          && hasScrollableOverflow(container)
          && !isAtScrollBottom(container),
      );
      if (owner === "bottom") {
        if (
          feed?.dataset.conversationVirtualFeed === "true"
          && (isNewSession || topologyChanged)
        ) {
          scheduleScrollToBottom("auto");
        } else {
          scheduleFollowLatest();
        }
      }
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
    scheduleScrollToBottom,
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

  // 只观察内容轨道增长。静态 Feed 用可见锚点区分“上方增长”和“尾部增长”；
  // 虚拟 Feed 的普通测高只由 Virtualizer 修正，不能再叠加共享 bottom 写入。
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
        const feed = feedRef.current;
        const restoredScrollTop = viewportAnchorRef.current.restore(
          currentContainer,
          feed,
        );
        if (restoredScrollTop !== null) {
          lastScrollTopRef.current = restoredScrollTop;
        }
        const owner = resolveConversationFollowCommitOwner({
          bottomScrollActive: animatorRef.current?.isActive?.() ?? false,
          isNewSession: false,
          isVirtualFeed: feed?.dataset.conversationVirtualFeed === "true",
          topologyChanged: false,
          viewportAnchorRestored: restoredScrollTop !== null,
        });
        if (owner === "bottom") {
          setScrollToBottomVisibility(false);
          scheduleFollowLatest();
          return;
        }
        const remainsAtBottom = isAtScrollBottom(currentContainer);
        if (owner === "virtualizer" && !remainsAtBottom) {
          // Virtualizer 没有把这次变化判为底部跟随时，保留当前画面并进入
          // READING；后续 token 也不能再由共享 FOLLOW 突然拉回底部。
          shouldFollowLatestRef.current = false;
        }
        setScrollToBottomVisibility(
          hasScrollableOverflow(currentContainer) && !remainsAtBottom,
        );
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
    if (container) {
      viewportAnchorRef.current.capture(container, feedRef.current);
    }
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
