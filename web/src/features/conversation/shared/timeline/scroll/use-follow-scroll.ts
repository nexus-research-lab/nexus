/**
 * INPUT: 会话内容版本、初始/实时内容锚点、历史前插令牌与滚动容器尺寸变化。
 * OUTPUT: DM、Room、Thread 共用的跟随状态、可顶部起始的 live 高度保护、阅读锚定与用户滚动处理器。
 * POS: FOLLOW 单一滚动所有权、Virtualizer 测高委托、live 高度负债、READING 锚定和资源清理的 React 编排层。
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
import {
  clearConversationRoundNavigationTarget,
  getConversationRoundNavigationTarget,
} from "./round-scroll";
import {
  useConversationLiveHeightGuard,
  type ConversationLiveContentAlignment,
} from "./use-conversation-live-height-guard";
import { useFollowScrollInteractions } from "./use-follow-scroll-interactions";

interface UseFollowScrollOptions {
  messageCount: number;
  atomicLayoutKey?: string | null;
  contentKey?: string | null;
  historyPrependToken?: number;
  initialScrollAnchor?: "bottom" | "top";
  liveContentAlignment?: ConversationLiveContentAlignment;
  liveLayoutActive?: boolean;
  sessionKey: string | null;
  topologyKey?: string | null;
}

interface UseFollowScrollReturn {
  scrollRef: React.RefObject<HTMLDivElement | null>;
  feedRef: React.RefObject<HTMLDivElement | null>;
  bottomAnchorRef: React.RefObject<HTMLDivElement | null>;
  isBottomScrollActive: () => boolean;
  isFollowingLatest: () => boolean;
  isUserScrollActive: () => boolean;
  liveLayoutActive: boolean;
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
  initialScrollAnchor = "bottom",
  liveContentAlignment = "end",
  liveLayoutActive = false,
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
  const bottomCommitPendingRef = useRef(false);
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

  bottomCommitPendingRef.current = (
    revisionSessionKeyRef.current !== sessionKey
    || (
      revisionSessionKeyRef.current === sessionKey
      && previousTopologyKeyRef.current !== topologyKey
    )
  );
  useConversationLiveHeightGuard({
    active: liveLayoutActive,
    contentAlignment: liveContentAlignment,
    feedRef,
    revision: `${messageCount}\u001f${topologyKey ?? ""}`,
    scopeKey: sessionKey,
  });

  const isBottomScrollActive = useCallback(
    () => (
      bottomCommitPendingRef.current
      || (animatorRef.current?.isActive?.() ?? false)
    ),
    [],
  );
  const isFollowingLatest = useCallback(
    () => shouldFollowLatestRef.current,
    [],
  );

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
    if (getConversationRoundNavigationTarget(container)) {
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
    if (container && getConversationRoundNavigationTarget(container)) {
      return;
    }
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
      const container = scrollRef.current;
      if (container) {
        clearConversationRoundNavigationTarget(container);
      }
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

  const interactions = useFollowScrollInteractions({
    lastScrollTopRef,
    pauseFollowLatest,
    scrollRef,
    updateFollowState,
  });
  const isUserScrollActive = interactions.isUserScrollActive;

  useLayoutEffect(() => {
    const container = scrollRef.current;
    const isNewSession = revisionSessionKeyRef.current !== sessionKey;
    const topologyChanged = (
      !isNewSession
      && previousTopologyKeyRef.current !== topologyKey
    );
    revisionSessionKeyRef.current = sessionKey;
    previousTopologyKeyRef.current = topologyKey;
    bottomCommitPendingRef.current = false;
    if (!container) {
      return;
    }

    if (getConversationRoundNavigationTarget(container)) {
      cancelAnimation();
      setScrollToBottomVisibility(
        hasScrollableOverflow(container) && !isAtScrollBottom(container),
      );
      return;
    }

    if (isNewSession && initialScrollAnchor === "top") {
      cancelAnimation();
      viewportAnchorRef.current.reset();
      container.scrollTop = 0;
      lastScrollTopRef.current = 0;
      shouldFollowLatestRef.current = true;
      setScrollToBottomVisibility(false);
      viewportAnchorRef.current.capture(container, feedRef.current);
      return;
    }

    if (shouldFollowLatestRef.current) {
      if (isNewSession) {
        viewportAnchorRef.current.reset();
      }
      const feed = feedRef.current;
      const owner = resolveConversationFollowCommitOwner({
        bottomScrollActive: animatorRef.current?.isActive?.() ?? false,
        isNewSession,
        isVirtualFeed: feed?.dataset.conversationVirtualFeed === "true",
        topologyChanged,
      });
      setScrollToBottomVisibility(false);
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
      {
        allowVirtualFeed: topologyChanged,
        userScrollActive: isUserScrollActive(),
      },
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
    initialScrollAnchor,
    isUserScrollActive,
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

  // 只观察整条内容轨道的聚合尺寸。ResizeObserver 会把同一布局周期里多个
  // Agent 的子树变化合成一次父轨道通知：FOLLOW 只贴真实 bottom，READING
  // 只恢复可见轮次；虚拟 Feed 的普通逐项测高仍由 Virtualizer 独占。
  useEffect(() => {
    if (typeof ResizeObserver === "undefined") {
      return;
    }

    const observer = new ResizeObserver(() => {
      const currentContainer = scrollRef.current;
      if (!currentContainer) {
        return;
      }
      if (getConversationRoundNavigationTarget(currentContainer)) {
        return;
      }

      if (shouldFollowLatestRef.current) {
        const feed = feedRef.current;
        const owner = resolveConversationFollowCommitOwner({
          bottomScrollActive: animatorRef.current?.isActive?.() ?? false,
          isNewSession: false,
          isVirtualFeed: feed?.dataset.conversationVirtualFeed === "true",
          topologyChanged: false,
        });
        setScrollToBottomVisibility(false);
        if (owner === "bottom") {
          scheduleFollowLatest();
          return;
        }
        // 测量过程中暂时离开数值 bottom 不能替用户切换到 READING；
        // FOLLOW/READING 只由明确的用户滚动意图改变。
        return;
      }

      const restoredScrollTop = viewportAnchorRef.current.restore(
        currentContainer,
        feedRef.current,
        { userScrollActive: isUserScrollActive() },
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
    isUserScrollActive,
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
      if (getConversationRoundNavigationTarget(container)) {
        viewportSizeRef.current = getConversationViewportSize(container);
        return;
      }
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
    if (container) {
      clearConversationRoundNavigationTarget(container);
    }
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
    isBottomScrollActive,
    isFollowingLatest,
    liveLayoutActive,
    showScrollToBottom,
    scrollToBottom,
    pauseFollowLatest,
    prepareHistoryPrependRestore,
    cancelHistoryPrependRestore,
    ...interactions,
    onScroll,
  };
}
