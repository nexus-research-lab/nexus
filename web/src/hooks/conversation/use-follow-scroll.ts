/**
 * useFollowScroll — 自动跟随底部的滚动管理 hook
 *
 * 封装聊天面板的滚动控制逻辑：
 * - 新消息 / loading 时自动滚到底部
 * - 用户上滚时暂停跟随
 * - 内容 resize 时保持位置
 * - 支持触摸手势取消自动跟随
 */

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";

const BOTTOM_THRESHOLD_PX = 80;
const SMOOTH_SCROLL_DURATION_MS = 420;
const EASE_X1 = 0.23;
const EASE_Y1 = 1;
const EASE_X2 = 0.32;
const EASE_Y2 = 1;

function sampleCubic(a: number, b: number, c: number, t: number): number {
  return ((a * t + b) * t + c) * t;
}

function sampleCubicDerivative(
  a: number,
  b: number,
  c: number,
  t: number,
): number {
  return (3 * a * t + 2 * b) * t + c;
}

function solveBezierProgress(progress: number): number {
  const clampedProgress = Math.min(Math.max(progress, 0), 1);
  const cx = 3 * EASE_X1;
  const bx = 3 * (EASE_X2 - EASE_X1) - cx;
  const ax = 1 - cx - bx;
  const cy = 3 * EASE_Y1;
  const by = 3 * (EASE_Y2 - EASE_Y1) - cy;
  const ay = 1 - cy - by;

  let t = clampedProgress;
  for (let iteration = 0; iteration < 5; iteration += 1) {
    const x = sampleCubic(ax, bx, cx, t) - clampedProgress;
    const derivative = sampleCubicDerivative(ax, bx, cx, t);
    if (Math.abs(derivative) < 1e-6) {
      break;
    }
    t -= x / derivative;
  }

  let lower = 0;
  let upper = 1;
  t = Math.min(Math.max(t, 0), 1);
  for (let iteration = 0; iteration < 8; iteration += 1) {
    const x = sampleCubic(ax, bx, cx, t);
    if (Math.abs(x - clampedProgress) < 1e-5) {
      break;
    }
    if (x > clampedProgress) {
      upper = t;
    } else {
      lower = t;
    }
    t = (lower + upper) / 2;
  }

  return sampleCubic(ay, by, cy, t);
}

interface UseFollowScrollOptions {
  /** 消息数量变化时触发滚动 */
  messageCount: number;
  /** 权限/插槽块变化时触发滚动 */
  auxiliaryBlockCount?: number;
  /** 辅助块内容变化时触发滚动，例如系统消息文本变化。 */
  auxiliaryBlockKey?: string | null;
  /** loading 变化时触发滚动 */
  isLoading: boolean;
  /** session 切换时重置跟随状态 */
  sessionKey: string | null;
  /** 历史消息 prepend 完成后，用于恢复滚动锚点 */
  historyPrependToken?: number;
}

interface UseFollowScrollReturn {
  /** 挂载到滚动容器的 ref */
  scrollRef: React.RefObject<HTMLDivElement | null>;
  /** 挂载到 feed 内容区的 ref（ResizeObserver 用） */
  feedRef: React.RefObject<HTMLDivElement | null>;
  /** 底部锚点 ref */
  bottomAnchorRef: React.RefObject<HTMLDivElement | null>;
  /** 是否显示"回到底部"按钮 */
  showScrollToBottom: boolean;
  /** 滚动到底部 */
  scrollToBottom: (behavior?: ScrollBehavior) => void;
  /** 用户主动跳转到非底部内容时，暂停自动跟随最新消息。 */
  pauseFollowLatest: () => void;
  /** 在 prepend 历史消息前记录当前滚动锚点 */
  prepareHistoryPrependRestore: () => void;
  /** prepend 被取消或失败时清理锚点 */
  cancelHistoryPrependRestore: () => void;
  /** 事件处理器：挂载到滚动容器的 onScroll */
  onScroll: () => void;
  /** 事件处理器：挂载到滚动容器的 onWheel */
  onWheel: (event: React.WheelEvent<HTMLDivElement>) => void;
  /** 事件处理器：挂载到滚动容器的 onTouchStart */
  onTouchStart: (event: React.TouchEvent<HTMLDivElement>) => void;
  /** 事件处理器：挂载到滚动容器的 onTouchMove */
  onTouchMove: (event: React.TouchEvent<HTMLDivElement>) => void;
  /** 事件处理器：挂载到滚动容器的 onTouchEnd */
  onTouchEnd: () => void;
}

export function useFollowScroll({
  messageCount,
  auxiliaryBlockCount = 0,
  auxiliaryBlockKey = null,
  isLoading,
  sessionKey,
  historyPrependToken = 0,
}: UseFollowScrollOptions): UseFollowScrollReturn {
  const scrollRef = useRef<HTMLDivElement>(null);
  const feedRef = useRef<HTMLDivElement>(null);
  const bottomAnchorRef = useRef<HTMLDivElement>(null);
  const shouldFollowLatestRef = useRef(true);
  const lastScrollTopRef = useRef(0);
  const pendingScrollFrameRef = useRef<number | null>(null);
  const pendingScrollInnerFrameRef = useRef<number | null>(null);
  const pendingPrependRestoreRef = useRef<{
    scrollHeight: number;
    scrollTop: number;
  } | null>(null);
  const touchStartYRef = useRef<number | null>(null);
  const showScrollToBottomRef = useRef(false);
  const [showScrollToBottom, setShowScrollToBottom] = useResettableState(false, sessionKey ?? "");

  // ==================== 跟随状态 ====================

  const setScrollToBottomVisibility = useCallback((visible: boolean) => {
    if (showScrollToBottomRef.current === visible) {
      return;
    }

    showScrollToBottomRef.current = visible;
    setShowScrollToBottom(visible);
  }, []);

  const updateFollowState = useCallback(() => {
    const container = scrollRef.current;
    if (!container) return;

    const distanceToBottom =
      container.scrollHeight - container.scrollTop - container.clientHeight;
    const isNearBottom = distanceToBottom <= BOTTOM_THRESHOLD_PX;
    shouldFollowLatestRef.current = isNearBottom;
    setScrollToBottomVisibility(!isNearBottom);
  }, [setScrollToBottomVisibility]);

  // ==================== 滚动调度 ====================

  const cancelPendingScroll = useCallback(() => {
    if (pendingScrollFrameRef.current !== null) {
      cancelAnimationFrame(pendingScrollFrameRef.current);
      pendingScrollFrameRef.current = null;
    }
    if (pendingScrollInnerFrameRef.current !== null) {
      cancelAnimationFrame(pendingScrollInnerFrameRef.current);
      pendingScrollInnerFrameRef.current = null;
    }
  }, []);

  const scheduleScrollToBottom = useCallback(
    (behavior: ScrollBehavior = "smooth") => {
      cancelPendingScroll();

      const container = scrollRef.current;
      if (!container) return;

      // 流式输出时直接贴到底部，避免等待两帧后再修正位置导致换行抖动
      if (behavior === "auto") {
        container.scrollTop = container.scrollHeight;
        lastScrollTopRef.current = container.scrollTop;
        return;
      }

      pendingScrollFrameRef.current = requestAnimationFrame(() => {
        const next = scrollRef.current;
        if (!next) return;
        const targetTop = next.scrollHeight;
        const startTop = next.scrollTop;
        const distance = targetTop - startTop;

        if (Math.abs(distance) < 1) {
          next.scrollTop = targetTop;
          lastScrollTopRef.current = next.scrollTop;
          return;
        }

        const startTime = performance.now();

        // 用固定时长动画替代浏览器默认 smooth，
        // 这样不同容器的滚动速度更一致，也更容易微调。
        const step = (now: number) => {
          const elapsed = now - startTime;
          const progress = Math.min(elapsed / SMOOTH_SCROLL_DURATION_MS, 1);
          const easedProgress = solveBezierProgress(progress);

          next.scrollTop = startTop + distance * easedProgress;
          lastScrollTopRef.current = next.scrollTop;

          if (progress < 1) {
            pendingScrollInnerFrameRef.current =
              requestAnimationFrame(step);
          } else {
            pendingScrollInnerFrameRef.current = null;
          }
        };

        pendingScrollInnerFrameRef.current = requestAnimationFrame(step);
      });
    },
    [cancelPendingScroll],
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
    cancelPendingScroll();
    shouldFollowLatestRef.current = false;
    setScrollToBottomVisibility(true);
  }, [cancelPendingScroll, setScrollToBottomVisibility]);

  const prepareHistoryPrependRestore = useCallback(() => {
    const container = scrollRef.current;
    if (!container) {
      return;
    }
    cancelPendingScroll();
    shouldFollowLatestRef.current = false;
    pendingPrependRestoreRef.current = {
      scrollHeight: container.scrollHeight,
      scrollTop: container.scrollTop,
    };
  }, [cancelPendingScroll]);

  const cancelHistoryPrependRestore = useCallback(() => {
    pendingPrependRestoreRef.current = null;
  }, []);

  // ==================== 副作用 ====================

  // 新消息 / loading 变化时自动滚动
  useLayoutEffect(() => {
    if (!shouldFollowLatestRef.current) {
      // 用户主动离开底部后，仅保持按钮可见，避免流式消息期间重复触发同步 setState。
      setScrollToBottomVisibility(true);
      return;
    }
    scheduleScrollToBottom(isLoading ? "auto" : "smooth");
  }, [
    auxiliaryBlockCount,
    auxiliaryBlockKey,
    isLoading,
    messageCount,
    scheduleScrollToBottom,
    setScrollToBottomVisibility,
  ]);

  useLayoutEffect(() => {
    const container = scrollRef.current;
    const snapshot = pendingPrependRestoreRef.current;
    if (!container || !snapshot) {
      return;
    }
    pendingPrependRestoreRef.current = null;
    const heightDelta = container.scrollHeight - snapshot.scrollHeight;
    const nextScrollTop = snapshot.scrollTop + heightDelta;
    container.scrollTop = nextScrollTop;
    lastScrollTopRef.current = nextScrollTop;
    setScrollToBottomVisibility(true);
  }, [historyPrependToken, setScrollToBottomVisibility]);

  // feed 内容高度变化时保持跟随
  useEffect(() => {
    const feed = feedRef.current;
    if (!feed || typeof ResizeObserver === "undefined") return;

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

  // session 切换时重置
  useEffect(() => {
    shouldFollowLatestRef.current = true;
    showScrollToBottomRef.current = false;
    lastScrollTopRef.current = scrollRef.current?.scrollTop || 0;
    pendingPrependRestoreRef.current = null;
  }, [sessionKey]);

  // 卸载时清理
  useEffect(() => {
    return () => cancelPendingScroll();
  }, [cancelPendingScroll]);

  // ==================== 事件处理器 ====================

  const onScroll = useCallback(() => {
    const container = scrollRef.current;
    if (!container) return;

    const currentScrollTop = container.scrollTop;
    const isScrollingUp = currentScrollTop < lastScrollTopRef.current;
    lastScrollTopRef.current = currentScrollTop;

    if (isScrollingUp) cancelPendingScroll();
    updateFollowState();
  }, [cancelPendingScroll, updateFollowState]);

  const onWheel = useCallback(
    (event: React.WheelEvent<HTMLDivElement>) => {
      if (event.deltaY < 0) cancelPendingScroll();
    },
    [cancelPendingScroll],
  );

  const onTouchStart = useCallback(
    (event: React.TouchEvent<HTMLDivElement>) => {
      touchStartYRef.current = event.touches[0]?.clientY ?? null;
    },
    [],
  );

  const onTouchMove = useCallback(
    (event: React.TouchEvent<HTMLDivElement>) => {
      const currentY = event.touches[0]?.clientY;
      if (currentY === undefined || touchStartYRef.current === null) return;
      if (currentY > touchStartYRef.current) cancelPendingScroll();
    },
    [cancelPendingScroll],
  );

  const onTouchEnd = useCallback(() => {
    touchStartYRef.current = null;
  }, []);

  return {
    scrollRef: scrollRef,
    feedRef: feedRef,
    bottomAnchorRef: bottomAnchorRef,
    showScrollToBottom: showScrollToBottom,
    scrollToBottom: scrollToBottom,
    pauseFollowLatest: pauseFollowLatest,
    prepareHistoryPrependRestore: prepareHistoryPrependRestore,
    cancelHistoryPrependRestore: cancelHistoryPrependRestore,
    onScroll: onScroll,
    onWheel: onWheel,
    onTouchStart: onTouchStart,
    onTouchMove: onTouchMove,
    onTouchEnd: onTouchEnd,
  };
}
